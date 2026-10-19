// Closed customer relation vocabulary; satisfies Project's existing keyPattern.
// Do not loosen the owner's validation to accommodate an adapter defect.
export const chains=['client_to_qualification','qualification_to_our_company'];
const chainKeys={client_to_qualification:'client',qualification_to_our_company:'income'};
const feeKeys={design_fee:'design',survey_fee:'survey',consultation_fee:'consultation'};
export function commercialRelation(kind,chain,fee){
 if(!['contract','cash','invoice'].includes(kind)||!Object.hasOwn(chainKeys,chain)||(kind!=='contract'&&!Object.hasOwn(feeKeys,fee)))throw new Error('经营关联分类必须明确');
 return `reference-oa-${kind}-v1-${chainKeys[chain]}`+(kind==='contract'?'':'-'+feeKeys[fee]);
}
// Project POST returns ResourceLinkResult, not the GET Details projection.
export function validateCommercialLinkReceipt(value,projectID,tenantID,input){
 const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
 const p=value?.project,l=value?.resource_link;
 if(p?.id!==projectID||p.tenant_id!==tenantID||p.revision!==input.expected_revision+1||!uuid.test(l?.id||'')||l.project_id!==projectID||l.tenant_id!==tenantID||l.target_service!==input.target_service||l.resource_type!==input.resource_type||l.resource_id!==input.resource_id||l.source_revision!==input.source_revision||l.relation_key!==input.relation_key||l.unlinked_at)throw new Error('项目关联原生回执与原意图不一致');
 return l;
}
