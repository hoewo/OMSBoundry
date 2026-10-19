import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import {productionTabs, productionTab} from './production-tabs.mjs';
const apps=['approval-audit-results','cost-outsourcing','bonus-distribution'];
test('original production tab order and grouping retained for all four OA types',()=>{
  const tabs=productionTabs(['design','survey','consultation','mapping'],apps);
  assert.deepEqual(tabs.map(t=>t.key),['overview','personnel','approval-audit','scheme','preliminary','construction','change-completion','test','consultation','cost','commission','bonus']);
  assert.deepEqual(tabs.filter(t=>t.groupStart).map(t=>t.key),['approval-audit','cost']);
});
test('survey includes both the original test and consultation tabs, no design tabs',()=>{
  const keys=productionTabs(['survey'],[]).map(t=>t.key);
  assert(keys.includes('test'));assert(keys.includes('consultation'));
  assert(!keys.includes('preliminary'));assert(!keys.includes('construction'));
});
test('original OA stage surfaces stay navigable while native ownership remains explicit',()=>{
  assert.deepEqual(productionTabs(['design'],apps).filter(t=>t.ready).map(t=>t.key),['overview','approval-audit','scheme','preliminary','construction','change-completion','cost','commission','bonus']);
  assert.deepEqual(productionTabs(['design'],[]).filter(t=>t.ready).map(t=>t.key),['overview','scheme','preliminary','construction','change-completion']);
  assert.equal(productionTab('bonus',['design'],apps),'bonus');
  assert.equal(productionTab('commission',['design'],apps),'commission');
  for(const key of ['scheme','preliminary','construction','change-completion'])assert.equal(productionTab(key,['design'],apps),key);
  assert.equal(productionTab('wrong',['design'],apps),'overview');
  assert.deepEqual(productionTabs(['design'],apps).filter(t=>t.native===false).map(t=>t.key),['personnel','scheme','preliminary','construction','change-completion']);
  assert.equal(productionTab('bonus',[],[]),'overview');
});
test('original shell imports its verbatim stylesheet and has no fabricated summary calculations',()=>{
  const source=readFileSync(new URL('./production-layout.tsx',import.meta.url),'utf8');
  assert(source.includes("../vendor/components/production/ProductionInfo.css"));
  assert(source.includes('project-workspace__summary--production'));
  assert(source.includes('className="project-workspace__tabs"'));
  assert(!source.includes('|| 0}'));assert(source.includes('未读取，不代表零'));
});
test('D01 remains unavailable until its exact profile and both independent owners are selected',()=>{
 const profile='reference-oa-production-personnel-v1',owners=['project-workspace','people-groups'];
 assert.equal(productionTab('personnel',['design'],owners),'overview');
 assert.equal(productionTab('personnel',['design'],owners,profile),'personnel');
 for(const available of [[],['project-workspace'],['people-groups']])assert.equal(productionTab('personnel',['design'],available,profile),'overview');
 assert.equal(productionTab('personnel',['design'],owners,'wrong'),'overview');
});
test('one shared scheme tab is navigable, but native only with exact profile and owners',()=>{
 const profile='reference-oa-stage-materials-v1',owners=['production-operations','project-workspace','evidence-service'];
 for(const types of [['design'],['consultation'],['survey'],['mapping'],['design','consultation','survey','mapping']]){
  assert.equal(productionTab('scheme',types,owners,'',profile),'scheme');const tab=productionTabs(types,owners,'',profile).find(t=>t.key==='scheme');assert(tab);assert.equal(tab.native,true);
 }
 for(const subset of [[],owners.slice(0,2),owners.slice(1)]){assert.equal(productionTab('scheme',['design'],subset,'',profile),'scheme');assert.equal(productionTabs(['design'],subset,'',profile).find(t=>t.key==='scheme').native,false)}
 assert.equal(productionTabs(['design'],owners).find(t=>t.key==='scheme').native,false);assert.equal(productionTab('scheme',[],owners,'',profile),'overview');
});
test('type-scoped test and report tabs become native only under the same exact stage profile',()=>{
 const profile='reference-oa-stage-materials-v1',owners=['production-operations','project-workspace','evidence-service'];
 const survey=productionTabs(['survey'],owners,'',profile);
 assert.equal(survey.find(t=>t.key==='test').native,true);assert.equal(survey.find(t=>t.key==='consultation').native,true);
 const consultation=productionTabs(['consultation'],owners,'',profile);assert.equal(consultation.find(t=>t.key==='consultation').native,true);
 assert.equal(productionTabs(['survey'],owners).find(t=>t.key==='test').native,false);
});
