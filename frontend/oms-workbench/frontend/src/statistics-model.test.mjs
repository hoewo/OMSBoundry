import test from 'node:test'
import assert from 'node:assert/strict'
import {minor,money,records,filterRecords,buckets,bonusRecords,costRecords,matrixRows,companyRecords,currencyTotals,excelXML} from './statistics-model.mjs'
const project={id:'project-a',name:'道路设计',project_number:'OA-2026-001'}
const segment=(id,amount='100.01',currency='CNY')=>({source_application:'money-flow-ledger',resource_type:'transaction',resource_id:id,relation_key:'reference-oa-cash-v1-income-design',state:'available',source_revision:1,transaction:{currency,effective_amount:amount,transaction_type:'receipt',occurred_on:'2026-10-07'}})
test('large decimal money remains exact; unsafe numeric invoice amounts are hidden',()=>{
 assert.equal(minor('9007199254740993.01'),900719925474099301n);assert.equal(money(-10001n),'CNY -100.01');assert.equal(minor(1.01),null)
 const raw={source_application:'billing-invoice',resource_type:'invoice_registration',resource_id:'inv',state:'available',invoice_registration:{amount_minor:9007199254740992,currency:'CNY'}}
 assert.equal(records({project_id:project.id,segments:[raw]},project)[0].amount,null)
})
test('source identity, range and duplicate conflicts fail closed',()=>{
 assert.throws(()=>records({project_id:'other',segments:[]},project))
 assert.throws(()=>records({project_id:project.id,segments:[{...segment('x'),source_application:'contract-ledger'}]},project))
 assert.throws(()=>records({project_id:project.id,segments:[segment('x'),segment('x','200.00')]},project))
 assert.equal(records({project_id:project.id,segments:[segment('x'),segment('x')]},project).length,1)
})
test('currency/source/meaning remain separated and unknown is never zero',()=>{
 const list=records({project_id:project.id,segments:[segment('a'),segment('b','20.00','USD'),{...segment('c'),state:'forbidden',transaction:undefined}]},project)
 const result=buckets(list);assert.equal(result.length,3);assert.equal(result.find(v=>v.currency==='CNY').amount,10001n);assert.equal(result.find(v=>v.currency==='').amount,null)
 const sameCurrencyUnknown=[...list,{...list[0],amount:null}];assert.equal(buckets(sameCurrencyUnknown).find(v=>v.currency==='CNY').amount,null)
})
test('keyword, fees, chain and inclusive date filters select the same export rows',()=>{
 const list=records({project_id:project.id,segments:[segment('a'),{...segment('b'),relation_key:'reference-oa-cash-v1-client-survey',transaction:{...segment('b').transaction,occurred_on:'2026-10-06'}}]},project)
 assert.equal(filterRecords(list,{keyword:'2026-001',fee:'design',chain:'income',start:'2026-10-07',end:'2026-10-07'}).length,1)
 assert.equal(filterRecords(list,{start:'2026-10-08'}).length,0)
})
test('Excel text cells escape formulas and XML markup without rounding',()=>{
 const xml=excelXML(['名称'],[['=HYPERLINK("bad") & <x>'],['9007199254740993.01']]);assert.match(xml,/ss:Type="String"/);assert.match(xml,/&amp; &lt;x&gt;/);assert.match(xml,/9007199254740993\.01/)
})
test('bonus totals only contain recorded rows from the precise tenant/project/scope',()=>{
 const row={id:'a',tenant_id:'tenant-a',project:{id:project.id},scope:'business',person:{id:'p',display_name:'陈工'},state:'recorded',amount_minor:'12345',currency:'CNY',allowed_actions:['bonus-manual-allocation.amount.read']}
 const page={profile:'reference-oa-bonus-v2',project_id:project.id,scope:'business',items:[row,{...row,id:'b',state:'withdrawn'}]}
 assert.equal(bonusRecords(page,project,'business','tenant-a').length,1);assert.equal(bonusRecords(page,project,'business','tenant-a')[0].amount,12345n)
 assert.throws(()=>bonusRecords(page,project,'business','tenant-b'));assert.equal(bonusRecords({...page,items:[{...row,allowed_actions:[]}]},project,'business','tenant-a')[0].amount,null)
})
test('fee matrix uses native obligation balances, never inferred contract less receipts',()=>{
 const list=records({project_id:project.id,segments:[{source_application:'money-flow-ledger',resource_type:'obligation',resource_id:'obl',relation_key:'reference-oa-cash-v1-income-design',state:'available',obligation:{currency:'CNY',direction:'receivable',effective_amount:'100.00',allocated_amount:'20.00',outstanding_amount:'70.00'}}]},project)
 const matrix=matrixRows(list);assert.equal(matrix.find(r=>r.label==='未清余额').total,7000n);assert.equal(matrix[0].design,10000n);assert.equal(matrix[0].survey,undefined)
})
test('multi-project references are excluded rather than counted twice',()=>{
 const rows=records({project_id:project.id,segments:[segment('shared')]},project),result=companyRecords([...rows,{...rows[0],projectID:'project-b'}]);assert.equal(result.rows.length,0);assert.equal(result.ambiguous.length,1)
})
test('cost projection checks profile, exact side, recorded state and amount permission',()=>{
 const row={id:'cost',profile:'reference-oa-cost-v1',project:{id:project.id},tenant_id:'tenant',scope:'business',category:'vehicle',state:'recorded',currency:'CNY',amount_minor:'9007199254740993',allowed_actions:['cost-entry.amount.read']},page={profile:row.profile,project_id:project.id,scope:'business',items:[row,{...row,id:'gone',state:'withdrawn'}]}
 assert.equal(costRecords(page,project,'business','tenant')[0].amount,9007199254740993n);assert.equal(costRecords(page,project,'business','tenant').length,1);assert.throws(()=>costRecords(page,project,'production','tenant'));assert.equal(costRecords({...page,items:[{...row,allowed_actions:[]}]},project,'business','tenant')[0].amount,null)
})
test('bonus and cost currency totals never merge currencies or unknown amounts',()=>{
 const groups=currencyTotals([{currency:'CNY',amount:1n},{currency:'USD',amount:3n},{currency:'CNY',amount:null}]);assert.equal(groups.length,2);assert.equal(groups.find(r=>r.currency==='CNY').amount,null);assert.equal(groups.find(r=>r.currency==='USD').amount,3n)
})
