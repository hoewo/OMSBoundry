package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomeHidesTechnicalApplications(t *testing.T) {
	hidden := false
	manifest := suite{
		SchemaVersion: "1",
		SuiteID:       "test-suite",
		TenantID:      "77777777-7777-4777-8777-777777777777",
		Applications: []application{
			{ID: "project-workspace", Version: "0.1.0", Enabled: true, RoutePrefix: "/project", EntryAction: "project-workspace.read"},
			{ID: "contract-ledger", Version: "0.1.0", Enabled: true, EntryAction: "contract-ledger.view"},
			{ID: "business-engagement", Version: "0.1.0", Enabled: true, EntryAction: "business-engagement.read"},
			{ID: "billing-invoice", Version: "0.1.0", Enabled: true, EntryAction: "billing-invoice.read"},
			{ID: "money-flow-ledger", Version: "0.1.0", Enabled: true, RoutePrefix: "/money", EntryAction: "money-flow-ledger.view", MetricProjection: &metricProjection{SchemaVersion: "1", Path: "/api/v1/metrics/operating-summary", Action: "money-flow-ledger.statistics.read"}},
			{ID: "evidence-service", Version: "0.1.0", Enabled: true, EntryAction: "evidence-service.view", ShowInWorkbench: &hidden},
			{ID: "production-operations", Version: "0.1.0", Enabled: true, RoutePrefix: "/production", EntryAction: "production-work-item.read", WorkItemProjection: &workItemProjection{SchemaVersion: "1", Path: "/api/v1/work-items", Action: "production-work-item.work-items.read"}},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	request.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, "contract-ledger") || !strings.Contains(body, "business-engagement") || !strings.Contains(body, "billing-invoice") || strings.Contains(body, `action="/gateway/authorize/evidence-service"`) || !strings.Contains(body, `data-source-application="production-operations"`) || !strings.Contains(body, `data-projection-path="/api/v1/work-items"`) || !strings.Contains(body, `id="companyMetrics"`) || !strings.Contains(body, `data-source-application="money-flow-ledger"`) || !strings.Contains(body, `data-projection-path="/api/v1/metrics/operating-summary"`) || !strings.Contains(body, "不跨来源合计") || !strings.Contains(body, "assets/workbench.js") || !strings.Contains(body, "assets/reference-oa-workspace.css") || !strings.Contains(body, "assets/reference-oa-business.css") || !strings.Contains(body, "北京京宏勘察设计有限公司") || !strings.Contains(body, "项目立项") || !strings.Contains(body, "项目管理") || !strings.Contains(body, "项目资料管理") || !strings.Contains(body, "项目统计信息") || !strings.Contains(body, `class="app-sidebar__nav`) || !strings.Contains(body, `class="project-list-page__timeline-document`) || !strings.Contains(body, `class="project-status-beads`) || !strings.Contains(body, `data-view="initiation-reference"`) || !strings.Contains(body, `data-view="files-reference"`) || !strings.Contains(body, `class="file-management-page"`) || !strings.Contains(body, `data-view="statistics-reference"`) || !strings.Contains(body, `class="statistics-page"`) || !strings.Contains(body, `data-view="project-detail"`) || !strings.Contains(body, `class="project-detail-page__primary-nav"`) || !strings.Contains(body, `data-view="clients"`) || !strings.Contains(body, `data-view="qualification-units"`) || !strings.Contains(body, `data-view="company-revenue"`) || !strings.Contains(body, `data-view="project-revenue"`) || !strings.Contains(body, `data-view="contract-detail"`) || !strings.Contains(body, `data-view="guide"`) || !strings.Contains(body, "经营管理") || !strings.Contains(body, "开票管理") || !strings.Contains(body, "生产管理") {
		t.Fatalf("unexpected workbench response status=%d body=%q", response.Code, body)
	}
	for _, marker := range []string{
		"assets/reference-oa-business-runtime.js",
		`data-real-business-tab="overview"`,
		`data-real-business-tab="client"`,
		`data-real-business-tab="personnel"`,
		`data-real-business-tab="bidding"`,
		`data-real-business-tab="contracts"`,
		`data-real-business-tab="payments"`,
		`data-real-business-tab="invoices"`,
		`data-real-business-tab="costs"`,
		`data-real-business-tab="bonus"`,
		`data-real-detail-panel="production"`,
		"assets/reference-oa-detail.css",
		`data-page-template="files-reference"`,
		`data-page-template="statistics-reference"`,
		`data-page-template="projects"`,
		`data-page-template="initiation-reference"`,
		`data-page-template="users"`,
		`data-page-template="permissions"`,
		`data-project-mode="paged"`,
		`data-project-package-sample`,
		`data-project-package="north-ring"`,
		`data-project-package-toggle`,
		`project-list-page__project-node--child`,
		"北环快速路工程项目包",
		`data-initiation-drawer="create"`,
		`data-initiation-drawer="complete"`,
		`data-initiation-create-validate`,
		`data-initiation-complete-validate`,
		`data-project-route-prefix="/project"`,
		`data-initiation-runtime-status`,
		`data-initiation-business-owner`,
		"创建待立项项目",
		`data-initiation-complete-number`,
		`data-view="project-real-detail"`,
		`data-real-project-list`,
		`data-real-profile-business-owner`,
		`data-real-profile-production-owner`,
		`data-project-business-filter`,
		`data-user-role-filter`,
		`data-user-drawer`,
		`data-user-form-open="create"`,
		`data-user-form-drawer="create"`,
		`data-user-form-drawer="edit"`,
		`data-user-form-note`,
		`data-entity-form-open="client-create"`,
		`data-entity-form-open="qualification-create"`,
		`data-entity-form-drawer="client-edit"`,
		`data-entity-form-drawer="qualification-edit"`,
		`data-entity-delete-dialog`,
		`data-permission-area="project_roles"`,
		`data-permission-validation-note`,
		`data-permission-entry="general"`,
		`data-permission-entry="office"`,
		`data-permission-entry="business-personnel"`,
		`data-permission-entry="estimator"`,
		`data-permission-groups`,
		"身份治理 → OA 职位规则 → 项目角色／临时授权 → IAM 原生能力与目标应用 allowed_actions",
		`data-statistics-tab="personnel-bonus"`,
		`data-file-project="jh-2026-018"`,
		`data-download-dialog="contract-file"`,
		`data-detail-template="overview"`,
		`data-project-profile-edit-open`,
		`data-project-profile-drawer`,
		`data-project-profile-form`,
		"编辑项目基本信息",
		"管理费率（%）",
		`data-project-dialogs-template`,
		`data-project-child-drawer`,
		`data-project-child-form`,
		`data-project-conversion-dialog`,
		`data-project-delete-dialog`,
		`data-project-deactivate-dialog`,
		`data-contract-drawer`,
		`data-contract-form`,
		`data-contract-delete-dialog`,
		`data-contract-create`,
		`data-contract-open`,
		`data-bidding-method`,
		`data-bidding-preview-drawer`,
		`data-bidding-delete-dialog`,
		`data-bidding-file-slot="tender"`,
		`data-bidding-file-slot="bid"`,
		`data-bidding-file-slot="award"`,
		`contract-panel__group--client`,
		`contract-panel__group--income`,
		`transaction-panel__group--client`,
		`transaction-panel__group--income`,
		`data-transaction-open`,
		`data-transaction-create`,
		`data-transaction-drawer`,
		`data-transaction-form`,
		`data-transaction-delete-dialog`,
		`data-cost-open`,
		`data-cost-create`,
		`data-cost-drawer`,
		`data-cost-form`,
		`data-cost-delete-dialog`,
		`data-bonus-pool-form`,
		`data-bonus-open`,
		`data-bonus-create`,
		`data-bonus-drawer`,
		`data-bonus-form`,
		`data-bonus-delete-dialog`,
		`data-production-personnel`,
		`data-production-person-drawer`,
		`data-production-key-drawer`,
		`data-production-person-delete-dialog`,
		`data-approval-audit`,
		`data-approval-audit-drawer`,
		`data-approval-file-action-dialog`,
		`data-stage-panel="scheme"`,
		`data-stage-panel="preliminary"`,
		`data-stage-panel="construction"`,
		`data-stage-edit-drawer`,
		`data-auxiliary-panel="change"`,
		`data-auxiliary-panel="completion"`,
		`data-aux-upload-drawer`,
		`data-stage-file-action-dialog`,
		`data-workspace-tab="test"`,
		`data-workspace-pane="test"`,
		`data-project-type-survey`,
		`data-survey-test-panel`,
		`data-survey-test-input`,
		`data-survey-test-save`,
		`data-production-summary-progress`,
		`data-production-summary-stage`,
		`data-production-overview-stages`,
		`data-production-primary-action`,
		`data-production-optional-toggle`,
		`data-production-optional-actions`,
		`production-info__action-row`,
		`data-workflow-assistant`,
		`data-workflow-settings`,
		`data-feedback-toggle`,
		`data-feedback-menu`,
		"临河新区岩土勘察试验报告.pdf",
		"文件保存后自动计入勘察试验阶段",
		"京哈高速改造工程交通影响评价报告.pdf",
		"京哈高速初设批复.pdf",
		"京哈高速阶段审计报告.pdf",
		"添加子项目",
		"调整项目包归属",
		"原始合同区域",
		"补充区域",
		"定案表文件",
		`data-detail-template="operation"`,
		`data-detail-template="production"`,
		`data-workspace-tabs="business"`,
		`data-workspace-tabs="production"`,
		"合同结算金额",
		"经营参与人",
		"施工图设计",
		"记录饭费、住宿、交通、方案评审专家费等生产侧支出",
		`data-cost-scope-label="生产"`,
		"生产奖金基数",
		`data-bonus-scope-label="生产"`,
		"生产奖金分配",
		"项目管理系统结构化操作手册",
		`id="guide-structure"`,
		`id="guide-entry"`,
		`id="guide-business"`,
		`id="guide-production"`,
		`id="guide-files"`,
		`id="guide-users"`,
		`id="guide-faq"`,
		"经营模块整体有什么内容？",
		"生产模块整体有什么内容？",
		"为什么奖金分配里“添加人员”后没有可选人？",
		"管理费比例配置",
		"项目财务统计",
		"进入公司收入管理",
		`data-project-revenue-config-open`,
		`data-view-target="contract-detail"`,
		"查看完整详情",
		`data-external-commission`,
		"维护受托方、委托内容、合同文件和支付记录。",
		"京哈高速桥梁检测委托合同.pdf",
		`data-commission-drawer`,
		`data-commission-preview-drawer`,
		`data-commission-delete-dialog`,
		`data-business-personnel`,
		"维护项目经营参与人，并展示其项目角色和公司角色。",
		`data-business-personnel-drawer`,
		`data-business-personnel-delete-dialog`,
		`data-project-client-panel`,
		`data-project-client-edit-open`,
		`data-project-client-form`,
		"名称和联系人直接保存到当前项目，不会新建或修改甲方主数据。",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("reference OA detail response missing %q", marker)
		}
	}
	if got := strings.Count(body, `data-entity-delete-dialog`); got != 2 {
		t.Fatalf("expected one scoped delete dialog for each entity directory, got %d", got)
	}
	for _, marker := range []string{`data-view="initiation"`, `data-view="files"`, `data-view="statistics"`, `data-view="project-detail-legacy"`, `legacy-project-detail`} {
		if strings.Contains(body, marker) {
			t.Fatalf("reference OA response still contains retired placeholder %q", marker)
		}
	}
	if strings.Contains(body, `data-initiation-item`) {
		t.Fatal("authenticated project initiation must not render preview records as real Project facts")
	}
	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequest(http.MethodGet, "/assets/workbench.js", nil))
	for _, marker := range []string{"Math.min(4,sources.length)", "page.subject_id!==subject", "page.tenant_id!==tenant", "item.redacted===true", "item.value===null&&item.source_digest===null", "其他来源不受影响", "/gateway/authorize/", "function showView(name)", "function openProjectDetail(opener)", "function renderProjectTypeProfile(detail,fixture)", "['design','设计'],['survey','勘察']", "function updateSurveyTestSelection(panel)", "请先选择试验阶段文件", "function renderProjectDetailActions(detail,fixture)", "function renderProjectDetailHierarchy(detail,fixture)", "function resetProjectDetailTabs(detail)", "function setProjectProfileDrawer(open)", "function setProjectChildDrawer(open)", "function setProjectConversionDialog(open)", "function setProjectDeleteDialog(open)", "function setProjectDeactivateDialog(open)", "function setProductionPersonDrawer(open,opener)", "function setProductionKeyDrawer(open,node)", "function setProductionPersonDeleteDialog(open,opener)", "function setBusinessPersonnelDrawer(open,node)", "function setBusinessPersonnelDeleteDialog(open,node)", "function updateApprovalAuditTotal(form)", "function setApprovalAuditDrawer(open,opener)", "function showApprovalFileAction(action,node)", "function setStageEditDrawer(open,opener)", "function setAuxUploadDrawer(open,opener)", "function showStageFileAction(action,node)", "function setContractDrawer(open,opener)", "function updateContractAmounts(form)", "function setContractDeleteDialog(open)", "function setBiddingPreview(open,opener)", "function showBiddingActionNote(action)", "function setBiddingDeleteDialog(open,opener)", "function setTransactionDrawer(open,opener)", "function setTransactionDeleteDialog(open)", "function findScopedDrawer(node,selector)", "function setCostDrawer(open,opener)", "function setCostDeleteDialog(open,node)", "function updateBonusForm(form)", "function setBonusDrawer(open,opener)", "function setBonusDeleteDialog(open,node)", "function setCommissionDrawer(open,opener)", "function setCommissionPreview(open,opener)", "function setCommissionDeleteDialog(open,opener)", "data-commission-payment-add", "function syncProjectRevenueConfig(form)", "function setProjectRevenueConfigDialog(open)", "data-contract-delete-open", "确认删除项目包及全部子项目", "function updateProjectOverviewCount(form)", "project-detail-page--package", "project-detail-page--child", "'initiation-reference':'项目立项'", "'files-reference':'项目资料管理'", "'statistics-reference':'项目统计信息'", "'company-revenue':'公司收入管理'", "'contract-detail':'合同详情'", "guide:'使用指南'", "[data-detail-tab]", "function updateProjectWorkspace()", "function setProjectMode(mode)", "function syncProjectPackages()", "data-project-package-sample", "data-project-package-toggle", "function updateInitiationWorkspace()", "function setInitiationDrawer(name,open)", "function setInitiationNodeType(type)", "function ensureProjectCredential()", "function projectJSON(path,options)", "function loadRealInitiations()", "project-workspace.create", "project-workspace.hierarchy.manage", "new window.ReferenceOAInitiationController", "data-initiation-create-validate", "data-initiation-complete-validate", "function filterUsers()", "function openUserDrawer(item)", "function setUserFormDrawer(mode,open)", "function setEntityFormDrawer(name,open)", "function renderPermissionGroups()", "permissionAllKeys", "project.production.bonus.allocate", "permission.template.maintain", "function updatePermissionEditor()", "function setPermissionArea(area)", "app-sidebar__nav-item--active", "data-sidebar-collapse", "data-filter-toggle", "data-reference-tab", "data-guide-toggle", "toggle.setAttribute('aria-expanded',String(open))", "ant-collapse-item-active", "data-reference-drawer", "data-detail-template", "data-page-template", "function activateWorkspaceTab(scope,key)", "function activateStatisticsTab(scope,key)", "function toggleFileProject(key)", "function toggleBonusRow(key)", "data-download-dialog", "data-business-open", "data-production-open"} {
		if script.Code != http.StatusOK || !strings.Contains(script.Body.String(), marker) {
			t.Fatalf("workbench script missing %q status=%d", marker, script.Code)
		}
	}
	businessCSS := httptest.NewRecorder()
	handler.ServeHTTP(businessCSS, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-business.css", nil))
	for _, marker := range []string{".company-revenue-statistics", ".revenue-project-ledger", ".project-revenue-config", ".project-revenue-statistics", ".project-revenue-table", ".contract-form", ".contract-panel__group", ".contract-panel__item", ".business-panel-surface__groups", ".transaction-panel__group", ".transaction-panel__item", ".transaction-detail-drawer", ".cost-panel__summary", ".cost-panel__item", ".cost-detail-drawer", ".oa2026-controls__bonus-values", ".bonus-allocation__item", ".bonus-detail-drawer", ".bidding-files__grid", ".bidding-file-slot", ".bidding-preview-drawer", ".external-commission__summary", ".external-commission__item", ".external-commission-form__payments", ".external-commission-preview-drawer", ".business-personnel-panel", ".business-personnel-list__item", ".business-personnel-drawer", ".usage-guide__hero", ".usage-guide__collapse .ant-collapse-item-active", ".usage-guide__hero-actions", ".contract-amendment-list", ".reference-entity-form", ".client-management__delete-blocked"} {
		if businessCSS.Code != http.StatusOK || businessCSS.Header().Get("Content-Type") != "text/css; charset=utf-8" || !strings.Contains(businessCSS.Body.String(), marker) {
			t.Fatalf("reference business css missing %q status=%d", marker, businessCSS.Code)
		}
	}
	detailCSS := httptest.NewRecorder()
	handler.ServeHTTP(detailCSS, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-detail.css", nil))
	for _, marker := range []string{".basic-info-tab__readonly-grid", ".project-profile-form__type-control", ".project-profile-drawer", ".project-detail-page__conversion-button", ".project-detail-page__child-drawer-intro", ".project-package-conversion__mode-group", ".project-package-conversion__fields[hidden]", ".confirm-delete__dialog", ".contract-detail-drawer__panel", ".contract-detail-drawer__fee-grid", ".contract-delete-dialog__facts", ".business-info-tab__overview-grid", ".production-info__summary-stats", ".project-workspace__tabs", ".reference-stage-panel", ".production-personnel__item", ".approval-audit__grid", ".approval-audit-drawer__amount-grid", ".stage-file-panel__summary", ".auxiliary-file-panel__item", ".consultation-report-panel__surface", ".production-stage-file-field__drop", ".survey-test-panel", ".oa2026-workflow-assistant", ".reference-feedback-menu", "[data-project-type-survey][hidden]"} {
		if detailCSS.Code != http.StatusOK || detailCSS.Header().Get("Content-Type") != "text/css; charset=utf-8" || !strings.Contains(detailCSS.Body.String(), marker) {
			t.Fatalf("reference detail css missing %q status=%d", marker, detailCSS.Code)
		}
	}
	workspaceCSS := httptest.NewRecorder()
	handler.ServeHTTP(workspaceCSS, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-workspace.css", nil))
	for _, marker := range []string{".statistics-page__bonus-projects", ".reference-bar-chart", ".reference-modal-mask", ".file-management-page__people-list", ".project-list-page__filter-count", "[data-project-mode-state=\"paged\"]", ".oa-view.is-active", ".initiation-workspace__toolbar", ".initiation-drawer-mask", "height: 100dvh", ".initiation-drawer--complete", ".initiation-validation-note", ".architecture-lens-trigger", ".architecture-lens__guardrails", ".statistics-page__revenue-entry"} {
		if workspaceCSS.Code != http.StatusOK || workspaceCSS.Header().Get("Content-Type") != "text/css; charset=utf-8" || !strings.Contains(workspaceCSS.Body.String(), marker) {
			t.Fatalf("reference P0 workspace css missing %q status=%d", marker, workspaceCSS.Code)
		}
	}
	adminCSS := httptest.NewRecorder()
	handler.ServeHTTP(adminCSS, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-admin.css", nil))
	for _, marker := range []string{".user-management-page__drawer-hero", ".user-management-page__drawer-grid", ".user-directory-form__grid", ".permission-settings__validation-note", ".permission-settings__item--added"} {
		if adminCSS.Code != http.StatusOK || adminCSS.Header().Get("Content-Type") != "text/css; charset=utf-8" || !strings.Contains(adminCSS.Body.String(), marker) {
			t.Fatalf("reference interactive admin css missing %q status=%d", marker, adminCSS.Code)
		}
	}
}

func TestArchitectureLensIsAdministratorOnly(t *testing.T) {
	manifest := suite{
		SchemaVersion: "1",
		SuiteID:       "test-suite",
		TenantID:      "77777777-7777-4777-8777-777777777777",
		Applications: []application{
			{ID: "project-workspace", Version: "0.1.0", Enabled: true, RoutePrefix: "/project", EntryAction: "project-workspace.read"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	memberRequest := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	memberRequest.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	memberResponse := httptest.NewRecorder()
	handler.ServeHTTP(memberResponse, memberRequest)
	if strings.Contains(memberResponse.Body.String(), `data-architecture-lens-open`) {
		t.Fatal("architecture lens must not be rendered for a non-administrator")
	}

	adminRequest := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	adminRequest.Header.Set("X-OMS-Subject-ID", "99999999-9999-4999-8999-999999999999")
	adminRequest.Header.Set("X-OMS-Administrator", "true")
	adminResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminResponse, adminRequest)
	for _, marker := range []string{`data-architecture-lens-open`, `data-architecture-lens-steps`, "Nginx 负责边缘转发，启用 HTTPS 时处理 TLS", "Workbench 只组织界面", "IAM 只签发能力", "Project Workspace 才拥有并写入项目事实"} {
		if adminResponse.Code != http.StatusOK || !strings.Contains(adminResponse.Body.String(), marker) {
			t.Fatalf("administrator architecture lens missing %q status=%d", marker, adminResponse.Code)
		}
	}
}

func TestLoginUsesReferenceOABrandAndIAMContract(t *testing.T) {
	manifest := suite{
		SchemaVersion: "1",
		SuiteID:       "test-suite",
		TenantID:      "77777777-7777-4777-8777-777777777777",
		Applications:  []application{{ID: "identity-access", Version: "0.1.0", Enabled: true, RoutePrefix: "/configured-auth"}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/?view=legacy&return_to=%2Fworkbench%2F%3Fview%3Dlegacy", nil)
	request.Header.Set("X-OMS-CSRF-Token", "csrf-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, marker := range []string{
		"Road Design OA",
		"北京京宏勘察设计有限公司",
		"公路设计项目经营、生产、资料与统计协同平台",
		`action="/gateway/login"`,
		`name="csrf_token" value="csrf-token"`,
		`name="return_to" value="/workbench/?view=legacy"`,
		`name="username"`,
		`name="password"`,
		`class="login-page__watermark"`,
		`class="login-page__brand-title"`,
		`class="login-page__card"`,
		`class="login-page__form ant-form ant-form-vertical"`,
		`assets/reference-oa-login.css`,
		`assets/reference-oa-login.js`,
		`assets/reference-oa-initiation-runtime.js`,
		`data-iam-route-prefix="/configured-auth"`,
		`data-login-error`,
		`assets/jinghong-logo.webp`,
		`data-login-panel="sms"`,
		`data-login-panel="iam"`,
		`data-sms-phone`,
		`data-sms-send`,
		"验证码登录",
		"请输入组织内手机号获取验证码",
		"当前验证环境尚未接入短信服务",
		"真实登录由 IAM 处理",
	} {
		if response.Code != http.StatusOK || !strings.Contains(body, marker) {
			t.Fatalf("login page missing %q status=%d body=%q", marker, response.Code, body)
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("identity-bearing HTML must not be cached")
	}

	cssResponse := httptest.NewRecorder()
	handler.ServeHTTP(cssResponse, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-login.css", nil))
	if cssResponse.Code != http.StatusOK || cssResponse.Header().Get("Content-Type") != "text/css; charset=utf-8" || !strings.Contains(cssResponse.Body.String(), ".login-page__watermark-mark--main") || !strings.Contains(cssResponse.Body.String(), ".login-page__support-copy") {
		t.Fatalf("reference login css was not served correctly status=%d", cssResponse.Code)
	}
	jsResponse := httptest.NewRecorder()
	handler.ServeHTTP(jsResponse, httptest.NewRequest(http.MethodGet, "/assets/reference-oa-login.js", nil))
	if jsResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("unversioned login controller must not be cached")
	}
	for _, marker := range []string{"phonePattern", "function setMode(mode)", "data-login-panel", "data-sms-notice", "ReferenceOAGatewayCSRF", "await login.submit", "data-login-error"} {
		if jsResponse.Code != http.StatusOK || jsResponse.Header().Get("Content-Type") != "application/javascript; charset=utf-8" || !strings.Contains(jsResponse.Body.String(), marker) {
			t.Fatalf("reference login script missing %q status=%d", marker, jsResponse.Code)
		}
	}
	logoResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoResponse, httptest.NewRequest(http.MethodGet, "/assets/jinghong-logo.webp", nil))
	if logoResponse.Code != http.StatusOK || logoResponse.Header().Get("Content-Type") != "image/webp" || len(logoResponse.Body.Bytes()) < 40_000 || string(logoResponse.Body.Bytes()[:4]) != "RIFF" {
		t.Fatalf("reference logo was not served correctly status=%d bytes=%d", logoResponse.Code, logoResponse.Body.Len())
	}
}

func TestReferenceOADemoPreviewShowsAuthenticatedShellWithoutForgingIAM(t *testing.T) {
	manifest := suite{
		SchemaVersion: "1",
		SuiteID:       "test-suite",
		TenantID:      "77777777-7777-4777-8777-777777777777",
		Applications: []application{
			{ID: "project-workspace", Version: "0.1.0", Enabled: true, RoutePrefix: "/project", EntryAction: "project-workspace.read"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?preview=oa", nil))
	body := response.Body.String()
	for _, marker := range []string{
		`data-demo-preview="true"`,
		"OA 界面演示",
		"演示数据 · 不读取或写入真实业务",
		"OA 演示用户",
		`data-view="projects"`,
		`data-view="project-detail"`,
		`data-view="settings"`,
	} {
		if response.Code != http.StatusOK || !strings.Contains(body, marker) {
			t.Fatalf("preview page missing %q status=%d", marker, response.Code)
		}
	}
	if strings.Contains(body, `data-source-application="project-workspace"`) {
		t.Fatal("preview must not invoke protected projection sources with a fabricated actor")
	}

	script := httptest.NewRecorder()
	handler.ServeHTTP(script, httptest.NewRequest(http.MethodGet, "/assets/workbench.js", nil))
	for _, marker := range []string{"const demoPreview=", "function showPreviewNotice()", `form[action^="/gateway/"]`, "真实业务操作必须通过 IAM 登录"} {
		if script.Code != http.StatusOK || !strings.Contains(script.Body.String(), marker) {
			t.Fatalf("preview guard script missing %q status=%d", marker, script.Code)
		}
	}
}

func TestOMSQualificationUnitDemoIsReadOnlyAndSelfContained(t *testing.T) {
	manifest := suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: "77777777-7777-4777-8777-777777777777", Applications: []application{{ID: "project-workspace", Version: "0.1.0", Enabled: true, RoutePrefix: "/project", EntryAction: "project-workspace.read"}}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/?preview=oa&view=oms-reference", nil))
	body := response.Body.String()
	for _, marker := range []string{
		`data-view-target="oms-reference"`,
		`data-view="oms-reference"`,
		`data-page-template="oms-reference"`,
		`data-oms-demo`,
		"合同与资金经营验证版",
		"不读取或写入真实业务",
		"事实所有者 · Contract Ledger",
		"Billing + Money Flow Ledger",
		"候选模型 · 待客户确认",
		"27c6148",
		"assets/reference-oms-demo.css",
		"assets/reference-oms-demo.js",
	} {
		if response.Code != http.StatusOK || !strings.Contains(body, marker) {
			t.Fatalf("OMS demo page missing %q status=%d", marker, response.Code)
		}
	}

	for _, asset := range []struct {
		path, contentType, marker string
	}{
		{"/assets/reference-oms-demo.css", "text/css; charset=utf-8", ".oms-demo__topbar"},
		{"/assets/reference-oms-demo.js", "application/javascript; charset=utf-8", "data-oms-money-filter"},
		{"/assets/workbench.js", "application/javascript; charset=utf-8", "requestedView==='oms-reference'"},
	} {
		assetResponse := httptest.NewRecorder()
		handler.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, asset.path, nil))
		if assetResponse.Code != http.StatusOK || assetResponse.Header().Get("Content-Type") != asset.contentType || !strings.Contains(assetResponse.Body.String(), asset.marker) {
			t.Fatalf("OMS demo asset %s missing %q status=%d", asset.path, asset.marker, assetResponse.Code)
		}
	}
}

func TestOMSPhase1IsAuthenticatedOwnerComposition(t *testing.T) {
	operatingPartyID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	manifest := suite{SchemaVersion: "1", SuiteID: "test-suite", TenantID: "77777777-7777-4777-8777-777777777777", Applications: []application{
		{ID: "identity-access", Version: "0.1.0", Enabled: true, RoutePrefix: "/configured-iam"},
		{ID: "project-workspace", Version: "0.1.0", Enabled: true, RoutePrefix: "/project", EntryAction: "project-workspace.read"},
		{ID: "party-directory", Version: "0.1.0", Enabled: true, RoutePrefix: "/party", EntryAction: "party-directory.read"},
		{ID: "billing-invoice", Version: "0.1.0", Enabled: true, RoutePrefix: "/billing", EntryAction: "billing-invoice.read"},
		{ID: "money-flow-ledger", Version: "0.1.0", Enabled: true, RoutePrefix: "/money", EntryAction: "money-flow-ledger.view"},
	}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := NewWithProfiles(path, ReferenceOAProfiles{OMSOperatingPartyID: operatingPartyID})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/?view=oms-v1", nil)
	request.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, marker := range []string{
		`<body class="oms-v1-active"`,
		`data-view="oms-v1"`,
		`data-page-hydrated="true"`,
		`data-page-template="oms-v1"`,
		`data-oms-v1`,
		`data-identity-route-prefix="/configured-iam"`,
		`data-operating-party-id="` + operatingPartyID + `"`,
		`data-oms-v1-nav="overview"`,
		`data-oms-v1-nav="projects"`,
		`data-oms-v1-nav="receivable"`,
		`data-oms-v1-nav="payable"`,
		`data-oms-v1-nav-group="projects"`,
		`data-oms-v1-nav-group="receivable"`,
		`data-oms-v1-nav-group="payable"`,
		`data-oms-v1-nav-group="internal"`,
		`data-oms-v1-tab="project-list"`,
		`data-oms-v1-tab-page="projects"`,
		`data-oms-v1-tab="outgoing-invoices"`,
		`data-oms-v1-tab="receipts"`,
		`data-oms-v1-tab="payments"`,
		`data-oms-v1-tab="incoming-invoices"`,
		`data-oms-v1-tab="members"`,
		`data-oms-v1-tab="payroll"`,
		`data-oms-v1-tab="finance"`,
		`data-oms-v1-invoice-search="incoming"`,
		`data-oms-v1-commitment-body="receivable"`,
		`data-oms-v1-commitment-body="payable"`,
		`data-oms-v1-export="receivable"`,
		`data-oms-v1-export="payment"`,
		`data-oms-v1-export="projects"`,
		`data-oms-v1-export="clients"`,
		`data-oms-v1-export="contractors"`,
		`data-oms-v1-project-summary`,
		`data-oms-v1-context-group`,
		`data-oms-v1-application="project-workspace" data-oms-v1-route="/project"`,
		`data-oms-v1-application="party-directory" data-oms-v1-route="/party"`,
		`data-oms-v1-application="billing-invoice" data-oms-v1-route="/billing"`,
		`data-oms-v1-application="money-flow-ledger" data-oms-v1-route="/money"`,
		"assets/oms-phase1.css?v=20261025.2",
		"assets/oms-phase1-pagination.js?v=20261022.6",
		"assets/oms-phase1.js?v=20261023.2",
		`data-oms-v1-flow="receivable-total"`,
		`data-oms-v1-flow-count="receipt"`,
		`data-oms-v1-flow-count="payment"`,
		"data-oms-v1-upstream-summary",
		"data-oms-v1-contractor-list",
		"data-oms-v1-project-invoice-list=\"outgoing\"",
		"data-oms-v1-project-invoice-list=\"incoming\"",
		"data-oms-v1-contract-changes",
		"data-oms-v1-project-history",
		"data-oms-v1-project-payable",
		"data-oms-v1-project-evidence",
		"data-oms-v1-project-edit",
		`data-oms-v1-sort-list="projects"`,
		`data-oms-v1-sort-list="commitment-receivable"`,
		`data-oms-v1-sort-list="invoice-outgoing"`,
		`data-oms-v1-sort-list="cash-payment"`,
		`>应收状态</button>`,
		`>应付状态</button>`,
		"oms-v1__detail-section--identity",
		"oms-v1__section-actions",
		">项目资料</span><h2>基本信息</h2>",
		">甲方业务</span><h2>合同与回款</h2>",
		">施工方业务</span><h2>合同与支付</h2>",
		">合同详情</button>",
		"data-oms-v1-preview",
		"data-oms-v1-preview-download",
		"data-oms-v1-preview-body",
		"data-oms-v1-dialog",
		"data-oms-v1-dialog-body",
		"data-oms-v1-dialog-submit",
		"data-internal-member-body",
		"data-internal-payroll-body",
		"data-internal-payroll-sync",
		"data-internal-payroll-select-all",
		"data-internal-payroll-clear",
		"data-internal-finance-body",
		"data-internal-finance-select-all",
		"data-internal-finance-clear",
		`data-subject-id="88888888-8888-4888-8888-888888888888"`,
		"data-internal-storage-notice",
		"assets/oms-phase1-internal-model.js?v=20261023.2",
		"assets/oms-phase1-internal.js?v=20261025.2",
		"assets/oms-phase1-payroll-owner.js?v=20261025.1",
		"assets/oms-phase1-finance-owner.js?v=20261025.1",
	} {
		if response.Code != http.StatusOK || !strings.Contains(body, marker) {
			t.Fatalf("OMS phase 1 page missing %q status=%d", marker, response.Code)
		}
	}
	if strings.Contains(body, `<aside class="ant-layout-sider app-shell__sider">`) || strings.Contains(body, `src="assets/workbench.js"`) {
		t.Fatal("primary OMS response must not render or boot the legacy OA shell")
	}
	if strings.Contains(body, `href="/legacy/"`) || strings.Contains(body, `view=oms-reference`) {
		t.Fatal("primary OMS response must not advertise a retired UI")
	}
	for _, heavy := range []string{`src="assets/reference-oa-react-bidding.js"`, `src="assets/reference-oa-initiation-runtime.js"`, `href="assets/reference-oa-workspace.css"`, `href="assets/reference-oa-business.css"`} {
		if strings.Contains(body, heavy) {
			t.Fatalf("primary OMS response must not eagerly load legacy asset %q", heavy)
		}
	}
	if strings.Contains(omsPhase1HTML, "虚构演示数据") {
		t.Fatal("OMS phase 1 template must not inherit the reference demo data label")
	}
	if strings.Contains(omsPhase1HTML, `class="oms-v1__tabs"`) {
		t.Fatal("OMS phase 1 secondary navigation must live in the sidebar, not repeat as content tabs")
	}
	if strings.Contains(omsPhase1HTML, "data-oms-v1-drawer") || strings.Contains(omsPhase1HTML, "oms-v1__drawer") {
		t.Fatal("OMS phase 1 must use the centered dialog surface instead of the former side drawer")
	}
	if !strings.Contains(omsPhase1HTML, `<header><button type="button" data-oms-v1-project-back>← 返回项目列表</button></header>`) {
		t.Fatal("OMS phase 1 project detail top bar must only provide the return action")
	}
	for _, forbidden := range []string{"一期真实能力工作面", "稳定 Party ID", "尚未安全接通"} {
		if strings.Contains(omsPhase1HTML, forbidden) {
			t.Fatalf("OMS phase 1 template must not expose implementation copy %q", forbidden)
		}
	}
	if strings.Contains(omsPhase1HTML, "业务日期") {
		t.Fatal("OMS phase 1 template must use action-specific date labels")
	}
	for _, label := range []string{"开票日期", "收票日期", "回款日期", "支付日期", "项目开启日期"} {
		if !strings.Contains(omsPhase1HTML, label) {
			t.Fatalf("OMS phase 1 template missing action-specific date label %q", label)
		}
	}
	for _, forbidden := range []string{"应收 / 未回", "应付 / 未付", ">应收情况</button>", ">应付情况</button>"} {
		if strings.Contains(omsPhase1HTML, forbidden) {
			t.Fatalf("OMS phase 1 project table must not use ambiguous money heading %q", forbidden)
		}
	}

	for _, asset := range []struct {
		path, contentType, marker string
	}{
		{"/assets/oms-phase1.css", "text/css; charset=utf-8", ".oms-v1__nav-sub"},
		{"/assets/oms-phase1-pagination.js", "application/javascript; charset=utf-8", "async function loadOffsetCollection(api, id, base, params = {})"},
		{"/assets/oms-phase1.js", "application/javascript; charset=utf-8", "loadOffsetCollection('project-workspace','/api/v1/project-directory',{status:'all'})"},
		{"/assets/oms-phase1-internal-model.js", "application/javascript; charset=utf-8", "scope.OMSInternalModel"},
		{"/assets/oms-phase1-internal.js", "application/javascript; charset=utf-8", "oms-v1.company-internal"},
		{"/assets/oms-phase1-payroll-owner.js", "application/javascript; charset=utf-8", "payments:batch"},
		{"/assets/oms-phase1-finance-owner.js", "application/javascript; charset=utf-8", "finance-payments/batch"},
		{"/assets/oms-phase1-evidence.js", "application/javascript; charset=utf-8", "createEvidenceTransfer"},
		{"/assets/workbench.js", "application/javascript; charset=utf-8", "window.location.assign('/workbench/?view=oms-v1')"},
	} {
		assetResponse := httptest.NewRecorder()
		handler.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, asset.path, nil))
		if assetResponse.Code != http.StatusOK || assetResponse.Header().Get("Content-Type") != asset.contentType || !strings.Contains(assetResponse.Body.String(), asset.marker) {
			t.Fatalf("OMS phase 1 asset %s missing %q status=%d", asset.path, asset.marker, assetResponse.Code)
		}
	}
	ownerAsset := httptest.NewRecorder()
	handler.ServeHTTP(ownerAsset, httptest.NewRequest(http.MethodGet, "/assets/oms-phase1-internal.js", nil))
	for _, marker := range []string{
		"ownerAPI.api('people-groups','/api/v1/persons')",
		"ownerAPI.api('payroll','/api/v1/compensation-profiles')",
		"ownerAPI.api('payroll','/api/v1/payment-profiles')",
		"person=member?{...member,revision:member.person_revision}:null",
		"成员基础资料已保存，但薪酬或账户档案未全部完成",
	} {
		if ownerAsset.Code != http.StatusOK || !strings.Contains(ownerAsset.Body.String(), marker) {
			t.Fatalf("OMS P3 owner composition asset missing %q status=%d", marker, ownerAsset.Code)
		}
	}
	for _, assetPath := range []string{"/assets/oms-phase1.css?v=20261025.2", "/assets/oms-phase1-pagination.js?v=20261022.6", "/assets/oms-phase1.js?v=20261023.2", "/assets/oms-phase1-internal-model.js?v=20261023.2", "/assets/oms-phase1-internal.js?v=20261025.2", "/assets/oms-phase1-payroll-owner.js?v=20261025.1", "/assets/oms-phase1-finance-owner.js?v=20261025.1", "/assets/oms-phase1-evidence.js?v=20261022.6"} {
		assetResponse := httptest.NewRecorder()
		handler.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, assetPath, nil))
		if got := assetResponse.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Fatalf("versioned asset %s must be immutable, got %q", assetPath, got)
		}
	}
	for _, marker := range []string{"contractor-party", "party-directory.create", "project-workspace.resource-link.manage", "project-workspace.update", "method:'POST'", "method:'PUT'", "/api/v1/parties", "/api/v1/project-party-candidates", "function activePartiesForRole(kind)", "function renderOverview()", "decimalMinor", "async function openInvoice(id,direction", "function openInvoiceCreate(incoming)", "/api/v1/invoice-registrations", "data-invoice-direction", "function renderCommitments(direction)", "/commercial-summary", "function pageItems(items,key)", "data-list-size", "state.filters[key]", "function activeFilterCount(filter,defaults={})", "function updateFilterButton(button,filter,defaults={})", "function compareSortValue(left,right,direction)", "function sortedItems(items,list,accessors)", "function syncSortHeaders(list)", "const sortRenderers=", "function projectInvoiceCount(projectID,direction)", "function projectInvoiceFacts(projectID,direction,partyID='')", "function projectInvoiceItems(projectID,direction)", "function renderProjectInvoiceDetails(projectID)", "function projectMoneyStatusRank(projectID,direction)", "function projectMoneyStatusCell(projectID,direction)", "projectMoneyStatusCell(item.id,'receivable')", "projectMoneyStatusCell(item.id,'payable')", "function projectCashFacts(projectID,type,partyID='')", "function projectContractorItems(project)", "async function hydrateProjectDetail(project,contractors,token)", "/history?limit=8", "workspace?.instruments", "expected_revision:project.revision", "项目基本信息已更新", "function downloadCSV", "function exportDefinition(kind)", "function exportFilterFields(kind,current)", "function exportWithFilter(kind,filter)", "data-oms-v1-export-count", "filteredCommitmentItems(value.direction)", "filteredInvoiceItems(kind)", "filteredCashItems(kind)", "button.dataset.omsV1TabPage", "data-oms-v1-mobile-subnav", "data-oms-v1-create-client", "activatePage('projects','clients')", "/api/v1/contracts/workspace-context", "contract-ledger','/api/v1/contracts", "relation_key:'primary-contract'", "relation_key:'downstream-contract'", "party_reference:{party_id:party.id", "function createDownstreamContract", "value.party_reference?.party_id||value.party_id", "workspace.contract?.draft_version", "function downstreamContractByParty", "function authoritativePartyID(party)", "party?.party_reference?.party_id||party?.id", "project_type_keys:['operations']", "function isOMSProject(item)", ".filter(isOMSProject)", ".filter(item=>!!invoiceProject(item))", ".filter(item=>!!cashProject(item))", "function financialFacts(projectID,direction)", "function openCashCreate(receipt)", "/api/v1/cash-workspace", "ledger_owner_party_reference", "counterparty_party_reference", "票据已保存", "主体档案已更新，历史业务记录保持不变", "经营数据已更新至最新状态。", "const sourceIssue=", "async function authorize(id,force=false)", "ReferenceOAGatewayCSRF", "class GatewayCSRF", "root.dataset.identityRoutePrefix", "await csrfProvider.token()", "if(mutating)await authorize(id,true)", "credentialRedirect(response)", "response?.type==='opaqueredirect'", "error.code==='credential_required'", "async function evidenceFetch(path,options={})", "async function ensureEvidenceRuntime()", "assets/oms-phase1-evidence.js?v=20261022.6", "new Headers(options.headers||{})", "state.evidenceTransfer=factory(evidenceFetch)", "upload-sessions", "/evidence'", "async function openProjectContractEvidence", "async function openProjectContractEditor", "data-oms-v1-contract-edit", "expected_draft_version:contract.draft_version", "新合同金额", "历史版本已保留", "cash-transactions/'+encodeURIComponent(id)+'/evidence", "function previewableEvidenceName", "async function openEvidencePreview", "transfer.getPreviewPolicy(descriptor)", "transfer.resolvePreviewUrl(descriptor,policy.mode)", "data-oms-v1-evidence-preview", "OMSPhase1Pagination", "loadCursorCollection('billing-invoice'", "loadOffsetCollection('money-flow-ledger'", "经营数据实时汇总", "暂无已关联项目的甲方", "暂无施工方档案", "项目累计", "function assertExternalCounterparty", ".filter(isExternalParty)", ".filter(item=>roleIDs.has(item.id))", "不能登记自开自收票据", "提交内容不符合业务规则", "addEventListener('click',loadAll)"} {
		if marker == "暂无已关联项目的甲方" || marker == ".filter(item=>roleIDs.has(item.id))" {
			continue
		}
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS phase 1 owner command wiring missing %q", marker)
		}
	}
	for _, marker := range []string{"function setNavGroupExpanded(group,expanded)", "group.classList.contains('is-expanded')", "setNavGroupExpanded(group,true);activatePage", "root.querySelectorAll('[data-oms-v1-tab]')", "button.removeAttribute('aria-current')"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS phase 1 independent navigation expansion missing %q", marker)
		}
	}
	for _, marker := range []string{"function closeProjectDetail()", "state.selectedProjectID=''", "state.projectDetailToken++", "closeProjectDetail();closeInlineFilter()", "if(closeProjectDetail())return", "addEventListener('click',()=>closeProjectDetail())"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS phase 1 navigation must close the project detail surface; missing %q", marker)
		}
	}
	for _, marker := range []string{"const authorizationTasks=new Map()", "if(authorizationTasks.has(id))return authorizationTasks.get(id)", "if(mutating)await authorize(id,true)", "const projectTask=", "Promise.allSettled([projectTask,partyTask,partyCandidateTask])", "项目与主体已就绪 · 正在汇总票款", "if(loadTask)return loadTask"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS phase 1 progressive loading optimization missing %q", marker)
		}
	}
	if strings.Contains(omsPhase1JS, "await authorize(id,mutating)") {
		t.Fatal("OMS phase 1 reads must reuse a valid capability before requesting a replacement")
	}
	for _, forbidden := range []string{"资金事实归 Money", "先保存 Billing 票据", "当前条件在 Owner", "精确 Party revision", "已保存到 Money", "已保存到 Billing", "项目修订 r"} {
		if strings.Contains(omsPhase1JS, forbidden) {
			t.Fatalf("OMS phase 1 must not expose implementation copy %q", forbidden)
		}
	}
	if strings.Contains(omsPhase1JS, "业务日期") {
		t.Fatal("OMS phase 1 behavior must use action-specific date labels")
	}
	if strings.Contains(omsPhase1JS, "showDrawer") || strings.Contains(omsPhase1JS, "closeDrawer") || strings.Contains(omsPhase1CSS, ".oms-v1__drawer") {
		t.Fatal("OMS phase 1 must not retain the former side-drawer implementation")
	}
	if strings.Contains(omsPhase1JS, "<small>未回 ") {
		t.Fatal("OMS project money cells must not render the former two-line receivable copy")
	}
	for _, forbidden := range []string{"function projectMoneyCell(", "projectMoneyCell(receivable", "projectMoneyCell(payable"} {
		if strings.Contains(omsPhase1JS, forbidden) {
			t.Fatalf("OMS project list must not render receivable or payable amount copy %q", forbidden)
		}
	}
	for _, marker := range []string{"status==='complete'?'已完成'", "status==='anomaly'?'金额异常':'未完成'"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS project list settlement status copy missing %q", marker)
		}
	}
	for _, marker := range []string{"const invoiceDateLabel=", "const cashDateLabel=", "headers:[invoiceDateLabel(kind)", "headers:[cashDateLabel(kind)"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS phase 1 action-specific date wiring missing %q", marker)
		}
	}
	for _, marker := range []string{"function partyRoleProjection()", "state.partyCandidates", "/api/v1/project-party-candidates", "function partyVisibleForRole(kind,id,clientIDs,contractorIDs)", "clientIDs.has(id)||!contractorIDs.has(id):contractorIDs.has(id)", "kind==='client'?'暂无甲方档案':'暂无施工方档案'"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS party directory visibility regression; missing %q", marker)
		}
	}
	if strings.Contains(omsPhase1JS, `form[action="/gateway/logout"]`) {
		t.Fatal("OMS commands must not reuse the CSRF token embedded in the initial logout form")
	}
	for _, marker := range []string{"document.body.classList.add('oms-v1-dialog-open')", "document.body.classList.remove('oms-v1-dialog-open')", "function showDialog(", "function closeDialog()", "event.stopImmediatePropagation()", "{capture:true}"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS phase 1 dialog behavior missing %q", marker)
		}
	}
	if !strings.Contains(omsPhase1CSS, "body.oms-v1-active.oms-v1-dialog-open .architecture-lens-trigger{display:none}") {
		t.Fatal("OMS phase 1 dialog must hide the overlapping architecture lens trigger")
	}
	for _, marker := range []string{".oms-v1__money-chain", ".oms-v1__contractor-item", ".oms-v1__invoice-ledger", ".oms-v1__invoice-row", ".oms-v1__export-filters", ".oms-v1__sort", ".oms-v1__sort.is-active", ".oms-v1__project-status", ".oms-v1__detail-lower", ".oms-v1__detail-description", ".oms-v1__section-actions", ".oms-v1__preview-mask[hidden]", ".oms-v1__preview-image"} {
		if !strings.Contains(omsPhase1CSS, marker) {
			t.Fatalf("OMS phase 1 complete project detail style missing %q", marker)
		}
	}
	for _, marker := range []string{"BF-155: readable-density baseline", "grid-template-columns:196px minmax(0,1fr)", ".oms-v1__content{width:100%;max-width:none", ".oms-v1__table{font-size:13px}", ".oms-v1__project-detail>main{width:min(1720px,100%)"} {
		if !strings.Contains(omsPhase1CSS, marker) {
			t.Fatalf("OMS phase 1 readable-density style missing %q", marker)
		}
	}
	for _, marker := range []string{"function enhanceUnifiedSelect(select)", "function positionUnifiedSelect()", "function dialogRecordDetail", "dismissOnly:true", "setAttribute('role','combobox')"} {
		if !strings.Contains(omsPhase1JS, marker) {
			t.Fatalf("OMS phase 1 unified detail/select behavior missing %q", marker)
		}
	}
	for _, marker := range []string{"BF-156: one detail hierarchy", ".oms-v1__select-menu", ".oms-v1__select-trigger", ".oms-v1__record-detail", ".oms-v1__record-metrics", ".oms-v1__record-facts", ".oms-v1__dialog-mask{", ".oms-v1__dialog{", "place-items:center", "@keyframes oms-v1-dialog-in", ".oms-v1__dialog-mask{place-items:stretch;padding:0}"} {
		if !strings.Contains(omsPhase1CSS, marker) {
			t.Fatalf("OMS phase 1 unified detail/select style missing %q", marker)
		}
	}
	for _, marker := range []string{".oms-v1__toolbar>span:not(.oms-v1__select){margin-left:auto", ".oms-v1__toolbar>span:not(.oms-v1__select){display:none}"} {
		if !strings.Contains(omsPhase1CSS, marker) {
			t.Fatalf("OMS phase 1 toolbar select alignment regression; missing %q", marker)
		}
	}
	if strings.Contains(omsPhase1CSS, ".oms-v1__toolbar>span{margin-left:auto") || strings.Contains(omsPhase1CSS, ".oms-v1__toolbar>span{display:none}") {
		t.Fatal("OMS phase 1 toolbar generic span styles must not displace or hide unified selects")
	}
	for _, marker := range []string{"BF-168: navigation groups expand independently", ".oms-v1__nav-group.is-expanded .oms-v1__nav-sub", ".oms-v1__pagination>.oms-v1__select{flex:0 0 112px", ".oms-v1__pagination .oms-v1__select-trigger>i{flex:0 0 7px"} {
		if !strings.Contains(omsPhase1CSS, marker) {
			t.Fatalf("OMS phase 1 navigation/pagination interaction style missing %q", marker)
		}
	}
	if strings.Contains(omsPhase1HTML, `style="height:18%"`) || strings.Contains(omsPhase1HTML, `style="height:28%"`) {
		t.Fatal("OMS phase 1 overview must not ship static trend values that look like business facts")
	}
	preview := httptest.NewRecorder()
	handler.ServeHTTP(preview, httptest.NewRequest(http.MethodGet, "/?preview=oa&view=oms-v1", nil))
	if strings.Contains(preview.Body.String(), operatingPartyID) {
		t.Fatal("demo preview must not expose a live operating Party ID")
	}
	if _, err := NewWithProfiles(path, ReferenceOAProfiles{OMSOperatingPartyID: "not-a-party-id"}); err == nil {
		t.Fatal("invalid OMS operating Party ID accepted")
	}
}

