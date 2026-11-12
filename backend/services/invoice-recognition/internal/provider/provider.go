package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	contract "omsbizfoundry/invoice-recognition-contract"
	"omsbizfoundry/invoice-recognition/internal/recognition"
)

type Router struct {
	ai, local recognition.Provider
}

func NewRouter(ai, local recognition.Provider) *Router { return &Router{ai: ai, local: local} }
func (r *Router) Recognize(ctx context.Context, document recognition.Document) (recognition.ProviderResult, error) {
	switch strings.ToLower(document.Evidence.MIMEType) {
	case "application/xml", "text/xml":
		return XML{}.Recognize(ctx, document)
	case "image/jpeg", "image/png", "application/pdf":
		if r.ai != nil {
			return r.ai.Recognize(ctx, document)
		}
		if r.local != nil {
			return r.local.Recognize(ctx, document)
		}
		return recognition.ProviderResult{}, fmt.Errorf("image/PDF OCR provider is not configured")
	default:
		if r.ai == nil {
			return recognition.ProviderResult{}, fmt.Errorf("OFD OCR provider is not configured")
		}
		return r.ai.Recognize(ctx, document)
	}
}

type XML struct{}

var normalizedTag = regexp.MustCompile(`[^a-z0-9]`)

func (XML) Recognize(_ context.Context, document recognition.Document) (recognition.ProviderResult, error) {
	decoder := xml.NewDecoder(bytes.NewReader(document.Bytes))
	values := map[string]string{}
	var stack []string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return recognition.ProviderResult{}, fmt.Errorf("invoice XML is invalid: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			stack = append(stack, normalizedTag.ReplaceAllString(strings.ToLower(value.Name.Local), ""))
		case xml.CharData:
			if len(stack) != 0 {
				text := strings.TrimSpace(string(value))
				if text != "" && values[stack[len(stack)-1]] == "" {
					values[stack[len(stack)-1]] = text
				}
			}
		case xml.EndElement:
			if len(stack) != 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	candidate := contract.Candidate{
		InvoiceCode:        pick(values, "invoicecode", "fpdm", "fapiaodaima"),
		InvoiceNumber:      pick(values, "invoicenumber", "invoiceno", "fpdmhm", "fphm", "fapiaohaoma"),
		IssuedOn:           normalizeDate(pick(values, "issuedate", "invoicedate", "kprq", "kaipiaoriqi")),
		SellerName:         pick(values, "sellername", "xsfmc", "salesname"),
		SellerTaxID:        pick(values, "sellertaxid", "selleruscc", "xsfnsrsbh", "salestaxno"),
		BuyerName:          pick(values, "buyername", "gmfmc", "purchasername"),
		BuyerTaxID:         pick(values, "buyertaxid", "buyeruscc", "gmfnsrsbh", "purchasertaxno"),
		Currency:           "CNY",
		FieldConfidenceBPS: map[string]int64{},
	}
	warnings := []string{}
	for field, raw := range map[string]string{
		"amount_excluding_tax_minor": pick(values, "amountwithouttax", "hjje", "totalamount", "notaxamount"),
		"tax_amount_minor":           pick(values, "taxamount", "hjse", "totaltaxamount"),
		"total_amount_minor":         pick(values, "totalamountwithtax", "jshj", "amountincludetax", "totalamountandtax"),
	} {
		if raw == "" {
			continue
		}
		minor, err := decimalMinor(raw)
		if err != nil {
			warnings = append(warnings, field+" 无法按人民币金额解析")
			continue
		}
		switch field {
		case "amount_excluding_tax_minor":
			candidate.AmountExcludingTaxMinor = &minor
		case "tax_amount_minor":
			candidate.TaxAmountMinor = &minor
		case "total_amount_minor":
			candidate.TotalAmountMinor = &minor
		}
	}
	for field, present := range map[string]bool{
		"invoice_code": candidate.InvoiceCode != "", "invoice_number": candidate.InvoiceNumber != "", "issued_on": candidate.IssuedOn != "",
		"seller_name": candidate.SellerName != "", "seller_tax_id": candidate.SellerTaxID != "", "buyer_name": candidate.BuyerName != "", "buyer_tax_id": candidate.BuyerTaxID != "",
		"amount_excluding_tax_minor": candidate.AmountExcludingTaxMinor != nil, "tax_amount_minor": candidate.TaxAmountMinor != nil, "total_amount_minor": candidate.TotalAmountMinor != nil, "currency": true,
	} {
		if present {
			candidate.FieldConfidenceBPS[field] = 10000
		}
	}
	if candidate.InvoiceNumber == "" {
		warnings = append(warnings, "XML 中未识别到发票号码，请人工补充")
	}
	return recognition.ProviderResult{ProviderKey: "structured-invoice-xml", ProviderModel: "namespace-tolerant-field-map", ProviderRevision: "v1", Candidate: candidate, Warnings: warnings}, nil
}

func pick(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}
func normalizeDate(value string) string {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006-01-02", "20060102", "2006/01/02", "2006年01月02日"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format("2006-01-02")
		}
	}
	return value
}
func decimalMinor(raw string) (int64, error) {
	raw = strings.NewReplacer(",", "", "￥", "", "¥", "", " ", "").Replace(strings.TrimSpace(raw))
	if strings.HasPrefix(raw, "-") {
		return 0, fmt.Errorf("negative amount")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return 0, fmt.Errorf("invalid amount")
	}
	whole, fraction := int64(0), ""
	for _, char := range parts[0] {
		if char < '0' || char > '9' || whole > (1<<63-1-int64(char-'0'))/10 {
			return 0, fmt.Errorf("invalid amount")
		}
		whole = whole*10 + int64(char-'0')
	}
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 2 {
		return 0, fmt.Errorf("amount has more than two decimals")
	}
	for len(fraction) < 2 {
		fraction += "0"
	}
	cents := int64(0)
	for _, char := range fraction {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("invalid amount")
		}
		cents = cents*10 + int64(char-'0')
	}
	if whole > (1<<63-1-cents)/100 {
		return 0, fmt.Errorf("amount overflow")
	}
	return whole*100 + cents, nil
}

