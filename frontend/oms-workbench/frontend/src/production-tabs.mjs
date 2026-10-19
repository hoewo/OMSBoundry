// Original ProductionInfo tab order/type rules. `ready` only describes a
// mounted, navigable OA work surface. `native` separately records whether the
// surface has an accepted owner port; it never grants a capability or invents
// an OA fact.
export function productionTabs(types = [], applications = [], personnelProfile = '',stageProfile='') {
  const has = key => types.includes(key), app = key => applications.includes(key);
  const nativeStage = stageProfile==='reference-oa-stage-materials-v1'&&app('production-operations')&&app('evidence-service')&&app('project-workspace');
  return [
    {key:'overview',label:'总览',group:'info',ready:true,native:true},
    {key:'personnel',label:'人员配置',group:'info',ready:personnelProfile==='reference-oa-production-personnel-v1'&&app('project-workspace')&&app('people-groups'),native:personnelProfile==='reference-oa-production-personnel-v1'&&app('project-workspace')&&app('people-groups')},
    {key:'approval-audit',label:'批复审计',group:'document',groupStart:true,ready:app('approval-audit-results'),native:app('approval-audit-results')},
    ...(types.length ? [{key:'scheme',label:'方案阶段',group:'document',ready:true,native:nativeStage}] : []),
    ...(has('design') ? [
      {key:'preliminary',label:'初步设计',group:'document',ready:true,native:nativeStage},
      {key:'construction',label:'施工图设计',group:'document',ready:true,native:nativeStage},
      {key:'change-completion',label:'变更竣工',group:'document',ready:true,native:nativeStage},
    ] : []),
    ...(has('survey') ? [{key:'test',label:'试验阶段',group:'document',ready:true,native:nativeStage}] : []),
    ...(has('consultation') || has('survey') || has('mapping') ? [{key:'consultation',label:'咨询报告',group:'document',ready:true,native:nativeStage}] : []),
    ...(app('cost-outsourcing') ? [
      {key:'cost',label:'生产成本管理',group:'finance',groupStart:true,ready:true,native:true},
      {key:'commission',label:'对外委托',group:'finance',ready:true,native:true},
    ] : []),
    ...(app('bonus-distribution') ? [{key:'bonus',label:'奖金分配',group:'finance',ready:true,native:true}] : []),
  ];
}
export function productionTab(key, types = [], applications = [], personnelProfile = '',stageProfile='') {
  return productionTabs(types, applications, personnelProfile,stageProfile).some(t => t.key === key && t.ready) ? key : 'overview';
}
