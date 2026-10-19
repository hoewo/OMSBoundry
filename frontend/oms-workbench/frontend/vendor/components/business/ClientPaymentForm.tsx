// Original OA form layout retained; Money owns immutable actual cash.
import {useEffect} from 'react'
import {Alert,Form,Input,InputNumber,DatePicker,Button,Space,message,Row,Col,Select} from 'antd'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {businessService} from '@/services/business'
import {canManageCash} from '@/services/permission'
import {cashForProject} from '@/services/registry'
import type {BusinessChainType,CreateCashPaymentRequest} from '@/types'
import dayjs from 'dayjs'
import {getBusinessChainLabel} from '@/utils/businessChain'
import {getErrorMessage} from '@/utils/error'
import {cents} from '../../../src/contract-money.mjs'
import './BusinessTransactionForms.css'
const {TextArea}=Input
interface ClientPaymentFormProps {projectId:string;recordId?:string;chainType?:BusinessChainType;onSuccess?:()=>void;onCancel?:()=>void}
const fees=[{label:'设计费',value:'design_fee'},{label:'勘察费',value:'survey_fee'},{label:'咨询费',value:'consultation_fee'}]
const chains=['client_to_qualification','qualification_to_our_company'] as const
export const ClientPaymentForm=({projectId,recordId,chainType,onSuccess,onCancel}:ClientPaymentFormProps)=>{
  const [form]=Form.useForm(),query=useQueryClient(),adapter=cashForProject(projectId)
  const {data:canManage}=useQuery({queryKey:['canManageCash',projectId],queryFn:()=>canManageCash(projectId)})
  const {data:record,isLoading,isError,error}=useQuery({queryKey:['cashPayment',projectId,recordId],queryFn:()=>businessService.getCashPayment(projectId,recordId!),enabled:!!recordId})
  useEffect(()=>{if(record)form.setFieldsValue({...record,occurred_at:dayjs(record.occurred_at)})},[record,form])
  const create=useMutation({mutationFn:(data:CreateCashPaymentRequest)=>businessService.createCashPayment(projectId,data)})
  const disabled=!!recordId||!canManage||!!adapter.pending||adapter.working
  const submit=async(values:any)=>{try{if(recordId||disabled)throw new Error('当前不能新登记资金');if(!values.occurred_at)throw new Error('请选择实际支付日期');await create.mutateAsync({...values,chain_type:chainType||values.chain_type,occurred_at:values.occurred_at.format('YYYY-MM-DD')});message.success('实际收款及项目关联已回读确认');void query.invalidateQueries({queryKey:['projectCash',projectId]});onSuccess?.()}catch(e){message.error(getErrorMessage(e,'支付操作未全部确认，请继续原操作'))}}
  const parties=adapter.context?.parties.map((p:any)=>({value:p.id,label:p.legal_name+' · r'+p.revision}))||[]
  if(isError)return <Alert type="error" showIcon message={getErrorMessage(error,'资金详情未读取')} />
  if(isLoading)return <div>正在读取支付详情…</div>
  return <Form form={form} layout="vertical" onFinish={submit} initialValues={{chain_type:chainType}}>
    <Alert type="info" showIcon style={{marginBottom:16}} message={recordId?'已确认实际资金只读；纠错须明确冲销，保留原历史。':'实际收款登记：明确选择 Money 本目录双方主体；不会按甲方名称匹配身份。'} />
    {!chainType&&<Form.Item label="往来对象" name="chain_type" rules={[{required:true,message:'请选择往来对象'}]}><Select disabled={disabled} options={chains.map(value=>({value,label:getBusinessChainLabel(value)}))} /></Form.Item>}
    <Row gutter={16}><Col span={12}><Form.Item label="收款账簿主体" name="ledger_owner_party_id" rules={[{required:true,message:'请选择收款账簿主体'}]}><Select showSearch optionFilterProp="label" placeholder="请选择收款账簿主体" disabled={disabled} options={record?[{value:record.payee.id,label:record.payee.legal_name}]:parties} /></Form.Item></Col><Col span={12}><Form.Item label="支付时间" name="occurred_at" rules={[{required:true,message:'请选择支付时间'}]}><DatePicker style={{width:'100%'}} format="YYYY-MM-DD" disabled={disabled} /></Form.Item></Col></Row>
    <Row gutter={16}><Col span={12}><Form.Item label="付款主体" name="counterparty_party_id" rules={[{required:true,message:'请选择付款主体'}]}><Select showSearch optionFilterProp="label" placeholder="请选择付款主体" disabled={disabled} options={record?[{value:record.payer.id,label:record.payer.legal_name}]:parties} /></Form.Item></Col><Col span={12}><Form.Item label="实际收款账户" name="account_reference" rules={[{required:true,whitespace:true,message:'请填写实际账户'}]}><Input disabled={disabled} maxLength={200} /></Form.Item></Col></Row>
    <Row gutter={16}><Col span={12}><Form.Item label="费用类型" name="fee_type" rules={[{required:true,message:'请选择费用类型'}]}><Select placeholder="请选择费用类型" options={fees} disabled={disabled} /></Form.Item></Col><Col span={12}>{record&&!record.amount_visible?<Form.Item label="支付金额"><div className="business-transaction-form__context-field">无金额读取权限</div></Form.Item>:<Form.Item label="支付金额" name="amount" rules={[{required:true,message:'请输入支付金额'},{validator:async(_,v)=>{if(cents(v)<=0n)throw new Error('金额必须大于0且最多两位小数')}}]}><InputNumber<string> stringMode style={{width:'100%'}} prefix="¥" placeholder="请输入支付金额" disabled={disabled} /></Form.Item>}</Col></Row>
    <Form.Item label="原始资金依据" name="manual_source_reference" rules={[{required:true,whitespace:true,message:'请填写凭证号或原始依据'}]}><Input placeholder="银行凭证号／原始收款依据" disabled={disabled} maxLength={200} /></Form.Item>
    <Form.Item label="关联发票" tooltip="Billing 关联尚未接通，资金登记不会伪装成开票成功"><Select aria-label="关联发票" disabled placeholder="尚未接通（不能保存关联）" /></Form.Item>
    <Form.Item name="description" label="备注"><TextArea rows={3} placeholder="请输入备注信息（可选）" disabled={disabled} maxLength={2000} /></Form.Item>
    {record?<Alert style={{marginBottom:16}} type={record.status==='reversed'?'warning':'success'} message={record.status==='reversed'?'已冲销：'+(record.reversal?.reason||'原因未回读'):'已确认实际收款'} />:null}
    <Form.Item><Space>{!recordId&&<Button type="primary" htmlType="submit" loading={create.isPending} disabled={disabled}>提交</Button>}{onCancel&&<Button onClick={onCancel}>{recordId?'关闭':'取消'}</Button>}</Space></Form.Item>
  </Form>
}
