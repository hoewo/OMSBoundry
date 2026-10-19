import fs from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { SpreadsheetFile, Workbook } from "@oai/artifact-tool";

const outputPath = fileURLToPath(new URL("./北区道路维护事项-经营台账-2026-09-10.xlsx", import.meta.url));
const previewPath = "/tmp/oms-bf138-ledger-preview.png";
const fontFamily = "Arial";

const workbook = Workbook.create();
const sheet = workbook.worksheets.add("经营台账");
sheet.showGridLines = false;
sheet.freezePanes.freezeRows(3);

sheet.mergeCells("A1:J1");
sheet.getRange("A1").values = [["北区道路维护事项 · 经营台账"]];
sheet.getRange("A1:J1").format = {
  fill: "#F6F8F4",
  font: { name: fontFamily, size: 16, bold: true, color: "#15231D" },
  verticalAlignment: "center",
};
sheet.getRange("A1:J1").format.rowHeight = 34;

const headers = [
  "事项编号",
  "事项名称",
  "方向",
  "业务环节",
  "累计值",
  "单位",
  "最近记录",
  "待处理",
  "状态",
  "统计日期",
];

const reportDate = new Date("2026-09-10T00:00:00+08:00");
const rows = [
  ["HT-2026-001", "北区道路维护事项", "上游", "履约计量与验收", 2, "期", "JL-2026-002 · 第2期", "确认依据待补", "待确认", reportDate],
  ["HT-2026-001", "北区道路维护事项", "上游", "向上游开票", 320, "万元", "FP-2026-002 · 120万", "合同余480万", "已登记", reportDate],
  ["HT-2026-001", "北区道路维护事项", "上游", "确认应收", 320, "万元", "YS-2026-002 · 160万", "未回100万", "已确认", reportDate],
  ["HT-2026-001", "北区道路维护事项", "上游", "实际回款", 220, "万元", "HK-2026-002 · 60万", "60万逾期", "有风险", reportDate],
  ["HT-2026-001", "北区道路维护事项", "下游", "承接范围与履约", 2, "个", "企业1 · 班组1", "1项资料待核", "履约中", reportDate],
  ["HT-2026-001", "北区道路维护事项", "下游", "下游结算", 268, "万元", "JS-2026-002 · 96万", "—", "已登记", reportDate],
  ["HT-2026-001", "北区道路维护事项", "下游", "确认应付", 268, "万元", "YF-2026-002 · 96万", "未付148万", "已确认", reportDate],
  ["HT-2026-001", "北区道路维护事项", "下游", "实际付款", 120, "万元", "已登记1笔", "96万待判断", "待处理", reportDate],
];

sheet.getRange("A3:J11").values = [headers, ...rows];
const table = sheet.tables.add("A3:J11", true, "OperatingLedgerTable");
table.style = "TableStyleMedium4";
table.showFilterButton = true;
table.showBandedColumns = false;

sheet.getRange("A3:J3").format = {
  fill: "#176241",
  font: { name: fontFamily, size: 10, bold: true, color: "#FFFFFF" },
  verticalAlignment: "center",
};
sheet.getRange("A3:J3").format.rowHeight = 28;

sheet.getRange("A4:J11").format.font = { name: fontFamily, size: 10, color: "#24312B" };
sheet.getRange("A4:J11").format.verticalAlignment = "center";
sheet.getRange("A4:J11").format.rowHeight = 26;
sheet.getRange("A4:J7").format.fill = "#F3F8F5";
sheet.getRange("A8:J11").format.fill = "#F1F5F8";
sheet.getRange("A8:J8").format.borders = {
  top: { style: "medium", color: "#5E7487" },
};
sheet.getRange("C4:C7").format.font = { name: fontFamily, size: 10, bold: true, color: "#176241" };
sheet.getRange("C8:C11").format.font = { name: fontFamily, size: 10, bold: true, color: "#5E7487" };
sheet.getRange("E4:E11").format.numberFormat = "#,##0";
sheet.getRange("J4:J11").format.numberFormat = "yyyy-mm-dd";

sheet.getRange("A3:A11").format.columnWidth = 16;
sheet.getRange("B3:B11").format.columnWidth = 24;
sheet.getRange("C3:C11").format.columnWidth = 9;
sheet.getRange("D3:D11").format.columnWidth = 17;
sheet.getRange("E3:E11").format.columnWidth = 11;
sheet.getRange("F3:F11").format.columnWidth = 9;
sheet.getRange("G3:G11").format.columnWidth = 24;
sheet.getRange("H3:H11").format.columnWidth = 18;
sheet.getRange("I3:I11").format.columnWidth = 12;
sheet.getRange("J3:J11").format.columnWidth = 14;

sheet.getRange("E4:F11").format.horizontalAlignment = "right";
sheet.getRange("C4:D11").format.horizontalAlignment = "center";
sheet.getRange("I4:J11").format.horizontalAlignment = "center";

sheet.getRange("H4:I4").format = {
  fill: "#FFF7E2",
  font: { name: fontFamily, size: 10, bold: true, color: "#925F00" },
};
sheet.getRange("H7:I7").format = {
  fill: "#FFF0EC",
  font: { name: fontFamily, size: 10, bold: true, color: "#B5483F" },
};
sheet.getRange("H8:I8").format = {
  fill: "#FFF7E2",
  font: { name: fontFamily, size: 10, bold: true, color: "#925F00" },
};
sheet.getRange("H11:I11").format = {
  fill: "#FFF7E2",
  font: { name: fontFamily, size: 10, bold: true, color: "#925F00" },
};

workbook.recalculate();
const tableCheck = await workbook.inspect({
  kind: "table",
  range: "经营台账!A1:J11",
  include: "values,formulas",
  tableMaxRows: 11,
  tableMaxCols: 10,
  maxChars: 5000,
});
const errorCheck = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A|#NUM!|#NULL!|#SPILL!|#CALC!",
  options: { useRegex: true, maxResults: 100 },
  summary: "final formula error scan",
});
console.log(tableCheck.ndjson);
console.log(errorCheck.ndjson);
const preview = await workbook.render({
  sheetName: "经营台账",
  range: "A1:J11",
  scale: 1.25,
  format: "png",
});
await fs.writeFile(previewPath, new Uint8Array(await preview.arrayBuffer()));

const xlsx = await SpreadsheetFile.exportXlsx(workbook);
await xlsx.save(outputPath);

console.log(JSON.stringify({ outputPath, previewPath }));
