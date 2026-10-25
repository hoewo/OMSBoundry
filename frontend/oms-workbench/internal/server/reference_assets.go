package server

import _ "embed"

// referenceOALoginCSS is copied from the user-owned reference OA Login.css.
//
//go:embed reference_oa_login.css
var referenceOALoginCSS string

// referenceOALoginJS keeps the source OA's phone-validation interaction on the
// default login surface. The D1 validation environment does not claim an SMS
// provider: the script exposes the real IAM username/password form explicitly
// instead of treating a fake verification code as authentication.
//
//go:embed reference_oa_login.js
var referenceOALoginJS string

//go:embed reference_oa_initiation_runtime.js
var referenceOAInitiationRuntimeJS string

//go:embed reference_oa_project_directory.js
var referenceOAProjectDirectoryJS string

//go:embed reference_oa_business_runtime.js
var referenceOABusinessRuntimeJS string

//go:embed reference_oa_bidding_runtime.js
var referenceOABiddingRuntimeJS string

// Built from frontend/vendor original OA components and explicit owner adapters.
//
//go:embed reference_oa_react_bidding.js
var referenceOAReactBiddingJS string

//go:embed reference_oa_react_bidding.css
var referenceOAReactBiddingCSS string

//go:embed reference_oa_architecture_lens.js
var referenceOAArchitectureLensJS string

//go:embed reference_oa_real_projects.html
var referenceOARealProjectsHTML string

// referenceOAWorkspaceCSS is copied from the reference OA application shell,
// sidebar, section heading, project list and project status styles.
//
//go:embed reference_oa_workspace.css
var referenceOAWorkspaceCSS string

// referenceOAAdminCSS is copied from the reference OA settings, people,
// permission, client and qualification management styles.
//
//go:embed reference_oa_admin.css
var referenceOAAdminCSS string

// referenceOABusinessCSS is copied from the reference OA revenue, contract,
// and usage-guide surfaces. The workbench keeps these source class names so
// the visual verification surface stays aligned with the owned OA sample.
//
//go:embed reference_oa_business.css
var referenceOABusinessCSS string

// referenceOADetailCSS carries the reference OA project-detail, business and
// production workspace class contract used by the composed validation UI.
//
//go:embed reference_oa_detail.css
var referenceOADetailCSS string

// referenceOMSDemo is a read-only qualification-unit perspective assembled in
// the Workbench. It contains no business owner calls or persisted facts.
//
//go:embed reference_oms_demo.html
var referenceOMSDemoHTML string

//go:embed reference_oms_demo.css
var referenceOMSDemoCSS string

//go:embed reference_oms_demo.js
var referenceOMSDemoJS string

// omsPhase1 is the authenticated first-version OMS work surface. It composes
// owner APIs through Gateway/IAM and does not contain demo business amounts.
//
//go:embed oms_phase1.html
var omsPhase1HTML string

//go:embed oms_phase1.css
var omsPhase1CSS string

//go:embed oms_phase1.js
var omsPhase1JS string

// omsPhase1InternalModel contains exact local calculations for the company
// internal-management interaction candidate. It does not own persisted facts.
//
//go:embed oms_phase1_internal_model.js
var omsPhase1InternalModelJS string

// omsPhase1Internal composes People & Groups and Payroll for member profiles.
// Internal expense-request candidates remain browser-local until P5 replaces
// that view.
//
//go:embed oms_phase1_internal.js
var omsPhase1InternalJS string

// omsPhase1PayrollOwner composes formal Payroll execution facts and commands;
// it is inert when the Payroll or People & Groups application is not assembled.
//
//go:embed oms_phase1_payroll_owner.js
var omsPhase1PayrollOwnerJS string

// omsPhase1FinanceOwner composes the formal Internal Finance Requests owner
// and is inert when that application is not assembled.
//
//go:embed oms_phase1_finance_owner.js
var omsPhase1FinanceOwnerJS string

// omsPhase1OpenAPI lets each authenticated employee manage only their own API
// keys and inspect masked usage metadata through the IAM-owned API.
//
//go:embed oms_phase1_open_api.js
var omsPhase1OpenAPIJS string

// omsPhase1Feedback mounts one isolated Feedback SDK V2 iframe and obtains its
// runtime credential only after Gateway verifies the current IAM capability.
//
//go:embed oms_phase1_feedback.js
var omsPhase1FeedbackJS string

//go:embed oms_phase1_pagination.js
var omsPhase1PaginationJS string

// omsPhase1LoginJS is the current product login runtime. It intentionally owns
// only CSRF refresh and IAM form behavior so the retired OA initiation runtime
// is not loaded on the OMS login path.
//
//go:embed oms_phase1_login.js
var omsPhase1LoginJS string

// omsPhase1EvidenceJS is a narrow current-OMS file transfer bundle. It keeps
// Evidence and bounded OSS upload support without loading the retired OA UI.
//
//go:embed oms_phase1_evidence.js
var omsPhase1EvidenceJS string

// businessViewsHTML contains the remaining reference OA route surfaces. It is
// kept outside server.go so each source-level page can be reviewed in isolation.
//
//go:embed business_views.html
var businessViewsHTML string

// projectDetailPanelsHTML contains the three detailed OA workspaces that are
// hydrated into the project-detail shell after the server renders IAM forms.
//
//go:embed project_detail_panels.html
var projectDetailPanelsHTML string

// referenceOAP0WorkspacesHTML contains the source-shaped Statistics and
// FileManagement workspaces. Client-side hydration replaces the earlier
// shallow placeholders while keeping IAM/Evidence forms server-rendered.
//
//go:embed reference_oa_p0_workspaces.html
var referenceOAP0WorkspacesHTML string

// referenceOAInteractiveWorkspacesHTML contains the interactive ProjectList,
// UserManagement and PermissionSettings surfaces. They preserve the reference
// OA class contract while keeping business and IAM mutations at their owners.
//
//go:embed reference_oa_interactive_workspaces.html
var referenceOAInteractiveWorkspacesHTML string

// referenceOAInitiationWorkspaceHTML contains the source-shaped project
// initiation list, creation drawer and completion drawer. The visual workflow
// stays interactive while real mutations remain owned by Project Workspace.
//
//go:embed reference_oa_initiation_workspace.html
var referenceOAInitiationWorkspaceHTML string

// jinghongLogoBase64 is a WebP encoding of the reference OA branding asset.
//
//go:embed jinghong_logo.webp.b64
var jinghongLogoBase64 string
