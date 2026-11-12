package provider

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	contract "omsbizfoundry/invoice-recognition-contract"
	"omsbizfoundry/invoice-recognition/internal/recognition"
)

// LocalImage runs a bounded, offline OCR process. It deliberately returns a
// review proposal rather than a business fact; no OCR result can post an
// invoice or reimbursement by itself.
type LocalImage struct {
	tesseractPath string
	pdfRasterizer string
	timeout       time.Duration
}

func NewLocalImage(tesseractCommand, pdfRasterizerCommand string, timeout time.Duration) (*LocalImage, error) {
	if timeout <= 0 || timeout > 2*time.Minute {
		return nil, fmt.Errorf("local invoice OCR timeout is invalid")
	}
	tesseractPath, err := exec.LookPath(strings.TrimSpace(tesseractCommand))
	if err != nil {
		return nil, fmt.Errorf("local invoice OCR command is unavailable: %w", err)
	}
	pdfRasterizer := ""
	if strings.TrimSpace(pdfRasterizerCommand) != "" {
		pdfRasterizer, err = exec.LookPath(strings.TrimSpace(pdfRasterizerCommand))
		if err != nil {
			return nil, fmt.Errorf("invoice PDF rasterizer is unavailable: %w", err)
		}
	}
	return &LocalImage{tesseractPath: tesseractPath, pdfRasterizer: pdfRasterizer, timeout: timeout}, nil
}

func (p *LocalImage) Recognize(ctx context.Context, document recognition.Document) (recognition.ProviderResult, error) {
	if document.Evidence.MIMEType != "image/jpeg" && document.Evidence.MIMEType != "image/png" && document.Evidence.MIMEType != "application/pdf" {
		return recognition.ProviderResult{}, fmt.Errorf("local OCR only supports JPEG, PNG and PDF")
	}
	dir, err := os.MkdirTemp("", "invoice-ocr-*")
	if err != nil {
		return recognition.ProviderResult{}, err
	}
	defer os.RemoveAll(dir)
	extension := ".png"
	if document.Evidence.MIMEType == "image/jpeg" {
		extension = ".jpg"
	} else if document.Evidence.MIMEType == "application/pdf" {
		extension = ".pdf"
	}
	input := filepath.Join(dir, "invoice"+extension)
	if err = os.WriteFile(input, document.Bytes, 0o600); err != nil {
		return recognition.ProviderResult{}, err
	}
	ocrInput := input
	ocrCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	if document.Evidence.MIMEType == "application/pdf" {
		if p.pdfRasterizer == "" {
			return recognition.ProviderResult{}, fmt.Errorf("local PDF rasterizer is not configured")
		}
		prefix := filepath.Join(dir, "page")
		command := exec.CommandContext(ocrCtx, p.pdfRasterizer, "-f", "1", "-singlefile", "-png", "-r", "220", input, prefix)
		if output, runErr := command.CombinedOutput(); runErr != nil {
			return recognition.ProviderResult{}, fmt.Errorf("rasterize first invoice page: %v (%s)", runErr, shortOutput(output))
		}
		ocrInput = prefix + ".png"
	}
	// Sparse electronic invoices are tables rather than one uniform text block.
	// PSM 4 preserves rows and the buyer/seller columns substantially better
	// than PSM 6 while still returning word-level coordinates for annotation.
	command := exec.CommandContext(ocrCtx, p.tesseractPath, ocrInput, "stdout", "-l", "chi_sim+eng", "--psm", "4", "tsv")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err = command.Run(); err != nil {
		return recognition.ProviderResult{}, fmt.Errorf("run local invoice OCR: %v (%s)", err, shortOutput(stderr.Bytes()))
	}
	result, err := invoiceFromTSV(stdout.String())
	if err != nil {
		return recognition.ProviderResult{}, err
	}
	if result.Candidate.IssuedOn == "" {
		if date, line, found := p.recognizeHeaderDate(ocrCtx, ocrInput); found {
			result.Candidate.IssuedOn = date
			setRegion(&result, "issued_on", line, date)
			result.Candidate.FieldConfidenceBPS["issued_on"] = line.confidence
			result.Warnings = removeWarning(result.Warnings, "开票日期未可靠识别，请对照原图补充")
		}
	}
	return result, nil
}

