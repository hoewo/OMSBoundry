package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	cmdGroupType          = "10000000-0000-4000-8000-000000000001"
	cmdMembershipRole     = "10000000-0000-4000-8000-000000000002"
	cmdPerson             = "10000000-0000-4000-8000-000000000003"
	cmdGroup              = "10000000-0000-4000-8000-000000000004"
	cmdMembership         = "10000000-0000-4000-8000-000000000005"
	cmdLeadRole           = "10000000-0000-4000-8000-000000000006"
	cmdDeliveryCapability = "10000000-0000-4000-8000-000000000007"
	cmdSecondPerson       = "10000000-0000-4000-8000-000000000008"
	cmdSecondMembership   = "10000000-0000-4000-8000-000000000009"
	cmdCapability         = "10000000-0000-4000-8000-000000000010"
	cmdRosterSnapshotOne  = "10000000-0000-4000-8000-000000000011"
	cmdOwnerProfile       = "10000000-0000-4000-8000-000000000012"
	cmdOwnerIAMSubject    = "10000000-0000-4000-8000-000000000013"
	cmdOwnerRole          = "10000000-0000-4000-8000-000000000014"
	cmdEndSecondMember    = "10000000-0000-4000-8000-000000000015"
	cmdDeactivateSecond   = "10000000-0000-4000-8000-000000000016"
	cmdRosterSnapshotTwo  = "10000000-0000-4000-8000-000000000017"
	// The 21/32 prefixes are the second version of the reference OA journey.
	// Version the whole dependent command set whenever the Project initiation
	// payload changes so preserved databases never see a reused command ID with
	// a different Project or Design payload.
	cmdProject            = "21000000-0000-4000-8000-000000000001"
	cmdBusinessOwner      = "21000000-0000-4000-8000-000000000002"
	cmdProductionOwner    = "21000000-0000-4000-8000-000000000003"
	cmdCompleteInitiation = "21000000-0000-4000-8000-000000000004"
	cmdDesign             = "32000000-0000-4000-8000-000000000001"
	cmdDesignOwner        = "32000000-0000-4000-8000-000000000002"
	cmdSchemeDeliverable  = "32000000-0000-4000-8000-000000000003"
	cmdSchemeReplacement  = "32000000-0000-4000-8000-000000000004"
	cmdSchemeComplete     = "32000000-0000-4000-8000-000000000005"
	cmdPreliminarySkip    = "32000000-0000-4000-8000-000000000006"
	cmdDrawingDeliverable = "32000000-0000-4000-8000-000000000007"
	cmdDrawingComplete    = "32000000-0000-4000-8000-000000000008"
	cmdSupportDeliverable = "32000000-0000-4000-8000-000000000009"
	cmdSupportComplete    = "32000000-0000-4000-8000-000000000010"
	cmdPreliminaryRestore = "32000000-0000-4000-8000-000000000011"
	cmdPreliminaryOutput  = "32000000-0000-4000-8000-000000000012"
	cmdPreliminaryDone    = "32000000-0000-4000-8000-000000000013"
	cmdDrawingDoneAgain   = "32000000-0000-4000-8000-000000000014"
	cmdSupportDoneAgain   = "32000000-0000-4000-8000-000000000015"
	cmdDrawingReopen      = "32000000-0000-4000-8000-000000000016"
	cmdDrawingCorrected   = "32000000-0000-4000-8000-000000000017"
	cmdSupportCorrected   = "32000000-0000-4000-8000-000000000018"
	cmdEvidenceLink       = "21000000-0000-4000-8000-000000000005"
	cmdDesignLink         = "21000000-0000-4000-8000-000000000006"
	cmdContractLink       = "21000000-0000-4000-8000-000000000007"
	cmdObligationLink     = "21000000-0000-4000-8000-000000000008"
	cmdTransactionLink    = "21000000-0000-4000-8000-000000000009"
	cmdConsultGroupType   = "31000000-0000-4000-8000-000000000001"
	cmdConsultPerson      = "31000000-0000-4000-8000-000000000002"
	cmdConsultGroup       = "31000000-0000-4000-8000-000000000003"
	cmdConsultProject     = "31000000-0000-4000-8000-000000000004"
	cmdConsultBusiness    = "31000000-0000-4000-8000-000000000005"
	cmdConsultProduction  = "31000000-0000-4000-8000-000000000006"
	cmdConsultInitiation  = "31000000-0000-4000-8000-000000000007"
	cmdConsultWorkItem    = "31000000-0000-4000-8000-000000000008"
	cmdConsultOwner       = "31000000-0000-4000-8000-000000000009"
	cmdConsultSkipScheme  = "31000000-0000-4000-8000-000000000010"
	cmdConsultReport      = "31000000-0000-4000-8000-000000000011"
	cmdConsultComplete    = "31000000-0000-4000-8000-000000000012"
)