func TestOMSPhase1LoginDoesNotLoadRetiredOARuntime(t *testing.T) {
	manifest := suite{
		SchemaVersion: "1",
		SuiteID:       "test-suite",
		TenantID:      "77777777-7777-4777-8777-777777777777",
		Applications:  []application{{ID: "identity-access", Version: "0.1.0", Enabled: true, RoutePrefix: "/configured-iam"}},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/?view=oms-v1", nil)
	request.Header.Set("X-OMS-CSRF-Token", "csrf-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	body := response.Body.String()
	for _, marker := range []string{
		`data-oms-login`,
		`assets/oms-phase1-login.css?v=20261022.6`,
		`assets/oms-phase1-login.js?v=20261022.6`,
		`data-iam-route-prefix="/configured-iam"`,
		`action="/gateway/login"`,
	} {
		if response.Code != http.StatusOK || !strings.Contains(body, marker) {
			t.Fatalf("OMS login page missing %q status=%d", marker, response.Code)
		}
	}
	for _, retired := range []string{
		`assets/reference-oa-initiation-runtime.js`,
		`assets/reference-oa-login.js`,
		`assets/reference-oa-login.css`,
		`assets/reference-oa-workspace.css`,
		`assets/reference-oms-demo.css`,
	} {
		if strings.Contains(body, retired) {
			t.Fatalf("OMS login page still loads retired asset %q", retired)
		}
	}

	for _, asset := range []struct {
		path, contentType, marker string
	}{
		{"/assets/oms-phase1-login.css?v=20261022.6", "text/css; charset=utf-8", ".login-page__watermark-mark--main"},
		{"/assets/oms-phase1-login.js?v=20261022.6", "application/javascript; charset=utf-8", "OMSPhase1GatewayCSRF"},
	} {
		assetResponse := httptest.NewRecorder()
		handler.ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, asset.path, nil))
		if assetResponse.Code != http.StatusOK || assetResponse.Header().Get("Content-Type") != asset.contentType || !strings.Contains(assetResponse.Body.String(), asset.marker) {
			t.Fatalf("OMS login asset %s missing %q status=%d", asset.path, asset.marker, assetResponse.Code)
		}
		if got := assetResponse.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Fatalf("versioned OMS login asset %s must be immutable, got %q", asset.path, got)
		}
	}
}