// recognizeHeaderDate retries only the standard invoice header area at a
// larger raster size. Whole-page OCR can locate the row while still dropping
// one narrow month/day digit; the bounded second pass avoids guessing from
// unrelated numbers and still requires a complete, valid Gregorian date.
func (p *LocalImage) recognizeHeaderDate(ctx context.Context, sourcePath string) (string, ocrLine, bool) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return "", ocrLine{}, false
	}
	configuration, _, err := image.DecodeConfig(file)
	if err != nil || configuration.Width <= 0 || configuration.Height <= 0 || int64(configuration.Width)*int64(configuration.Height) > 50_000_000 {
		_ = file.Close()
		return "", ocrLine{}, false
	}
	if _, err = file.Seek(0, 0); err != nil {
		_ = file.Close()
		return "", ocrLine{}, false
	}
	source, _, err := image.Decode(file)
	_ = file.Close()
	if err != nil {
		return "", ocrLine{}, false
	}
	bounds := source.Bounds()
	if bounds.Dx() < 200 || bounds.Dy() < 120 {
		return "", ocrLine{}, false
	}
	crop := image.Rect(
		bounds.Min.X+bounds.Dx()*55/100,
		bounds.Min.Y,
		bounds.Max.X,
		bounds.Min.Y+bounds.Dy()*25/100,
	).Intersect(bounds)
	if crop.Empty() {
		return "", ocrLine{}, false
	}
	scale := 3
	for scale > 1 && (crop.Dx()*scale > 4096 || crop.Dy()*scale > 2048) {
		scale--
	}
	scaled := scaleNearest(source, crop, scale)
	datePath := filepath.Join(filepath.Dir(sourcePath), "invoice-header-date.png")
	output, err := os.OpenFile(datePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", ocrLine{}, false
	}
	err = png.Encode(output, scaled)
	closeErr := output.Close()
	if err != nil || closeErr != nil {
		return "", ocrLine{}, false
	}
	command := exec.CommandContext(ctx, p.tesseractPath, datePath, "stdout", "-l", "chi_sim+eng", "--psm", "4", "tsv")
	var stdout bytes.Buffer
	command.Stdout = &stdout
	if command.Run() != nil {
		return "", ocrLine{}, false
	}
	lines, err := parseTSV(stdout.String())
	if err != nil {
		return "", ocrLine{}, false
	}
	value, line, found := labeledMatch(lines, []string{"开票日期", "出票日期", "日期"}, dateRE)
	if !found {
		return "", ocrLine{}, false
	}
	value = normalizeOCRDate(value)
	if value == "" {
		return "", ocrLine{}, false
	}
	line.pageWidth = bounds.Dx()
	line.pageHeight = bounds.Dy()
	line.left = crop.Min.X - bounds.Min.X + line.left/scale
	line.top = crop.Min.Y - bounds.Min.Y + line.top/scale
	line.right = crop.Min.X - bounds.Min.X + (line.right+scale-1)/scale
	line.bot = crop.Min.Y - bounds.Min.Y + (line.bot+scale-1)/scale
	return value, line, true
}

func scaleNearest(source image.Image, crop image.Rectangle, scale int) *image.RGBA {
	target := image.NewRGBA(image.Rect(0, 0, crop.Dx()*scale, crop.Dy()*scale))
	for y := 0; y < target.Bounds().Dy(); y++ {
		for x := 0; x < target.Bounds().Dx(); x++ {
			target.Set(x, y, source.At(crop.Min.X+x/scale, crop.Min.Y+y/scale))
		}
	}
	return target
}

