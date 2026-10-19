import { cents, decimal } from '../../src/contract-money.mjs'
export type MoneyValue=string|number|null|undefined
export const formatAmount=(value:MoneyValue,fallback='—'):string=>{
  if(value==null||value==='')return fallback
  try{const raw=decimal(cents(value));const [whole,fraction]=raw.split('.');return whole.replace(/\B(?=(\d{3})+(?!\d))/g,',')+'.'+fraction}catch{return fallback}
}
export const formatMoney=(value:MoneyValue,fallback='—')=>{const s=formatAmount(value,fallback);return s===fallback?s:s.startsWith('-')?'-¥'+s.slice(1):'¥'+s}
