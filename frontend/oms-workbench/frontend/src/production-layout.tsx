// Reused visual shell from user-owned ProductionInfo.tsx (TabLabel,
// withTabScroll, productionTabItems and summary/Tabs render). Original CSS is
// verbatim in vendor. Native ports replace the monolith's queries/children;
// unknown statistics MUST NOT inherit the original `|| 0` fallback.
import React, {type ReactNode} from 'react'
import {createRoot, type Root} from 'react-dom/client'
import {flushSync} from 'react-dom'
import {Button, ConfigProvider, Empty, Tabs, Tooltip} from 'antd'
import zhCN from 'antd/locale/zh_CN'
import {AppstoreOutlined, TeamOutlined, AuditOutlined, BulbOutlined, GoldOutlined, FileTextOutlined, SwapOutlined, ReadOutlined, DollarOutlined, ApartmentOutlined, GiftOutlined} from '@ant-design/icons'
import {productionTabs, productionTab} from './production-tabs.mjs'
import '../vendor/components/project/ProjectWorkspace.css'
import '../vendor/components/production/ProductionInfo.css'
import '../vendor/components/production/StageFilePanel.css'

const TabLabel = ({icon, label, group='default', groupStart=false}: {
  icon:ReactNode;label:string;group?:string;groupStart?:boolean
}) => (
  <span className={[
    'project-workspace__tab-label',
    `project-workspace__tab-label--${group}`,
    groupStart ? 'project-workspace__tab-label--group-start' : '',
  ].filter(Boolean).join(' ')}>
    <span className="project-workspace__tab-label-icon" aria-hidden="true">{icon}</span>
    <span className="project-workspace__tab-label-text">{label}</span>
  </span>
)
const withTabScroll = (content:ReactNode) => <div className="project-workspace__tab-scroll">{content}</div>
const OverviewRow = ({stage,label}:{stage:string;label:string}) => <div className="production-info__overview-row" data-real-production-overview-stage={stage} hidden>
  <span className="production-info__overview-key">{label}</span>
  <strong className="production-info__overview-value production-info__overview-value--pending" data-real-production-overview-stage-value={stage}>未读取</strong>
