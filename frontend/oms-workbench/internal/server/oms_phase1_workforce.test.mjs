import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';

const html=readFileSync(new URL('./oms_phase1.html',import.meta.url),'utf8');
const shell=readFileSync(new URL('./oms_phase1.js',import.meta.url),'utf8');
const workforce=readFileSync(new URL('./oms_phase1_workforce.js',import.meta.url),'utf8');
const css=readFileSync(new URL('./oms_phase1.css',import.meta.url),'utf8');
const server=readFileSync(new URL('./server.go',import.meta.url),'utf8');

test('internal navigation exposes attendance and invoice recognition without replacing existing tabs',()=>{
  for(const marker of ['data-oms-v1-tab="members"','data-oms-v1-tab="attendance"','data-oms-v1-tab="payroll"','data-oms-v1-tab="finance"','data-oms-v1-tab="invoice-recognition"'])assert.ok(html.includes(marker),`missing ${marker}`);
  assert.match(shell,/attendance:'考勤与请假'/);
  assert.match(shell,/'invoice-recognition':'发票识别'/);
  assert.match(server,/oms-phase1-workforce\.js\?v=20261108\.5/);
});

test('attendance journey keeps policy, facts, immutable summary and explicit payroll application separate',()=>{
  for(const marker of [
    "ownerAPI.api('workforce-attendance','/api/v1/policies')",
    "'/api/v1/attendance-records'",
    "'/api/v1/leave-records'",
    "'/api/v1/monthly-summaries:calculate'",
    "'/attendance-summary'",
    'expected_run_revision',
    'expected_line_revision',
    'summary_id',
  ])assert.ok(workforce.includes(marker),`missing ${marker}`);
  assert.ok(html.includes('月中入离职由 People 任职快照自动折算'));
});

test('attendance policy configuration stays hidden while leave and absence entry remain available',()=>{
  assert.match(workforce,/calculateControl\.hidden=!currentPolicy/);
  assert.match(workforce,/月度折算配置暂未开放/);
  assert.match(html,/data-workforce-policy hidden/);
  assert.match(html,/data-workforce-absence/);
  assert.match(html,/data-workforce-leave/);
});

test('invoice recognition uploads a trusted evidence version and requires review before confirmation',()=>{
  assert.match(shell,/content_sha256:'sha256:'/);
  assert.match(workforce,/purpose:'invoice',status:progress/,'recognized invoice source must request invoice-v1 Evidence validation');
  assert.doesNotMatch(workforce,/purpose:'invoice_recognition_source'/,'generic Evidence uploads cannot be consumed by Finance');
  for(const marker of [
    'OMSPhase1EvidenceBridge',
    "ownerAPI.api('invoice-recognition','/api/v1/proposals')",
    "'/api/v1/proposals'",
    "'/confirm'",
    'content_sha256',
    'expected_revision',
  ])assert.ok(workforce.includes(marker),`missing ${marker}`);
  assert.ok(html.includes('确认后可用于新增报销、付款申请'));
  assert.ok(workforce.includes('recognizeForFinance'));
  assert.ok(workforce.includes('recognizeForPayment:recognizeForFinance'));
  assert.ok(workforce.includes("reviewField('发票代码（选填）'"));
  assert.ok(workforce.indexOf("reviewField('发票号码'")<workforce.indexOf("reviewField('发票代码（选填）'"));
  assert.doesNotMatch(workforce,/reviewField\('发票代码','invoice_code'.*required:paymentRequired/);
  for(const marker of ['data-invoice-review-workspace','data-recognition-box','field_regions','previewFile','layout:\'recognition\'','recognitionWarningText','data-invoice-viewer','data-invoice-zoom','bindInvoiceViewer','data-recognition-region-field'])assert.ok(workforce.includes(marker),`missing review marker ${marker}`);
  assert.ok(workforce.includes('oms-v1__invoice-review-processing-image'), 'upload progress preview must not reuse the transformable review layer');
  assert.ok(workforce.includes('text.replaceAll(field,label)'), 'persisted internal warning keys must be presented as user-facing field labels');
  for(const marker of ['.oms-v1__invoice-review{','.oms-v1__invoice-review-box{','.oms-v1__dialog-mask.is-recognition .oms-v1__dialog{','.oms-v1__invoice-viewer{','.oms-v1__invoice-viewer-toolbar{'])assert.ok(css.includes(marker),`missing review style ${marker}`);
  assert.match(shell,/dialog\.classList\.toggle\('is-recognition',layout==='recognition'\)/);
  assert.doesNotMatch(css,/\.oms-v1__dialog\.is-recognition\{/,'recognition class belongs to the mask, not the dialog section');
  for(const marker of ["regionField:'amount_excluding_tax_minor'","regionField:'tax_amount_minor'","regionField:'total_amount_minor'"])assert.ok(workforce.includes(marker),`amount card must map to OCR region: ${marker}`);
  assert.ok(html.includes('JPEG、PNG、PDF 首页面与电子发票 XML 可直接识别'));
  assert.doesNotMatch(workforce,/自动入账|自动报销|自动付款/);
});

test('invoice recognition supports audited discard, structured export and internal payment handoff',()=>{
  for(const marker of [
    "'/discard'",
    'expected_revision:proposal.revision',
    "value.status!=='discarded'",
    'invoice-recognition.delivery/v1',
    'data-invoice-recognition-export',
    'OMSInvoiceRecognitionBridge',
    'recognizeForPayment',
  ])assert.ok((marker.startsWith('data-')?html:workforce).includes(marker),`missing recognition delivery marker ${marker}`);
  assert.doesNotMatch(html,/关联使用/);
  assert.doesNotMatch(workforce,/data-invoice-use=/,'recognition list must not expose external billing association');
  assert.ok(shell.includes("openInvoiceCreate(direction==='incoming',proposal)"),'legacy external handoff may remain implemented but unreachable');
});

test('invoice recognition preflights tenant capabilities and terminates failed progress',()=>{
  assert.match(workforce,/await authorizeRecognition\(\);/);
  assert.match(workforce,/\['evidence-service','附件上传'\],\['invoice-recognition','发票识别'\]/);
  assert.match(workforce,/RECOGNITION_CAPABILITY_REQUIRED/);
  assert.match(workforce,/权限按组织独立配置，请切换到有权限的组织/);
  assert.match(workforce,/indicator\.hidden=true/);
  assert.match(workforce,/title\.textContent='识别未完成'/);
  assert.match(workforce,/catch\(error\)\{markRecognitionFailure\(error\);throw error\}/);
  assert.doesNotMatch(workforce,/Evidence service is unreachable/);
});
