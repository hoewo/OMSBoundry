import {useEffect,useState} from 'react'
import {Alert,Button,Checkbox,DatePicker,Input,InputNumber,Modal,Select,Space,Typography} from 'antd'
import {useQuery} from '@tanstack/react-query'
import dayjs from 'dayjs'
import {contractForID} from './services/registry'
import {formatMoney} from '@/utils/money'

// This native command block lives inside the original finalization section.
// It does not convert editable previews or completed uploads into settlements.
export function ContractSettlement({contractId}:{contractId:string}){
 const adapter=contractForID(contractId),{data:v}=useQuery({queryKey:['contractNative',contractId],queryFn:async()=>adapter.native.get(contractId)})
 const [kind,setKind]=useState('final'),[number,setNumber]=useState(''),[title,setTitle]=useState(''),[amount,setAmount]=useState(''),[date,setDate]=useState(''),[basis,setBasis]=useState(''),[positions,setPositions]=useState<number[]>([]),[busy,setBusy]=useState(false),[error,setError]=useState('')
 useEffect(()=>{setAmount(v?.contract?.total_amount||'');setPositions([])},[v?.contract?.current_revision])
 if(!v)return null
 const rows=v.settlements||[],parties=(v.party_snapshots||[]).filter((p:any)=>p.revision===v.contract.current_revision),canManage=adapter.canRecordSettlement(contractId),disabled=!canManage||busy,final=rows.some((s:any)=>s.kind==='final')
 const confirm=()=>Modal.confirm({title:'确认正式合同结算',okText:'确认并冻结结算',cancelText:'取消',content:<><p>合同 {v.contract.contract_number} · 正式来源版本 {v.contract.current_revision}。</p><p>{kind==='final'?'最终结算':'阶段结算'} · 累计金额 {formatMoney(amount)} · 确认日期 {date}。</p><p>只确认本次结算，冻结所选主体及依据。不是付款或奖金发布；最终结算不能覆盖，后续更正能力尚未实现。</p></>,onOk:async()=>{setBusy(true);setError('');try{await adapter.recordSettlement(contractId,{kind,statement_number:number,title,amount,confirmed_on:date,basis,confirming_party_positions:positions})}catch(e){setError(e instanceof Error?e.message:'结算确认尚未完成')}finally{setBusy(false)}}})
 return <section data-native-contract-settlement data-selected-party-positions={JSON.stringify(positions)} data-source-revision={v.contract.current_revision} style={{marginTop:16}}>
  <Typography.Title level={5}>正式结算记录</Typography.Title>
  <Typography.Paragraph type="secondary">下列记录由 Contract 冻结。上方金额是可编辑意图／预览；附件登记不等于定案确认。</Typography.Paragraph>
  {rows.map((s:any)=><div key={s.id} data-settlement-id={s.id} className="contract-form__inline-note"><strong>{s.statement_number} · {s.kind==='final'?'最终结算':'阶段结算'} · {formatMoney(s.amount)}</strong><p>来源正式版本 {s.source_revision} · 结算序号 {s.sequence} · 确认日期 {s.confirmed_on}</p><p>{s.title} · {s.basis}</p><p>冻结确认主体：{(s.confirming_parties||[]).map((p:any)=>p.legal_name+' · '+p.role+'（版本 '+p.source_party_revision+'）').join('、')||'尚未完整读取'}</p></div>)}
  {!rows.length&&<Typography.Paragraph>尚未确认正式结算。</Typography.Paragraph>}
  {final?<Alert type="info" message="最终结算已冻结。后续更正不能覆盖历史，当前更正用例尚未实现。" />:!canManage?<Alert type="info" message="需正式生效合同及本合同原生结算权限；当前只读，不自动确认。" />:<Space direction="vertical" style={{width:'100%'}}>
   <Space wrap><Select aria-label="正式结算类型" value={kind} disabled={disabled} onChange={setKind} options={[{value:'progress',label:'阶段结算'},{value:'final',label:'最终结算'}]} style={{minWidth:160}}/><Input aria-label="结算文书编号" placeholder="结算文书编号" value={number} onChange={e=>setNumber(e.target.value)} disabled={disabled} maxLength={100}/><Input aria-label="正式结算标题" placeholder="正式结算标题" value={title} onChange={e=>setTitle(e.target.value)} disabled={disabled} maxLength={200}/></Space>
   <Space wrap><InputNumber stringMode aria-label="正式累计结算金额" min="0" precision={2} value={amount} onChange={n=>setAmount(n===null?'':String(n))} disabled={disabled} style={{width:240}}/><DatePicker aria-label="正式结算确认日期" value={date?dayjs(date):null} onChange={d=>setDate(d?.format('YYYY-MM-DD')||'')} disabled={disabled}/></Space>
   <Input.TextArea aria-label="正式结算依据" placeholder="确认依据或口径说明" rows={2} value={basis} onChange={e=>setBasis(e.target.value)} disabled={disabled} maxLength={1000}/>
   <div><Typography.Text>选择当前正式版本的确认主体：</Typography.Text><Checkbox.Group aria-label="结算确认主体" value={positions} onChange={v=>setPositions(v as number[])} disabled={disabled} options={parties.map((p:any)=>({value:p.position,label:p.legal_name+' · '+p.role}))}/></div>
   <Button type="primary" loading={busy} disabled={disabled||!number.trim()||!title.trim()||!amount||!date||!basis.trim()||!positions.length} onClick={confirm}>确认正式结算</Button>
  </Space>}
  {error&&<Alert type="error" message={error} style={{marginTop:12}}/>}
 </section>
}
