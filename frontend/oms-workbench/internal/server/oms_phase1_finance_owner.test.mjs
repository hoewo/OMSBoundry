import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';

const owner=readFileSync(new URL('./oms_phase1_finance_owner.js',import.meta.url),'utf8');
const shell=readFileSync(new URL('./oms_phase1.js',import.meta.url),'utf8');
const local=readFileSync(new URL('./oms_phase1_internal.js',import.meta.url),'utf8');
const server=readFileSync(new URL('./server.go',import.meta.url),'utf8');

test('formal finance owner replaces browser-local finance when assembled',()=>{
  assert.match(local,/const ownerFinance=Boolean\(ownerAPI\?\.app\?\.\('internal-finance-requests'\)\)/);
  assert.match(local,/if\(!ownerFinance\)renderFinance\(\)/);
  assert.match(server,/oms-phase1-finance-owner\.js\?v=20261112\.1/);
});

test('new reimbursement and payment requests offer manual or recognized invoice entry',()=>{
  for(const marker of [
    '发票优先录入',
    '上传发票，自动填写基本信息',
    '直接录入',
    '上传并识别',
    'OMSInvoiceRecognitionBridge',
    "invoice_mode:'recognition'",
    "invoiceMode==='recognition'",
    "purpose:'invoice'",
    'invoice={code:',
    'invoiceEntryMarkup(draft||{},recognized,type)',
    'openEditor(item,type,next,proposal)',
    'recognizeForFinance',
  ])assert.ok(owner.includes(marker),`missing payment invoice marker ${marker}`);
  assert.ok(owner.includes('申请金额必须与已确认发票价税合计一致'));
  assert.ok(owner.includes('请填写发票号码'));
  assert.ok(owner.includes('发票代码（选填）'));
  assert.ok(owner.indexOf('发票号码 *')<owner.indexOf('发票代码（选填）'));
  assert.doesNotMatch(owner,/发票代码 \*/);
  assert.ok(owner.includes("requestLabel=reimbursement?'报销':'付款'"));
  assert.doesNotMatch(owner,/paymentInvoiceMode/);
  assert.ok(owner.includes('发票优先录入'));
  assert.ok(owner.includes('上传发票并识别'));
  assert.ok(owner.includes('oms-v1__finance-invoice-primary'));
  assert.ok(owner.includes("querySelectorAll('[data-finance-recognize]')"));
  assert.ok(owner.indexOf('invoiceEntryMarkup(draft||{},recognized,type)')<owner.indexOf("field(copy+' *'"),'invoice entry must appear before basic information');
});

test('uploaded reimbursement invoices use the strict finance Evidence contract',()=>{
  assert.match(owner,/function financeEvidenceRef\(ref\)/);
  assert.match(owner,/sha256:String\(ref\?\.content_sha256\|\|ref\?\.sha256\|\|''\)\.replace/);
  assert.match(owner,/refs\.map\(financeEvidenceRef\)/);
  assert.match(owner,/class="oms-v1__category-row"/);
  assert.doesNotMatch(owner,/name="enabled_'\+item\.id\+'"[^>]*> 启用/);
});

test('finance owner covers revisioned requests, payment, independent exports and categories',()=>{
  for(const marker of [
    '/api/v1/finance-requests',
    '/api/v1/finance-payments/batch',
    '/sensitive-account?purpose=',
    '/api/v1/finance-request-exports',
    '/api/v1/finance-request-sensitive-exports',
    '/api/v1/expense-categories',
    'missing_evidence_acknowledged',
    'expected_revision',
    'payment_unknown_count',
    'operator_subject_id',
    'details?.attribution?.department',
    'details?.attribution?.contract',
    'amount_',
    'date_',
  ])assert.ok(owner.includes(marker),`missing ${marker}`);
});

test('finance operator ids resolve through current organization IAM names',()=>{
  for(const marker of [
    '/api/v1/admin/subjects',
    'actors:new Map()',
    'const actorLabel=',
    "operator=item.deletion?.actor_subject_id||item.payments?.at(-1)?.operator_subject_id||item.applicant_subject_id",
    'safe(actorLabel(value.operator_subject_id))',
    "actorLabel(item.deletion?.actor_subject_id||item.payments?.at(-1)?.operator_subject_id||item.applicant_subject_id)",
  ])assert.ok(owner.includes(marker),`missing ${marker}`);
  assert.ok(owner.includes("return displayName||'账号 '+id.slice(0,8)"),'unknown operators must not expose a full subject id');
});

test('finance batch selection follows the request lifecycle instead of disabling every draft',()=>{
  for(const marker of [
    'const paymentSelectable=',
    'const submitSelectable=',
    'const batchKind=',
    'const selectedBatchKind=',
    'const selectionReason=',
    'data-finance-owner-kind',
    'data-finance-owner-submit',
    'data-submit-single-request',
    'function openSubmit(items)',
    "selectedBatchKind()==='submit'?openSubmit(items):openPayment(items)",
    "missing_evidence_acknowledged:item.request_type==='payment'&&!hasEvidence(item)",
    "action.textContent=active==='submit'?'进入待支付':active==='payment'?'确认已支付':'选择后操作'",
    '确认进入待支付',
    'hasEnteredPayable',
    'isRevisionConflict',
    '申请已被更新，请关闭窗口并重新确认',
  ])assert.ok(owner.includes(marker),`missing lifecycle batch marker ${marker}`);
  assert.doesNotMatch(owner,/const selectable=/);
  assert.doesNotMatch(owner,/selectable\(item\)/);
});

test('strong invoice duplicates block submission and identify the occupying request',()=>{
  for(const marker of [
    'const duplicateIDs=',
    'const duplicateConflict=',
    'const duplicateSummary=',
    'conflictTargets:new Map()',
    'hydrateConflictTargets',
    'data-finance-owner-conflict',
    'data-open-duplicate-request',
    '查看占用申请',
    '当前发票不能再次提交',
    '这张发票已经使用过',
    '不能覆盖或再次提交',
    '修改／更换发票',
    'openDuplicateResolution',
    "error?.code==='INVOICE_STRONG_DUPLICATE'",
    'error?.data?.matched_request_ids',
    "&&!duplicateConflict(item)",
  ])assert.ok(owner.includes(marker),`missing duplicate blocker marker ${marker}`);
  assert.ok(shell.includes('error.data=data'),'Owner API errors must preserve structured conflict references');
});

test('audited deletion covers reimbursement and payment requests before actual funds',()=>{
  for(const marker of [
    "method:'DELETE'",
    'data-finance-owner-delete',
    'data-delete-request',
    '删除当前草稿',
    'function openDelete(item)',
    '删除原因 *',
    'expected_revision:item.revision',
    "confirmation:'DELETE'",
    '/api/v1/finance-requests?include_deleted=true',
    "status==='deleted'?deleted",
    '删除原因：',
    '已经实际付款或付款结果未知的申请不能删除',
  ])assert.ok(owner.includes(marker),`missing audited request deletion marker ${marker}`);
});
