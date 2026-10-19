import { useQuery } from '@tanstack/react-query'
import { businessService } from '@/services/business'
import { formatMoney } from '@/utils/money'

interface BonusAllocationStatisticsProps {
  projectId: string
  bonusType: 'business' | 'production'
}

/**
 * 奖金分配统计组件
 * 横排显示有效登记金额、有效登记条数、登记人数
 * T497: 创建可复用的奖金分配统计组件
 */
export const BonusAllocationStatistics = ({
  projectId,
  bonusType,
}: BonusAllocationStatisticsProps) => {
  // 根据奖金类型获取统计信息
  const { data: statistics, isLoading } = useQuery({
    queryKey: [bonusType === 'production' ? 'productionBonusStatistics' : 'businessBonusStatistics', projectId],
    queryFn: () => 
      bonusType === 'production'
        ? businessService.getProductionBonusStatistics(projectId)
        : businessService.getBusinessBonusStatistics(projectId),
    enabled: !!projectId,
  })

  return (
    <div className="bonus-allocation__stats" aria-busy={isLoading}>
      <div className="bonus-allocation__stat bonus-allocation__stat--amount">
        <span>有效登记金额</span>
        <strong>{isLoading ? '-' : formatMoney(statistics?.total_amount)}</strong>
      </div>
      <div className="bonus-allocation__stat bonus-allocation__stat--count">
        <span>有效登记条数</span>
        <strong>{isLoading ? '-' : `${statistics?.record_count || 0} 次`}</strong>
      </div>
      <div className="bonus-allocation__stat bonus-allocation__stat--people">
        <span>登记人数</span>
        <strong>{isLoading ? '-' : `${statistics?.recipient_count || 0} 人`}</strong>
      </div>
    </div>
  )
}
