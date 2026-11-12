package provider

import "testing"

func TestInvoiceFromTSVReturnsFieldsAndVisualRegions(t *testing.T) {
	tsv := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"1\t1\t0\t0\t0\t0\t0\t0\t2000\t1200\t-1\t\n" +
		"5\t1\t1\t1\t1\t1\t1200\t80\t180\t40\t96.4\t发票号码：\n" +
		"5\t1\t1\t1\t1\t2\t1400\t80\t250\t40\t98.1\t12345678\n" +
		"5\t1\t1\t1\t2\t1\t1200\t140\t180\t40\t95.0\t开票日期：\n" +
		"5\t1\t1\t1\t2\t2\t1400\t140\t260\t40\t97.0\t2026年11月06日\n" +
		"5\t1\t2\t1\t1\t1\t180\t320\t120\t40\t94.0\t名称：\n" +
		"5\t1\t2\t1\t1\t2\t320\t320\t360\t40\t93.0\t北京购买方有限公司\n" +
		"5\t1\t2\t1\t2\t1\t180\t380\t240\t40\t96.0\t纳税人识别号：\n" +
		"5\t1\t2\t1\t2\t2\t440\t380\t420\t40\t98.0\t91110000987654321A\n" +
		"5\t1\t3\t1\t1\t1\t180\t760\t120\t40\t94.0\t名称：\n" +
		"5\t1\t3\t1\t1\t2\t320\t760\t360\t40\t93.0\t北京销售方有限公司\n" +
		"5\t1\t3\t1\t2\t1\t180\t820\t240\t40\t96.0\t纳税人识别号：\n" +
		"5\t1\t3\t1\t2\t2\t440\t820\t420\t40\t98.0\t91110000123456789X\n" +
		"5\t1\t4\t1\t1\t1\t900\t1000\t260\t50\t97.0\t价税合计（小写）\n" +
		"5\t1\t4\t1\t1\t2\t1200\t1000\t240\t50\t99.0\t￥113.00\n"
	result, err := invoiceFromTSV(tsv)
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidate.InvoiceNumber != "12345678" || result.Candidate.IssuedOn != "2026-11-06" || result.Candidate.BuyerName != "北京购买方有限公司" || result.Candidate.SellerName != "北京销售方有限公司" || result.Candidate.TotalAmountMinor == nil || *result.Candidate.TotalAmountMinor != 11300 {
		t.Fatalf("candidate=%+v", result.Candidate)
	}
	for _, field := range []string{"invoice_number", "issued_on", "buyer_name", "seller_name", "buyer_tax_id", "seller_tax_id", "total_amount_minor"} {
		regions := result.FieldRegions[field]
		if len(regions) != 1 || regions[0].WidthBPS <= 0 || regions[0].HeightBPS <= 0 {
			t.Fatalf("field %s regions=%+v", field, regions)
		}
	}
}

func TestInvoiceFromTSVRejectsUnpositionedNoise(t *testing.T) {
	_, err := invoiceFromTSV("level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n")
	if err == nil {
		t.Fatal("empty OCR output accepted")
	}
}

func TestLocalOCRWarningsUseUserFacingFieldLabels(t *testing.T) {
	if got := localOCRFieldLabel("tax_amount_minor"); got != "税额" {
		t.Fatalf("tax amount label=%q", got)
	}
	if got := localOCRFieldLabel("unknown_internal_key"); got != "识别字段" {
		t.Fatalf("unknown field label=%q", got)
	}
}

