import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./oms_phase1.js', import.meta.url), 'utf8');

const start = source.indexOf("root.querySelector('[data-oms-v1-new-project]')");
const end = source.indexOf("for(const button of root.querySelectorAll('[data-oms-v1-new-party]')", start);
assert.ok(start >= 0 && end > start, 'new-project flow must be present');
const projectFlow = source.slice(start, end);

assert.match(projectFlow, /canCreateProject=state\.projectActions\.has\('project-workspace\.create'\)/, 'Project Workspace permission must independently control project creation');
assert.match(projectFlow, /enabled=canCreateProject&&externalParties\.length>0/, 'Contract readiness must not disable the Project command');
assert.doesNotMatch(projectFlow, /enabled=.*contractAssemblyReady/, 'deferred Contract assembly must not become a project gate');
assert.match(projectFlow, /if\(!contractAssemblyReady\)\{closeDialog\(\);await loadAll\(\)/, 'a project-only success path must stop before Contract commands');
assert.match(projectFlow, /甲方合同和应收金额可在合同功能就绪后从项目“合同”页补录/, 'project-only completion must explain the deferred facts');
assert.match(projectFlow, /contractAssemblyReady\?'<label>应收总额/, 'the amount field must only be collected when Contract can own it');
assert.match(projectFlow, /本次可先保存项目，甲方合同和应收金额稍后在项目“合同”页补录/, 'the modal must explain the bounded fallback before submission');

assert.match(source, /function openPrimaryContractCreate\(project\)/, 'deferred upstream Contract must have a later completion action');
assert.match(source, /relation_key:'primary-contract',note:'OMS 一期甲方合同补录'/, 'later completion must link the Contract through Project Workspace');
assert.match(source, /primaryAction\.textContent='补录甲方合同'/, 'a project without a Contract must expose the completion action in its Contract tab');
assert.match(source, /function primaryContractCreationState\(project\)/, 'later Contract creation must retain its own readiness checks');

console.log('oms_phase1_project_creation.test.mjs: 11 assertions passed');
