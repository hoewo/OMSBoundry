import {test} from 'node:test';import assert from 'node:assert/strict';import {readFileSync} from 'node:fs';
import {commercialRelation,chains,validateCommercialLinkReceipt} from './commercial-relations.mjs';
test('all fourteen OA relations satisfy the actual unchanged Project key contract',()=>{
 const source=readFileSync(new URL('../../../../backend/services/project-workspace/internal/projects/model.go',import.meta.url),'utf8');
 const pattern=source.match(/var keyPattern = regexp\.MustCompile\(`([^`]+)`\)/);assert(pattern);
 const nativePattern=new RegExp(pattern[1]),seen=new Set();
 for(const chain of chains)for(const kind of ['contract','cash','invoice'])for(const fee of kind==='contract'?[null]:['design_fee','survey_fee','consultation_fee']){const key=commercialRelation(kind,chain,fee);assert(nativePattern.test(key),key);assert(!seen.has(key),key);seen.add(key);}
 assert.equal(seen.size,14);
 for(const key of ['reference-oa-contract-client_to_qualification','reference-oa:cash:v1:client_to_qualification:design_fee'])assert.equal(nativePattern.test(key),false);
 assert.throws(()=>commercialRelation('invoice','unknown','design_fee'));
});
test('Project command receipt is its actual singular ResourceLinkResult, not mocked GET Details',()=>{
 const source=readFileSync(new URL('../../../../backend/services/project-workspace/internal/projects/model.go',import.meta.url),'utf8');
 assert.match(source,/type ResourceLinkResult struct \{\s*Project\s+Project\s+`json:"project"`\s*ResourceLink\s+ResourceLink\s+`json:"resource_link"`/);
 const id='11111111-1111-4111-8111-111111111111',tenant='22222222-2222-4222-8222-222222222222',resource='33333333-3333-4333-8333-333333333333';
 const input={expected_revision:4,target_service:'contract-ledger',resource_type:'contract',resource_id:resource,source_revision:1,relation_key:commercialRelation('contract',chains[0])};
 const link={id:'44444444-4444-4444-8444-444444444444',project_id:id,tenant_id:tenant,...input},project={id,tenant_id:tenant,revision:5};
 assert.equal(validateCommercialLinkReceipt({project,resource_link:link},id,tenant,input),link);
 assert.throws(()=>validateCommercialLinkReceipt({project,resource_links:[link],allowed_actions:[]},id,tenant,input));
 for(const patch of [{tenant_id:id},{resource_id:id},{source_revision:2},{relation_key:'wrong'},{unlinked_at:'2026-10-04'},{id:'wrong'}])assert.throws(()=>validateCommercialLinkReceipt({project,resource_link:{...link,...patch}},id,tenant,input));
 assert.throws(()=>validateCommercialLinkReceipt({project:{...project,revision:6},resource_link:link},id,tenant,input));
});
