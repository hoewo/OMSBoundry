(()=>{
  const root=document.querySelector('[data-oms-demo]');
  if(!root)return;

  const workspaceButtons=[...root.querySelectorAll('[data-oms-workspace]')];
  const panels=[...root.querySelectorAll('[data-oms-panel]')];
  const activate=name=>{
    for(const button of workspaceButtons){
      const active=button.dataset.omsWorkspace===name;
      button.classList.toggle('is-active',active);
      button.setAttribute('aria-current',active?'page':'false');
    }
    for(const panel of panels)panel.classList.toggle('is-active',panel.dataset.omsPanel===name);
    root.scrollIntoView({behavior:'smooth',block:'start'});
  };
  for(const button of workspaceButtons)button.addEventListener('click',()=>activate(button.dataset.omsWorkspace));
  for(const button of root.querySelectorAll('[data-oms-jump]'))button.addEventListener('click',()=>activate(button.dataset.omsJump));
  root.querySelector('[data-oms-start]')?.addEventListener('click',()=>activate('contract'));

  const drawer=root.querySelector('[data-oms-drawer]');
  const drawerLabel=root.querySelector('[data-oms-drawer-label]');
  const drawerTitle=root.querySelector('[data-oms-drawer-title]');
  const drawerAmount=root.querySelector('[data-oms-drawer-amount]');
  const drawerState=root.querySelector('[data-oms-drawer-state]');
  const drawerSummary=root.querySelector('[data-oms-drawer-summary]');
  const drawerQuestion=root.querySelector('[data-oms-drawer-question]');
  const open=button=>{
    drawerLabel.textContent=button.dataset.label||'候选业务节点';
    drawerTitle.textContent=button.dataset.title||'详情';
    drawerAmount.textContent=button.dataset.amount||'—';
    drawerState.textContent=button.dataset.state||'—';
    drawerSummary.textContent=button.dataset.summary||'这是一条虚构演示记录。';
    drawerQuestion.textContent=button.dataset.question||'该内容是否符合真实业务？';
    drawer.hidden=false;
    drawer.querySelector('[data-oms-drawer-close]')?.focus();
  };
  const close=()=>{drawer.hidden=true};
  for(const button of root.querySelectorAll('[data-oms-inspect]'))button.addEventListener('click',()=>open(button));
  for(const button of root.querySelectorAll('[data-oms-drawer-close]'))button.addEventListener('click',close);
  drawer.addEventListener('click',event=>{if(event.target===drawer)close()});
  document.addEventListener('keydown',event=>{if(event.key==='Escape'&&!drawer.hidden)close()});

  for(const button of root.querySelectorAll('[data-oms-money-filter]'))button.addEventListener('click',()=>{
    const filter=button.dataset.omsMoneyFilter;
    for(const item of root.querySelectorAll('[data-oms-money-filter]')){
      const active=item===button;
      item.classList.toggle('is-active',active);
      item.setAttribute('aria-pressed',String(active));
    }
    for(const row of root.querySelectorAll('[data-oms-money-row]'))row.hidden=filter!=='all'&&row.dataset.omsMoneyRow!==filter;
  });
})();
