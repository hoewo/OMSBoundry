/* Small browser-only architecture explanation. Not a server audit or telemetry bus. */
(function(global){
  'use strict';
  const uuid=/^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
  const keys=new Set(['flow','nginx','workbench','gateway','iam','project','business-engagement','production-operations','cost-outsourcing','bonus-distribution','bidding-command']);
  const defaults=[
    {key:'nginx',title:'Nginx：公网 HTTPS 入口',copy:'边缘转发与 TLS，不拥有业务事实。',state:'waiting'},
    {key:'workbench',title:'Workbench：组织界面与操作',copy:'组合版本化入口，不保存业务事实。',state:'waiting'},
    {key:'gateway',title:'Gateway／IAM：入口和可信能力',copy:'Gateway 清洗外来身份；IAM 签发当前账号的有界能力。',state:'waiting'},
    {key:'project',title:'目标小系统：最终授权、自有事实',copy:'Project 拥有项目；Business Engagement 拥有承接和材料引用；Evidence 拥有文件版本。下面仅解释已观察到的调用，不是服务器审计。',state:'waiting'}
  ];
  function valid(step){return !!step&&keys.has(step.key)&&typeof step.title==='string'&&step.title.length>0&&step.title.length<=100&&typeof step.copy==='string'&&step.copy.length<=400&&['waiting','complete','error'].includes(step.state);}
  function create({tenant,subject,enabled,storage}={}) {
    const active=enabled===true&&uuid.test(tenant||'')&&uuid.test(subject||'');
    const storageKey=active?'omsbizfoundry-architecture-lens-v2:'+tenant+':'+subject:null;
    let saved=[],epoch=0;
    if(active)try{storage=storage||global.sessionStorage;const payload=JSON.parse(storage?.getItem(storageKey)||'null');if(payload?.version===2&&Array.isArray(payload.steps)&&payload.steps.length<=keys.size&&payload.steps.every(valid)&&new Set(payload.steps.map(step=>step.key)).size===payload.steps.length)saved=payload.steps;}catch{}
    return {
      steps(){return (saved.length?saved:defaults).map(step=>({...step}));},
      epoch(){return epoch;},
      record(key,title,copy,state='complete',observedEpoch) {
        if(!active||!valid({key,title,copy,state})||(observedEpoch!==undefined&&observedEpoch!==epoch))return;
        if(key==='flow'&&state==='waiting'){saved=[];epoch++;}
        saved=saved.filter(step=>step.key!==key);saved.push({key,title,copy,state});
        try{storage?.setItem(storageKey,JSON.stringify({version:2,steps:saved}));}catch{}
      }
    };
  }
  global.ReferenceOAArchitectureLens={create};
})(globalThis);
