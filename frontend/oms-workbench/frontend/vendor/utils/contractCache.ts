import type { Contract } from '@/types'

const getTimestamp = (value?: string) => {
  if (!value) return 0
  const timestamp = Date.parse(value)
  return Number.isNaN(timestamp) ? 0 : timestamp
}

export const upsertContractInList = (
  contracts: Contract[] | undefined,
  savedContract: Contract
): Contract[] => {
  const nextContracts = [
    ...(contracts || []).filter((contract) => contract.id !== savedContract.id),
    savedContract,
  ]

  return nextContracts.sort((left, right) => {
    const signDateDifference =
      getTimestamp(right.sign_date) - getTimestamp(left.sign_date)
    if (signDateDifference !== 0) return signDateDifference

    return getTimestamp(right.created_at) - getTimestamp(left.created_at)
  })
}

