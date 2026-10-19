import { Select, Spin, Tag, message } from 'antd'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { oa2026Service } from '@/services/oa2026'
import { getErrorMessage } from '@/utils/error'
import './OA2026Controls.css'

export const BiddingMethodControl = ({ projectId, canEdit }: { projectId: string; canEdit: boolean }) => {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['oa2026', 'bidding', projectId],
    queryFn: () => oa2026Service.getBidding(projectId),
  })
  const mutation = useMutation({
    mutationFn: (value: 'comparison' | 'competitive_negotiation' | 'tender') => oa2026Service.updateBidding(projectId, value),
    onSuccess: (next) => {
      queryClient.setQueryData(['oa2026', 'bidding', projectId], next)
      queryClient.invalidateQueries({ queryKey: ['oa2026', 'progress', projectId] })
      message.success('项目承接方式已保存')
    },
    onError: (error) => message.error(getErrorMessage(error, '承接方式保存失败')),
  })
  if (isLoading || !data) return <Spin size="small" />
  return (
    <section className="oa2026-controls__bidding">
      <div><strong>项目承接方式</strong><span>比选、竞争性谈判和招投标三选一；系统据此判断招标文件是否必传。</span></div>
      {data.not_applicable ? <Tag color="default">本阶段无/不适用</Tag> : (
        <Select
          value={data.engagement_method || undefined}
          placeholder="请选择承接方式"
          disabled={!canEdit}
          loading={mutation.isPending}
          onChange={(value) => mutation.mutate(value)}
          options={[
            { value: 'comparison', label: '比选' },
            { value: 'competitive_negotiation', label: '竞争性谈判' },
            { value: 'tender', label: '招投标' },
          ]}
        />
      )}
      {!data.not_applicable ? (
        <div className="oa2026-controls__requirements">
          <Tag color={!data.engagement_method ? 'default' : data.tender_file_required ? (data.tender_file_complete ? 'success' : 'warning') : 'default'}>
            招标文件：{!data.engagement_method ? '待选方式' : data.tender_file_required ? (data.tender_file_complete ? '已上传' : '必传') : '不适用'}
          </Tag>
          <Tag color={data.bid_file_complete ? 'success' : 'warning'}>投标文件：{data.bid_file_complete ? '已上传' : '必传'}</Tag>
          <Tag color={data.award_file_complete ? 'success' : 'warning'}>中标文件：{data.award_file_complete ? '已上传' : '必传'}</Tag>
        </div>
      ) : null}
    </section>
  )
}

