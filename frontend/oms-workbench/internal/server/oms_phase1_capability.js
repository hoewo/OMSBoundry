((scope)=>{
  const definitiveDenial=error=>[401,403,404].includes(Number(error?.status));
  function create(options={}){
    const exchange=options.exchange;
    if(typeof exchange!=='function')throw new TypeError('capability exchange is required');
    const now=typeof options.now==='function'?options.now:()=>Date.now();
    const setTimer=typeof options.setTimer==='function'?options.setTimer:(callback,delay)=>setTimeout(callback,delay);
    const clearTimer=typeof options.clearTimer==='function'?options.clearTimer:handle=>clearTimeout(handle);
    const onLease=typeof options.onLease==='function'?options.onLease:()=>{};
    const onClear=typeof options.onClear==='function'?options.onClear:()=>{};
    const leases=new Map(),tasks=new Map(),timers=new Map();

    function cancelTimer(id){
      if(!timers.has(id))return;
      clearTimer(timers.get(id));timers.delete(id);
    }
    function clear(id){
      cancelTimer(id);leases.delete(id);onClear(id);
    }
    function validate(id,audience,value){
      const seconds=Number(value?.expires_in_seconds);
      if(value?.application_id!==id||value?.audience!==audience||!Number.isInteger(seconds)||seconds<1||seconds>300)throw new Error('Gateway 返回了无效的能力租约');
      if(typeof value.expires_at!=='string'||!Number.isFinite(Date.parse(value.expires_at)))throw new Error('Gateway 返回了无效的能力租约');
      const issuedAt=now(),ttl=seconds*1000,lead=Math.min(60000,Math.max(1000,Math.floor(ttl*.2)));
      return Object.freeze({applicationId:id,audience,expiresAt:issuedAt+ttl,renewAt:Math.max(issuedAt+250,issuedAt+ttl-lead),expiresInSeconds:seconds});
    }
    function schedule(id,audience,at){
      cancelTimer(id);
      const delay=Math.max(0,at-now());
      timers.set(id,setTimer(()=>{timers.delete(id);refresh(id,audience).catch(()=>{})},delay));
    }
    function retainOrClear(id,audience,previous,error){
      if(definitiveDenial(error)){clear(id);return}
      const remaining=(previous?.expiresAt||0)-now();
      if(remaining<=0){clear(id);return}
      const retryDelay=Math.min(15000,Math.max(1000,Math.floor(remaining/3)));
      schedule(id,audience,Math.min(previous.expiresAt-100,now()+retryDelay));
    }
    function refresh(id,audience){
      if(tasks.has(id))return tasks.get(id);
      const previous=leases.get(id);
      const task=(async()=>{
        try{
          const lease=validate(id,audience,await exchange(id,audience));
          leases.set(id,lease);schedule(id,audience,lease.renewAt);onLease(id,lease);return lease;
        }catch(error){retainOrClear(id,audience,previous,error);throw error}
      })();
      tasks.set(id,task);
      task.finally(()=>{if(tasks.get(id)===task)tasks.delete(id)}).catch(()=>{});
      return task;
    }
    async function ensure(id,audience){
      const current=leases.get(id);
      if(current&&current.audience!==audience)clear(id);
      const lease=leases.get(id);
      if(lease&&now()<lease.renewAt)return lease;
      try{return await refresh(id,audience)}catch(error){
        const retained=leases.get(id);
        if(retained&&retained.audience===audience&&now()<retained.expiresAt)return retained;
        throw error;
      }
    }
    function requireFresh(id,audience){return refresh(id,audience)}
    function invalidate(id){clear(id)}
    async function recheck(){
      const renewals=[];
      for(const [id,lease] of leases){
        if(now()>=lease.expiresAt){clear(id);continue}
        if(now()>=lease.renewAt)renewals.push(refresh(id,lease.audience));
      }
      return Promise.allSettled(renewals);
    }
    function dispose(){for(const id of [...leases.keys()])clear(id);for(const id of [...timers.keys()])cancelTimer(id)}
    function snapshot(id){return leases.get(id)||null}
    return Object.freeze({ensure,requireFresh,invalidate,recheck,dispose,snapshot});
  }
  scope.OMSPhase1CapabilityCoordinator=Object.freeze({create});
})(globalThis);
