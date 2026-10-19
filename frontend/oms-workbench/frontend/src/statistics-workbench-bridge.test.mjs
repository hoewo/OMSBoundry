import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import test from 'node:test'
import vm from 'node:vm'
const source=readFileSync(new URL('../../internal/server/server.go',import.meta.url),'utf8'),start=source.indexOf('async function businessApplicationFetch('),bridge=source.slice(start,source.indexOf('async function businessApplicationJSON(',start)),project='33333333-3333-4333-8333-333333333333'
function fixture(app,route='/owner',deny=false){
 const calls=[],context=vm.createContext({URL,URLSearchParams,Headers,Error,document:{querySelectorAll:()=>[{dataset:{realApplication:app,realRoute:route}}]},location:{origin:'https://private.example'},architectureTrace:null,recordArchitectureStep(){},referenceOACSRF:{token:async()=>'csrf'},referenceOAController:null,initiationWorkspace:null,projectAPIError:(status,data)=>Object.assign(new Error(data.error.message),{status}),fetch:async(path,options)=>{calls.push({path,options});return {ok:!deny,status:deny?403:200,url:'https://private.example'+route+'/'}}})
 vm.runInContext(bridge,context);return {calls,request:context.businessApplicationFetch}
}
for(const app of ['bonus-distribution','cost-outsourcing'])for(const scope of ['business','production'])test(`statistics ${app}/${scope} scope is independent of the project-detail selection`,async()=>{
 const f=fixture(app),name=app==='bonus-distribution'?'bonus':'cost',path='/api/v1/reference-oa/'+(name==='bonus'?'bonus-allocations':'cost-entries')+'?project_id='+project+'&scope='+scope,options={statisticsProject:project,[name==='bonus'?'statisticsBonusScope':'statisticsCostScope']:scope}
 await f.request(app,path,options);assert.deepEqual(Object.fromEntries(f.calls[0].options.body),{csrf_token:'csrf',resource_type:scope+'_'+name+'_project',resource_id:project});assert.equal(f.calls[1].options.statisticsProject,undefined);assert.equal(f.calls[1].options.statisticsCostScope,undefined);assert.equal(f.calls[1].options.statisticsBonusScope,undefined)
})
for(const app of ['money-flow-ledger','billing-invoice'])test(`statistics ${app} export acquires the exact native resource`,async()=>{
 const f=fixture(app),cash=app==='money-flow-ledger',path='/api/v1/'+(cash?'cash-transactions':'invoice-registrations')+'/'+project+'/export-view';await f.request(app,path,{statisticsExport:project});assert.deepEqual(Object.fromEntries(f.calls[0].options.body),{csrf_token:'csrf',resource_type:cash?'cash_transaction':'invoice_registration',resource_id:project});assert.equal(f.calls[1].options.statisticsExport,undefined)
})
test('mismatched project/side/export, unknown app and IAM denial make no owner call',async()=>{
 for(const [app,path,options] of [['cost-outsourcing','/api/v1/reference-oa/cost-entries?project_id=other&scope=business',{statisticsProject:project,statisticsCostScope:'business'}],['bonus-distribution','/api/v1/reference-oa/bonus-allocations?project_id='+project+'&scope=other',{statisticsProject:project,statisticsBonusScope:'other'}],['billing-invoice','/api/v1/invoice-registrations/other/export-view',{statisticsExport:project}],['contract-ledger','/api/v1/contracts',{statisticsProject:project,statisticsCostScope:'business'}]]){const f=fixture(app);await assert.rejects(f.request(app,path,options));assert.equal(f.calls.length,0)}
 const f=fixture('money-flow-ledger','/owner',true);await assert.rejects(f.request('money-flow-ledger','/api/v1/cash-transactions/'+project+'/export-view',{statisticsExport:project}));assert.equal(f.calls.length,1)
})
