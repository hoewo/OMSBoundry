import {useEffect,useState} from 'react'
import {Alert,Button,DatePicker,Input,Modal,Select,Space,Typography} from 'antd'
import {useQuery} from '@tanstack/react-query'
import dayjs from 'dayjs'
import {contractForID} from './services/registry'
import {formatMoney} from '@/utils/money'

// A small native-state block inside the reused Contract drawer. It does not
// own Party facts, silently activate a draft, or confirm amendment intentions.
export function ContractNativePreparation({contractId}:{contractId:string}){
  const adapter=contractForID(contractId)
  const {data:value}=useQuery({queryKey:['contractNative',contractId],queryFn:async()=>adapter.native.get(contractId)})
  const [error,setError]=useState(''),[busy,setBusy]=useState(false),[parties,setParties]=useState<any[]>([]),[party,setParty]=useState<string>(),[role,setRole]=useState(''),[date,setDate]=useState<string>('')
  useEffect(()=>{setDate(value?.contract?.effective_on||'')},[value?.contract?.effective_on])
  const run=async(fn:()=>Promise<unknown>)=>{setError('');setBusy(true);try{await fn()}catch(e){setError(e instanceof Error?e.message:'原生操作尚未确认')}finally{setBusy(false)}}
  if(!value)return null
  const c=value.contract,ops=value.allowed_operations||[],disabled=busy||adapter.working||!!adapter.pending,active=(value.parties||[]).filter((p:any)=>p.active)
  const mutable=c.status==='draft',canAdd=mutable&&ops.includes('draft.party.add'),canUpdate=mutable&&ops.includes('draft.update')
  const confirm=()=>Modal.confirm({title:'确认原始合同生效',content:<><p>仅确认原始合同 {c.contract_number}，金额 {formatMoney(c.total_amount)}。协议草稿和结算意图不会自动成为正式事实。</p><p>生效日期：{c.effective_on||'尚未填写'}；当前签约主体：{active.map((p:any)=>p.legal_name+'（'+p.role+'）').join('、')||'尚未选择'}。</p><p>确认后形成不可变 revision，不能按草稿覆盖。</p></>,okText:'确认生效',cancelText:'取消',onOk:()=>run(()=>adapter.prepareNative(contractId,'activate',{}))})
  return <section className="contract-form__block" data-native-contract-preparation>
    <header className="contract-form__block-header"><h3 className="contract-form__block-title">签约主体与原始合同确认</h3><p className="contract-form__block-description">当前 Contract 本地主体目录，尚不代表 Party 统一迁移完成。草稿保存与正式生效分开。</p></header>
    {error?<Alert type="error" showIcon message={error} style={{marginBottom:12}}/>:null}
    <Typography.Paragraph>状态：{mutable?'未确认草稿':c.status==='active'?'已生效':c.status}；正式版本：{c.current_revision||'尚未形成'}。</Typography.Paragraph>
    {mutable?<>
      <Space wrap style={{marginBottom:12}}><span>生效日期</span><DatePicker aria-label="原始合同生效日期" value={date?dayjs(date):null} disabled={disabled||!canUpdate} onChange={d=>setDate(d?.format('YYYY-MM-DD')||'')}/>{canUpdate?<Button disabled={disabled||!date} onClick={()=>void run(()=>adapter.prepareNative(contractId,'effective-date',{effective_on:date}))}>保存生效日期</Button>:null}</Space>
      {active.map((p:any)=><div key={p.id} style={{marginBottom:8}}><Space><span>{p.legal_name} · {p.role}</span>{ops.includes('draft.party.remove')?<Button danger size="small" disabled={disabled} onClick={()=>Modal.confirm({title:'移出此签约主体？',content:'仅结束合同中的草稿关联，保留主体目录与关联历史。',okText:'确认移出',cancelText:'取消',onOk:()=>run(()=>adapter.prepareNative(contractId,'party.remove',{party_link_id:p.id}))})}>移出签约主体</Button>:null}</Space></div>)}
      {canAdd?<Space wrap style={{marginBottom:12}}><Button disabled={disabled} onClick={()=>void run(async()=>setParties(await adapter.partyCandidates()))}>读取合同主体目录</Button><Select aria-label="选择合同签约主体" style={{minWidth:200}} value={party} disabled={disabled} options={parties.map(p=>({value:p.id,label:p.legal_name}))} onChange={setParty}/><Input aria-label="合同内主体角色" placeholder="合同中的业务角色" value={role} disabled={disabled} onChange={e=>setRole(e.target.value)} style={{width:180}}/><Button disabled={disabled||!party||!role.trim()} onClick={()=>void run(()=>adapter.prepareNative(contractId,'party.add',{party_id:party,role}))}>添加签约主体</Button></Space>:null}
      {ops.includes('contract.activate')?<div><Button type="primary" disabled={disabled||!c.effective_on||new Set(active.map((p:any)=>p.party_id)).size<2} onClick={confirm}>确认原始合同生效</Button><Typography.Text type="secondary"> 须有签订／生效日期和两个不同法律主体；服务端仍作最终校验。</Typography.Text></div>:null}
    </>:<>{(value.party_snapshots||[]).filter((p:any)=>p.revision===c.current_revision).map((p:any)=><Typography.Paragraph key={p.position}>{p.legal_name} · {p.role}（冻结主体版本 {p.source_party_revision}）</Typography.Paragraph>)}</>}
  </section>
}
