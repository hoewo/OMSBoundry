// Native UI port shared by the private preview and explicitly selected normal
// Workbench. Main grants, matching release and public acceptance remain separate.
import {createWorkspace as mount} from './production-personnel-ui'
import {createNativePersonnelPort,personnelPendingStore} from './production-personnel-native.mjs'
import type {ProductionPersonnelUIPort} from './production-personnel-ports'
export {createNativePersonnelPort,personnelPendingStore}
export interface NativePersonnelOptions {
 projectID:string
 request:(path:string,method?:string,body?:unknown)=>Promise<any>
 candidates:(projectID:string)=>Promise<any[]>
 pendingStore:{load:()=>any;save:(value:any)=>void;clear:()=>void}
 enroll?:(projectRevision:number)=>Promise<void>
 commandID?:()=>ReturnType<Crypto['randomUUID']>
}
export const createWorkspace=(options:{root:Element;native:NativePersonnelOptions})=>{
 const port=createNativePersonnelPort(options.native)
 const recovery=document.createElement('div'),button=document.createElement('button'),status=document.createElement('span')
 button.type='button';button.textContent='继续上一笔未明确结果的保存'
 recovery.style.cssText='padding:12px;margin-bottom:12px;border:1px solid #e6a23c;background:#fffbe6'
 recovery.append(status,button);options.root.before(recovery)
 const update=()=>{try{const p=port.pendingOperation();recovery.hidden=!p;status.textContent=p?'请复核原意图；仅重试原命令，不重复登记。 ':''}catch(e){recovery.hidden=false;status.textContent=(e as Error).message;button.disabled=true}}
 const workspace=mount({root:options.root,port:port as ProductionPersonnelUIPort})
 button.onclick=async()=>{button.disabled=true;try{await port.retryPending();workspace.refresh()}catch(e){status.textContent=(e as Error).message}finally{button.disabled=false;update()}}
 const timer=setInterval(update,250);update()
 return {...workspace,refresh(){workspace.refresh();update()},unmount(){clearInterval(timer);recovery.remove();workspace.unmount()}}
}
