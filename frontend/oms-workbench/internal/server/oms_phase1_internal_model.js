((scope)=>{
  'use strict';

  const decimalMinor=value=>{
    const match=String(value??'').trim().match(/^(-?)(\d+)(?:\.(\d{1,2}))?$/);
    if(!match)return null;
    try{return String((match[1]?-1n:1n)*(BigInt(match[2])*100n+BigInt((match[3]||'').padEnd(2,'0'))))}catch{return null}
  };
  const integer=value=>{try{return BigInt(String(value??0))}catch{return 0n}};
  const money=value=>{
    try{
      const raw=integer(value),negative=raw<0n,absolute=negative?-raw:raw;
      return (negative?'-':'')+'¥'+(absolute/100n).toLocaleString('zh-CN')+'.'+String(absolute%100n).padStart(2,'0');
    }catch{return '—'}
  };
  const contribution=(baseMinor,basisPoints)=>{
    const base=integer(baseMinor),rate=integer(basisPoints);
    if(base<=0n||rate<=0n)return '0';
    return String((base*rate+5000n)/10000n);
  };
  const components=[
    ['pension','养老保险'],
    ['medical','医疗保险'],
    ['unemployment','失业保险'],
    ['injury','工伤保险'],
    ['maternity','生育保险'],
    ['housing','住房公积金'],
  ];
  const payrollLine=(member,performanceMinor,scheme)=>{
    const socialBase=integer(member.social_base_minor),housingBase=integer(member.housing_base_minor);
    const employee={},employer={};
    for(const [key] of components){
      const base=key==='housing'?housingBase:socialBase;
      employee[key]=contribution(base,scheme.rates?.[key]?.employee_bps||0);
      employer[key]=contribution(base,scheme.rates?.[key]?.employer_bps||0);
    }
    const employeeTotal=Object.values(employee).reduce((sum,value)=>sum+integer(value),0n);
    const employerTotal=Object.values(employer).reduce((sum,value)=>sum+integer(value),0n);
    const gross=integer(member.fixed_salary_minor)+integer(performanceMinor);
    return {
      fixed_salary_minor:String(integer(member.fixed_salary_minor)),
      performance_minor:String(integer(performanceMinor)),
      gross_minor:String(gross),
      employee_contributions:employee,
      employer_contributions:employer,
      employee_contribution_minor:String(employeeTotal),
      employer_contribution_minor:String(employerTotal),
      payable_minor:String(gross-employeeTotal),
      employer_cost_minor:String(gross+employerTotal),
    };
  };
  const lineStatus=(line,payDate,today)=>line.paid_at?'paid':String(payDate||'')<=String(today||'')?'confirm':'planned';
  const periodStatus=(lines,payDate,today)=>{
    if(!lines.length)return 'empty';
    const paid=lines.filter(line=>line.paid_at).length;
    if(paid===lines.length)return 'paid';
    if(paid>0)return 'partial';
    return String(payDate||'')<=String(today||'')?'confirm':'planned';
  };

  scope.OMSInternalModel=Object.freeze({decimalMinor,money,contribution,components,payrollLine,lineStatus,periodStatus});
})(globalThis);