func TestPrimaryOMSAndLegacyWorkbenchAreSeparateSurfaces(t *testing.T) {
	manifest := suite{
		SchemaVersion: "1",
		SuiteID:       "test-suite",
		TenantID:      "77777777-7777-4777-8777-777777777777",
		Applications: []application{
			{ID: "identity-access", Version: "0.1.0", Enabled: true, RoutePrefix: "/configured-iam"},
			{ID: "project-workspace", Version: "0.1.0", Enabled: true, RoutePrefix: "/project", EntryAction: "project-workspace.read"},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := New(path)
	if err != nil {
		t.Fatal(err)
	}

	primaryRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	primaryRequest.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	primary := httptest.NewRecorder()
	handler.ServeHTTP(primary, primaryRequest)
	primaryBody := primary.Body.String()
	for _, marker := range []string{`<body class="oms-v1-active"`, `data-page-hydrated="true"`, `src="assets/oms-phase1.js?v=20261023.2"`} {
		if primary.Code != http.StatusOK || !strings.Contains(primaryBody, marker) {
			t.Fatalf("primary OMS missing %q status=%d", marker, primary.Code)
		}
	}
	for _, legacyMarker := range []string{`<aside class="ant-layout-sider app-shell__sider">`, `src="assets/workbench.js"`} {
		if strings.Contains(primaryBody, legacyMarker) {
			t.Fatalf("primary OMS unexpectedly contains legacy runtime %q", legacyMarker)
		}
	}

	legacyRequest := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
	legacyRequest.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
	legacy := httptest.NewRecorder()
	handler.ServeHTTP(legacy, legacyRequest)
	legacyBody := legacy.Body.String()
	for _, marker := range []string{`<aside class="ant-layout-sider app-shell__sider">`, `旧版功能`, `返回 OMS 一期`, `OMS 参考版`, `src="assets/workbench.js"`} {
		if legacy.Code != http.StatusOK || !strings.Contains(legacyBody, marker) {
			t.Fatalf("legacy workbench missing %q status=%d", marker, legacy.Code)
		}
	}
	if strings.Contains(legacyBody, `<body class="oms-v1-active"`) || strings.Contains(legacyBody, `src="assets/oms-phase1.js`) {
		t.Fatal("legacy workbench must not boot the current OMS runtime")
	}

	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodGet, "/", nil))
	loginBody := login.Body.String()
	for _, marker := range []string{`name="return_to" value="/workbench/?view=oms-v1"`, `OMS Phase 1`, `项目、票据与资金经营工作台`, `登录 OMS 一期`} {
		if !strings.Contains(loginBody, marker) {
			t.Fatalf("default login must introduce and return to the primary OMS surface; missing %q", marker)
		}
	}
}