</div>
const icons:Record<string,ReactNode> = {
  overview:<AppstoreOutlined/>,personnel:<TeamOutlined/>, 'approval-audit':<AuditOutlined/>,
  scheme:<BulbOutlined/>,preliminary:<GoldOutlined/>,construction:<FileTextOutlined/>,
  'change-completion':<SwapOutlined/>,test:<FileTextOutlined/>,consultation:<ReadOutlined/>,
  cost:<DollarOutlined/>,commission:<ApartmentOutlined/>,bonus:<GiftOutlined/>,
}
const pendingStages:Record<string,{title:string;mainFileLabel:string;subtitle:string;tone:'scheme'|'preliminary'|'construction'}> = {
  scheme:{title:'方案阶段',mainFileLabel:'方案文件',subtitle:'维护方案文件，校审单与评分可按实际情况补充。',tone:'scheme'},
  preliminary:{title:'初步设计',mainFileLabel:'初步设计文件',subtitle:'维护初步设计文件，校审单与评分可按实际情况补充。',tone:'preliminary'},
  construction:{title:'施工图设计',mainFileLabel:'施工图设计文件',subtitle:'维护施工图设计文件，校审单与评分可按实际情况补充。',tone:'construction'},
  test:{title:'试验阶段资料',mainFileLabel:'勘察试验阶段资料',subtitle:'勘察项目专用；保存有效资料后由生产系统计算阶段状态。',tone:'preliminary'},
  consultation:{title:'咨询报告',mainFileLabel:'咨询报告附件',subtitle:'集中维护咨询成果文件及其 Evidence 版本。',tone:'construction'},
}
function PendingStagePanel({stageKey}:{stageKey:string}) {
  const stage=pendingStages[stageKey]
  if(!stage)return null
  return <section className={`project-workspace__card stage-file-panel stage-file-panel--${stage.tone}`} data-oa-stage-surface={stageKey}>
    <header className="stage-file-panel__header">
      <div><h3 className="project-workspace__card-title">{stage.title}</h3><p className="stage-file-panel__subtitle">{stage.subtitle}</p></div>
      <Tooltip title="原生阶段资料端口尚未实现；不能用方案阶段接口冒充写入">
        <Button disabled>编辑阶段资料</Button>
      </Tooltip>
    </header>
    <div className="stage-file-panel__summary">
      <div className="stage-file-panel__summary-item stage-file-panel__summary-item--state"><span>阶段状态</span><strong>—</strong><em>尚未读取，不代表未开始</em></div>
      <div className="stage-file-panel__summary-item stage-file-panel__summary-item--progress"><span>文件进度</span><strong>—</strong><em>原生端口待接通</em></div>
      <div className="stage-file-panel__summary-item stage-file-panel__summary-item--time"><span>最近维护</span><strong>—</strong><em>不显示伪造时间</em></div>
    </div>
    <div className="stage-file-panel__body">
      <div className="stage-file-panel__files-stack">
        <section className="stage-file-panel__section stage-file-panel__section--main"><div className="stage-file-panel__section-title"><span>{stage.mainFileLabel}</span><em>未读取</em></div><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="原生阶段资料接线待完成；当前不把缺少接口解释为空数据"/></section>
        <section className="stage-file-panel__section stage-file-panel__section--review"><div className="stage-file-panel__section-title"><span>校审单</span><em>未读取</em></div><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="校审资料未读取"/></section>
      </div>
      <aside className="stage-file-panel__score-card"><div className="stage-file-panel__score-label">阶段评分</div><div className="stage-file-panel__score"><strong>—</strong></div><p>评分未读取，不显示 0，也不复用其他阶段事实。</p></aside>
    </div>
  </section>
}
function ChangeCompletionPending() {
  return <div className="project-workspace__stack" data-oa-stage-surface="change-completion">
    <PendingStagePanel stageKey="change"/>
    <PendingStagePanel stageKey="completion"/>
  </div>
}
pendingStages.change={title:'变更洽商',mainFileLabel:'变更洽商文件',subtitle:'设计变更、现场洽商及相关确认材料。',tone:'preliminary'}
pendingStages.completion={title:'竣工验收',mainFileLabel:'竣工验收文件',subtitle:'竣工报告、验收文件及最终归档材料。',tone:'construction'}
function Layout({workspace}:{workspace:ProductionLayout}) {
  const productionTabItems = workspace.tabs().map(tab => ({
    key:tab.key,
    label:<Tooltip title={!tab.ready ? '原 OA 此栏目未装入' : tab.native===false ? '原 OA 页面已开放；原生业务数据端口待接通' : undefined}>
      <TabLabel icon={icons[tab.key]} label={tab.label} group={tab.group} groupStart={tab.groupStart}/>
    </Tooltip>,
    disabled:!tab.ready,
    // Stable, eagerly mounted DOM hosts for independent native React roots.
    // forceRender creates hosts only: it must not load hidden owner data.
    forceRender:true,
    children:withTabScroll(tab.key==='cost'
      ? <section data-real-production-cost><div data-real-cost-react/></section>
      : tab.key==='approval-audit'
      ? <section data-real-production-approval-audit><div data-real-approval-audit-react/></section>
      : tab.key==='commission'
      ? <section data-real-production-commission><div data-real-commission-react/></section>
      : tab.key==='bonus'
      ? <section data-real-production-bonus><div data-real-bonus-react/></section>
      : tab.key==='personnel'
      ? <section data-real-production-personnel><div data-real-production-personnel-react/></section>
      : ['scheme','preliminary','construction','test','consultation'].includes(tab.key) && tab.native
      ? <section data-real-production-stage={tab.key}><div data-real-production-stage-react={tab.key}/></section>
      : tab.key==='change-completion' && tab.native
      ? <section data-real-production-stage="change-completion"><div data-real-production-stage-react="change-completion"/></section>
      : tab.key==='change-completion'
      ? <ChangeCompletionPending/>
      : pendingStages[tab.key]
      ? <PendingStagePanel stageKey={tab.key}/>
      : tab.key==='overview'
      ? <div className="project-workspace__stack">
          <div className="project-workspace__grid">
            <div className="project-workspace__grid-item--8">
              <section className="project-workspace__card production-info__mini-card">
                <header className="project-workspace__card-header">
                  <div><h3 className="project-workspace__card-title">生产状态</h3><p className="project-workspace__card-subtitle">按 Production 自有任务、适用阶段和有效资料一次计算</p></div>
                  <button className="ant-btn" type="button" data-real-production-reload>刷新</button>
                </header>
                <div className="production-info__overview-grid">
                  <section className="production-info__overview-section">
                    <header className="production-info__overview-section-header"><div className="production-info__overview-section-title">阶段资料</div><span className="production-info__overview-badge production-info__overview-badge--pending" data-real-production-overview-badge>未读取</span></header>
                    <div className="production-info__overview-list">
                      <OverviewRow stage="scheme" label="方案阶段"/><OverviewRow stage="preliminary-design" label="初步设计"/><OverviewRow stage="construction-drawing" label="施工图设计"/><OverviewRow stage="test" label="试验阶段"/><OverviewRow stage="report" label="咨询／勘察／测绘报告"/><OverviewRow stage="change" label="变更洽商"/><OverviewRow stage="completion" label="竣工验收"/>
                    </div>
                  </section>
                  <section className="production-info__overview-section">
                    <header className="production-info__overview-section-header"><div className="production-info__overview-section-title">各类型进度</div><span className="production-info__overview-badge production-info__overview-badge--ready">同一事实</span></header>
                    <div className="production-info__overview-list"><OverviewRow stage="type-consultation" label="咨询"/><OverviewRow stage="type-design" label="设计"/><OverviewRow stage="type-mapping" label="测绘"/><OverviewRow stage="type-survey" label="勘察"/></div>
                  </section>
                </div>
                <p role="status" data-real-production-status/>
                <div data-real-production-content hidden/>
              </section>
            </div>
            <div className="project-workspace__grid-item--4">
              <section className="project-workspace__card production-info__mini-card production-info__mini-card--hint">
                <header className="project-workspace__card-header"><div><h3 className="project-workspace__card-title">阶段入口</h3><p className="project-workspace__card-subtitle">沿用原 OA 栏目，进入后维护该阶段资料</p></div></header>
                <div className="production-info__next-steps">{workspace.tabs().filter(item=>['scheme','preliminary','construction','change-completion','test','consultation'].includes(item.key)&&item.ready).map(item=><button type="button" className="production-info__quick-entry" key={item.key} onClick={()=>workspace.selected?.(item.key)}><div><strong>{item.label}</strong><span>查看当前项目的原生记录</span></div><em>进入</em></button>)}</div>
              </section>
            </div>
          </div>
        </div>
      : null),
  }))
  return <div className="project-workspace">
    <section className="project-workspace__summary project-workspace__summary--production">
      <div className="project-workspace__summary-header">
        <div><h2 className="project-workspace__summary-title">生产概览</h2></div>
        <div className="project-workspace__summary-meta">
          <div className="project-workspace__summary-pill">生产进度 <span data-real-production-summary-progress>未读取</span></div>
          <div className="project-workspace__summary-pill">批复状态 未读取</div>
          <div className="project-workspace__summary-pill">审计状态 未读取</div>
        </div>
      </div>
      <div className="project-workspace__summary-stats production-info__summary-stats">
        <div className="project-workspace__stat production-info__stat production-info__stat--stage production-info__stat--group-end">
          <div className="project-workspace__stat-label">生产阶段</div>
          <div className="project-workspace__stat-value production-info__stat-text" data-real-production-summary-state>—</div>
          <div className="project-workspace__stat-note" data-real-production-summary-note>尚未读取，不代表未开始</div>
        </div>
        <div className="project-workspace__stat production-info__stat production-info__stat--review">
          <div className="project-workspace__stat-label">批复金额</div><div className="project-workspace__stat-value">—</div>
          <div className="project-workspace__stat-note">未读取，不代表零</div>
        </div>
        <div className="project-workspace__stat production-info__stat production-info__stat--review production-info__stat--group-end">
          <div className="project-workspace__stat-label">审计金额</div><div className="project-workspace__stat-value">—</div>
          <div className="project-workspace__stat-note">未读取，不代表零</div>
        </div>
        {workspace.applications.includes('cost-outsourcing') && <div className="project-workspace__stat production-info__stat production-info__stat--finance">
          <div className="project-workspace__stat-label">对外支付</div><div className="project-workspace__stat-value">—</div>
          <div className="project-workspace__stat-note">委托支付汇总尚未接通</div>
        </div>}
        {(workspace.applications.includes('cost-outsourcing') || workspace.applications.includes('bonus-distribution')) && <div className="project-workspace__stat production-info__stat production-info__stat--finance">
          <div className="project-workspace__stat-label">生产成本总计</div><div className="project-workspace__stat-value">—</div>
          <div className="project-workspace__stat-note">各栏目按需读取，不以缺失金额计算合计</div>
        </div>}
      </div>
    </section>
    <div className="project-workspace__panel">
      <Tabs activeKey={workspace.tab} className="project-workspace__tabs" onChange={key=>workspace.selected?.(key)} items={productionTabItems}/>
    </div>
  </div>
}
class ProductionLayout {
  root:Root;tab='overview';types:string[]=[];applications:string[];personnelProfile:string;stageProfile:string;selected?: (key:string)=>void
  constructor(options:{root:Element;applications:string[];personnelProfile?:string;stageProfile?:string}) {
    this.applications=options.applications
    this.personnelProfile=options.personnelProfile||''
    this.stageProfile=options.stageProfile||''
    this.root=createRoot(options.root)
    this.render()
  }
  tabs(){return productionTabs(this.types,this.applications,this.personnelProfile,this.stageProfile)}
  select(key:string,details:any) {
    this.types=Array.isArray(details?.project?.project_type_keys) ? details.project.project_type_keys : []
    this.tab=productionTab(key,this.types,this.applications,this.personnelProfile,this.stageProfile)
    this.render()
    return this.tab
  }
  render(){flushSync(()=>this.root.render(<ConfigProvider locale={zhCN}><Layout workspace={this}/></ConfigProvider>))}
}
export const createProductionLayout=(options:{root:Element;applications:string[];personnelProfile?:string;stageProfile?:string})=>new ProductionLayout(options)
