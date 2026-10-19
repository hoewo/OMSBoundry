// Original OA form/grid/styles. Billing ports replace shared FinancialRecord CRUD.
import {useEffect,useState} from 'react'
import {Alert,Form,Input,InputNumber,DatePicker,Button,Space,message,Row,Col,Select} from 'antd'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {businessService} from '@/services/business'
import {canManageInvoice} from '@/services/permission'
import {invoiceForProject} from '@/services/registry'
import {InvoiceMaterials} from '../../../src/invoice-materials'
import type {BusinessChainType,CreateInvoiceRequest} from '@/types'
import {cents} from '../../../src/contract-money.mjs'
import {minorInteger} from '../../../src/invoice-adapter.mjs'
import dayjs from 'dayjs'
import {getBusinessChainLabel} from '@/utils/businessChain'
import {getErrorMessage} from '@/utils/error'
import './BusinessTransactionForms.css'
const fees=[{label:'设计费',value:'design_fee'},{label:'勘察费',value:'survey_fee'},{label:'咨询费',value:'consultation_fee'}]
interface Props {projectId:string;recordId?:string;chainType?:BusinessChainType;onSuccess?:()=>void;onCancel?:()=>void;onPreviewStateChange?:(value:boolean)=>void}
export function OurInvoiceForm({projectId,recordId,chainType,onSuccess,onCancel}:Props){
 const [form]=Form.useForm(),query=useQueryClient(),adapter=invoiceForProject(projectId),[history,setHistory]=useState<any[]|null>(null),[historyError,setHistoryError]=useState('')
 const {data:canManage}=useQuery({queryKey:['canManageInvoice',projectId],queryFn:()=>canManageInvoice(projectId)})
 const {data:record,isLoading,isError,error}=useQuery({queryKey:['invoice',projectId,recordId],queryFn:()=>businessService.getInvoice(projectId,recordId!),enabled:!!recordId})
 useEffect(()=>{if(record)form.setFieldsValue({...record,occurred_at:dayjs(record.occurred_at),reason:''})},[record,form])
 const save=useMutation({mutationFn:(values:CreateInvoiceRequest & {reason?:string})=>recordId?businessService.correctInvoice(projectId,recordId,values,values.reason||''):businessService.createInvoice(projectId,values)})
 const disabled=!!adapter.pending||adapter.working||(recordId?!record?.can_correct:!canManage)
 const submit=async(values:any)=>{try{if(disabled)throw new Error('当前不能登记／纠正开票');if(!values.occurred_at)throw new Error('请选择开票日期');await save.mutateAsync({...values,chain_type:record?.chain_type||chainType||values.chain_type,fee_type:record?.fee_type||values.fee_type,occurred_at:values.occurred_at.format('YYYY-MM-DD')});message.success('Billing 登记及当前项目引用已回读确认');void query.invalidateQueries({queryKey:['projectInvoices',projectId]});onSuccess?.()}catch(e){message.error(getErrorMessage(e,'开票操作尚未确认，请继续原操作'))}}
 const parties=(adapter.parties||[]).map((p:any)=>({value:p.id,label:p.legal_name+' · r'+p.revision}))
 const options=(snapshot:any)=>snapshot&&!parties.some((p:any)=>p.value===snapshot.id)?[{value:snapshot.id,label:snapshot.legal_name+'（发生时快照）'},...parties]:parties
 if(isError)return <Alert type="error" showIcon message={getErrorMessage(error,'开票详情未完整读取')} />
 if(isLoading)return <p>正在读取开票详情…</p>
 return <Form form={form} layout="vertical" onFinish={submit} initialValues={{chain_type:chainType}} onKeyDown={event=>{if(event.key==='Enter'&&(event.target as HTMLElement).tagName==='INPUT')event.preventDefault()}}>
  <Alert type="info" showIcon style={{marginBottom:16}} message={recordId?'纠正追加新 revision，不覆盖历史；登记无效不是税务作废。':'只登记已开具票据；明确选择 Party 当前主体，不自动认定甲方身份。'} />
  {!chainType&&<Form.Item label="往来对象" name="chain_type" rules={[{required:true,message:'请选择往来对象'}]}><Select disabled={disabled||!!recordId} options={(['client_to_qualification','qualification_to_our_company'] as const).map(value=>({value,label:getBusinessChainLabel(value)}))} /></Form.Item>}
  <Row gutter={16}><Col span={12}><Form.Item label="开票方" name="issuer_party_id" rules={[{required:true,message:'请选择开票方'}]}><Select disabled={disabled} showSearch optionFilterProp="label" placeholder="选择明确开票方" options={options(record?.issuer)} /></Form.Item></Col><Col span={12}><Form.Item label="开票时间" name="occurred_at" rules={[{required:true,message:'请选择开票时间'}]}><DatePicker disabled={disabled} style={{width:'100%'}} format="YYYY-MM-DD" /></Form.Item></Col></Row>
  <Row gutter={16}><Col span={12}><Form.Item label="受票方" name="recipient_party_id" rules={[{required:true,message:'请选择受票方'}]}><Select disabled={disabled} showSearch optionFilterProp="label" placeholder="选择明确受票方" options={options(record?.recipient)} /></Form.Item></Col><Col span={12}><Form.Item label="费用类型" name="fee_type" rules={[{required:true,message:'请选择费用类型'}]}><Select disabled={disabled||!!recordId} options={fees} placeholder="请选择费用类型" /></Form.Item></Col></Row>
  <Row gutter={16}><Col span={12}>{record&&!record.amount_visible?<Form.Item label="开票金额"><div className="business-transaction-form__context-field">无金额读取权限</div></Form.Item>:<Form.Item label="开票金额" name="amount" rules={[{required:true,message:'请输入开票金额'},{validator:async(_,v)=>{minorInteger(cents(v).toString())}}]}><InputNumber<string> stringMode style={{width:'100%'}} prefix="¥" disabled={disabled} placeholder="请输入开票金额" /></Form.Item>}</Col><Col span={12}><Form.Item label="关联支付记录" tooltip="关联尚未接通，不把开票登记当实际付款"><Select aria-label="关联支付记录" disabled placeholder="尚未接通（不能保存关联）" /></Form.Item></Col></Row>
  <Form.Item name="description" label="备注"><Input.TextArea rows={3} disabled={disabled} placeholder="请输入备注信息（可选）" maxLength={2000} /></Form.Item>
  {recordId&&record?.can_correct&&<Form.Item label="纠正原因" name="reason" rules={[{required:true,whitespace:true,message:'请填写纠正原因'}]}><Input.TextArea rows={2} disabled={disabled} maxLength={1000} /></Form.Item>}
  {recordId?<InvoiceMaterials projectId={projectId} invoiceId={recordId} />:<Alert type="info" style={{marginBottom:16}} message="本候选先登记结构化票据，再在详情上传并登记文件引用；文件与登记分别确认。" />}
  {recordId&&<Space style={{marginBottom:16}}><span>Billing revision {record?.revision} · 关联时 r{record?.link_source_revision}</span>{adapter.native.get(recordId)?.allowed_actions.includes('history_read')&&<Button onClick={()=>void adapter.history(recordId).then((v:any[])=>{setHistory(v);setHistoryError('')}).catch((e:Error)=>setHistoryError(e.message))}>查看登记历史</Button>}</Space>}
  {historyError&&<Alert type="error" message={historyError} />}
  {history&&<div>{history.map(r=><p key={r.revision}>r{r.revision} · {r.event_kind} · {r.reason||'首次登记'} · {r.snapshot.amount_visible?('CNY '+(BigInt(r.snapshot.amount_minor_decimal)/100n).toString()+'.'+(BigInt(r.snapshot.amount_minor_decimal)%100n).toString().padStart(2,'0')):'无金额读取权限'}</p>)}</div>}
  {record?.status==='registration_invalidated'&&<Alert type="warning" style={{marginBottom:16}} message="该登记已无效，原金额／历史保留；不能修改或新增文件。" />}
  <Form.Item><Space>{(!recordId||record?.can_correct)&&<Button type="primary" htmlType="submit" disabled={disabled} loading={save.isPending}>{recordId?'追加纠正':'提交'}</Button>}{onCancel&&<Button onClick={onCancel}>{recordId?'关闭':'取消'}</Button>}</Space></Form.Item>
 </Form>
}