type config struct {
	token          string
	referenceToken string
	adminUsername  string
	adminPassword  string
	contractURL    string
	moneyURL       string
	peopleURL      string
	evidenceURL    string
	projectURL     string
	designURL      string
	statePath      string
}

type state struct {
	ProjectID                   string `json:"project_id"`
	ContractID                  string `json:"contract_id"`
	ContractOwnerPartyCreated   bool   `json:"contract_owner_party_created"`
	ContractClientPartyCreated  bool   `json:"contract_client_party_created"`
	ContractOwnerLinked         bool   `json:"contract_owner_linked"`
	ContractClientLinked        bool   `json:"contract_client_linked"`
	ContractActivated           bool   `json:"contract_activated"`
	MoneyOwnerPartyID           string `json:"money_owner_party_id"`
	MoneyClientPartyID          string `json:"money_client_party_id"`
	ObligationID                string `json:"obligation_id"`
	ObligationConfirmed         bool   `json:"obligation_confirmed"`
	CashTransactionID           string `json:"cash_transaction_id"`
	AllocationID                string `json:"allocation_id"`
	AllocationReversed          bool   `json:"allocation_reversed"`
	ReplacementAllocationID     string `json:"replacement_allocation_id"`
	SecondCashTransactionID     string `json:"second_cash_transaction_id"`
	SecondAllocationID          string `json:"second_allocation_id"`
	MoneyAdjusted               bool   `json:"money_adjusted"`
	MoneyRemissionRecorded      bool   `json:"money_remission_recorded"`
	ObligationClosed            bool   `json:"obligation_closed"`
	ErrorCashTransactionID      string `json:"error_cash_transaction_id"`
	ErrorCashReversed           bool   `json:"error_cash_reversed"`
	ContractChanged             bool   `json:"contract_changed"`
	ContractCorrected           bool   `json:"contract_corrected"`
	ContractTerminated          bool   `json:"contract_terminated"`
	ContractProgressSettled     bool   `json:"contract_progress_settled"`
	ContractFinalSettled        bool   `json:"contract_final_settled"`
	ContractClosed              bool   `json:"contract_closed"`
	EvidenceFileID              string `json:"evidence_file_id"`
	EvidenceVersion             int    `json:"evidence_version"`
	ReplacementEvidenceFileID   string `json:"replacement_evidence_file_id"`
	ReplacementEvidenceVersion  int    `json:"replacement_evidence_version"`
	PreliminaryEvidenceFileID   string `json:"preliminary_evidence_file_id"`
	PreliminaryEvidenceVersion  int    `json:"preliminary_evidence_version"`
	ConsultationProjectID       string `json:"consultation_project_id"`
	ConsultationWorkItemID      string `json:"consultation_work_item_id"`
	ConsultationEvidenceFileID  string `json:"consultation_evidence_file_id"`
	ConsultationEvidenceVersion int    `json:"consultation_evidence_version"`
}

type person struct {
	ID           string `json:"id"`
	DisplayName  string `json:"display_name"`
	Status       string `json:"status"`
	IAMSubjectID string `json:"iam_subject_id"`
	Revision     int64  `json:"revision"`
}

type group struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type membership struct {
	ID            string `json:"id"`
	RoleKey       string `json:"role_key"`
	Status        string `json:"status"`
	Revision      int64  `json:"revision"`
	GroupRevision int64  `json:"group_revision"`
}

type capability struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	GroupRevision int64  `json:"group_revision"`
}

type snapshotSummary struct {
	ID            string `json:"id"`
	GroupRevision int64  `json:"group_revision"`
	SHA256        string `json:"sha256"`
}

type groupDetails struct {
	Group       group             `json:"group"`
	Memberships []membership      `json:"memberships"`
	Snapshots   []snapshotSummary `json:"snapshots"`
}

type project struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type assignment struct {
	ID                     string `json:"id"`
	CreatedProjectRevision int64  `json:"created_project_revision"`
}

type assignmentResult struct {
	Project    project    `json:"project"`
	Assignment assignment `json:"assignment"`
}

type resourceLinkResult struct {
	Project project `json:"project"`
}

