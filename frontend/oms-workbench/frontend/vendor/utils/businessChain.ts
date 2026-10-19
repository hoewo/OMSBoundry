import type {BusinessChainType,Contract} from '@/types'
export const DEFAULT_BUSINESS_CHAIN:BusinessChainType='client_to_qualification'
export const BUSINESS_CHAIN_ORDER:BusinessChainType[]=['client_to_qualification','qualification_to_our_company']
export const getContractChainType=(contract:Pick<Contract,'chain_type'>):BusinessChainType=>{
  if(!BUSINESS_CHAIN_ORDER.includes(contract.chain_type))throw new Error('合同两链上下文不明确')
  return contract.chain_type
}
export const normalizeBusinessChainType=(chain?:BusinessChainType|null):BusinessChainType=>{if(chain==null)return DEFAULT_BUSINESS_CHAIN;if(!BUSINESS_CHAIN_ORDER.includes(chain))throw new Error('合同链无效');return chain}
export const getBusinessChainLabel=(chain?:string)=>chain==='client_to_qualification'?'甲方 → 资质单位':chain==='qualification_to_our_company'?'资质单位 → 我方公司':'未明确往来对象'
