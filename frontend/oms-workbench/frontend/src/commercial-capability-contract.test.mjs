import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {ContractAdapter} from './contract-adapter.mjs';
import {CashAdapter} from './cash-adapter.mjs';
import {InvoiceAdapter} from './invoice-adapter.mjs';
const catalog=JSON.parse(readFileSync(new URL('../../../../contracts/access/v1/examples/project-workspace-capabilities.json',import.meta.url)));
const permission=catalog.capabilities.find(v=>v.id==='project-workspace.resource-link.manage');
assert(permission?.resource_types.includes('project'));
for(const [name,Adapter,context] of [
 ['contract',ContractAdapter,{allowed_operations:['draft.create']}],
 ['cash',CashAdapter,{allowed_operations:['cash.record']}],
 ['invoice',InvoiceAdapter,{allowed_actions:['record']}],
])test(name+' new button uses the declared Project capability, never an invented alias',()=>{
 const details={project:{id:'11111111-1111-4111-8111-111111111111',tenant_id:'22222222-2222-4222-8222-222222222222'},allowed_actions:[permission.id]};
 const adapter=new Adapter({details:()=>details,tenantID:details.project.tenant_id,subjectID:'33333333-3333-4333-8333-333333333333'});
 adapter.context=context;adapter.parties=[];
 assert.equal(adapter.canManage(),true);
 details.allowed_actions=['project-workspace.resource.manage'];assert.equal(adapter.canManage(),false);
 details.allowed_actions=[permission.id];adapter.context={allowed_operations:[],allowed_actions:[]};assert.equal(adapter.canManage(),false);
});