type HTTP struct {
	client                  *http.Client
	endpoint, apiKey, model string
}

func NewHTTP(endpoint, apiKey, model string, timeout time.Duration) (*HTTP, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || len(strings.TrimSpace(apiKey)) < 16 || strings.TrimSpace(model) == "" || timeout <= 0 || timeout > 2*time.Minute {
		return nil, fmt.Errorf("invoice AI provider configuration is invalid")
	}
	return &HTTP{client: &http.Client{Timeout: timeout}, endpoint: strings.TrimRight(endpoint, "/"), apiKey: apiKey, model: strings.TrimSpace(model)}, nil
}
func (p *HTTP) Recognize(ctx context.Context, document recognition.Document) (recognition.ProviderResult, error) {
	payload, _ := json.Marshal(map[string]any{
		"schema_version": "invoice-recognition.provider-request/v1", "model": p.model,
		"evidence": map[string]any{"mime_type": document.Evidence.MIMEType, "content_base64": base64.StdEncoding.EncodeToString(document.Bytes)},
	})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return recognition.ProviderResult{}, err
	}
	request.Header.Set("Authorization", "Bearer "+p.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return recognition.ProviderResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		return recognition.ProviderResult{}, fmt.Errorf("invoice AI provider returned HTTP %d", response.StatusCode)
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var result recognition.ProviderResult
	if err = decoder.Decode(&result); err != nil {
		return recognition.ProviderResult{}, err
	}
	return result, nil
}

var _ recognition.Provider = (*Router)(nil)
var _ recognition.Provider = (*HTTP)(nil)
