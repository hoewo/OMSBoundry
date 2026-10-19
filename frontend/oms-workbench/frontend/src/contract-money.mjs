// Exact CNY view calculations. No IEEE-754 conversion at business boundaries.
/** @param {string|number|null|undefined} value */
export function cents(value='0') {
  if(typeof value==='number'&&!Number.isSafeInteger(value))throw new Error('金额不能经过浮点数转换');
  const raw=String(value ?? '0');if(!/^-?\d{1,18}(\.\d{1,2})?$/.test(raw))throw new Error('金额须为最多两位小数的十进制数');
  const negative=raw.startsWith('-'),[whole,fraction='']=raw.replace(/^-/,'').split('.');
  const n=BigInt(whole+fraction.padEnd(2,'0'));return negative?-n:n;
}
export function nonnegative(value){try{return cents(value)>=0n}catch{return false}}
export function decimal(n){const sign=n<0n?'-':'',a=n<0n?-n:n;return sign+(a/100n).toString()+'.'+(a%100n).toString().padStart(2,'0');}
export const sumMoney=(...values)=>decimal(values.reduce((sum,v)=>sum+cents(v),0n));
export function rateBPS(value){if(value==null||value==='')return null;const n=cents(value);if(n<0n||n>10000n)throw new Error('费率须在 0～100% 之间');return Number(n);}
export const percentFromBPS=value=>value==null?undefined:decimal(BigInt(value));
// Match the server's UTF-8 lexical ordering, never browser locale collation.
const compareText=(a,b)=>{const x=Array.from(a),y=Array.from(b);for(let i=0;i<Math.min(x.length,y.length);i++){const d=x[i].codePointAt(0)-y[i].codePointAt(0);if(d)return d;}return x.length-y.length;};
export const orderedAmendments=rows=>[...rows].sort((a,b)=>compareText(a.sign_date,b.sign_date)||compareText(a.amendment_number,b.amendment_number)||compareText(a.id,b.id));
export function previewFeeRows(contract){
  const keys=['design_fee','survey_fee','consultation_fee'],fees=Object.fromEntries(keys.map(k=>[k,cents(contract[k])]));
  const rows=[];for(const a of orderedAmendments(contract.amendments||[])){if(a.cancelled)continue;
    for(const k of keys){if(a.change_type==='adjustment')fees[k]+=cents(a[k]);else if(a.change_type==='revision')fees[k]=cents(a[k]);else if(a.change_type!=='no_amount_change')throw new Error('协议影响方式不明确');if(fees[k]<0n)throw new Error('协议预览分项不能为负数');}
    rows.push({...a,effective_amount:decimal(keys.reduce((n,k)=>n+fees[k],0n))});
  }
  return {amendments:rows,...Object.fromEntries(keys.map(k=>['current_'+k,decimal(fees[k])]))};
}
export function previewAmount(contract,baseAmount){
  let total=cents(baseAmount??contract?.contract_amount??'0');
  const amendments=orderedAmendments(contract?.amendments||[]);
  for(const a of amendments){if(a.cancelled)continue;const amount=cents(a.amendment_amount);if(a.change_type==='adjustment')total+=amount;else if(a.change_type==='revision')total=amount;}
  return decimal(total);
}
