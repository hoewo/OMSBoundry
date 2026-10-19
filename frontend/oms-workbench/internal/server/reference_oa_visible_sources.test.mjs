import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const source=readFileSync(new URL('./server.go',import.meta.url),'utf8');
const helpers=source.slice(source.indexOf('async function runSources('),source.indexOf('if(workRoot){'));
function setup(){let visible=false,hidden=false;const listeners=[];
 const context=vm.createContext({document:{addEventListener:(name,fn)=>{assert.equal(name,'reference-oa-view-shown');listeners.push(fn)}}});vm.runInContext(helpers,context);
 const root={closest:selector=>selector==='[data-view]'?{classList:{contains:()=>visible}}:hidden?{}:null};
 return {context,root,setVisible:value=>visible=value,setHidden:value=>hidden=value,listeners};
}
test('hidden projections issue no requests; view entry and reentry read with fresh authorization',async()=>{
 const f=setup(),calls=[];const run=f.context.loadVisibleSources(f.root,['one','two'],async x=>calls.push(x));
 assert.deepEqual(calls,[]);await run();assert.deepEqual(calls,[]);
 f.setVisible(true);await run();assert.deepEqual(calls,['one','two']);
 f.setHidden(true);await run();assert.equal(calls.length,2);
 f.setHidden(false);await run();assert.equal(calls.length,4);assert.equal(f.listeners.length,1);
 assert(source.includes('loadVisibleSources(workRoot,sources,loadWork)'));assert(source.includes('loadVisibleSources(metricRoot,sources,loadMetrics)'));
 assert(!source.includes('\nrunSources(sources,loadWork)'));assert(!source.includes('\nrunSources(sources,loadMetrics)'));
 const authorize=source.slice(source.indexOf('async function authorize(source,'),source.indexOf('async function runSources('));assert(authorize.includes('await referenceOACSRF.token()'));
});
test('concurrent view notifications share the running read batch, not new credentials',async()=>{
 const f=setup();f.setVisible(true);let resolve,calls=0;const blocked=new Promise(r=>resolve=r);
 const run=f.context.loadVisibleSources(f.root,['one'],async()=>{calls++;await blocked});const first=run();await run();await f.listeners[0]();assert.equal(calls,1);resolve();await first;await run();assert.equal(calls,2);
});
test('a failed read releases the visibility guard for an explicit next entry',async()=>{
 const f=setup();f.setVisible(true);let fail=true,calls=0;const run=f.context.loadVisibleSources(f.root,['one'],async()=>{calls++;if(fail)throw new Error('unavailable')});await assert.rejects(run(),/unavailable/);fail=false;await run();assert.equal(calls,2);
});
