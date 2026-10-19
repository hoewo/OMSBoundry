import test from 'node:test'
import assert from 'node:assert/strict'
import {restorePermissionDraft} from './permission-model.mjs'
const catalog=[['read'],['write']],admin={key:'admin',area:'positions',title:'管理员',description:'',template:'管理员模板',scope:'全局生效',sort:0,active:true,system:true,keys:['read','write']},member={...admin,key:'member',title:'员工',sort:1,keys:['read']},base=[admin,member],options={catalog,base,tenantID:'tenant',subjectID:'actor'}
const raw=entries=>JSON.stringify({schema_version:'1',tenant_id:'tenant',subject_id:'actor',entries})
test('valid property changes and custom positions survive a same-account reload',()=>{const custom={...member,key:'custom',system:false,title:'新职位'};assert.equal(restorePermissionDraft(raw([admin,{...member,title:'新名称'},custom]),options)[1].title,'新名称')})
test('draft cannot remove system entries, weaken admin or introduce unknown actions',()=>{for(const entries of [[member],[{...admin,keys:['read']},member],[admin,{...member,keys:['unknown']}],[admin,member,member],[admin,{...member,system:false}]])assert.equal(restorePermissionDraft(raw(entries),options),base)})
test('tenant and subject isolation, malformed drafts, sort bounds are enforced',()=>{assert.equal(restorePermissionDraft(raw(base),{...options,subjectID:'another'}),base);assert.equal(restorePermissionDraft(raw(base),{...options,tenantID:'another'}),base);assert.equal(restorePermissionDraft('{',options),base);assert.equal(restorePermissionDraft(raw([admin,{...member,sort:null}]),options),base)})
