// Reused OA list/drawer composition. Confirmed Money facts cannot be edited/deleted.
import {useMemo,useState} from 'react'
import {Alert,Button,Card,Drawer,Empty,Input,Modal,message} from 'antd'
import {UndoOutlined,PayCircleOutlined,RightOutlined} from '@ant-design/icons'
import {useMutation,useQuery,useQueryClient} from '@tanstack/react-query'
import {businessService} from '@/services/business'
import {canManageCash} from '@/services/permission'
import {ClientPaymentForm} from './ClientPaymentForm'
import {SectionTitle} from '@/components/shared/SectionTitle'
import {CreateActionButton} from '@/components/shared/CreateActionButton'
import type {BusinessChainType,CashPaymentView} from '@/types'
import {getBusinessChainLabel} from '@/utils/businessChain'
import {formatMoney} from '@/utils/money'
import './BusinessTransactionsPanel.css'
interface PaymentRecordListProps {projectId:string;chainType?:BusinessChainType;title?:string;subtitle?:string;hideCard?:boolean;groupTitle?:string;groupDescription?:string;groupKicker?:string;groupTone?:'client'|'income'}
const getFeeTypeLabel=(fee?:string)=>({design_fee:'设计费',survey_fee:'勘察费',consultation_fee:'咨询费'}[fee||'']||fee||'-')
const getFeeTypeTone=(fee?:string)=>({design_fee:'design',survey_fee:'survey',consultation_fee:'consultation'}[fee||'']||'default')
export const PaymentRecordList=({projectId,chainType,title,subtitle,hideCard=false,groupTitle,groupDescription,groupKicker,groupTone='client'}:PaymentRecordListProps)=>{
  const [drawerOpen,setDrawerOpen]=useState(false),[editingRecord,setEditingRecord]=useState<CashPaymentView|null>(null),[reverseOpen,setReverseOpen]=useState(false),[reason,setReason]=useState(''),[reverseError,setReverseError]=useState('')
  const query=useQueryClient()
  const {data:canManage}=useQuery({queryKey:['canManageCash',projectId],queryFn:()=>canManageCash(projectId)})
  const {data:cash,isLoading,isError,error}=useQuery({queryKey:['projectCash',projectId],queryFn:()=>businessService.getProjectCash(projectId)})
  const records=useMemo(()=>cash?.payment_records.filter(v=>!chainType||v.chain_type===chainType)||[],[cash,chainType])
  const current=records.find(v=>v.id===editingRecord?.id)||editingRecord
  const reverse=useMutation({mutationFn:()=>businessService.reverseCashPayment(projectId,current!.id,reason.trim())})
  const close=()=>{setDrawerOpen(false);setEditingRecord(null)}
  const success=()=>{close();void query.invalidateQueries({queryKey:['projectCash',projectId]})}
  const confirmReverse=async()=>{setReverseError('');try{if(!reason.trim())throw new Error('请填写冲销原因');await reverse.mutateAsync();message.success('资金冲销及原历史已回读确认');setReverseOpen(false);setReason('');success()}catch(e){setReverseError(e instanceof Error?e.message:'冲销未确认，请继续原操作')}}
  const create=()=>{setEditingRecord(null);setDrawerOpen(true)}
  const listContent=<div className="transaction-panel__group-body">
    {isError?<Alert type="error" showIcon message={error instanceof Error?error.message:'资金读取未完成，不表示没有记录'} />:isLoading?<div className="transaction-panel__loading">加载中...</div>:!records.length?<div className="transaction-panel__empty"><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前还没有支付记录" /></div>:<div className="transaction-panel__list">{records.map(record=><button key={record.id} type="button" className={'transaction-panel__item transaction-panel__item--'+getFeeTypeTone(record.fee_type)} onClick={()=>{setEditingRecord(record);setDrawerOpen(true)}}>
      <div className="transaction-panel__item-primary"><div className="transaction-panel__fee-badge">{getFeeTypeLabel(record.fee_type)}</div><div className="transaction-panel__item-date"><span>{record.occurred_at}</span></div></div>
      <div className="transaction-panel__item-amount"><strong>{record.amount_visible?formatMoney(record.amount!):'无金额权限'}</strong><span>{record.status==='reversed'?'已冲销（原金额保留）':'支付金额'}</span></div>
      <div className="transaction-panel__item-summary"><div className="transaction-panel__item-summary-line">{record.payer.legal_name} → {record.payee.legal_name}</div><div className="transaction-panel__item-summary-line">{record.description||'暂无备注'}</div></div>
      <span className="transaction-panel__item-arrow"><RightOutlined /></span>
    </button>)}</div>}
    <Drawer title={editingRecord?'支付记录详情':'新增支付记录'} open={drawerOpen} onClose={close} width={640} destroyOnHidden extra={current?.can_reverse?<Button danger icon={<UndoOutlined />} onClick={()=>{setReason('');setReverseError('');setReverseOpen(true)}}>冲销</Button>:null}>
      <ClientPaymentForm projectId={projectId} recordId={editingRecord?.id} chainType={chainType} onSuccess={success} onCancel={close} />
    </Drawer>
    <Modal title="确认冲销实际资金" open={reverseOpen} onCancel={()=>setReverseOpen(false)} onOk={()=>void confirmReverse()} confirmLoading={reverse.isPending} okText="确认冲销" okButtonProps={{danger:true,disabled:!reason.trim()||!current?.can_reverse}} destroyOnHidden>
      <Alert type="warning" showIcon message="冲销保留原资金历史；已核销资金须先反向核销。不会删除记录或静默重建。" style={{marginBottom:16}} />
      <label>冲销原因<Input.TextArea aria-label="冲销原因" value={reason} onChange={e=>setReason(e.target.value)} maxLength={1000} /></label>
      {reverseError?<Alert style={{marginTop:16}} type="error" showIcon message={reverseError} />:null}
    </Modal>
  </div>
  if(hideCard)return <section className={'business-panel-surface__group transaction-panel__group transaction-panel__group--'+groupTone}>
    <div className="transaction-panel__group-head"><div className="transaction-panel__group-heading">{groupKicker&&<span className="transaction-panel__group-kicker">{groupKicker}</span>}<h4 className="business-panel-surface__group-title">{groupTitle||getBusinessChainLabel(chainType)}</h4>{(groupDescription||subtitle)&&<p className="business-panel-surface__group-desc">{groupDescription||subtitle}</p>}</div>
    <div className="transaction-panel__group-actions"><span className="transaction-panel__group-count">{records.length} 条记录</span>{canManage&&<CreateActionButton size="small" compact onClick={create}>新增支付</CreateActionButton>}</div></div>{listContent}
  </section>
  return <Card title={<SectionTitle icon={<PayCircleOutlined />}>{title||'支付记录'}</SectionTitle>} extra={canManage&&<CreateActionButton size="small" compact onClick={create}>添加支付记录</CreateActionButton>}>{listContent}</Card>
}