func TestNewRejectsInvalidMetricProjection(t *testing.T) {
	manifest := suite{
		SchemaVersion: "1",
		SuiteID:       "test-suite",
		TenantID:      "77777777-7777-4777-8777-777777777777",
		Applications: []application{
			{ID: "money-flow-ledger", Version: "0.1.0", Enabled: true, RoutePrefix: "/money", EntryAction: "money-flow-ledger.view", MetricProjection: &metricProjection{SchemaVersion: "2", Path: "/api/v1/metrics/operating-summary", Action: "money-flow-ledger.statistics.read"}},
		},
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "suite.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path); err == nil || !strings.Contains(err.Error(), "metric projection is invalid") {
		t.Fatalf("invalid metric projection error=%v", err)
	}
}

func TestStageProfileRequiresOwnersAndRendersExplicitSelection(t *testing.T) {
	writeManifest := func(apps []application) string {
		t.Helper()
		data, err := json.Marshal(suite{SchemaVersion: "1", SuiteID: "stage-test", TenantID: "77777777-7777-4777-8777-777777777777", Applications: apps})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "suite.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	apps := []application{{ID: "project-workspace", Enabled: true}, {ID: "production-operations", Enabled: true}, {ID: "evidence-service", Enabled: true}, {ID: "people-groups", Enabled: true}}
	profile := ReferenceOAProfiles{ProductionStage: "reference-oa-stage-materials-v1"}
	for _, missing := range []string{"project-workspace", "production-operations", "evidence-service", "people-groups"} {
		copyApps := append([]application(nil), apps...)
		for i := range copyApps {
			if copyApps[i].ID == missing {
				copyApps[i].Enabled = false
			}
		}
		if _, err := NewWithProfiles(writeManifest(copyApps), profile); err == nil {
			t.Fatalf("accepted missing %s", missing)
		}
	}
	path := writeManifest(apps)
	if _, err := NewWithProfiles(path, ReferenceOAProfiles{ProductionStage: "unknown"}); err == nil {
		t.Fatal("accepted unknown stage profile")
	}
	for _, on := range []bool{false, true} {
		p := ReferenceOAProfiles{}
		if on {
			p = profile
		}
		handler, err := NewWithProfiles(path, p)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodGet, "/?view=legacy", nil)
		request.Header.Set("X-OMS-Subject-ID", "88888888-8888-4888-8888-888888888888")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		marker := `data-production-stage-profile=""`
		if on {
			marker = `data-production-stage-profile="reference-oa-stage-materials-v1"`
		}
		if !strings.Contains(response.Body.String(), marker) {
			t.Fatalf("missing explicit marker on=%v", on)
		}
	}
}
