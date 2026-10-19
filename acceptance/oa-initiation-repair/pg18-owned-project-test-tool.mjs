#!/usr/bin/env node
// PG18 host-tool adapter for the one task-owned container and random Go test DBs.
// This is not a production backup tool. It refuses any Main URL or broad DB.
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
import {readFileSync,writeFileSync} from 'node:fs';
import {basename,resolve} from 'node:path';
const tool=basename(process.argv[1]);assert(['pg_dump','pg_restore'].includes(tool));
const container='oms-bonus-overlay-test-20261004';
assert(/^[0-9a-f]{64}$/.test(process.env.TEST_PG18_CONTAINER||''));
assert.equal(execFileSync('docker',['inspect','--format','{{.Id}}',container],{encoding:'utf8'}).trim(),process.env.TEST_PG18_CONTAINER);
assert.equal(execFileSync('docker',['inspect','--format','{{index .Config.Labels "omsbizfoundry.task"}}',container],{encoding:'utf8'}).trim(),'oa-bonus-20261004');
const args=process.argv.slice(2);if(args.length===1&&args[0]==='--version'){process.stdout.write(execFileSync('docker',['exec',container,tool,'--version']));process.exit(0)}
let database,file;const flags=[];
for(const arg of args){if(arg.startsWith('--dbname=')){assert(!database);const u=new URL(arg.slice(9));assert.equal(u.hostname,'127.0.0.1');assert.equal(u.port,'55477');assert.equal(u.username,'postgres');assert.equal(u.password,'');database=u.pathname.slice(1);assert(/^project_workspace_test_[0-9a-f]+$/.test(database))}else if(arg.startsWith('--file=')){assert(tool==='pg_dump'&&!file);file=arg.slice(7)}else if(!arg.startsWith('-')){assert(tool==='pg_restore'&&!file);file=arg}else{assert(['--format=custom','--no-owner','--no-acl','--exit-on-error'].includes(arg));flags.push(arg)}}
assert(database&&file);const path=resolve(file);assert(path.endsWith('.dump')&&/\/Test[A-Za-z0-9_]+[0-9]+\/[0-9]+\/[^/]+\.dump$/.test(path),'dump must belong to the Go test temporary directory');
const command=['exec',...(tool==='pg_restore'?['-i']:[]),container,tool,...flags,'-U','postgres','-d',database];
const result=execFileSync('docker',command,{...(tool==='pg_restore'?{input:readFileSync(path)}:{}),maxBuffer:64*1024*1024});
if(tool==='pg_dump')writeFileSync(path,result,{mode:0o600});else process.stdout.write(result);
