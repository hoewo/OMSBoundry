/* Reference OA commands. The browser organizes actions, never owns facts. */
(() => {
  'use strict';
  const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
  const actions = {
    update: 'project-workspace.update',
    correct: 'project-workspace.profile.correct',
    complete: 'project-workspace.initiation.complete',
    assignment: 'project-workspace.assignment.manage',
    business: 'project-workspace.assignment.business-owner.manage',
    production: 'project-workspace.assignment.production-owner.manage',
    contactRead: 'project-workspace.counterparty-context.read',
    contactManage: 'project-workspace.counterparty-context.manage'
  };
  // An open form may outlive its one-hour CSRF cookie, or another tab may
  // rotate it. Never fall back to the token embedded in the initial HTML.
  class GatewayCSRF {
    constructor({route, request = (...args) => fetch(...args), cookies = () => document.cookie}) {
      this.route = route;
      this.request = request;
      this.cookies = cookies;
      this.pending = null;
    }
    current() {
      const value = this.cookies().split(';').map(item => item.trim()).find(item => item.startsWith('oms_iam_csrf='));
      const token = value ? value.slice('oms_iam_csrf='.length) : '';
      return /^[A-Za-z0-9_-]{16,256}$/.test(token) ? token : '';
    }
    async token() {
      const current = this.current();
      if (current) return current;
      if (!this.pending) {
        this.pending = this.refresh().finally(() => { this.pending = null; });
      }
      return this.pending;
    }
    async refresh() {
      // Route comes from the enabled IAM entry in Suite Manifest. Forbid
      // external/protocol-relative URLs; no credentials or permission change.
      if (!/^\/[A-Za-z0-9_-]+(?:\/[A-Za-z0-9_-]+)*$/.test(this.route || '')) {
        throw new Error('安全令牌更新入口未配置；输入已保留，请刷新后重试');
      }
      try {
        const response = await this.request(this.route + '/api/v1/csrf', {method: 'GET', credentials: 'same-origin', cache: 'no-store', redirect: 'error', headers: {Accept: 'application/json'}});
        if (!response.ok || response.redirected) throw new Error('csrf unavailable');
        const data = await response.json(), current = this.current();
        if (!current || data.csrf_token !== current) throw new Error('csrf cookie mismatch');
        return current;
      } catch {
        throw new Error('安全令牌暂时无法更新；输入已保留，请重试，不会自动提交业务');
      }
    }
  }
  function nameCompletion(value, names) {
    const query = value.trim();
    if (!query || query !== value) return null;
    const lower = query.toLocaleLowerCase();
    const prefix = names.find(name => name.toLocaleLowerCase().startsWith(lower) && name.toLocaleLowerCase() !== lower);
    if (prefix) return {value: prefix, suffix: prefix.slice(query.length)};
    const fuzzy = names.find(name => name.toLocaleLowerCase().includes(lower) && name.toLocaleLowerCase() !== lower);
    return fuzzy ? {value: fuzzy, suffix: ' → ' + fuzzy} : null;
  }
  class InitiationFlow {
    constructor({request, authorize, uuid}) {
      this.request = request;
      this.authorize = authorize;
      this.uuid = uuid;
      this.details = new Map();
      this.contacts = new Map();
      this.commands = new Map();
      this.busy = false;
    }
    can(details, action) { return details?.allowed_actions?.includes(action) === true; }
    completionFailure(id, error, before) {
      const current = this.details.get(id)?.project;
      if (current?.status === 'active') return {title: '立项已完成，详情读取失败', message: error.message + '；请重新打开此项目，不要再次创建。'};
      const paths = new Set(['/api/v1/projects/' + id + '/initiation-info-changes', '/api/v1/projects/' + id + '/complete-initiation']);
      const unknown = [...this.commands.keys()].some(key => paths.has(JSON.parse(key)[0]));
      const saved = current && current.revision !== before ? '部分资料已确认保存。' : '';
      if (unknown) return {title: '提交结果待确认', message: saved + error.message + '；输入已保留。请重试同一操作或重新加载资料确认，不要另建项目。'};
      return {title: '立项尚未完成', message: saved + error.message + '；可重试或重新加载资料。'};
    }
    async command(path, body, method = 'POST') {
      const key = JSON.stringify([path, method, body]);
      const command = this.commands.get(key) || {...body, command_id: this.uuid()};
      this.commands.set(key, command);
      try {
        const value = await this.request(path, {method, headers: {'Content-Type': 'application/json'}, body: JSON.stringify(command)});
        this.commands.delete(key);
        return value;
      } catch (error) {
        // An uncertain network result retains the ID. A definite rejection may
        // be corrected; no automatic revision overwrite or cross-app retry.
        if (error.status >= 400 && error.status < 500) this.commands.delete(key);
        throw error;
      }
    }
    async exclusively(work) {
      if (this.busy) throw new Error('正在提交，请等待当前操作完成');
      this.busy = true;
      try { return await work(); } finally { this.busy = false; }
    }
    async load(id) {
      if (!uuidPattern.test(id)) throw new Error('项目标识无效');
      await this.authorize();
      const details = await this.request('/api/v1/projects/' + id);
      if (details.project?.id !== id) throw new Error('项目响应与当前项目不一致');
      this.details.set(id, details);
      return details;
    }
    async dispatch({name, node_type, business_owner}) {
      name = String(name || '').trim();
      if (!name || name.length > 200) throw new Error('请输入项目名称（不超过 200 字）');
      if (!['project', 'package'].includes(node_type)) throw new Error('请选择立项类型');
      if (!uuidPattern.test(business_owner?.person_id || '') || !Number.isSafeInteger(business_owner?.source_revision) || business_owner.source_revision < 1) {
        throw new Error('请选择已加载的项目经营负责人');
      }
      return this.exclusively(async () => {
        await this.authorize();
        return this.command('/api/v1/initiation-dispatches', {name, node_type, business_owner});
      });
    }
    async assignProduction(id, person) {
      const details = this.details.get(id);
      if (!details || (!this.can(details, actions.assignment) && !this.can(details, actions.production))) throw new Error('当前账号没有生产负责人分工权限');
      if (!uuidPattern.test(person?.person_id || '') || !Number.isSafeInteger(person?.source_revision) || person.source_revision < 1) throw new Error('请选择已加载的生产负责人');
      const existing = details.assignments?.find(value => value.role_key === 'production_owner' && !value.ended_at);
      if (existing) {
        if (existing.subject_kind === 'person' && existing.subject_id === person.person_id) return details.project;
        throw new Error('已有生产负责人；更换须显式结束旧分工，不能覆盖历史');
      }
      const result = await this.command('/api/v1/projects/' + id + '/assignments', {
        expected_revision: details.project.revision, operation: 'assign', subject_kind: 'person',
        subject_id: person.person_id, source_revision: person.source_revision, role_key: 'production_owner'
      });
      details.project = result.project;
      details.assignments = [...(details.assignments || []), result.assignment];
      return result.project;
    }
    profileFields(project, fields) {
      const number = String(fields.project_number || '').trim(), name = String(fields.name || '').trim();
      const types = [...new Set(fields.project_type_keys || [])];
      if (!name || [...name].length > 200) throw new Error('请输入项目名称（不超过 200 字）');
      if ((!number && project.node_type !== 'package') || (number && !/^[A-Za-z0-9][A-Za-z0-9._/-]{0,63}$/.test(number))) throw new Error('请填写有效的正式项目编号');
      if (!types.length || types.length > 16 || types.some(value => !/^[a-z][a-z0-9-]{0,63}$/.test(value))) throw new Error('请选择至少一种项目类型（最多 16 种）');
      const summary = String(fields.summary || '').trim();
      if ([...summary].length > 2000) throw new Error('项目概况不能超过 2000 字');
      return {project_number: number, name, summary,
        planned_start_date: fields.planned_start_date || null, planned_end_date: project.planned_end_date || null,
        project_type_keys: types};
    }
    async correctProfile(id, fields) {
      return this.exclusively(async () => {
        const details = this.details.get(id);
        if (!this.can(details, actions.correct)) throw new Error('当前账号没有编辑该项目基本资料的权限');
        if (details.project.status !== 'active') throw new Error('当前项目不处于已立项状态，请重新打开项目');
        const reason = String(fields.correction_reason || '').trim();
        if (!reason || [...reason].length > 1000) throw new Error('请填写更正说明（不超过 1000 字）');
        const body = this.profileFields(details.project, fields);
        await this.authorize();
        const project = await this.command('/api/v1/projects/' + id + '/profile-corrections', {
          ...body, expected_revision: details.project.revision, correction_reason: reason
        });
        details.project = project;
        return project;
      });
    }
    async changeBasicInfo(id, fields, owners = []) {
      return this.exclusively(async () => {
        const details = this.details.get(id);
        if (!details || details.project.status !== 'active') throw new Error('当前项目不处于已立项状态，请重新打开项目');
        const body = {expected_revision: details.project.revision, reason: '修改项目基本信息'};
        if (fields) {
          if (!this.can(details, actions.correct)) throw new Error('当前账号没有编辑基本资料的权限');
          body.profile = this.profileFields(details.project, fields);
        }
        const seen = new Set();
        body.owners = owners.map(value => {
          const action = value.role_key === 'business_owner' ? actions.business : value.role_key === 'production_owner' ? actions.production : null;
          if (!action || seen.has(value.role_key)) throw new Error('负责人类型无效或重复');
          seen.add(value.role_key);
          if (!this.can(details, actions.assignment) && !this.can(details, action)) throw new Error('当前账号没有该类负责人调整权限');
          if (value.operation === 'clear' && !value.person) return {role_key: value.role_key, operation: 'clear'};
          if (value.operation !== 'set' || !uuidPattern.test(value.person?.person_id || '') || !Number.isSafeInteger(value.person.source_revision) || value.person.source_revision < 1) throw new Error('请选择已加载的负责人');
          return {role_key: value.role_key, operation: 'set', person: {person_id: value.person.person_id, source_revision: value.person.source_revision}};
        });
        if (!body.profile && !body.owners.length) return {project: details.project, owners: []};
        await this.authorize();
        const result = await this.command('/api/v1/projects/' + id + '/basic-info-changes', body);
        if (result.project?.id !== id || !Array.isArray(result.owners)) throw new Error('保存响应与当前项目不一致，请复读确认');
        details.project = result.project;
        for (const change of result.owners) {
          if (change.changed) {
            if (change.previous) details.assignments = (details.assignments || []).map(value => value.id === change.previous.id ? change.previous : value);
            if (change.current) details.assignments = [...(details.assignments || []), change.current];
          }
        }
        return result;
      });
    }
    async suggestClientNames(query) {
      query = String(query).trim();
      if (!query) return [];
      if ([...query].length > 200) throw new Error('名称查询过长');
      await this.authorize();
      const value = await this.request('/api/v1/counterparty-context/name-suggestions?q=' + encodeURIComponent(query) + '&limit=10');
      if (value.schema_version !== 'project-counterparty-name-suggestions-v1' || !Array.isArray(value.names) || value.names.length > 10 || value.names.some(name => typeof name !== 'string' || !name.trim() || [...name].length > 200 || !name.toLocaleLowerCase().includes(query.toLocaleLowerCase())) || new Set(value.names).size !== value.names.length) throw new Error('历史名称提示响应无效，可直接填写');
      return value.names;
    }
    async loadCounterparty(id) {
      if (!uuidPattern.test(id)) throw new Error('项目标识无效');
      await this.authorize();
      const value = await this.request('/api/v1/projects/' + id + '/counterparty-context');
      if (value.project_id !== id || !Number.isSafeInteger(value.project_revision) || value.project_revision < 1 || !Array.isArray(value.allowed_actions) || (value.context && (value.context.schema_version !== 'project-counterparty-context-v1' || value.context.role !== 'client' || typeof value.context.name !== 'string'))) throw new Error('甲方信息响应不符合当前项目契约');
      this.contacts.set(id, value);
      return value;
    }
    async saveCounterparty(id, fields) {
      return this.exclusively(async () => {
        const current = this.contacts.get(id);
        if (!current || current.project_status !== 'active' || !current.allowed_actions.includes(actions.contactManage)) throw new Error('当前账号没有编辑该项目甲方信息的权限，或状态已变化');
        const name = String(fields.name || '').trim(), contact_name = String(fields.contact_name || '').trim(), contact_phone = String(fields.contact_phone || '').trim();
        if (!name || [...name].length > 200) throw new Error('请输入甲方名称（不超过 200 字）');
        if ([...contact_name].length > 80 || [...contact_phone].length > 50 || !/^[0-9+\-\s()]*$/.test(contact_phone)) throw new Error('请输入正确的联系人和联系电话');
        await this.authorize();
        const value = await this.command('/api/v1/projects/' + id + '/counterparty-context', {expected_revision: current.project_revision, reason: '修改项目甲方信息', name, contact_name, contact_phone});
        if (value.project_id !== id || !Number.isSafeInteger(value.project_revision) || !value.context) throw new Error('保存响应异常，请复读确认');
        this.contacts.set(id, {...value, allowed_actions: current.allowed_actions});
        const detail = this.details.get(id); if (detail) detail.project.revision = value.project_revision;
        return value;
      });
    }
    async complete(id, fields) {
      return this.exclusively(async () => {
        const details = this.details.get(id);
        if (!details || !this.can(details, actions.complete)) throw new Error('当前账号没有完成该项目立项的权限');
        if (!['draft', 'pending_completion'].includes(details.project.status)) throw new Error('当前项目不处于待立项状态，请重新打开项目');
        const project = details.project;
        const body = this.profileFields(project, fields);
        const changed = Object.entries(body).some(([key, value]) => JSON.stringify(value) !== JSON.stringify(project[key] ?? (key === 'summary' ? '' : null)));
        if (changed && !this.can(details, actions.update)) throw new Error('当前账号没有修改立项资料的权限');
        const currentOwner = role => details.assignments?.find(value => value.role_key === role && !value.ended_at);
        if (!currentOwner('business_owner') && !fields.business_owner) throw new Error('该旧待立项项目缺少经营负责人，请在下方补齐后重试');
        const owners = [];
        for (const [role, person, action] of [['business_owner', fields.business_owner, actions.business], ['production_owner', fields.production_owner, actions.production]]) {
          if (!person) continue; // A blank optional selection never clears an existing owner.
          const current = currentOwner(role);
          if (current?.subject_kind === 'person' && current.subject_id === person.person_id) continue;
          if (!this.can(details, actions.assignment) && !this.can(details, action)) throw new Error('当前账号没有该类负责人调整权限，请联系管理员授权');
          if (!uuidPattern.test(person.person_id || '') || !Number.isSafeInteger(person.source_revision) || person.source_revision < 1) throw new Error('请选择已加载的负责人');
          owners.push({role_key: role, operation: 'set', person: {person_id: person.person_id, source_revision: person.source_revision}});
        }
        await this.authorize();
        if (changed || owners.length) {
          const input = {expected_revision: project.revision, reason: '补齐项目立项资料', owners};
          if (changed) input.profile = body;
          const saved = await this.command('/api/v1/projects/' + id + '/initiation-info-changes', input);
          if (saved.project?.id !== id || !Array.isArray(saved.owners)) throw new Error('保存响应与当前项目不一致，请重新加载资料确认');
          details.project = saved.project;
          for (const change of saved.owners) {
            if (!change.changed) continue;
            if (change.previous) details.assignments = (details.assignments || []).map(value => value.id === change.previous.id ? change.previous : value);
            if (change.current) details.assignments = [...(details.assignments || []), change.current];
          }
        }
        try {
          details.project = await this.command('/api/v1/projects/' + id + '/complete-initiation', {
            expected_revision: details.project.revision, initiation_profile: 'business-owner-v1'
          });
          return details.project;
        } catch (error) {
          const persisted = error.data?.error?.project || error.data?.project;
          if (persisted?.id === id) details.project = persisted;
          throw error;
        }
      });
    }
  }

  function normalizePeople(data, tenantID) {
    if (!Array.isArray(data)) throw new Error('人员目录响应无效');
    const ids = new Set();
    return data.filter(value => value.status === 'active').map(value => {
      if (!uuidPattern.test(value.id) || value.tenant_id !== tenantID || !Number.isSafeInteger(value.revision) || value.revision < 1 || typeof value.display_name !== 'string' || !value.display_name.trim() || ids.has(value.id)) throw new Error('人员目录的租户、标识或版本不一致');
      ids.add(value.id);
      return {person_id: value.id, source_revision: value.revision, name: value.display_name.trim()};
    });
  }

  class InitiationController {
    constructor({root, request, authorize, applicationRequest, evidenceFetch, csrf, drawer, showView, reloadInitiations, record}) {
      this.csrf = csrf || new GatewayCSRF({route: root.dataset.identityRoutePrefix});
      this.root = root;
      this.drawer = drawer;
      this.showView = view => { this.clearClientNameHint(); showView(view); };
      this.reloadInitiations = reloadInitiations;
      this.record = record;
      this.flow = new InitiationFlow({request, authorize, uuid: () => crypto.randomUUID()});
      const businessRoot = document.querySelector('[data-view="project-real-detail"]');
      if (businessRoot && globalThis.ReferenceOABusiness && applicationRequest) {
        const readers = new globalThis.ReferenceOABusiness.BusinessFlow({projectRequest: request, projectAuthorize: authorize, applicationRequest, tenantID: root.dataset.tenantId});
        const biddingRoot=businessRoot.querySelector('[data-real-business-panel="bidding"]');
        const evidenceReferenceRequest=(id,version)=>applicationRequest('evidence-service','/v1/references/files/'+encodeURIComponent(id)+'/versions/'+String(version));
        const bidding=biddingRoot && globalThis.ReferenceOABidding && globalThis.ReferenceOAReactBidding ? globalThis.ReferenceOAReactBidding.createWorkspace({root:biddingRoot,flow:new globalThis.ReferenceOABidding.BiddingFlow({projectRequest:request,projectAuthorize:authorize,applicationRequest,evidenceReferenceRequest,tenantID:root.dataset.tenantId,projectUpdated:details=>this.flow.details.set(details.project.id,details)}),details:()=>this.flow.details.get(this.projectID),evidenceReferenceRequest,evidenceFetch,tenantID:root.dataset.tenantId,subjectID:root.dataset.subjectId,record}) : null;
        const personnelRoot=businessRoot.querySelector('[data-real-business-panel="personnel"]');
        const personnel=personnelRoot && globalThis.ReferenceOAReactBidding?.createPersonnelWorkspace ? globalThis.ReferenceOAReactBidding.createPersonnelWorkspace({root:personnelRoot,details:()=>this.flow.details.get(this.projectID),request,authorize,applicationRequest,tenantID:root.dataset.tenantId,subjectID:root.dataset.subjectId,updated:details=>this.flow.details.set(details.project.id,details)}) : null;
        const contractRoot=businessRoot.querySelector('[data-real-business-panel="contracts"]');
        const contracts=contractRoot&&globalThis.ReferenceOAReactBidding?.createContractWorkspace?globalThis.ReferenceOAReactBidding.createContractWorkspace({root:contractRoot,details:()=>this.flow.details.get(this.projectID),projectRequest:request,projectAuthorize:authorize,applicationRequest,evidenceFetch,evidenceReferenceRequest,tenantID:root.dataset.tenantId,subjectID:root.dataset.subjectId,updated:details=>this.flow.details.set(details.project.id,details)}):null;
const cashRoot=businessRoot.querySelector('[data-real-business-panel="payments"]');
        const cash=cashRoot&&globalThis.ReferenceOAReactBidding?.createCashWorkspace?globalThis.ReferenceOAReactBidding.createCashWorkspace({root:cashRoot,details:()=>this.flow.details.get(this.projectID),projectRequest:request,projectAuthorize:authorize,applicationRequest,tenantID:root.dataset.tenantId,subjectID:root.dataset.subjectId,updated:details=>this.flow.details.set(details.project.id,details)}):null;
        const invoiceRoot=businessRoot.querySelector('[data-real-business-panel="invoices"]');
        const invoices=invoiceRoot&&globalThis.ReferenceOAReactBidding?.createInvoiceWorkspace?globalThis.ReferenceOAReactBidding.createInvoiceWorkspace({root:invoiceRoot,details:()=>this.flow.details.get(this.projectID),projectRequest:request,projectAuthorize:authorize,applicationRequest,evidenceFetch,evidenceReferenceRequest,tenantID:root.dataset.tenantId,subjectID:root.dataset.subjectId,updated:details=>this.flow.details.set(details.project.id,details)}):null;
        const costOptions={details:()=>this.flow.details.get(this.projectID),applicationRequest,evidenceFetch,evidenceReferenceRequest,tenantID:root.dataset.tenantId,subjectID:root.dataset.subjectId};
        const productionRoot=businessRoot.querySelector('[data-real-production-layout]');
        const personnelProfile=root.dataset.productionPersonnelProfile||'',bonusProfile=root.dataset.bonusProfile||'reference-oa-bonus-v2';
        const stageProfile=root.dataset.productionStageProfile||'';
        const productionLayout=productionRoot&&globalThis.ReferenceOAReactBidding?.createProductionLayout?globalThis.ReferenceOAReactBidding.createProductionLayout({root:productionRoot,personnelProfile,stageProfile,applications:[...businessRoot.querySelectorAll('[data-real-application]')].map(node=>node.dataset.realApplication)}):null;
        const costRoot=businessRoot.querySelector('[data-real-business-panel="costs"]'),productionCostRoot=businessRoot.querySelector('[data-real-production-cost]');
        const costs=costRoot&&globalThis.ReferenceOAReactBidding?.createCostWorkspace?globalThis.ReferenceOAReactBidding.createCostWorkspace({...costOptions,root:costRoot,scope:'business'}):null;
        const productionCosts=productionCostRoot&&globalThis.ReferenceOAReactBidding?.createCostWorkspace?globalThis.ReferenceOAReactBidding.createCostWorkspace({...costOptions,root:productionCostRoot,scope:'production'}):null;
        const approvalRoot=businessRoot.querySelector('[data-real-production-approval-audit]');
        const approvalAudit=approvalRoot&&globalThis.ReferenceOAReactBidding?.createApprovalAuditWorkspace?globalThis.ReferenceOAReactBidding.createApprovalAuditWorkspace({...costOptions,root:approvalRoot}):null;
        const commissionRoot=businessRoot.querySelector('[data-real-production-commission]');
        const commission=commissionRoot&&globalThis.ReferenceOAReactBidding?.createCommissionWorkspace?globalThis.ReferenceOAReactBidding.createCommissionWorkspace({...costOptions,root:commissionRoot}):null;
        const productionPersonnelRoot=businessRoot.querySelector('[data-real-production-personnel]');
        // One native transport lease at a time: Project and Tenant catalogue
        // capabilities share a route cookie but must never share a scope.
        let personnelTransport=Promise.resolve();
        const personnelLease=task=>{const p=personnelTransport.then(task);personnelTransport=p.catch(()=>{});return p};
        const personnelRequest=(path,method='GET',body)=>personnelLease(()=>applicationRequest('project-workspace',path,{method,projectPersonnelScope:path.startsWith('/api/v1/reference-oa/')?'catalogue':'project',...(body?{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{})}));
        const productionPersonnel=productionPersonnelRoot&&personnelProfile==='reference-oa-production-personnel-v1'&&globalThis.ReferenceOAReactBidding?.createProductionPersonnelWorkspace?globalThis.ReferenceOAReactBidding.createProductionPersonnelWorkspace({...costOptions,root:productionPersonnelRoot,request:personnelRequest,profile:personnelProfile}):null;
        // Scheme is type-dependent: its child host does not exist before the
        // current Project is read. Keep the stable layout root; the workspace
        // resolves the selected child's host lazily after layout.select().
        const stageRoot=productionRoot;
        const stageRequest=(path,method='GET',body,projectID)=>applicationRequest('production-operations',path,{method,productionStageScope:projectID,...(body?{headers:{'Content-Type':'application/json'},body:JSON.stringify(body)}:{})});
        const productionStage=stageRoot&&stageProfile==='reference-oa-stage-materials-v1'&&globalThis.ReferenceOAReactBidding?.createProductionStageWorkspace?globalThis.ReferenceOAReactBidding.createProductionStageWorkspace({...costOptions,root:stageRoot,request:stageRequest,profile:stageProfile}):null;
        const bonusOptions={...costOptions,profile:bonusProfile,beforeLoad:()=>personnelLease(async()=>{const id=this.projectID;if(!id)return;await authorize();const value=await request('/api/v1/projects/'+id);if(this.projectID===id&&value.project.id===id)this.flow.details.set(id,value)})};
        const bonusRoot=businessRoot.querySelector('[data-real-business-panel="bonus"]'),productionBonusRoot=businessRoot.querySelector('[data-real-production-bonus]');
        const bonus=bonusRoot&&globalThis.ReferenceOAReactBidding?.createBonusWorkspace?globalThis.ReferenceOAReactBidding.createBonusWorkspace({...bonusOptions,root:bonusRoot,scope:'business'}):null;
const productionBonus=productionBonusRoot&&globalThis.ReferenceOAReactBidding?.createBonusWorkspace?globalThis.ReferenceOAReactBidding.createBonusWorkspace({...bonusOptions,root:productionBonusRoot,scope:'production'}):null;
        this.businessWorkspace = new globalThis.ReferenceOABusiness.BusinessWorkspace({root: businessRoot, flow: readers, bidding, personnel, contracts, cash, invoices, costs, productionCosts, approvalAudit, commission, bonus, productionBonus, productionPersonnel, productionStage, productionLayout, details: () => this.flow.details.get(this.projectID), loadClient: explicit => this.loadClient(explicit), selected: (tab, updateURL) => { if (tab !== 'client') this.clientRequest = (this.clientRequest || 0) + 1; if (updateURL) this.setURL(this.projectID, true); }, productionSelected: (_, updateURL) => { if (updateURL) this.setURL(this.projectID, true); }});
        if(productionLayout)productionLayout.selected=tab=>void this.businessWorkspace.production(tab,true);
      }
      this.people = [];
      this.projectID = null;
      this.projects = [];
      this.directory = globalThis.ReferenceOAProjectDirectory;
      this.navigationKey = this.directory.navigationKey(root.dataset.tenantId, root.dataset.subjectId);
      try { this.navigationStorage = globalThis.sessionStorage; } catch { this.navigationStorage = null; }
      this.listState = this.directory.listState();
      this.detailNavigation = null;
      this.main = document.querySelector('.app-main');
      if (history.state?.referenceOA?.view === 'detail' || history.state?.referenceOA?.view === 'projects') this.listState = this.directory.readState(this.navigationStorage, this.navigationKey);
      document.querySelector('[data-initiation-people-reload]')?.addEventListener('click', () => this.loadPeople());
      this.field('[data-initiation-complete-reload]')?.addEventListener('click', async event => {
        if (this.flow.busy) return;
        event.currentTarget.disabled = true;
        try { await this.openCompletion(this.projectID); }
        catch (error) { this.note('complete', '重新加载未完成', error.message + '；表单仍保留，可稍后重试。'); }
        finally { event.target.disabled = false; }
      });
      const search = this.field('[data-real-project-search]');
      if (search) search.value = this.listState.searchText;
      search?.addEventListener('input', () => { this.listState.searchText = search.value; if (!search.value) this.searchProjects(); });
      search?.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); this.searchProjects(); } });
      this.field('[data-real-page-number]')?.addEventListener('change', event => this.changePage(Number(event.target.value)));
      this.field('[data-real-page-number]')?.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); this.changePage(Number(event.target.value)); } });
      this.field('[data-real-page-size]')?.addEventListener('change', event => { this.listState.pageSize = Number(event.target.value); this.changePage(1); });
      const clientName = this.field('[data-real-client-input="name"]');
      for (const input of document.querySelectorAll('[data-real-client-input]')) input.addEventListener('input', () => this.updateClientClearButtons());
      clientName?.addEventListener('focus', () => this.updateClientNameHint());
      clientName?.addEventListener('input', () => this.updateClientNameHint());
      clientName?.addEventListener('blur', () => this.clearClientNameHint());
      clientName?.addEventListener('keydown', event => {
        if (event.defaultPrevented || event.key !== 'Tab' || event.shiftKey || !this.clientNameMatch) return;
        clientName.value = this.clientNameMatch.value;
        this.clearClientNameHint(); // Normal Tab focus movement is intentionally not prevented.
        clientName.dispatchEvent(new Event('input', {bubbles: true}));
      });
      globalThis.addEventListener('popstate', () => this.restoreLocation().catch(error => this.detailError(error)));
      document.addEventListener('click', event => {
        const target = event.target.closest('button');
        if (!target) return;
        if (target.matches('[data-real-new-project]')) this.enterInitiations();
        if (target.matches('[data-real-detail-open]')) this.openDetail(target.dataset.projectId, {history: true}).catch(error => this.status(error.message));
        if (target.matches('[data-real-back-projects]')) {
          if (this.detailNavigation?.parentID) this.openDetail(this.detailNavigation.parentID, {history: true}).catch(error => this.field('[data-real-detail-status]').textContent = error.message);
          else this.returnToList(true);
        }
        if (target.matches('[data-real-list-mode]')) { this.listState.mode = target.dataset.realListMode; this.listState.scrollTop = 0; this.renderProjects(); this.main?.scrollTo({top: 0}); }
        if (target.matches('[data-real-project-search-submit]')) this.searchProjects();
        if (target.matches('[data-real-project-reload]')) this.loadProjects();
        if (target.matches('[data-real-page-prev]')) this.changePage(this.listState.page - 1);
        if (target.matches('[data-real-page-next]')) this.changePage(this.listState.page + 1);
        if (target.matches('[data-real-package-toggle]')) {
          const id = target.dataset.projectId, expanded = new Set(this.listState.expanded);
          expanded.has(id) ? expanded.delete(id) : expanded.add(id); this.listState.expanded = [...expanded]; this.renderProjects();
        }
        if (target.matches('[data-real-year-toggle]')) {
          const year = target.dataset.realYearToggle, collapsed = new Set(this.listState.collapsedYears);
          collapsed.has(year) ? collapsed.delete(year) : collapsed.add(year); this.listState.collapsedYears = [...collapsed]; this.renderProjects();
        }
        if (target.matches('[data-real-year-jump]')) this.field('[data-real-year-section="' + target.dataset.realYearJump + '"]')?.scrollIntoView({block: 'start'});
        if (target.matches('[data-real-profile-open]')) this.openProfile().catch(error => this.field('[data-real-detail-status]').textContent = error.message);
        if (target.matches('[data-real-profile-close]')) this.profileDrawer(false);
        if (target.matches('[data-real-profile-submit]')) this.saveProfile(target);
        if (target.matches('[data-real-profile-reload]')) this.openProfile().catch(error => this.field('[data-real-profile-status]').textContent = error.message);
        if (target.matches('[data-real-detail-tab]')) this.selectDetailTab(target.dataset.realDetailTab, true).catch(error => this.detailError(error));
        if (target.matches('[data-real-client-open]')) this.openClientEditor();
        if (target.matches('[data-real-client-clear]')) {
          const input = this.field('[data-real-client-input="'+target.dataset.realClientClear+'"]');
          if (input) { input.value = ''; input.focus(); input.dispatchEvent(new Event('input', {bubbles: true})); }
        }
        if (target.matches('[data-real-client-cancel]')) this.renderClient(this.flow.contacts.get(this.projectID));
        if (target.matches('[data-real-client-submit]')) this.saveClient(target);
        if (target.matches('[data-real-client-reload]')) this.loadClient(true);
      });
    }
    field(selector) { return document.querySelector(selector); }
    status(message) {
      const node = this.field('[data-real-project-status]');
      if (node) node.textContent = message;
    }
    detailError(error) {
      this.showView('projects');
      this.status('无法打开该项目：'+error.message+'。未创建或修改项目；可以重新读取列表。');
    }
    note(kind, title, message) {
      const note = this.field('[data-initiation-' + kind + '-note]');
      if (note) note.hidden = false;
      const heading = this.field('[data-initiation-' + kind + '-note-title]');
      const copy = this.field('[data-initiation-' + kind + '-note-copy]');
      if (heading) heading.textContent = title;
      if (copy) copy.textContent = message;
    }
    async loadPeople() {
      const select = this.field('[data-initiation-business-owner]');
      const status = this.field('[data-initiation-people-status]');
      if (this.peopleLoading) return this.peopleLoading;
      if (select) select.disabled = true;
      if (status) status.textContent = '正在读取人员目录…';
      this.peopleLoading = (async () => {
        const app = this.root.dataset.peopleApplication, route = this.root.dataset.peopleRoutePrefix;
        if (!app || !route) throw new Error('当前组合未配置人员目录');
        const csrf = await this.csrf.token();
        const authorized = await fetch('/gateway/authorize/' + encodeURIComponent(app), {method: 'POST', credentials: 'same-origin', body: new URLSearchParams({csrf_token: csrf})});
        if (!authorized.ok) throw new Error('没有读取人员目录的权限，或 IAM 暂不可用');
        const response = await fetch(route + '/api/v1/persons', {credentials: 'same-origin', headers: {Accept: 'application/json'}});
        if (!response.ok || response.redirected) throw new Error('人员目录读取失败，请重新加载');
        this.people = normalizePeople(await response.json(), this.root.dataset.tenantId);
        this.peopleAvailable = true;
        this.populatePeople();
        if (status) status.textContent = this.people.length ? '已读取可用负责人' : '尚无可用人员，请先登记人员';
      })().catch(error => {
        this.peopleAvailable = false;
        this.people = [];
        this.populatePeople();
        if (status) status.textContent = error.message;
      }).finally(() => { if (select) select.disabled = !this.people.length; this.peopleLoading = null; });
      return this.peopleLoading;
    }
    populatePeople() {
      for (const selector of ['[data-initiation-business-owner]', '[data-initiation-recovery-business-owner]', '[data-initiation-production-owner]', '[data-real-production-owner]']) {
        const select = this.field(selector);
        if (!select) continue;
        const chosen = select.value, blank = document.createElement('option');
        blank.value = ''; blank.textContent = selector.includes('business') ? '请选择项目经营负责人' : '暂不设置';
        select.replaceChildren(blank);
        for (const person of this.people) {
          const option = document.createElement('option');
          option.value = person.person_id; option.textContent = person.name;
          select.append(option);
        }
        if (selector === '[data-initiation-production-owner]') {
          const current = this.flow.details.get(this.projectID)?.assignments?.find(value => value.role_key === 'production_owner' && !value.ended_at);
          if (current) {
            blank.textContent = '保留当前负责人';
            if (!this.people.some(value => value.person_id === current.subject_id)) {
              const frozen = document.createElement('option'); frozen.value = current.subject_id;
              frozen.textContent = (current.display_snapshot || '现有负责人') + '（现有分工）'; frozen.disabled = true; select.append(frozen);
            }
          }
        }
        if ([...select.options].some(value => value.value === chosen)) select.value = chosen;
      }
    }
    selectedPerson(selector) {
      const id = this.field(selector)?.value;
      if (!id) return null;
      const person = this.people.find(value => value.person_id === id);
      if (!person) throw new Error('人员目录已更新，请重新选择负责人');
      return {person_id: person.person_id, source_revision: person.source_revision};
    }
    async dispatch(button, nodeType) {
      button.disabled = true;
      this.record?.('flow','派发当前项目立项','Workbench 组织输入；Project 拥有项目与负责人分工，People 只提供人员引用。','waiting');
      this.note('create', '正在派发项目立项', '正在保存项目与经营负责人。');
      try {
        const created = await this.flow.dispatch({name: this.field('[data-initiation-create-name]')?.value, node_type: nodeType,
          business_owner: this.selectedPerson('[data-initiation-business-owner]')});
        this.record('project', 'Project 已原子保存待立项任务', '项目和经营负责人分工由同一所有者保存；编号及类型留待补齐。', 'complete');
        this.drawer('create', false);
        this.field('[data-initiation-create-name]').value = '';
        await this.reloadInitiations();
        try { await this.openCompletion(created.id); }
        catch (error) { this.status('项目已创建；打开补全失败：' + error.message + '。请从待立项列表重新打开。'); }
      } catch (error) { this.note('create', '派发未完成', error.message + '；可重试，结果未知时沿用同一命令编号。'); }
      finally { button.disabled = false; }
    }
    async openCompletion(id, options = {}) {
      if (this.flow.busy) throw new Error('正在保存，请等待当前操作完成后重新加载');
      const details = await this.flow.load(id), project = details.project;
      if (project.status === 'active') {
        await this.openDetail(id);
        this.drawer('complete', false);
        return;
      }
      this.projectID = id;
      const values = {'[data-initiation-complete-name]': project.name, '[data-initiation-complete-number]': project.project_number,
        '[data-initiation-complete-summary]': project.summary, '[data-initiation-start-date]': project.planned_start_date?.slice(0, 10) || ''};
      for (const [selector, value] of Object.entries(values)) this.field(selector).value = value || '';
      for (const input of document.querySelectorAll('[data-initiation-complete-type]')) input.checked = project.project_type_keys.includes(input.value);
      this.field('[data-initiation-complete-title]').textContent = project.node_type === 'package' ? '完成项目包立项' : '完成项目立项';
      this.field('[data-initiation-number-label]').textContent = project.node_type === 'package' ? '项目编号（选填）' : '项目编号 *';
      this.field('[data-initiation-complete-note]').hidden = true;
      const production = this.field('[data-initiation-production-owner]');
      const currentProduction = details.assignments?.find(value => value.role_key === 'production_owner' && !value.ended_at);
      this.populatePeople();
      production.value = currentProduction?.subject_id || '';
      production.disabled = !this.flow.can(details, actions.assignment) && !this.flow.can(details, actions.production);
      const missingBusiness = !details.assignments?.some(value => value.role_key === 'business_owner' && !value.ended_at);
      this.field('[data-initiation-business-recovery-field]').hidden = !missingBusiness;
      const business = this.field('[data-initiation-recovery-business-owner]'); business.value = '';
      business.disabled = !this.flow.can(details, actions.assignment) && !this.flow.can(details, actions.business);
      if (missingBusiness) this.note('complete', '请补齐经营负责人', business.disabled ? '该旧项目未设置经营负责人，当前账号没有分工权限；请联系管理员授权后重新加载资料。' : '该旧项目未设置经营负责人，请在下方选择。生产负责人仍可后补。');
      this.field('[data-initiation-complete-validate]').disabled = !this.flow.can(details, actions.complete);
      this.showView('initiation-reference');
      if (options.history !== false) this.setURL(id, new URL(location.href).searchParams.get('project_id') !== id, 'initiation');
      this.drawer('complete', true);
      await this.loadPeople();
    }
    async complete(button) {
      button.disabled = true;
      this.record?.('flow','完成当前项目立项','Project 确认同一项目的立项事实；经营负责人必要，生产负责人可后补。','waiting');
      const id = this.projectID, before = this.flow.details.get(id)?.project.revision;
      this.note('complete', '正在完成立项', '保存资料后完成立项，生产负责人可以后补。');
      try {
        const date = this.field('[data-initiation-start-date]').value;
        const details = this.flow.details.get(id);
        const production = this.field('[data-initiation-production-owner]');
        const currentProduction = details.assignments?.find(value => value.role_key === 'production_owner' && !value.ended_at);
        const business = this.field('[data-initiation-recovery-business-owner]');
        await this.flow.complete(id, {name: this.field('[data-initiation-complete-name]').value,
          project_number: this.field('[data-initiation-complete-number]').value, summary: this.field('[data-initiation-complete-summary]').value,
          planned_start_date: date ? date + 'T00:00:00Z' : null,
          project_type_keys: [...document.querySelectorAll('[data-initiation-complete-type]:checked')].map(input => input.value),
          business_owner: this.field('[data-initiation-business-recovery-field]').hidden || business.disabled ? null : this.selectedPerson('[data-initiation-recovery-business-owner]'),
          production_owner: production.disabled || production.value === currentProduction?.subject_id ? null : this.selectedPerson('[data-initiation-production-owner]')});
        this.record('project', '同一 Project 已完成立项', '经营负责人已确定；生产负责人可以后补，没有进入另一套系统。', 'complete');
        await this.openDetail(id);
        this.drawer('complete', false);
      } catch (error) {
        const failure = this.flow.completionFailure(id, error, before);
        this.note('complete', failure.title, failure.message);
      } finally {
        const details = this.flow.details.get(id);
        button.disabled = details?.project.status === 'active' || !this.flow.can(details, actions.complete);
      }
    }
    saveListContext() {
      this.listState.searchText = this.field('[data-real-project-search]')?.value || '';
      this.listState.scrollTop = this.main?.scrollTop || 0;
      this.directory.saveState(this.navigationStorage, this.navigationKey, this.listState);
    }
    setURL(id, push = false, view = id ? 'detail' : 'projects') {
      const url = new URL(location.href);
      if (id) url.searchParams.set('project_id', id); else url.searchParams.delete('project_id');
      if (view === 'detail' && id) url.searchParams.set('tab', ['business','production'].includes(this.detailTab) ? this.detailTab : 'basic'); else url.searchParams.delete('tab');
      if (view === 'detail' && id && this.detailTab === 'business' && this.businessWorkspace) url.searchParams.set('business_tab', this.businessWorkspace.tab); else url.searchParams.delete('business_tab');
      if (view === 'detail' && id && this.detailTab === 'production' && this.businessWorkspace) url.searchParams.set('production_tab', this.businessWorkspace.productionTab || 'overview'); else url.searchParams.delete('production_tab');
      if (view === 'initiation') url.searchParams.set('view', 'initiation'); else url.searchParams.delete('view');
      const state = {...history.state, referenceOA: {schema_version: '1', view, projectID: id}};
      history[push ? 'pushState' : 'replaceState'](state, '', url);
    }
    enterInitiations(push = true) {
      this.projectID = null;
      this.setURL(null, push, 'initiation');
      this.showView('initiation-reference');
    }
    closeCompletion() {
      this.drawer('complete', false);
      this.enterInitiations(false);
    }
    async restoreLocation() {
      this.drawer('complete', false);
      this.profileDrawer(false);
      const url = new URL(location.href), id = url.searchParams.get('project_id');
      if (url.searchParams.get('view') === 'initiation') {
        this.showView('initiation-reference');
        if (id) await this.openCompletion(id, {history: false});
        else this.projectID = null;
      } else if (id) await this.openDetail(id, {history: false});
      else this.returnToList(false);
    }
    enterProjects() {
      this.listState = this.directory.listState();
      this.directory.saveState(this.navigationStorage, this.navigationKey, this.listState);
      const search = this.field('[data-real-project-search]'); if (search) search.value = '';
      this.restoreScroll = 0; this.setURL(null, Boolean(new URL(location.href).searchParams.get('project_id'))); this.showView('projects');
    }
    returnToList(push) {
      this.listState = this.directory.readState(this.navigationStorage, this.navigationKey);
      const search = this.field('[data-real-project-search]'); if (search) search.value = this.listState.searchText;
      this.restoreScroll = this.listState.scrollTop;
      if (push) this.setURL(null, true);
      this.showView('projects');
    }
    async openDetail(id, options = {}) {
      this.businessWorkspace?.invalidate();
      this.clientRequest = (this.clientRequest || 0) + 1;
      if (id !== this.projectID) this.field('[data-real-client-form]').hidden = true;
      const requestedTab = options.tab || (new URL(location.href).searchParams.get('project_id') === id ? new URL(location.href).searchParams.get('tab') : 'basic');
      const navigation = (this.detailRequest || 0) + 1; this.detailRequest = navigation;
      const details = await this.flow.load(id), project = details.project;
      if (navigation !== this.detailRequest) return;
      // Old bookmarks can name a draft without the new initiation view hint.
      // Restore its original completion workspace, never a second product UI.
      if (['draft', 'pending_completion'].includes(project.status)) return this.openCompletion(id, options);
      const fromList = this.field('[data-view="projects"]')?.classList.contains('is-active');
      if (fromList && options.history !== false) { this.saveListContext(); this.setURL(null); }
      const push = options.history !== false && id !== new URL(location.href).searchParams.get('project_id');
      this.projectID = id;
      this.detailNavigation = {parentID: details.parent?.id || null};
      const owner = role => details.assignments?.find(value => value.role_key === role && !value.ended_at)?.display_snapshot || '待设置';
      const typeNames = {design: '设计', consultation: '咨询', survey: '勘察', mapping: '测绘'};
      const fields = {name: project.name, number: project.project_number || '未编号', date: project.planned_start_date?.slice(0, 10) || '未设置',
        types: project.project_type_keys.map(value => typeNames[value] || value).join('、') || '未设置', summary: project.summary || '未填写', business: owner('business_owner'), production: owner('production_owner')};
      for (const node of document.querySelectorAll('[data-real-detail]')) node.textContent = fields[node.dataset.realDetail] || '—';
      this.field('[data-real-detail-status]').textContent = project.status === 'active' ? '已完成立项，生产负责人可后续补充。经营、生产页面及其他字段正在逐项接通，尚未完成整体仿制。' : '待完成立项；请从项目立项入口补齐资料。';
      this.field('[data-real-profile-open]').disabled = project.status !== 'active' || ![actions.correct, actions.assignment, actions.business, actions.production].some(action => this.flow.can(details, action));
      this.field('[data-real-back-projects]').textContent = details.parent ? '← 返回项目包' : '← 返回项目列表';
      const context = this.field('[data-real-package-context]'); context.replaceChildren();
      const related = [...(details.parent ? [details.parent] : []), ...(details.children || []).filter(value => value.status === 'active')];
      for (const item of related) {
        const button = document.createElement('button'); button.className = 'ant-btn'; button.type = 'button'; button.dataset.realDetailOpen = 'true'; button.dataset.projectId = item.id;
        button.textContent = (item.id === details.parent?.id ? '项目包：' : '子项目：') + item.name; context.append(button);
      }
      context.hidden = !related.length;
      this.showView('project-real-detail');
      await this.selectDetailTab(requestedTab, false);
      if (options.history !== false) this.setURL(id, push);
    }
    async selectDetailTab(tab, updateURL) {
      this.clearClientNameHint();
      const details = this.flow.details.get(this.projectID);
      const canRead = this.flow.can(details, actions.contactRead);
      this.field('[data-real-detail-tab="business"]').disabled = this.businessWorkspace ? false : !canRead;
      this.detailTab = tab === 'production' && this.businessWorkspace ? 'production' : tab === 'business' && (this.businessWorkspace || canRead) ? 'business' : 'basic';
      for (const button of document.querySelectorAll('[data-real-detail-tab]')) {
        const selected = button.dataset.realDetailTab === this.detailTab; button.classList.toggle('project-detail-page__primary-tab--active', selected); button.setAttribute('aria-selected', String(selected));
      }
      for (const panel of document.querySelectorAll('[data-real-detail-panel]')) panel.hidden = panel.dataset.realDetailPanel !== this.detailTab;
      if (this.detailTab === 'business' && this.businessWorkspace) {
        const url = new URL(location.href);
        const subtab = url.searchParams.get('project_id') === this.projectID ? url.searchParams.get('business_tab') || 'overview' : 'overview';
        const work = this.businessWorkspace.select(subtab, false);
        if (updateURL) this.setURL(this.projectID, true);
        await work;
      } else if (this.detailTab === 'production') {
        const url = new URL(location.href);
        const subtab = url.searchParams.get('project_id') === this.projectID ? url.searchParams.get('production_tab') || 'overview' : 'overview';
        const work = this.businessWorkspace.production(subtab, false);
        if (updateURL) this.setURL(this.projectID, true);
        await work;
      } else {
        if (updateURL) this.setURL(this.projectID, true);
        if (this.detailTab === 'business') await this.loadClient();
      }
    }
    clientStatus(text) { this.field('[data-real-client-status]').textContent = text; }
    updateClientClearButtons() {
      for (const key of ['name','contact_name','contact_phone']) this.field('[data-real-client-clear="'+key+'"]').hidden = !this.field('[data-real-client-input="'+key+'"]').value;
    }
    clearClientNameHint() {
      clearTimeout(this.clientNameTimer); this.clientNameRequest = (this.clientNameRequest || 0) + 1; this.clientNameMatch = null;
      const hint = this.field('[data-real-client-name-hint]'); if (hint) { hint.hidden = true; hint.replaceChildren(); }
    }
    updateClientNameHint() {
      this.clearClientNameHint();
      const input = this.field('[data-real-client-input="name"]'), value = input?.value || '', id = this.projectID, request = this.clientNameRequest;
      if (!value.trim() || document.activeElement !== input || this.field('[data-real-client-form]').hidden) return;
      this.clientNameTimer = setTimeout(async () => {
        try {
          const names = await this.flow.suggestClientNames(value);
          if (request !== this.clientNameRequest || id !== this.projectID || input.value !== value || document.activeElement !== input || this.field('[data-real-client-form]').hidden) return;
          const match = nameCompletion(value, names); if (!match) return;
          this.clientNameMatch = match;
          const hint = this.field('[data-real-client-name-hint]'), typed = document.createElement('span'), suffix = document.createElement('em'), key = document.createElement('kbd');
          typed.textContent = value; suffix.textContent = match.suffix; key.textContent = 'Tab'; hint.replaceChildren(typed, suffix, key); hint.hidden = false;
        } catch (_) { /* Suggestions never block manual entry or submit a write. */ }
      }, 150);
    }
    async loadClient(explicit = false) {
      this.clearClientNameHint();
      const id = this.projectID, request = (this.clientRequest || 0) + 1; this.clientRequest = request;
      this.clientLoadedAt ||= new Map(); this.clientInflight ||= new Map();
      const cached=this.flow.contacts.get(id),fresh=cached&&Date.now()-(this.clientLoadedAt.get(id)||0)<30000;
      if(!explicit&&fresh){this.renderClient(cached);return;}
      if(cached){this.renderClient(cached);this.clientStatus('正在后台更新甲方信息；当前先显示上次结果。');}
      else{this.field('[data-real-client-open]').hidden = true; this.field('[data-real-client-readonly]').hidden = true; this.field('[data-real-client-form]').hidden = true;this.clientStatus('正在读取甲方信息…');}
      try {
        let task=this.clientInflight.get(id);
        if(!task){task=Promise.resolve().then(()=>this.flow.loadCounterparty(id)).then(value=>{this.clientLoadedAt.set(id,Date.now());return value;}).finally(()=>{if(this.clientInflight.get(id)===task)this.clientInflight.delete(id);});this.clientInflight.set(id,task);}
        const value = await task;
        if (id !== this.projectID || request !== this.clientRequest || this.detailTab !== 'business') return;
        this.renderClient(value);
      } catch (error) {
        if (id === this.projectID && request === this.clientRequest) { this.clientStatus(cached?'后台更新失败；当前仍显示上次甲方信息，可手动刷新。':'读取失败：' + error.message + '。这不表示项目没有甲方资料。'); this.field('[data-real-client-reload]').hidden = false; }
      }
    }
    renderClient(value) {
      if (!value || value.project_id !== this.projectID) return;
      this.clearClientNameHint();
      this.clientConflicted = false;
      const contact = value.context;
      this.field('[data-real-client-form]').hidden = true;
      this.field('[data-real-client-readonly]').hidden = !contact;
      const button = this.field('[data-real-client-open]'); button.hidden = value.project_status !== 'active' || !value.allowed_actions.includes(actions.contactManage); button.textContent = contact ? '✎ 编辑' : '＋ 录入甲方信息';
      for (const key of ['name','contact_name','contact_phone']) this.field('[data-real-client-value="'+key+'"]').textContent = contact?.[key] || '—';
      const party = this.flow.details.get(this.projectID)?.project.client_party;
      this.field('[data-real-client-identity]').textContent = party ? '已验证主体快照：' + party.legal_name + '（与本项目联系资料分别保存；编辑此处不会改绑主体）' : '';
      this.field('[data-real-client-reload]').hidden = true;
      this.clientStatus(contact ? '' : '尚未录入甲方信息，可先完善甲方资料，再继续合同、支付与开票。');
    }
    openClientEditor() {
      const value = this.flow.contacts.get(this.projectID);
      if (!value?.allowed_actions?.includes(actions.contactManage) || value.project_status !== 'active') return;
      this.clientEditID = this.projectID; this.clientConflicted = false;
      for (const key of ['name','contact_name','contact_phone']) this.field('[data-real-client-input="'+key+'"]').value = value.context?.[key] || '';
      this.updateClientClearButtons();
      this.field('[data-real-client-form]').hidden = false; this.field('[data-real-client-readonly]').hidden = true; this.field('[data-real-client-open]').hidden = true;
      this.field('[data-real-client-submit]').disabled = false; this.field('[data-real-client-reload]').hidden = true; this.clientStatus(''); this.field('[data-real-client-input="name"]').focus();
    }
    async saveClient(button) {
      if (this.clientConflicted) return;
      this.record?.('flow','保存项目甲方联系信息','Project 保存本项目联系快照，不改写 Party 主体当前档案。','waiting');
      const id = this.clientEditID;
      button.disabled = true; this.field('[data-real-client-cancel]').disabled = true; this.clientStatus('正在保存甲方信息…');
      try {
        const fields = {}; for (const key of ['name','contact_name','contact_phone']) fields[key] = this.field('[data-real-client-input="'+key+'"]').value;
        await this.flow.saveCounterparty(id, fields);
        this.record('project','甲方资料已保存到当前项目','Project 保存项目专属名称和联系人快照；未调用或修改 Party 主档。','complete');
        try {
          const value = await this.flow.loadCounterparty(id);
          if (id === this.projectID) { this.clientLoadedAt ||= new Map();this.clientLoadedAt.set(id,Date.now());this.renderClient(value); this.clientStatus('甲方名称已保存到当前项目'); }
        } catch (error) {
          if (id === this.projectID) { this.clientConflicted = true; this.field('[data-real-client-reload]').hidden = false; this.clientStatus('资料已保存，复读失败：'+error.message+'。请重新读取，不要再次提交。'); }
        }
      } catch (error) {
        if (id === this.projectID) { this.clientConflicted = error.status === 409; this.field('[data-real-client-reload]').hidden = !this.clientConflicted; this.clientStatus(error.message + (this.clientConflicted ? '；当前输入保留，请重新读取最新资料后确认。' : '；当前输入保留，可重试。')); }
      } finally { button.disabled = this.clientConflicted; this.field('[data-real-client-cancel]').disabled = false; }
    }
    profileDrawer(open) {
      const mask = this.field('[data-real-profile-drawer]');
      if (open && mask.parentElement !== document.body) document.body.append(mask);
      mask.classList.toggle('is-open', open);
      if (open) this.field('[data-real-profile-name]').focus();
      else this.field('[data-real-profile-open]')?.focus();
    }
    async openProfile() {
      const id = this.projectID, details = await this.flow.load(id), project = details.project;
      if (project.status !== 'active' || ![actions.correct, actions.assignment, actions.business, actions.production].some(action => this.flow.can(details, action))) throw new Error('当前账号没有编辑该项目基本信息的权限，或项目状态已变化');
      this.editProjectID = id;
      this.editConflicted = false;
      const values = {name: project.name, number: project.project_number, summary: project.summary, date: project.planned_start_date?.slice(0, 10) || '', reason: '修改项目基本资料'};
      for (const [key, value] of Object.entries(values)) this.field('[data-real-profile-' + key + ']').value = value || '';
      const core = this.flow.can(details, actions.correct);
      for (const key of ['name','number','summary','date']) this.field('[data-real-profile-' + key + ']').disabled = !core;
      this.field('[data-real-profile-number-label]').textContent = project.node_type === 'package' ? '项目编号（选填）' : '项目编号 *';
      const types = this.field('[data-real-profile-types]');
      types.replaceChildren();
      const labels = {design: '设计', consultation: '咨询', survey: '勘察', mapping: '测绘'};
      for (const key of new Set([...Object.keys(labels), ...project.project_type_keys])) {
        const label = document.createElement('label'), input = document.createElement('input'), text = document.createElement('span');
        input.type = 'checkbox'; input.value = key; input.dataset.realProfileType = 'true'; input.checked = project.project_type_keys.includes(key); input.disabled = !core;
        text.textContent = labels[key] || key; label.append(input, text); types.append(label);
      }
      await this.loadPeople();
      for (const [role,action,key] of [['business_owner',actions.business,'business'],['production_owner',actions.production,'production']]) {
        const select = this.field('[data-real-profile-' + key + '-owner]'); select.replaceChildren();
        const blank = document.createElement('option'); blank.value = ''; blank.textContent = '不设置 / 清除当前负责人'; select.append(blank);
        for (const person of this.people) { const option = document.createElement('option'); option.value = person.person_id; option.textContent = person.name; select.append(option); }
        const owner = details.assignments?.find(value => value.role_key === role && !value.ended_at);
        if (owner && !this.people.some(value => value.person_id === owner.subject_id)) { const option = document.createElement('option'); option.value = owner.subject_id; option.textContent = owner.display_snapshot + '（原分工）'; select.append(option); }
        select.value = owner?.subject_id || '';
        select.disabled = !this.peopleAvailable || (!this.flow.can(details,actions.assignment) && !this.flow.can(details,action));
      }
      this.field('[data-real-profile-status]').textContent = this.peopleAvailable ? '修改后保留历史；基本资料与两类负责人分别授权。' : '人员目录不可用，不能调整负责人；有权限的基本资料仍可保存。';
      this.field('[data-real-profile-reload]').hidden = true;
      this.field('[data-real-profile-submit]').disabled = false;
      this.profileDrawer(true);
    }
    async saveProfile(button) {
      if (this.editConflicted) return;
      this.record?.('flow','更正项目基本资料和分工','Project 自有事实追加更正与分工历史，外部业务事实不被覆盖。','waiting');
      button.disabled = true;
      const id = this.editProjectID;
      this.field('[data-real-profile-status]').textContent = '正在保存基本资料…';
      try {
        const date = this.field('[data-real-profile-date]').value;
        const details = this.flow.details.get(id), owners = [];
        for (const [role,key] of [['business_owner','business'],['production_owner','production']]) {
          const select = this.field('[data-real-profile-' + key + '-owner]');
          const current = details.assignments?.find(value => value.role_key === role && !value.ended_at);
          if (!select.disabled && select.value !== (current?.subject_id || '')) owners.push(select.value ? {role_key:role,operation:'set',person:this.selectedPerson('[data-real-profile-' + key + '-owner]')} : {role_key:role,operation:'clear'});
        }
        const fields = this.flow.can(details,actions.correct) ? {name: this.field('[data-real-profile-name]').value,
          project_number: this.field('[data-real-profile-number]').value, summary: this.field('[data-real-profile-summary]').value,
          planned_start_date: date ? date + 'T00:00:00Z' : null,
          project_type_keys: [...document.querySelectorAll('[data-real-profile-type]:checked')].map(input => input.value),
          correction_reason: this.field('[data-real-profile-reason]').value} : null;
        await this.flow.changeBasicInfo(id,fields,owners);
        this.record('project', 'Project 基本信息已保存', '同一项目原子更正自有资料和负责人分工；保留旧分工历史，没有改写外部事实。', 'complete');
        try {
          await this.openDetail(id);
          this.profileDrawer(false);
          this.field('[data-real-detail-status]').textContent = '基本信息保存成功';
        } catch (error) {
          this.editConflicted = true;
          this.field('[data-real-profile-reload]').hidden = false;
          this.field('[data-real-profile-status]').textContent = '资料已保存，复读失败：' + error.message + '。请重新加载，不要再次提交。';
        }
      } catch (error) {
        this.editConflicted = error.status === 409;
        this.field('[data-real-profile-reload]').hidden = !this.editConflicted;
        this.field('[data-real-profile-status]').textContent = error.message + (this.editConflicted ? '；请重新加载最新资料后确认，当前输入暂时保留。' : '；当前输入保留，可重试。');
      } finally { button.disabled = this.editConflicted; }
    }
    async loadProjects() {
      this.directoryRequested = true;
      if (this.projectsLoading) return this.projectsLoading;
      this.projectsLoading = (async () => {
        while (this.directoryRequested) {
          this.directoryRequested = false;
          const keyword = this.listState.keyword;
          this.status('正在读取当前权限范围内的项目…');
          try {
            await this.flow.authorize();
            const items = await this.directory.readDirectory(this.flow.request, this.root.dataset.tenantId, keyword, (count, total) => this.status('正在读取项目 ' + count + ' / ' + total));
            if (keyword !== this.listState.keyword) continue;
            this.projects = items;
            if (keyword) this.listState.expanded = [...new Set([...this.listState.expanded, ...items.filter(value => value.children.length).map(value => value.id)])];
            this.renderProjects();
            if (this.restoreScroll !== undefined) { const top = this.restoreScroll; this.restoreScroll = undefined; requestAnimationFrame(() => this.main?.scrollTo({top, behavior: 'instant'})); }
          } catch (error) { this.status(error.message + '；原列表保留，请重新读取。'); }
        }
      })().finally(() => { this.projectsLoading = null; });
      return this.projectsLoading;
    }
    searchProjects() {
      this.listState.searchText = this.field('[data-real-project-search]').value;
      this.listState.keyword = this.listState.searchText.trim(); this.listState.page = 1; this.restoreScroll = 0;
      this.loadProjects();
    }
    changePage(page) {
      this.listState.page = Math.max(1, Math.min(Number.isSafeInteger(page) ? page : 1, Math.max(1, Math.ceil(this.projects.length / this.listState.pageSize))));
      this.renderProjects(); this.main?.scrollTo({top: 0});
    }
    projectCard(project, child = false) {
      const fragment = document.createDocumentFragment(), article = document.createElement('article'), button = document.createElement('button'), title = document.createElement('h3'), number = document.createElement('span'), meta = document.createElement('span');
      article.className = 'project-list-page__item real-directory-item' + (child ? ' project-list-page__item--child' : ''); button.className = 'project-list-page__item-primary';
      button.type = 'button'; button.dataset.realDetailOpen = 'true'; button.dataset.projectId = project.id;
      title.className = 'project-list-page__name'; title.textContent = project.name;
      number.className = 'project-list-page__number'; number.textContent = '项目编号：' + (project.project_number || '未编号（项目包选填）');
      const owner = role => project.assignments.find(value => value.role_key === role)?.display_snapshot || '待设置';
      meta.className = 'real-directory-meta'; meta.textContent = '承接日期：' + (project.planned_start_date?.slice(0, 10) || '未设置') + ' · 经营负责人：' + owner('business_owner') + ' · 生产负责人：' + owner('production_owner');
      button.append(title, number, meta); article.append(button);
      if (project.node_type === 'package') {
        const expanded = this.listState.expanded.includes(project.id), toggle = document.createElement('button');
        toggle.type = 'button'; toggle.className = 'ant-btn real-directory-expand'; toggle.dataset.realPackageToggle = 'true'; toggle.dataset.projectId = project.id;
        toggle.setAttribute('aria-expanded', String(expanded)); toggle.textContent = (expanded ? '收起' : '展开') + ' ' + project.children.length + ' 个子项目'; article.append(toggle);
      }
      fragment.append(article);
      if (this.listState.expanded.includes(project.id)) {
        const children = document.createElement('div'); children.className = 'project-list-page__children';
        for (const item of project.children) children.append(this.projectCard(item, true)); fragment.append(children);
      }
      return fragment;
    }
    renderProjects() {
      const list = this.field('[data-real-project-list]');
      if (!list) return;
      list.replaceChildren();
      for (const button of document.querySelectorAll('[data-real-list-mode]')) { const active = button.dataset.realListMode === this.listState.mode; button.classList.toggle('ant-segmented-item--selected', active); button.setAttribute('aria-pressed', String(active)); }
      const total = this.projects.length, pages = Math.max(1, Math.ceil(total / this.listState.pageSize)); this.listState.page = Math.min(this.listState.page, pages);
      if (this.listState.mode === 'paged') {
        const start = (this.listState.page - 1) * this.listState.pageSize;
        for (const project of this.projects.slice(start, start + this.listState.pageSize)) list.append(this.projectCard(project));
        this.field('[data-real-page-range]').textContent = (total ? start + 1 : 0) + '–' + Math.min(total, start + this.listState.pageSize) + ' / 共 ' + total + ' 个项目';
      } else {
        const documentNode = document.createElement('div'), outline = document.createElement('nav'), sections = document.createElement('div');
        documentNode.className = 'project-list-page__timeline-document'; outline.className = 'project-list-page__year-outline-sticky'; outline.setAttribute('aria-label', '按年份定位项目');
        for (const group of this.directory.yearGroups(this.projects)) {
          const key = group.year === null ? 'unassigned' : String(group.year), label = group.year === null ? '未按年份编号' : group.year + ' 年';
          const jump = document.createElement('button'), section = document.createElement('section'), heading = document.createElement('button'), items = document.createElement('div');
          jump.type = 'button'; jump.className = 'project-list-page__year-outline-item'; jump.dataset.realYearJump = key; jump.textContent = group.year === null ? '未编号' : group.year; outline.append(jump);
          section.className = 'project-list-page__year-section'; section.dataset.realYearSection = key;
          heading.type = 'button'; heading.className = 'real-directory-year-heading'; heading.dataset.realYearToggle = key; heading.textContent = label + ' · ' + group.projects.length + ' 个项目'; heading.setAttribute('aria-expanded', String(!this.listState.collapsedYears.includes(key)));
          items.className = 'project-list-page__list'; items.hidden = this.listState.collapsedYears.includes(key);
          for (const project of group.projects) items.append(this.projectCard(project)); section.append(heading, items); sections.append(section);
        }
        documentNode.append(outline, sections); list.append(documentNode);
      }
      this.field('[data-real-pagination]').hidden = this.listState.mode !== 'paged';
      this.field('[data-real-page-number]').value = this.listState.page; this.field('[data-real-page-number]').max = pages;
      this.field('[data-real-page-size]').value = this.listState.pageSize;
      this.field('[data-real-page-prev]').disabled = this.listState.page <= 1; this.field('[data-real-page-next]').disabled = this.listState.page >= pages;
      this.status(total ? '共 ' + total + ' 个已立项项目；项目包只计一项，子项目可展开。经营／生产进度投影仍待接通。' : (this.listState.keyword ? '暂无符合条件的项目' : '暂无已立项项目，请从项目立项入口创建。'));
    }
  }
  globalThis.ReferenceOAInitiationFlow = InitiationFlow;
  globalThis.ReferenceOAGatewayCSRF = GatewayCSRF;
  globalThis.referenceOANameCompletion = nameCompletion;
  globalThis.normalizeReferenceOAPeople = normalizePeople;
  globalThis.ReferenceOAInitiationController = InitiationController;
})();
