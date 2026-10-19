import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const source=readFileSync(new URL('../internal/httpserver/demo_overview.go',import.meta.url),'utf8');
const dialogs=[...source.matchAll(/<dialog id="(\w+)">([\s\S]*?)<\/dialog>/g)];
assert.equal(dialogs.length,4);
for(const [_,id,html] of dialogs){
 assert.match(html,/<button type="button" data-cancel-dialog class="quiet">取消<\/button>/,id);
 assert.match(html,/required/,id+' preserves required validation on submit');
 assert.doesNotMatch(html,/<button value="cancel"/);
}
const handler=source.split('\n').find(line=>line.startsWith("$$('dialog button[data-cancel-dialog]')"));
assert(handler);
const closed=[];
const buttons=dialogs.map(([_,id])=>({closest(selector){assert.equal(selector,'dialog');return{close(){closed.push(id)}}}}));
const context=vm.createContext({$$:selector=>{assert.equal(selector,'dialog button[data-cancel-dialog]');return buttons}});
vm.runInContext(handler,context);
for(const button of buttons)button.onclick();
assert.deepEqual(closed,dialogs.map(([_,id])=>id));
console.log('PASS four non-submit cancel buttons: own dialog closes without validation, API, credential save or business command; required submission fields retained');
