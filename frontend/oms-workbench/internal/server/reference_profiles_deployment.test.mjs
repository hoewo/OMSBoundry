// Exercise the actual D1 profile-selection guard only; no Docker/deployment.
import assert from 'node:assert/strict';
import {readFileSync,mkdtempSync,writeFileSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {spawnSync} from 'node:child_process';
import test from 'node:test';

const source=readFileSync(new URL('../../../../deployments/d1/ops/d1',import.meta.url),'utf8');
const start=source.indexOf('validate_env() {\n');
const end=source.indexOf('  for value in ',start);
assert(start>=0&&end>start);
const guard=source.slice(start,end)+'}\nvalidate_env\n';
const cases=[
 ['default off',{},true],
 ['legacy v2 without professional selection',{bonus:'reference-oa-bonus-v2'},true],
 ['native personnel selection',{personnel:'reference-oa-production-personnel-v1'},true],
 ['native v3 matching selection',{bonus:'reference-oa-bonus-v3',personnel:'reference-oa-production-personnel-v1'},true],
 ['unknown Bonus',{bonus:'reference-oa-bonus-v4'},false],
 ['unknown personnel',{personnel:'reference-oa-production-personnel-v2'},false],
 ['v3 without native personnel',{bonus:'reference-oa-bonus-v3'},false],
 ['v3 without selected Bonus',{bonus:'reference-oa-bonus-v3',personnel:'reference-oa-production-personnel-v1',enableBonus:'false'},false],
 ['personnel without Project',{personnel:'reference-oa-production-personnel-v1',enableProject:'false'},false],
 ['personnel without People',{personnel:'reference-oa-production-personnel-v1',enablePeople:'false'},false],
 ['shared scheme exact profile',{stage:'reference-oa-stage-materials-v1'},true],
 ['unknown stage profile',{stage:'reference-oa-stage-materials-v2'},false],
 ['stage without Production',{stage:'reference-oa-stage-materials-v1',enableProduction:'false'},false],
 ['stage without Evidence',{stage:'reference-oa-stage-materials-v1',enableEvidence:'false'},false],
 ['stage without Project',{stage:'reference-oa-stage-materials-v1',enableProject:'false'},false],
 ['stage without People',{stage:'reference-oa-stage-materials-v1',enablePeople:'false'},false],
];
for(const [name,v,expected] of cases)test('D1 explicit profile guard: '+name,()=>{
 const result=spawnSync('sh',['-c',guard],{encoding:'utf8',env:{PATH:process.env.PATH,reference_oa_bonus_profile:v.bonus||'',reference_oa_personnel_profile:v.personnel||'',reference_oa_stage_profile:v.stage||'',enable_project:v.enableProject||'true',enable_people:v.enablePeople||'true',enable_production:v.enableProduction||'true',enable_evidence:v.enableEvidence||'true',enable_bonus_distribution:v.enableBonus||'true'}});
 assert.equal(result.status,expected?0:2,result.stderr);
});

const backupStart=source.indexOf('verify_backup_selection_value() {');
const backupEnd=source.indexOf('\nrestore_backup() {',backupStart);
assert(backupStart>=0&&backupEnd>backupStart);
const backupGuards=source.slice(backupStart,backupEnd);
const flags=['CONTRACT','MONEY','PEOPLE','PARTY','PROJECT','EVIDENCE','BUSINESS_ENGAGEMENT','BILLING','PRODUCTION','APPROVAL_AUDIT_RESULTS','COST_OUTSOURCING','BONUS_DISTRIBUTION'];
const selection=flags.map(f=>'ENABLE_'+f+'=true').join('\n')+'\n';
const profileLines=(bonus='',personnel='',stage='')=>'REFERENCE_OA_BONUS_PROFILE='+bonus+'\nREFERENCE_OA_PRODUCTION_PERSONNEL_PROFILE='+personnel+'\nREFERENCE_OA_PRODUCTION_STAGE_PROFILE='+stage+'\n';
const native='reference-oa-production-personnel-v1';
const backupCases=[
 ['legacy selection with profiles off','',{},0],
 ['legacy selection cannot infer v2','',{bonus:'reference-oa-bonus-v2'},1],
 ['legacy selection cannot infer personnel','',{personnel:native},1],
 ['declared empty remains off',profileLines(),{},0],
 ['v2 exact',profileLines('reference-oa-bonus-v2'),{bonus:'reference-oa-bonus-v2'},0],
 ['v3 exact',profileLines('reference-oa-bonus-v3',native),{bonus:'reference-oa-bonus-v3',personnel:native},0],
 ['v2 cannot restore as v3',profileLines('reference-oa-bonus-v2',native),{bonus:'reference-oa-bonus-v3',personnel:native},1],
 ['cannot silently disable native personnel',profileLines('',native),{},1],
 ['unknown recorded Bonus rejected',profileLines('unknown'),{},1],
 ['duplicate empty declaration rejected',profileLines()+'REFERENCE_OA_BONUS_PROFILE=\n',{},1],
 ['duplicate nonempty declaration rejected',profileLines('reference-oa-bonus-v2')+'REFERENCE_OA_BONUS_PROFILE=reference-oa-bonus-v2\n',{bonus:'reference-oa-bonus-v2'},1],
 ['missing selection off remains compatible',null,{},0],
 ['missing selection native rejected',null,{personnel:native},1],
 ['stage exact',profileLines('','','reference-oa-stage-materials-v1'),{stage:'reference-oa-stage-materials-v1'},0],
 ['cannot infer stage from legacy selection','',{stage:'reference-oa-stage-materials-v1'},1],
 ['stage cannot silently switch off',profileLines('','','reference-oa-stage-materials-v1'),{},1],
 ['off cannot silently switch stage on',profileLines(),{stage:'reference-oa-stage-materials-v1'},1],
 ['unknown recorded stage rejected',profileLines('','','unknown'),{},1],
 ['duplicate stage rejected',profileLines()+'REFERENCE_OA_PRODUCTION_STAGE_PROFILE=\n',{},1],
 ['missing selection stage rejected',null,{stage:'reference-oa-stage-materials-v1'},1],
];
for(const [name,fields,v,expected] of backupCases)test('D1 pre-destructive restore profile guard: '+name,()=>{
 const directory=mkdtempSync(join(tmpdir(),'oms-profile-guard-'));
 try{
  const path=join(directory,'suite-selection.env');if(fields!==null)writeFileSync(path,selection+fields);
  const env={PATH:process.env.PATH,reference_oa_bonus_profile:v.bonus||'',reference_oa_personnel_profile:v.personnel||'',reference_oa_stage_profile:v.stage||''};for(const f of flags)env['enable_'+f.toLowerCase()]='true';
  const result=spawnSync('sh',['-c',backupGuards+'\nverify_backup_selection "$1"','guard',path],{encoding:'utf8',env});assert.equal(result.status,expected,result.stderr);
 }finally{rmSync(directory,{recursive:true})}
});

test('D1 profile restore checks precede any volume replacement',()=>{
 const restore=source.slice(backupEnd,source.indexOf('\nrun_party_map()',backupEnd)>backupEnd?source.indexOf('\nrun_party_map()',backupEnd):source.length);
 const guardAt=restore.indexOf('verify_backup_selection "$target/suite-selection.env"'),removeAt=restore.indexOf('down --volumes');assert(guardAt>=0&&removeAt>guardAt);
 assert(source.includes('backup_bonus_profile=$(compose_all exec -T bonus-distribution'));
 assert(source.includes('backup_personnel_profile=$(compose_all exec -T project-workspace'));
 assert(source.includes('backup_stage_profile=$(compose_all exec -T production-operations'));
});
