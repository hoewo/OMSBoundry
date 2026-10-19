import {useState} from 'react'
import {Alert,Button,DatePicker,Modal,Space,Typography} from 'antd'
import {useQuery} from '@tanstack/react-query'
import dayjs from 'dayjs'
import {contractForID} from './services/registry'
import {formatMoney} from '@/utils/money'

export function ContractAmendmentConfirmation({contractId,amendmentId}:{contractId:string;amendmentId:string}){
  const adapter=contractForID(contractId)
  const {data:value}=useQuery({queryKey:['contractNative',contractId],queryFn:async()=>adapter.native.get(contractId)})
  const [date,setDate]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false)
  if(!value)return null
  const confirmed=(value.reference_oa_confirmations||[]).find((c:any)=>c.amendment_id===amendmentId)
  if(confirmed)return <Typography.Paragraph type="success">已确认：正式版本 {confirmed.contract_revision}，生效日期 {confirmed.effective_on}，有效金额 {formatMoney(confirmed.effective_amount)}。原协议只读；后续更正须新增协议。</Typography.Paragraph>
  if(!value.allowed_operations?.includes('reference-oa.amendment.confirm'))return <Typography.Text type="secondary">未正式确认的协议意图</Typography.Text>
  const row=value.reference_oa_draft.amendments.find((a:any)=>a.id===amendmentId)
  const disabled=busy||adapter.working||!!adapter.pending
  const confirm=()=>Modal.confirm({title:'确认这份补充协议生效',content:<><p>仅确认 {row.amendment_number}，须按原日期／编号顺序确认第一份未确认协议。生效日期由本次输入，不自动套用签订日期。</p><p>将追加原生合同版本，冻结该协议意图；不是结算或付款，也不确认其他协议。</p></>,okText:'确认协议生效',cancelText:'取消',onOk:async()=>{setError('');setBusy(true);try{await adapter.confirmAmendment(contractId,amendmentId,date)}catch(e){setError(e instanceof Error?e.message:'协议确认未完成')}finally{setBusy(false)}}})
  return <div style={{marginTop:12,marginBottom:12}}>{error?<Alert type="error" showIcon message={error} style={{marginBottom:8}}/>:null}<Space wrap><DatePicker aria-label={'协议生效日期 '+row.amendment_number} disabled={disabled} value={date?dayjs(date):null} onChange={d=>setDate(d?.format('YYYY-MM-DD')||'')}/><Button disabled={disabled||!date} onClick={confirm}>确认协议生效</Button><Typography.Text type="secondary">未确认；当前原生版本 {value.contract.current_revision}</Typography.Text></Space></div>
}