func TestInvoiceFromTSVRecoversStandardInvoiceColumnsWhenLabelsAreNoisy(t *testing.T) {
	tsv := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"1\t1\t0\t0\t0\t0\t0\t0\t1200\t800\t-1\t\n" +
		"5\t1\t1\t1\t1\t1\t760\t70\t100\t30\t80\t发栗号鸢\n" +
		"5\t1\t1\t1\t1\t2\t880\t70\t280\t30\t99\t24932000000038888512\n" +
		"5\t1\t2\t1\t1\t1\t80\t240\t100\t35\t72\t吊称：\n" +
		"5\t1\t2\t1\t1\t2\t190\t240\t260\t35\t96\t示例企业名称\n" +
		"5\t1\t2\t1\t1\t3\t680\t240\t100\t35\t74\t名稍：\n" +
		"5\t1\t2\t1\t1\t4\t790\t240\t340\t35\t96\t宁波邻家网络科技有限公司\n" +
		"5\t1\t2\t1\t2\t1\t80\t300\t300\t35\t70\t统一社会信用代硖：\n" +
		"5\t1\t2\t1\t2\t2\t390\t300\t180\t35\t99\t910000000000000000\n" +
		"5\t1\t2\t1\t2\t3\t680\t300\t260\t35\t70\t纳税识别旨：\n" +
		"5\t1\t2\t1\t2\t4\t950\t300\t180\t35\t99\t913302035805217768\n" +
		"5\t1\t3\t1\t1\t1\t500\t500\t170\t35\t96\t2371.68\n" +
		"5\t1\t3\t1\t1\t2\t980\t500\t130\t35\t96\t308.32\n" +
		"5\t1\t3\t1\t2\t1\t760\t570\t170\t35\t96\t价税合计（小写）\n" +
		"5\t1\t3\t1\t2\t2\t950\t570\t150\t35\t99\t￥2680.00\n"
	result, err := invoiceFromTSV(tsv)
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidate.InvoiceNumber != "24932000000038888512" || result.Candidate.BuyerName != "示例企业名称" || result.Candidate.SellerName != "宁波邻家网络科技有限公司" || result.Candidate.BuyerTaxID != "910000000000000000" || result.Candidate.SellerTaxID != "913302035805217768" {
		t.Fatalf("candidate identities=%+v", result.Candidate)
	}
	if result.Candidate.TotalAmountMinor == nil || *result.Candidate.TotalAmountMinor != 268000 || result.Candidate.TaxAmountMinor == nil || *result.Candidate.TaxAmountMinor != 30832 || result.Candidate.AmountExcludingTaxMinor == nil || *result.Candidate.AmountExcludingTaxMinor != 237168 {
		t.Fatalf("candidate amounts=%+v", result.Candidate)
	}
	for _, field := range []string{"buyer_name", "seller_name", "buyer_tax_id", "seller_tax_id", "amount_excluding_tax_minor", "tax_amount_minor", "total_amount_minor"} {
		if len(result.FieldRegions[field]) != 1 {
			t.Fatalf("field %s regions=%+v", field, result.FieldRegions[field])
		}
	}
}

func TestHeaderDateTSVAcceptsCompleteChineseDateFromNoisyLabel(t *testing.T) {
	tsv := "level\tpage_num\tblock_num\tpar_num\tline_num\tword_num\tleft\ttop\twidth\theight\tconf\ttext\n" +
		"1\t1\t0\t0\t0\t0\t0\t0\t1500\t480\t-1\t\n" +
		"5\t1\t1\t1\t1\t1\t120\t200\t90\t40\t88\t开\n" +
		"5\t1\t1\t1\t1\t2\t220\t200\t90\t40\t72\t祭\n" +
		"5\t1\t1\t1\t1\t3\t320\t200\t90\t40\t93\t日\n" +
		"5\t1\t1\t1\t1\t4\t420\t200\t90\t40\t95\t期\n" +
		"5\t1\t1\t1\t1\t5\t560\t200\t190\t40\t97\t2024\n" +
		"5\t1\t1\t1\t1\t6\t760\t200\t60\t40\t95\t年\n" +
		"5\t1\t1\t1\t1\t7\t830\t200\t90\t40\t98\t06\n" +
		"5\t1\t1\t1\t1\t8\t930\t200\t60\t40\t94\t月\n" +
		"5\t1\t1\t1\t1\t9\t1000\t200\t90\t40\t98\t05\n" +
		"5\t1\t1\t1\t1\t10\t1100\t200\t60\t40\t94\t日\n"
	lines, err := parseTSV(tsv)
	if err != nil {
		t.Fatal(err)
	}
	value, _, found := labeledMatch(lines, []string{"开票日期", "出票日期", "日期"}, dateRE)
	if !found || normalizeOCRDate(value) != "2024-06-05" {
		t.Fatalf("date found=%v raw=%q normalized=%q", found, value, normalizeOCRDate(value))
	}
}

func TestNormalizeOCRDateRejectsIncompleteOrInvalidDates(t *testing.T) {
	for _, value := range []string{"2024年0月05日", "2024年02月30日", "202406", "2024年13月01日"} {
		if got := normalizeOCRDate(value); got != "" {
			t.Fatalf("normalizeOCRDate(%q)=%q", value, got)
		}
	}
	if got := normalizeOCRDate("2024 年 06 月 05 日"); got != "2024-06-05" {
		t.Fatalf("valid date=%q", got)
	}
}
