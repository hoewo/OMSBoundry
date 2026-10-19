// A configuration draft is never an IAM credential or a live grant.
export function restorePermissionDraft(raw,{base,catalog,tenantID,subjectID}){
 try{
  const value=JSON.parse(raw||'null'),validKeys=new Set(catalog.map(c=>c[0])),seen=new Set();
  if(value?.schema_version!=='1'||value.tenant_id!==tenantID||value.subject_id!==subjectID||!Array.isArray(value.entries)||value.entries.length>250)throw Error('invalid draft');
  for(const e of value.entries){
   const system=base.find(b=>b.key===e.key);
   if(typeof e.key!=='string'||!e.key||e.key.length>99||seen.has(e.key)||typeof e.title!=='string'||!e.title.trim()||e.title.length>80||typeof e.description!=='string'||e.description.length>500||typeof e.template!=='string'||e.template.length>100||!['全局生效','项目生效'].includes(e.scope)||!Number.isInteger(e.sort)||e.sort<0||e.sort>9999||typeof e.active!=='boolean'||!Array.isArray(e.keys)||new Set(e.keys).size!==e.keys.length||e.keys.some(k=>!validKeys.has(k)))throw Error('invalid entry');
   if(system?(e.system!==true||e.area!==system.area||!e.active):(e.system!==false||e.area!=='positions'))throw Error('invalid ownership');
   if(e.key==='admin'&&(e.scope!==system.scope||e.keys.length!==validKeys.size||e.keys.some(k=>!validKeys.has(k))))throw Error('invalid admin');
   seen.add(e.key);
  }
  if(base.some(b=>!seen.has(b.key)))throw Error('missing system template');
  return value.entries;
 }catch{return base}
}