func removeWarning(values []string, target string) []string {
	filtered := values[:0]
	for _, value := range values {
		if value != target {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func shortOutput(value []byte) string {
	text := strings.Join(strings.Fields(string(value)), " ")
	if len(text) > 240 {
		text = text[:240]
	}
	return text
}

type ocrWord struct {
	page, block, paragraph, line int
	left, top, width, height     int
	confidence                   int64
	text                         string
}

type ocrLine struct {
	pageWidth, pageHeight int
	words                 []ocrWord
	text                  string
	left, top, right, bot int
	confidence            int64
}

var (
	invoiceCodeRE   = regexp.MustCompile(`[0-9]{10,12}`)
	invoiceNumberRE = regexp.MustCompile(`[0-9]{20}|[0-9]{8}`)
	taxIDRE         = regexp.MustCompile(`[0-9A-Z]{15,20}`)
	dateRE          = regexp.MustCompile(`20[0-9]{2}(?:年|[-./])?[01]?[0-9](?:月|[-./])?[0-3]?[0-9]日?`)
	amountRE        = regexp.MustCompile(`(?:[¥￥])?[0-9][0-9,]*\.[0-9]{2}`)
)

func invoiceFromTSV(raw string) (recognition.ProviderResult, error) {
	lines, err := parseTSV(raw)
	if err != nil {
		return recognition.ProviderResult{}, err
	}
	result := recognition.ProviderResult{
		ProviderKey: "local-tesseract", ProviderModel: "chi_sim+eng", ProviderRevision: "tsv-v2",
		Candidate:    contract.Candidate{Currency: "CNY", FieldConfidenceBPS: map[string]int64{"currency": 10000}},
		FieldRegions: map[string][]contract.FieldRegion{},
	}
	// Standard electronic invoices place buyer and seller on the same OCR row.
	// Add left/right virtual rows so visual provenance and field extraction stay
	// scoped to the relevant party instead of highlighting the entire invoice.
	spatialLines := append(append([]ocrLine{}, lines...), splitLineHalves(lines)...)
	setText := func(field string, labels []string, pattern *regexp.Regexp, target *string, transform func(string) string) {
		value, line, ok := labeledMatch(lines, labels, pattern)
		if !ok {
			return
		}
		if transform != nil {
			value = transform(value)
		}
		if value == "" {
			return
		}
		*target = value
		setRegion(&result, field, line, value)
	}
	setText("invoice_code", []string{"发票代码", "票据代码"}, invoiceCodeRE, &result.Candidate.InvoiceCode, nil)
	setText("invoice_number", []string{"发票号码", "发票号", "票据号码"}, invoiceNumberRE, &result.Candidate.InvoiceNumber, nil)
	setText("issued_on", []string{"开票日期", "出票日期", "日期"}, dateRE, &result.Candidate.IssuedOn, normalizeOCRDate)

	nameLines := labeledNameLines(spatialLines)
	if len(nameLines) > 0 {
		if value := nameAfterLabel(nameLines[0].text); value != "" {
			result.Candidate.BuyerName = value
			setRegion(&result, "buyer_name", nameLines[0], value)
		}
	}
	if len(nameLines) > 1 {
		last := nameLines[len(nameLines)-1]
		if value := nameAfterLabel(last.text); value != "" {
			result.Candidate.SellerName = value
			setRegion(&result, "seller_name", last, value)
		}
	}
	if result.Candidate.BuyerName == "" || result.Candidate.SellerName == "" {
		buyer, seller := fallbackPartyNames(spatialLines)
		if result.Candidate.BuyerName == "" && buyer.text != "" {
			result.Candidate.BuyerName = partyNameAfterColon(buyer.text)
			setRegion(&result, "buyer_name", buyer, result.Candidate.BuyerName)
		}
		if result.Candidate.SellerName == "" && seller.text != "" {
			result.Candidate.SellerName = partyNameAfterColon(seller.text)
			setRegion(&result, "seller_name", seller, result.Candidate.SellerName)
		}
	}
	taxLines := labeledLines(spatialLines, []string{"纳税人识别号", "统一社会信用代码", "税号"}, taxIDRE)
	if len(taxLines) > 0 {
		result.Candidate.BuyerTaxID = taxIDRE.FindString(compactOCR(taxLines[0].text))
		setRegion(&result, "buyer_tax_id", taxLines[0], result.Candidate.BuyerTaxID)
	}
	if len(taxLines) > 1 {
		last := taxLines[len(taxLines)-1]
		result.Candidate.SellerTaxID = taxIDRE.FindString(compactOCR(last.text))
		setRegion(&result, "seller_tax_id", last, result.Candidate.SellerTaxID)
	}
	if result.Candidate.BuyerTaxID == "" || result.Candidate.SellerTaxID == "" {
		buyer, seller := fallbackPartyTaxIDs(spatialLines)
		if result.Candidate.BuyerTaxID == "" && buyer.text != "" {
			result.Candidate.BuyerTaxID = taxIDRE.FindString(compactOCR(buyer.text))
			setRegion(&result, "buyer_tax_id", buyer, result.Candidate.BuyerTaxID)
		}
		if result.Candidate.SellerTaxID == "" && seller.text != "" {
			result.Candidate.SellerTaxID = taxIDRE.FindString(compactOCR(seller.text))
			setRegion(&result, "seller_tax_id", seller, result.Candidate.SellerTaxID)
		}
	}

	setAmount := func(field string, labels []string, target **int64) {
		value, line, ok := labeledMatch(lines, labels, amountRE)
		if !ok {
			return
		}
		minor, parseErr := decimalMinor(value)
		if parseErr != nil {
			result.Warnings = append(result.Warnings, localOCRFieldLabel(field)+"无法按人民币金额解析")
			return
		}
		*target = &minor
		setRegion(&result, field, line, value)
	}
	setAmount("total_amount_minor", []string{"价税合计", "小写"}, &result.Candidate.TotalAmountMinor)
	setAmount("amount_excluding_tax_minor", []string{"合计金额", "不含税金额"}, &result.Candidate.AmountExcludingTaxMinor)
	setAmount("tax_amount_minor", []string{"合计税额", "税额合计"}, &result.Candidate.TaxAmountMinor)
	if result.Candidate.TaxAmountMinor == nil {
		if value, line, ok := fallbackTaxAmount(spatialLines, result.Candidate.TotalAmountMinor); ok {
			if minor, parseErr := decimalMinor(value); parseErr == nil {
				result.Candidate.TaxAmountMinor = &minor
				setRegion(&result, "tax_amount_minor", line, value)
			}
		}
	}
	if result.Candidate.AmountExcludingTaxMinor == nil && result.Candidate.TotalAmountMinor != nil && result.Candidate.TaxAmountMinor != nil && *result.Candidate.TotalAmountMinor >= *result.Candidate.TaxAmountMinor {
		minor := *result.Candidate.TotalAmountMinor - *result.Candidate.TaxAmountMinor
		result.Candidate.AmountExcludingTaxMinor = &minor
		if value, line, ok := fallbackAmountExcludingTax(spatialLines, minor); ok {
			setRegion(&result, "amount_excluding_tax_minor", line, value)
		} else if regions := result.FieldRegions["total_amount_minor"]; len(regions) != 0 {
			result.FieldRegions["amount_excluding_tax_minor"] = regions
		}
	}

	if result.Candidate.InvoiceNumber == "" {
		if value, line, ok := fallbackTopMatch(lines, invoiceNumberRE); ok {
			result.Candidate.InvoiceNumber = value
			setRegion(&result, "invoice_number", line, value)
		}
	}
	if result.Candidate.IssuedOn == "" {
		result.Warnings = append(result.Warnings, "开票日期未可靠识别，请对照原图补充")
	}
	if result.Candidate.InvoiceNumber == "" {
		result.Warnings = append(result.Warnings, "图片中未可靠识别到发票号码，请人工补充")
	}
	if len(result.FieldRegions) == 0 {
		return recognition.ProviderResult{}, fmt.Errorf("local OCR did not find recognizable invoice fields")
	}
	for field, regions := range result.FieldRegions {
		if len(regions) != 0 {
			result.Candidate.FieldConfidenceBPS[field] = regions[0].ConfidenceBPS
			if regions[0].ConfidenceBPS < 6500 {
				result.Warnings = append(result.Warnings, localOCRFieldLabel(field)+"识别置信度较低，请重点核对")
			}
		}
	}
	if result.Candidate.AmountExcludingTaxMinor != nil && result.Candidate.TaxAmountMinor != nil && result.Candidate.TotalAmountMinor != nil &&
		*result.Candidate.AmountExcludingTaxMinor+*result.Candidate.TaxAmountMinor != *result.Candidate.TotalAmountMinor {
		result.Warnings = append(result.Warnings, "识别金额未满足不含税金额＋税额＝价税合计，请人工核对")
		result.Candidate.AmountExcludingTaxMinor = nil
		result.Candidate.TaxAmountMinor = nil
		delete(result.FieldRegions, "amount_excluding_tax_minor")
		delete(result.FieldRegions, "tax_amount_minor")
		delete(result.Candidate.FieldConfidenceBPS, "amount_excluding_tax_minor")
		delete(result.Candidate.FieldConfidenceBPS, "tax_amount_minor")
	}
	return result, nil
}

func localOCRFieldLabel(field string) string {
	labels := map[string]string{
		"invoice_code":               "发票代码",
		"invoice_number":             "发票号码",
		"issued_on":                  "开票日期",
		"seller_name":                "销售方",
		"seller_tax_id":              "销售方税号",
		"buyer_name":                 "购买方",
		"buyer_tax_id":               "购买方税号",
		"amount_excluding_tax_minor": "不含税金额",
		"tax_amount_minor":           "税额",
		"total_amount_minor":         "价税合计",
	}
	if label := labels[field]; label != "" {
		return label
	}
	return "识别字段"
}

func parseTSV(raw string) ([]ocrLine, error) {
	rows := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	pageWidth, pageHeight := 0, 0
	groups := map[string][]ocrWord{}
	keys := []string{}
	for index, row := range rows {
		if index == 0 || strings.TrimSpace(row) == "" {
			continue
		}
		parts := strings.SplitN(row, "\t", 12)
		if len(parts) != 12 {
			continue
		}
		values := make([]int, 10)
		valid := true
		for i := 0; i < 10; i++ {
			values[i], _ = strconv.Atoi(parts[i])
			if i == 0 && values[i] < 1 {
				valid = false
			}
		}
		if !valid {
			continue
		}
		if values[0] == 1 && values[8] > 0 && values[9] > 0 {
			pageWidth, pageHeight = values[8], values[9]
		}
		if values[0] != 5 || strings.TrimSpace(parts[11]) == "" {
			continue
		}
		confidence, _ := strconv.ParseFloat(parts[10], 64)
		word := ocrWord{page: values[1], block: values[2], paragraph: values[3], line: values[4], left: values[6], top: values[7], width: values[8], height: values[9], confidence: int64(math.Round(math.Max(0, confidence) * 100)), text: strings.TrimSpace(parts[11])}
		key := fmt.Sprintf("%d:%d:%d:%d", word.page, word.block, word.paragraph, word.line)
		if _, exists := groups[key]; !exists {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], word)
	}
	if pageWidth <= 0 || pageHeight <= 0 || len(groups) == 0 {
		return nil, fmt.Errorf("local OCR returned no positioned text")
	}
	lines := make([]ocrLine, 0, len(groups))
	for _, key := range keys {
		words := groups[key]
		sort.SliceStable(words, func(i, j int) bool { return words[i].left < words[j].left })
		line := ocrLine{pageWidth: pageWidth, pageHeight: pageHeight, words: words, left: words[0].left, top: words[0].top, right: words[0].left + words[0].width, bot: words[0].top + words[0].height}
		var text []string
		var confidence int64
		for _, word := range words {
			text = append(text, word.text)
			confidence += word.confidence
			line.left = min(line.left, word.left)
			line.top = min(line.top, word.top)
			line.right = max(line.right, word.left+word.width)
			line.bot = max(line.bot, word.top+word.height)
		}
		line.text = strings.Join(text, " ")
		line.confidence = confidence / int64(len(words))
		lines = append(lines, line)
	}
	sort.SliceStable(lines, func(i, j int) bool {
		return lines[i].top < lines[j].top || (lines[i].top == lines[j].top && lines[i].left < lines[j].left)
	})
	return lines, nil
}

func splitLineHalves(lines []ocrLine) []ocrLine {
	result := []ocrLine{}
	for _, line := range lines {
		mid := line.pageWidth / 2
		leftWords, rightWords := []ocrWord{}, []ocrWord{}
		for _, word := range line.words {
			if word.left+word.width/2 < mid {
				leftWords = append(leftWords, word)
			} else {
				rightWords = append(rightWords, word)
			}
		}
		if len(leftWords) != 0 && len(rightWords) != 0 {
			result = append(result, lineFromWords(line.pageWidth, line.pageHeight, leftWords), lineFromWords(line.pageWidth, line.pageHeight, rightWords))
		}
	}
	return result
}

func lineFromWords(pageWidth, pageHeight int, words []ocrWord) ocrLine {
	line := ocrLine{pageWidth: pageWidth, pageHeight: pageHeight, words: words, left: words[0].left, top: words[0].top, right: words[0].left + words[0].width, bot: words[0].top + words[0].height}
	text := make([]string, 0, len(words))
	var confidence int64
	for _, word := range words {
		text = append(text, word.text)
		confidence += word.confidence
		line.left = min(line.left, word.left)
		line.top = min(line.top, word.top)
		line.right = max(line.right, word.left+word.width)
		line.bot = max(line.bot, word.top+word.height)
	}
	line.text = strings.Join(text, " ")
	line.confidence = confidence / int64(len(words))
	return line
}

func fallbackPartyNames(lines []ocrLine) (ocrLine, ocrLine) {
	var buyer, seller ocrLine
	for _, line := range lines {
		if line.top < line.pageHeight*15/100 || line.top > line.pageHeight*55/100 || line.right-line.left > line.pageWidth*60/100 || !strings.ContainsAny(line.text, ":：") {
			continue
		}
		value := partyNameAfterColon(line.text)
		if value == "" || taxIDRE.MatchString(compactOCR(line.text)) || amountRE.MatchString(compactOCR(line.text)) {
			continue
		}
		if line.left < line.pageWidth/2 && buyer.text == "" {
			buyer = line
		} else if line.left >= line.pageWidth/2 && seller.text == "" {
			seller = line
		}
	}
	return buyer, seller
}

func partyNameAfterColon(value string) string {
	if index := strings.IndexAny(value, ":："); index >= 0 {
		value = value[index+1:]
	}
	value = strings.Map(func(char rune) rune {
		if unicode.IsSpace(char) || strings.ContainsRune("|;；,，'\"`′“”", char) {
			return -1
		}
		return char
	}, value)
	value = strings.TrimSpace(value)
	value = strings.TrimFunc(value, func(char rune) bool { return !unicode.IsLetter(char) && !unicode.IsNumber(char) })
	if len([]rune(value)) < 2 || len([]rune(value)) > 80 {
		return ""
	}
	return value
}

func fallbackPartyTaxIDs(lines []ocrLine) (ocrLine, ocrLine) {
	var buyer, seller ocrLine
	for _, line := range lines {
		if line.top < line.pageHeight*15/100 || line.top > line.pageHeight*60/100 || !taxIDRE.MatchString(compactOCR(line.text)) {
			continue
		}
		if line.left < line.pageWidth/2 && buyer.text == "" {
			buyer = line
		} else if line.left >= line.pageWidth/2 && seller.text == "" {
			seller = line
		}
	}
	return buyer, seller
}

func fallbackTaxAmount(lines []ocrLine, total *int64) (string, ocrLine, bool) {
	// The tax total occupies the right half immediately above the grand total
	// on standard electronic invoices. This fallback is only used when the OCR
	// label itself is damaged; the value still comes from positioned OCR text.
	var value string
	var selected ocrLine
	for _, line := range lines {
		if line.left < line.pageWidth/2 || line.top < line.pageHeight*50/100 || line.top > line.pageHeight*85/100 {
			continue
		}
		matches := amountRE.FindAllString(compactOCR(line.text), -1)
		if len(matches) == 0 {
			continue
		}
		candidate := matches[len(matches)-1]
		minor, err := decimalMinor(candidate)
		if err != nil || minor <= 0 || (total != nil && minor >= *total) || strings.Contains(compactOCR(line.text), "价税合计") {
			continue
		}
		if selected.text == "" || line.top > selected.top {
			value, selected = candidate, line
		}
	}
	return value, selected, selected.text != ""
}

func fallbackAmountExcludingTax(lines []ocrLine, expectedMinor int64) (string, ocrLine, bool) {
	var value string
	var selected ocrLine
	for _, line := range lines {
		if line.top < line.pageHeight*40/100 || line.top > line.pageHeight*80/100 {
			continue
		}
		for _, candidate := range amountRE.FindAllString(compactOCR(line.text), -1) {
			minor, err := decimalMinor(candidate)
			if err == nil && minor == expectedMinor {
				return candidate, line, true
			}
		}
	}
	return value, selected, false
}

func compactOCR(value string) string {
	return strings.Map(func(char rune) rune {
		if unicode.IsSpace(char) || strings.ContainsRune(":：()（）", char) {
			return -1
		}
		return unicode.ToUpper(char)
	}, value)
}

func labeledMatch(lines []ocrLine, labels []string, pattern *regexp.Regexp) (string, ocrLine, bool) {
	for _, line := range lines {
		compact := compactOCR(line.text)
		matched := false
		for _, label := range labels {
			if strings.Contains(compact, compactOCR(label)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if value := pattern.FindString(compact); value != "" {
			return value, line, true
		}
	}
	return "", ocrLine{}, false
}

func labeledLines(lines []ocrLine, labels []string, pattern *regexp.Regexp) []ocrLine {
	result := []ocrLine{}
	for _, line := range lines {
		compact := compactOCR(line.text)
		for _, label := range labels {
			if strings.Contains(compact, compactOCR(label)) && (pattern == nil || pattern.MatchString(compact)) {
				result = append(result, line)
				break
			}
		}
	}
	return result
}

func labeledNameLines(lines []ocrLine) []ocrLine {
	result := []ocrLine{}
	for _, line := range lines {
		colonIndex := strings.IndexAny(line.text, ":：")
		if colonIndex < 0 || !strings.Contains(compactOCR(line.text[:colonIndex]), "名称") {
			continue
		}
		result = append(result, line)
	}
	return result
}

func fallbackTopMatch(lines []ocrLine, pattern *regexp.Regexp) (string, ocrLine, bool) {
	for _, line := range lines {
		if line.top > line.pageHeight*45/100 {
			continue
		}
		if value := pattern.FindString(compactOCR(line.text)); value != "" {
			return value, line, true
		}
	}
	return "", ocrLine{}, false
}

func nameAfterLabel(value string) string {
	compact := strings.TrimSpace(value)
	if index := strings.LastIndex(compact, "名称"); index >= 0 {
		compact = compact[index+len("名称"):]
	}
	compact = strings.Trim(compact, " :：")
	if len([]rune(compact)) < 2 || len([]rune(compact)) > 160 {
		return ""
	}
	return compact
}

func normalizeOCRDate(value string) string {
	digits := strings.Map(func(char rune) rune {
		if char >= '0' && char <= '9' {
			return char
		}
		return -1
	}, value)
	if len(digits) != 8 {
		return ""
	}
	parsed, err := time.Parse("20060102", digits)
	if err != nil {
		return ""
	}
	return parsed.Format("2006-01-02")
}

func setRegion(result *recognition.ProviderResult, field string, line ocrLine, text string) {
	if text == "" || line.pageWidth <= 0 || line.pageHeight <= 0 {
		return
	}
	toBPS := func(value, total int) int64 { return int64(math.Round(float64(value) * 10000 / float64(total))) }
	region := contract.FieldRegion{Page: 1, LeftBPS: toBPS(line.left, line.pageWidth), TopBPS: toBPS(line.top, line.pageHeight), WidthBPS: toBPS(line.right-line.left, line.pageWidth), HeightBPS: toBPS(line.bot-line.top, line.pageHeight), Text: text, ConfidenceBPS: line.confidence}
	if region.LeftBPS+region.WidthBPS > 10000 {
		region.WidthBPS = 10000 - region.LeftBPS
	}
	if region.TopBPS+region.HeightBPS > 10000 {
		region.HeightBPS = 10000 - region.TopBPS
	}
	if region.WidthBPS > 0 && region.HeightBPS > 0 {
		result.FieldRegions[field] = []contract.FieldRegion{region}
	}
}

var _ recognition.Provider = (*LocalImage)(nil)
