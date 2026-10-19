import type {Contract} from '@/types'
import {previewAmount} from '../../src/contract-money.mjs'
export const getContractEffectiveAmount=(contract?:Pick<Contract,'contract_amount'|'amendments'>|null,baseAmount?:string|number)=>previewAmount(contract,baseAmount)
export const getSettlementDisplayAmount=(contract:Contract)=>contract.settlement_follows_contract_amount?contract.current_effective_amount??getContractEffectiveAmount(contract):contract.settlement_amount??getContractEffectiveAmount(contract)