type workItem struct {
	ID       string `json:"id"`
	TypeKey  string `json:"type_key"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type designDetails struct {
	WorkItem     workItem `json:"work_item"`
	Deliverables []struct {
		StageKey       string `json:"stage_key"`
		EvidenceFileID string `json:"evidence_file_id"`
	} `json:"deliverables"`
}

type designHistoryPage struct {
	Items []struct {
		WorkItemRevision int64          `json:"work_item_revision"`
		EventType        string         `json:"event_type"`
		Payload          map[string]any `json:"payload"`
	} `json:"items"`
}

type uploadSession struct {
	SessionID     string `json:"session_id"`
	FileID        string `json:"file_id"`
	Version       int    `json:"version"`
	Authorization struct {
		UploadURL string `json:"upload_url"`
		Method    string `json:"method"`
	} `json:"authorization"`
}

type evidenceVersion struct {
	FileID  string `json:"file_id"`
	Version int    `json:"version"`
	Status  string `json:"status"`
}

type evidenceReference struct {
	FileID   string `json:"file_id"`
	Version  int    `json:"version"`
	Status   string `json:"status"`
	Revision int64  `json:"revision"`
}

type designReference struct {
	WorkItemID string `json:"work_item_id"`
	Status     string `json:"status"`
	Revision   int64  `json:"revision"`
}

type client struct {
	http  *http.Client
	token string
}

func main() {
	var err error
	if len(os.Args) == 3 && os.Args[1] == "verify-failure-isolation" {
		err = runFailureIsolation(context.Background(), os.Args[2])
	} else if len(os.Args) == 2 && os.Args[1] == "design-delivery" {
		err = run(context.Background(), true)
	} else if len(os.Args) == 2 && os.Args[1] == "commercial-ledger" {
		err = runCommercialLedger(context.Background())
	} else if len(os.Args) == 2 && os.Args[1] == "consultation-delivery" {
		err = runConsultationDelivery(context.Background())
	} else if len(os.Args) == 1 {
		err = run(context.Background(), false)
	} else {
		err = errors.New("usage: reference-oa-demo [design-delivery|commercial-ledger|consultation-delivery|verify-failure-isolation SERVICE]")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "reference OA demo failed:", err)
		os.Exit(1)
	}
}

func runCommercialLedger(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	current, err := readState(cfg.statePath)
	if err != nil {
		return err
	}
	fmt.Println("[J2-subset] 建立正式合同、应收义务、实际收款、核销和纠错历史")
	financial, err := runJ2(ctx, cfg, &current)
	if err != nil {
		return err
	}
	if err := writeState(cfg.statePath, current); err != nil {
		return err
	}
	fmt.Printf("  contract=%s/%s/r%d; obligation=%s/%s/r%d; receipt=%s/r%d\n", financial.Contract.ID, financial.Contract.Status, financial.Contract.Revision, financial.Obligation.ID, financial.Obligation.Status, financial.Obligation.Revision, financial.Transaction.ID, financial.Transaction.Revision)
	fmt.Println("PASS: synthetic commercial-ledger subset journey J2")
	return nil
}

func run(ctx context.Context, designDeliveryOnly bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	c := &client{http: &http.Client{Timeout: 10 * time.Second}, token: cfg.token}

	fmt.Println("[J1] 创建人员、设计生产组和项目")
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/definitions/group_type", cmdGroupType, map[string]any{
		"command_id": cmdGroupType, "key": "delivery-team", "display_name": "交付小组", "description": "OA 演示的设计生产小组",
	}, http.StatusCreated, nil); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/definitions/membership_role", cmdMembershipRole, map[string]any{
		"command_id": cmdMembershipRole, "key": "designer", "display_name": "设计人员", "description": "负责设计交付",
	}, http.StatusCreated, nil); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/definitions/membership_role", cmdLeadRole, map[string]any{
		"command_id": cmdLeadRole, "key": "lead_designer", "display_name": "设计负责人", "description": "负责设计统筹与交付把关",
	}, http.StatusCreated, nil); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/definitions/capability", cmdDeliveryCapability, map[string]any{
		"command_id": cmdDeliveryCapability, "key": "design_delivery", "display_name": "设计交付", "description": "能够承担设计成果交付",
	}, http.StatusCreated, nil); err != nil {
		return err
	}

	var owner person
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/persons", cmdPerson, map[string]any{
		"command_id": cmdPerson, "reference_code": "OA-DEMO-001", "display_name": "林澄",
	}, http.StatusCreated, &owner); err != nil {
		return err
	}
	var team group
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups", cmdGroup, map[string]any{
		"command_id": cmdGroup, "group_type_key": "delivery-team", "name": "青岚项目设计组", "purpose": "完成青岚办公空间改造设计",
	}, http.StatusCreated, &team); err != nil {
		return err
	}
	var joined membership
	if err := c.json(ctx, http.MethodPost, cfg.peopleURL+"/api/v1/groups/"+team.ID+"/memberships", cmdMembership, map[string]any{
		"command_id": cmdMembership, "expected_revision": int64(1), "person_id": owner.ID, "role_key": "designer", "start_date": "2026-09-25",
	}, http.StatusCreated, &joined); err != nil {
		return err
	}
	if joined.GroupRevision < 1 {
		return errors.New("People & Groups returned an invalid group revision")
	}
	var currentOwner person
	if err := c.json(ctx, http.MethodGet, cfg.peopleURL+"/api/v1/persons/"+owner.ID, "", nil, http.StatusOK, &currentOwner); err != nil {
		return err
	}
	var currentTeam groupDetails
	if err := c.json(ctx, http.MethodGet, cfg.peopleURL+"/api/v1/groups/"+team.ID, "", nil, http.StatusOK, &currentTeam); err != nil {
		return err
	}

	var p project
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects", "", map[string]any{
		"command_id": cmdProject, "project_number": "OA-DEMO-2026-002", "name": "青岚办公空间改造", "summary": "用独立小系统装配出的第一套 OA 参考业务", "project_type_keys": []string{"design"},
	}, http.StatusCreated, &p); err != nil {
		return err
	}
	var business assignmentResult
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/assignments", "", map[string]any{
		"command_id": cmdBusinessOwner, "expected_revision": int64(1), "operation": "assign", "subject_kind": "person", "subject_id": owner.ID, "source_revision": currentOwner.Revision, "role_key": "business_owner",
	}, http.StatusOK, &business); err != nil {
		return err
	}
	var production assignmentResult
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/assignments", "", map[string]any{
		"command_id": cmdProductionOwner, "expected_revision": int64(2), "operation": "assign", "subject_kind": "group", "subject_id": team.ID, "source_revision": currentTeam.Group.Revision, "role_key": "production_owner",
	}, http.StatusOK, &production); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/complete-initiation", "", map[string]any{
		"command_id": cmdCompleteInitiation, "expected_revision": int64(3),
	}, http.StatusOK, &p); err != nil {
		return err
	}
	if p.Status != "active" || p.Revision != 4 {
		return fmt.Errorf("project did not reach active revision 4: status=%s revision=%d", p.Status, p.Revision)
	}
	peopleHistory, err := runJ1History(ctx, c, cfg, owner, team, joined)
	if err != nil {
		return err
	}
	fmt.Printf("  project=%s status=%s revision=%d; group snapshot revisions=%d/%d; active members=%d\n", p.ID, p.Status, p.Revision, peopleHistory.FirstSnapshot.GroupRevision, peopleHistory.SecondSnapshot.GroupRevision, peopleHistory.ActiveMembers)

	demoState, err := readState(cfg.statePath)
	if err != nil {
		return err
	}
	if demoState.ProjectID != "" && demoState.ProjectID != p.ID {
		return fmt.Errorf("demo state project %s does not match idempotent project %s", demoState.ProjectID, p.ID)
	}
	demoState.ProjectID = p.ID
	if err := writeState(cfg.statePath, demoState); err != nil {
		return err
	}
	var financial financialRefs
	if !designDeliveryOnly {
		fmt.Println("[J2] 建立正式合同、应收义务、实际收款与核销")
		financial, err = runJ2(ctx, cfg, &demoState)
		if err != nil {
			return err
		}
	}

	fmt.Println("[J3] 上传交付证据并完成设计阶段")
	evidence, err := c.ensureEvidence(ctx, cfg, demoState)
	if err != nil {
		return err
	}
	demoState.EvidenceFileID, demoState.EvidenceVersion = evidence.FileID, evidence.Version
	if err := writeState(cfg.statePath, demoState); err != nil {
		return err
	}
	replacementEvidence, err := c.ensureNamedEvidence(ctx, cfg, demoState.ReplacementEvidenceFileID, demoState.ReplacementEvidenceVersion, "qinglan-scheme-reviewed.txt", "青岚办公空间改造——复核后的方案交付文件\n", "oa-demo-evidence-replacement")
	if err != nil {
		return err
	}
	demoState.ReplacementEvidenceFileID, demoState.ReplacementEvidenceVersion = replacementEvidence.FileID, replacementEvidence.Version
	if err := writeState(cfg.statePath, demoState); err != nil {
		return err
	}
	preliminaryEvidence, err := c.ensureNamedEvidence(ctx, cfg, demoState.PreliminaryEvidenceFileID, demoState.PreliminaryEvidenceVersion, "qinglan-preliminary-design.txt", "青岚办公空间改造——客户追加的初步设计成果\n", "oa-demo-evidence-preliminary")
	if err != nil {
		return err
	}
	demoState.PreliminaryEvidenceFileID, demoState.PreliminaryEvidenceVersion = preliminaryEvidence.FileID, preliminaryEvidence.Version
	if err := writeState(cfg.statePath, demoState); err != nil {
		return err
	}

	var details designDetails
	if err := c.json(ctx, http.MethodPost, cfg.designURL+"/api/v1/design-work-items", "", map[string]any{
		"command_id": cmdDesign, "project_id": p.ID, "project_source_revision": p.Revision,
	}, http.StatusCreated, &details); err != nil {
		return err
	}
	if err := c.json(ctx, http.MethodPost, cfg.designURL+"/api/v1/design-work-items/"+details.WorkItem.ID+"/owner", "", map[string]any{
		"command_id": cmdDesignOwner, "expected_revision": int64(1), "assignment_id": production.Assignment.ID, "assignment_source_revision": production.Assignment.CreatedProjectRevision,
	}, http.StatusOK, &details); err != nil {
		return err
	}

	steps := []struct {
		stage    string
		command  string
		kind     string
		reason   string
		revision int64
		evidence evidenceReference
	}{
		{stage: "scheme", command: cmdSchemeDeliverable, kind: "deliverable", revision: 2, evidence: evidence},
		{stage: "scheme", command: cmdSchemeReplacement, kind: "deliverable", revision: 3, evidence: replacementEvidence},
		{stage: "scheme", command: cmdSchemeComplete, kind: "complete", revision: 4},
		{stage: "preliminary-design", command: cmdPreliminarySkip, kind: "skip", reason: "第一次确认项目规模较小，方案后直接进入施工图", revision: 5},
		{stage: "construction-drawing", command: cmdDrawingDeliverable, kind: "deliverable", revision: 6, evidence: replacementEvidence},
		{stage: "construction-drawing", command: cmdDrawingComplete, kind: "complete", revision: 7},
		{stage: "construction-support", command: cmdSupportDeliverable, kind: "deliverable", revision: 8, evidence: replacementEvidence},
		{stage: "construction-support", command: cmdSupportComplete, kind: "complete", revision: 9},
		{stage: "preliminary-design", command: cmdPreliminaryRestore, kind: "restore", reason: "客户复核后要求补做初步设计", revision: 10},
		{stage: "preliminary-design", command: cmdPreliminaryOutput, kind: "deliverable", revision: 11, evidence: preliminaryEvidence},
		{stage: "preliminary-design", command: cmdPreliminaryDone, kind: "complete", revision: 12},
		{stage: "construction-drawing", command: cmdDrawingDoneAgain, kind: "complete", revision: 13},
		{stage: "construction-support", command: cmdSupportDoneAgain, kind: "complete", revision: 14},
		{stage: "construction-drawing", command: cmdDrawingReopen, kind: "reopen", reason: "施工图审查意见要求回退纠正", revision: 15},
		{stage: "construction-drawing", command: cmdDrawingCorrected, kind: "complete", revision: 16},
		{stage: "construction-support", command: cmdSupportCorrected, kind: "complete", revision: 17},
	}
	for _, step := range steps {
		base := cfg.designURL + "/api/v1/design-work-items/" + details.WorkItem.ID + "/stages/" + step.stage
		var body map[string]any
		switch step.kind {
		case "deliverable":
			base += "/deliverables"
			body = map[string]any{"command_id": step.command, "expected_revision": step.revision, "requirement_key": "primary-output", "evidence_file_id": step.evidence.FileID, "evidence_version": step.evidence.Version, "evidence_source_revision": step.evidence.Revision}
		case "complete":
			base += "/complete"
			body = map[string]any{"command_id": step.command, "expected_revision": step.revision}
		case "skip":
			base += "/applicability"
			body = map[string]any{"command_id": step.command, "expected_revision": step.revision, "operation": "set-not-applicable", "reason": step.reason}
		case "restore":
			base += "/applicability"
			body = map[string]any{"command_id": step.command, "expected_revision": step.revision, "operation": "restore-applicability", "reason": step.reason}
		case "reopen":
			base += "/reopen"
			body = map[string]any{"command_id": step.command, "expected_revision": step.revision, "reason": step.reason}
		}
		if step.kind == "deliverable" {
			var ignored json.RawMessage
			if err := c.json(ctx, http.MethodPost, base, "", body, http.StatusOK, &ignored); err != nil {
				return err
			}
		} else if err := c.json(ctx, http.MethodPost, base, "", body, http.StatusOK, &details); err != nil {
			return err
		}
	}
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/design-work-items/"+details.WorkItem.ID, "", nil, http.StatusOK, &details); err != nil {
		return err
	}
	if details.WorkItem.Status != "completed" || details.WorkItem.Revision != 18 || !hasDesignEvidence(details, replacementEvidence.FileID) || !hasDesignEvidence(details, preliminaryEvidence.FileID) {
		return fmt.Errorf("design work item did not retain replacement/correction history at completed revision 18: status=%s revision=%d deliverables=%d", details.WorkItem.Status, details.WorkItem.Revision, len(details.Deliverables))
	}
	if err := c.verifyDesignHistory(ctx, cfg, details.WorkItem.ID, evidence.FileID, replacementEvidence.FileID); err != nil {
		return err
	}
	fmt.Printf("  evidence=%s/v%d; design=%s status=%s revision=%d\n", evidence.FileID, evidence.Version, details.WorkItem.ID, details.WorkItem.Status, details.WorkItem.Revision)

	if designDeliveryOnly {
		fmt.Println("[J4-subset] 将文件与设计结果作为稳定引用回挂 Project")
	} else {
		fmt.Println("[J4] 将合同、资金、文件与设计结果作为稳定引用回挂 Project")
	}
	one := int64(1)
	var linked resourceLinkResult
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/resource-links", "", map[string]any{
		"command_id": cmdEvidenceLink, "expected_revision": int64(4), "operation": "link", "target_service": "evidence-service", "resource_type": "evidence-file", "resource_id": evidence.FileID, "source_revision": one, "relation_key": "design-evidence", "note": "设计阶段交付证据",
	}, http.StatusOK, &linked); err != nil {
		return err
	}
	designRevision := details.WorkItem.Revision
	if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/resource-links", "", map[string]any{
		"command_id": cmdDesignLink, "expected_revision": int64(5), "operation": "link", "target_service": "design-operations", "resource_type": "design-work-item", "resource_id": details.WorkItem.ID, "source_revision": designRevision, "relation_key": "design-production", "note": "经历成果替换、补做初设与施工图返工后完成的设计生产事项",
	}, http.StatusOK, &linked); err != nil {
		return err
	}
	expectedProjectRevision := int64(6)
	expectedSourceRevisions := map[string]int64{"evidence-service": 1, "design-operations": 18}
	if !designDeliveryOnly {
		contractRevision := financial.Contract.Revision
		if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/resource-links", "", map[string]any{
			"command_id": cmdContractLink, "expected_revision": int64(6), "operation": "link", "target_service": "contract-ledger", "resource_type": "contract", "resource_id": financial.Contract.ID, "source_revision": contractRevision, "relation_key": "primary-contract", "note": "项目主合同",
		}, http.StatusOK, &linked); err != nil {
			return err
		}
		obligationRevision := financial.Obligation.Revision
		if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/resource-links", "", map[string]any{
			"command_id": cmdObligationLink, "expected_revision": int64(7), "operation": "link", "target_service": "money-flow-ledger", "resource_type": "obligation", "resource_id": financial.Obligation.ID, "source_revision": obligationRevision, "relation_key": "receivable", "note": "合同首期应收义务",
		}, http.StatusOK, &linked); err != nil {
			return err
		}
		transactionRevision := financial.Transaction.Revision
		if err := c.json(ctx, http.MethodPost, cfg.projectURL+"/api/v1/projects/"+p.ID+"/resource-links", "", map[string]any{
			"command_id": cmdTransactionLink, "expected_revision": int64(8), "operation": "link", "target_service": "money-flow-ledger", "resource_type": "transaction", "resource_id": financial.Transaction.ID, "source_revision": transactionRevision, "relation_key": "receipt", "note": "首期实际收款",
		}, http.StatusOK, &linked); err != nil {
			return err
		}
		expectedProjectRevision = 9
		expectedSourceRevisions["contract-ledger"] = 4
		expectedSourceRevisions["money-flow-ledger/obligation"] = 2
		expectedSourceRevisions["money-flow-ledger/transaction"] = 1
	}
	if linked.Project.Revision != expectedProjectRevision {
		return fmt.Errorf("project did not reach revision %d after resource links: %d", expectedProjectRevision, linked.Project.Revision)
	}
	var designRef designReference
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/references/design-work-items/"+details.WorkItem.ID, "", nil, http.StatusOK, &designRef); err != nil {
		return err
	}
	if designRef.Status != "completed" || designRef.Revision != 18 {
		return fmt.Errorf("design reference is not stable and complete: %+v", designRef)
	}
	expectedAssignmentRevisions := map[string]int64{
		"business_owner":   currentOwner.Revision,
		"production_owner": currentTeam.Group.Revision,
	}
	if err := verifyProjectTraceability(ctx, c, cfg, p.ID, expectedProjectRevision, expectedAssignmentRevisions, expectedSourceRevisions); err != nil {
		return err
	}
	if designDeliveryOnly {
		var productionItems workItemList
		if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/production-work-items", "", nil, http.StatusOK, &productionItems); err != nil {
			return err
		}
		if len(productionItems.Items) != 1 || productionItems.Items[0].ID != details.WorkItem.ID || productionItems.Items[0].TypeKey != "design-v1" {
			return fmt.Errorf("design-only canonical list contains unselected production data: %+v", productionItems.Items)
		}
		fmt.Printf("  project=%s revision=%d contains two stable cross-application links and immutable assignment snapshots\n", p.ID, linked.Project.Revision)
		fmt.Println("PASS: synthetic design-delivery subset journey J1/J3/J4")
	} else {
		fmt.Printf("  project=%s revision=%d contains five stable cross-application links and immutable assignment snapshots\n", p.ID, linked.Project.Revision)
		fmt.Println("PASS: high-fidelity local OA business journey J1-J4")
	}
	return nil
}

func loadConfig() (config, error) {
	cfg := config{
		token:          strings.TrimSpace(os.Getenv("DEMO_TEMPLATE_ACCESS_TOKEN")),
		referenceToken: strings.TrimSpace(os.Getenv("DEMO_REFERENCE_ACCESS_TOKEN")),
		adminUsername:  strings.TrimSpace(os.Getenv("DEMO_ADMIN_USERNAME")),
		adminPassword:  os.Getenv("DEMO_ADMIN_PASSWORD"),
		contractURL:    envOr("DEMO_CONTRACT_URL", "http://127.0.0.1:18081"),
		moneyURL:       envOr("DEMO_MONEY_URL", "http://127.0.0.1:18083"),
		peopleURL:      envOr("DEMO_PEOPLE_URL", "http://127.0.0.1:18084"),
		evidenceURL:    envOr("DEMO_EVIDENCE_URL", "http://127.0.0.1:18085"),
		projectURL:     envOr("DEMO_PROJECT_URL", "http://127.0.0.1:18086"),
		designURL:      envOr("DEMO_DESIGN_URL", "http://127.0.0.1:18087"),
		statePath:      envOr("DEMO_STATE_PATH", "deployments/local-oa/.runtime/demo-state.json"),
	}
	if cfg.token == "" {
		return config{}, errors.New("DEMO_TEMPLATE_ACCESS_TOKEN is required")
	}
	if cfg.referenceToken == "" || cfg.adminUsername == "" || cfg.adminPassword == "" {
		return config{}, errors.New("DEMO_REFERENCE_ACCESS_TOKEN, DEMO_ADMIN_USERNAME and DEMO_ADMIN_PASSWORD are required")
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return strings.TrimRight(value, "/")
	}
	return fallback
}

func (c *client) ensureEvidence(ctx context.Context, cfg config, current state) (evidenceReference, error) {
	return c.ensureNamedEvidence(ctx, cfg, current.EvidenceFileID, current.EvidenceVersion, "qinglan-design-deliverable.txt", "青岚办公空间改造——设计交付演示文件\n", "oa-demo-evidence")
}

func (c *client) ensureNamedEvidence(ctx context.Context, cfg config, fileID string, version int, fileName, contentText, idempotencyPrefix string) (evidenceReference, error) {
	return c.ensureNamedEvidenceForContext(ctx, cfg, fileID, version, fileName, contentText, idempotencyPrefix, "OA-DEMO-2026-001", "design-deliverable")
}

func (c *client) ensureNamedEvidenceForContext(ctx context.Context, cfg config, fileID string, version int, fileName, contentText, idempotencyPrefix, ownerRef, purpose string) (evidenceReference, error) {
	if fileID != "" && version > 0 {
		var existing evidenceReference
		path := fmt.Sprintf("%s/v1/references/files/%s/versions/%d", cfg.evidenceURL, fileID, version)
		if err := c.json(ctx, http.MethodGet, path, "", nil, http.StatusOK, &existing); err == nil && existing.Status == "available" {
			return existing, nil
		}
	}
	content := []byte(contentText)
	digest := sha256.Sum256(content)
	digestText := hex.EncodeToString(digest[:])
	var session uploadSession
	if err := c.json(ctx, http.MethodPost, cfg.evidenceURL+"/v1/upload-sessions", idempotencyPrefix+"-create-v1", map[string]any{
		"file_name": fileName, "size_bytes": len(content), "media_type": "text/plain; charset=utf-8", "sha256": digestText,
		"client_context": map[string]any{"owner_ref": ownerRef, "purpose": purpose},
	}, http.StatusCreated, &session); err != nil {
		return evidenceReference{}, err
	}
	uploadURL, err := url.Parse(session.Authorization.UploadURL)
	if err != nil {
		return evidenceReference{}, fmt.Errorf("parse evidence upload URL: %w", err)
	}
	if uploadURL.Hostname() == "evidence-service" {
		publicBase, parseErr := url.Parse(cfg.evidenceURL)
		if parseErr != nil {
			return evidenceReference{}, parseErr
		}
		uploadURL.Scheme, uploadURL.Host = publicBase.Scheme, publicBase.Host
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadURL.String(), bytes.NewReader(content))
	if err != nil {
		return evidenceReference{}, err
	}
	req.Header.Set("Content-Type", "text/plain; charset=utf-8")
	response, err := c.http.Do(req)
	if err != nil {
		return evidenceReference{}, fmt.Errorf("PUT %s: %w", uploadURL.Redacted(), err)
	}
	if response.StatusCode != http.StatusNoContent {
		return evidenceReference{}, responseError(response, http.StatusNoContent)
	}
	response.Body.Close()
	var completed evidenceVersion
	if err := c.json(ctx, http.MethodPost, cfg.evidenceURL+"/v1/upload-sessions/"+session.SessionID+"/complete", idempotencyPrefix+"-complete-v1", map[string]any{
		"observed_size_bytes": len(content), "observed_sha256": digestText,
	}, http.StatusOK, &completed); err != nil {
		return evidenceReference{}, err
	}
	var reference evidenceReference
	path := fmt.Sprintf("%s/v1/references/files/%s/versions/%d", cfg.evidenceURL, completed.FileID, completed.Version)
	if err := c.json(ctx, http.MethodGet, path, "", nil, http.StatusOK, &reference); err != nil {
		return evidenceReference{}, err
	}
	return reference, nil
}

func hasDesignEvidence(details designDetails, fileID string) bool {
	for _, item := range details.Deliverables {
		if item.EvidenceFileID == fileID {
			return true
		}
	}
	return false
}

func (c *client) verifyDesignHistory(ctx context.Context, cfg config, workItemID, originalEvidenceID, replacementEvidenceID string) error {
	var history designHistoryPage
	if err := c.json(ctx, http.MethodGet, cfg.designURL+"/api/v1/design-work-items/"+workItemID+"/history?limit=100", "", nil, http.StatusOK, &history); err != nil {
		return err
	}
	if len(history.Items) != 18 {
		return fmt.Errorf("design history has %d events, want 18", len(history.Items))
	}
	encoded, err := json.Marshal(history)
	if err != nil {
		return err
	}
	text := string(encoded)
	for _, expected := range []string{originalEvidenceID, replacementEvidenceID, "stage_applicability_restored", "stage_reopened", "施工图审查意见要求回退纠正"} {
		if !strings.Contains(text, expected) {
			return fmt.Errorf("design history is missing %q", expected)
		}
	}
	return nil
}

func (c *client) json(ctx context.Context, method, endpoint, idempotencyKey string, input any, expected int, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, endpoint, err)
	}
	if response.StatusCode != expected {
		return responseError(response, expected)
	}
	defer response.Body.Close()
	if output == nil {
		_, err = io.Copy(io.Discard, response.Body)
		return err
	}
	if err := json.NewDecoder(response.Body).Decode(output); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, endpoint, err)
	}
	return nil
}

func responseError(response *http.Response, expected int) error {
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
	return fmt.Errorf("%s %s returned %d, expected %d: %s", response.Request.Method, response.Request.URL.Redacted(), response.StatusCode, expected, strings.TrimSpace(string(data)))
}

func readState(path string) (state, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return state{}, nil
	}
	if err != nil {
		return state{}, fmt.Errorf("read demo state: %w", err)
	}
	var value state
	if err := json.Unmarshal(data, &value); err != nil {
		return state{}, fmt.Errorf("decode demo state: %w", err)
	}
	return value, nil
}

func writeState(path string, value state) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create demo state directory: %w", err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write demo state: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("replace demo state: %w", err)
	}
	return nil
}
