package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	commandGroupType       = "63000000-0000-4000-8000-000000000001"
	commandDesignerRole    = "63000000-0000-4000-8000-000000000002"
	commandLeadRole        = "63000000-0000-4000-8000-000000000003"
	commandDeliveryCap     = "63000000-0000-4000-8000-000000000004"
	commandAlice           = "63000000-0000-4000-8000-000000000005"
	commandBob             = "63000000-0000-4000-8000-000000000006"
	commandGroup           = "63000000-0000-4000-8000-000000000007"
	commandJoinAlice       = "63000000-0000-4000-8000-000000000008"
	commandJoinBob         = "63000000-0000-4000-8000-000000000009"
	commandAssignCap       = "63000000-0000-4000-8000-000000000010"
	commandSnapshotOne     = "63000000-0000-4000-8000-000000000011"
	commandChangeAliceRole = "63000000-0000-4000-8000-000000000012"
	commandStaleRole       = "63000000-0000-4000-8000-000000000013"
	commandEndBob          = "63000000-0000-4000-8000-000000000014"
	commandEndCapability   = "63000000-0000-4000-8000-000000000015"
	commandSnapshotTwo     = "63000000-0000-4000-8000-000000000016"
	commandEndAlice        = "63000000-0000-4000-8000-000000000017"
	commandArchive         = "63000000-0000-4000-8000-000000000018"
	commandArchivedJoin    = "63000000-0000-4000-8000-000000000019"
	commandUpdateAlice     = "63000000-0000-4000-8000-000000000020"
	commandBindAlice       = "63000000-0000-4000-8000-000000000021"
	commandDeactivateBob   = "63000000-0000-4000-8000-000000000022"
	commandJoinInactiveBob = "63000000-0000-4000-8000-000000000023"
	commandDeactivateCap   = "63000000-0000-4000-8000-000000000024"
	commandAssignInactive  = "63000000-0000-4000-8000-000000000025"
	iamAliceSubject        = "73000000-0000-4000-8000-000000000001"
)

type config struct {
	baseURL     string
	accessToken string
}

type person struct {
	ID            string `json:"id"`
	ReferenceCode string `json:"reference_code"`
	DisplayName   string `json:"display_name"`
	Status        string `json:"status"`
	IAMSubjectID  string `json:"iam_subject_id"`
	Revision      int64  `json:"revision"`
}

type definition struct {
	Kind        string `json:"kind"`
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Revision    int64  `json:"revision"`
}

type group struct {
	ID           string `json:"id"`
	GroupTypeKey string `json:"group_type_key"`
	Name         string `json:"name"`
	Purpose      string `json:"purpose"`
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
}

type membership struct {
	ID            string `json:"id"`
	PersonID      string `json:"person_id"`
	RoleKey       string `json:"role_key"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	Status        string `json:"status"`
	Revision      int64  `json:"revision"`
	GroupRevision int64  `json:"group_revision"`
}

type capability struct {
	ID            string `json:"id"`
	CapabilityKey string `json:"capability_key"`
	Note          string `json:"note"`
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	Status        string `json:"status"`
	Revision      int64  `json:"revision"`
	GroupRevision int64  `json:"group_revision"`
}

type snapshotSummary struct {
	ID            string `json:"id"`
	GroupRevision int64  `json:"group_revision"`
	SHA256        string `json:"sha256"`
}

type groupDetails struct {
	Group        group             `json:"group"`
	Memberships  []membership      `json:"memberships"`
	Capabilities []capability      `json:"capabilities"`
	Snapshots    []snapshotSummary `json:"snapshots"`
}

type rosterSnapshot struct {
	SchemaVersion string `json:"schema_version"`
	SnapshotID    string `json:"snapshot_id"`
	Group         struct {
		ID       string `json:"id"`
		Revision int64  `json:"revision"`
		Status   string `json:"status"`
	} `json:"group"`
	Members []struct {
		MembershipID string `json:"membership_id"`
		RoleKey      string `json:"role_key"`
		Person       struct {
			ID            string `json:"id"`
			Revision      int64  `json:"revision"`
			DisplayName   string `json:"display_name"`
			ReferenceCode string `json:"reference_code"`
		} `json:"person"`
	} `json:"members"`
	Capabilities []struct {
		AssignmentID string `json:"assignment_id"`
		Key          string `json:"key"`
	} `json:"capabilities"`
}

type demoClient struct {
	baseURL string
	http    *http.Client
}

func main() {
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: people-groups-demo")
		os.Exit(2)
	}
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "People & Groups demo failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client, err := newDemoClient(cfg.baseURL)
	if err != nil {
		return err
	}

	fmt.Println("[1/12] 验证未登录与错误模板令牌失败关闭")
	if err := verifyAnonymousRedirect(ctx, cfg.baseURL); err != nil {
		return err
	}
	if err := client.verifyWrongToken(ctx); err != nil {
		return err
	}
	if err := client.login(ctx, cfg.accessToken); err != nil {
		return err
	}

	fmt.Println("[2/12] 建立群组类型、组内角色与能力目录")
	definitionSeeds := []struct {
		kind, command, key, name, description string
	}{
		{"group_type", commandGroupType, "design_delivery_team", "设计交付组", "承担设计成果交付的协作群组"},
		{"membership_role", commandDesignerRole, "designer", "设计师", "负责设计工作"},
		{"membership_role", commandLeadRole, "lead_designer", "主创建筑师", "负责设计统筹与交付把关"},
		{"capability", commandDeliveryCap, "design_delivery", "设计交付", "能够承担设计成果交付"},
	}
	for _, item := range definitionSeeds {
		if err := client.postJSON(ctx, "/api/v1/definitions/"+item.kind, map[string]any{
			"command_id": item.command, "key": item.key, "display_name": item.name, "description": item.description,
		}, http.StatusCreated, nil); err != nil {
			return err
		}
	}

	fmt.Println("[3/12] 建立两名人员与设计交付组")
	var alice, bob person
	if err := client.postJSON(ctx, "/api/v1/persons", map[string]any{
		"command_id": commandAlice, "reference_code": "BF-PG-001", "display_name": "林清和",
	}, http.StatusCreated, &alice); err != nil {
		return err
	}
	if err := client.postJSON(ctx, "/api/v1/persons", map[string]any{
		"command_id": commandBob, "reference_code": "BF-PG-002", "display_name": "周予安",
	}, http.StatusCreated, &bob); err != nil {
		return err
	}
	var team group
	if err := client.postJSON(ctx, "/api/v1/groups", map[string]any{
		"command_id": commandGroup, "group_type_key": "design_delivery_team", "name": "青岚设计交付组", "purpose": "完成星禾办公空间项目设计交付",
	}, http.StatusCreated, &team); err != nil {
		return err
	}
	if team.ID == "" {
		return errors.New("group command did not return an ID")
	}
	replayingCompletedStory := team.Status == "archived" && team.Revision == 11

	fmt.Println("[4/12] 加入两名成员并声明群组能力")
	var aliceMembership, bobMembership membership
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships", map[string]any{
		"command_id": commandJoinAlice, "expected_revision": 1, "person_id": alice.ID, "role_key": "designer", "start_date": "2026-09-01",
	}, http.StatusCreated, &aliceMembership); err != nil {
		return err
	}
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships", map[string]any{
		"command_id": commandJoinBob, "expected_revision": 2, "person_id": bob.ID, "role_key": "designer", "start_date": "2026-09-02",
	}, http.StatusCreated, &bobMembership); err != nil {
		return err
	}
	var assignment capability
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/capabilities", map[string]any{
		"command_id": commandAssignCap, "expected_revision": 3, "capability_key": "design_delivery", "note": "可承担办公空间方案与施工图交付", "start_date": "2026-09-01",
	}, http.StatusCreated, &assignment); err != nil {
		return err
	}

	fmt.Println("[5/12] 发布包含两名成员与群组能力的 revision 4 快照")
	var snapshotOne snapshotSummary
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/snapshots", map[string]any{
		"command_id": commandSnapshotOne, "expected_revision": 4,
	}, http.StatusCreated, &snapshotOne); err != nil {
		return err
	}
	if snapshotOne.GroupRevision != 4 {
		return fmt.Errorf("first snapshot revision=%d, want 4", snapshotOne.GroupRevision)
	}
	onePayload, oneDigest, err := client.getSnapshot(ctx, team.ID, snapshotOne.GroupRevision)
	if err != nil {
		return err
	}
	if err := verifySnapshot(onePayload, oneDigest, snapshotOne, 2, 1, "designer", "林清和", 1); err != nil {
		return fmt.Errorf("first snapshot: %w", err)
	}

	fmt.Println("[6/12] 修改人员资料并绑定外部 IAM Subject，推进活动名册 revision")
	if err := client.postJSON(ctx, "/api/v1/persons/"+alice.ID+"/profile", map[string]any{
		"command_id": commandUpdateAlice, "expected_revision": 1, "reference_code": "BF-PG-001", "display_name": "林清和（负责人）",
	}, http.StatusOK, &alice); err != nil {
		return err
	}
	if alice.Revision < 2 || alice.Revision > 3 || alice.DisplayName != "林清和（负责人）" {
		return fmt.Errorf("unexpected updated person: %+v", alice)
	}
	if err := client.postJSON(ctx, "/api/v1/persons/"+alice.ID+"/iam-subject", map[string]any{
		"command_id": commandBindAlice, "expected_revision": 2, "iam_subject_id": iamAliceSubject,
	}, http.StatusOK, &alice); err != nil {
		return err
	}
	if alice.Revision != 3 || alice.IAMSubjectID != iamAliceSubject {
		return fmt.Errorf("unexpected IAM Subject binding: %+v", alice)
	}
	var currentDetails groupDetails
	if err := client.getJSON(ctx, "/api/v1/groups/"+team.ID, &currentDetails); err != nil {
		return err
	}
	if !replayingCompletedStory && (currentDetails.Group.Status != "active" || currentDetails.Group.Revision != 6) {
		return fmt.Errorf("person changes did not advance active group to revision 6: %+v", currentDetails.Group)
	}
	if replayingCompletedStory && (currentDetails.Group.Status != "archived" || currentDetails.Group.Revision != 11) {
		return fmt.Errorf("replay did not retain the completed group: %+v", currentDetails.Group)
	}

	fmt.Println("[7/12] 调整负责人角色，并拒绝过期 revision 写入")
	var changed membership
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships/"+aliceMembership.ID+"/role", map[string]any{
		"command_id": commandChangeAliceRole, "expected_group_revision": 6, "expected_revision": 1, "role_key": "lead_designer",
	}, http.StatusOK, &changed); err != nil {
		return err
	}
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships/"+bobMembership.ID+"/role", map[string]any{
		"command_id": commandStaleRole, "expected_group_revision": 6, "expected_revision": 1, "role_key": "lead_designer",
	}, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("stale revision was not rejected: %w", err)
	}

	fmt.Println("[8/12] 结束普通成员、停用人员，并拒绝其重新加入")
	var endedBob membership
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships/"+bobMembership.ID+"/end", map[string]any{
		"command_id": commandEndBob, "expected_group_revision": 7, "expected_revision": 1, "end_date": "2026-09-24",
	}, http.StatusOK, &endedBob); err != nil {
		return err
	}
	if err := client.postJSON(ctx, "/api/v1/persons/"+bob.ID+"/status", map[string]any{
		"command_id": commandDeactivateBob, "expected_revision": 1, "status": "inactive",
	}, http.StatusOK, &bob); err != nil {
		return err
	}
	if bob.Status != "inactive" || bob.Revision != 2 {
		return fmt.Errorf("unexpected inactive person: %+v", bob)
	}
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships", map[string]any{
		"command_id": commandJoinInactiveBob, "expected_revision": 8, "person_id": bob.ID, "role_key": "lead_designer", "start_date": "2026-09-25",
	}, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("inactive person rejoin was not rejected: %w", err)
	}

	fmt.Println("[9/12] 结束群组能力、停用能力目录，并拒绝新的能力分配")
	var endedCapability capability
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/capabilities/"+assignment.ID+"/end", map[string]any{
		"command_id": commandEndCapability, "expected_group_revision": 8, "expected_revision": 1, "end_date": "2026-09-24",
	}, http.StatusOK, &endedCapability); err != nil {
		return err
	}
	var inactiveCapability definition
	if err := client.postJSON(ctx, "/api/v1/definitions/capability/design_delivery", map[string]any{
		"command_id": commandDeactivateCap, "expected_revision": 1, "display_name": "设计交付", "description": "能够承担设计成果交付", "status": "inactive",
	}, http.StatusOK, &inactiveCapability); err != nil {
		return err
	}
	if inactiveCapability.Status != "inactive" || inactiveCapability.Revision != 2 {
		return fmt.Errorf("unexpected inactive capability definition: %+v", inactiveCapability)
	}
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/capabilities", map[string]any{
		"command_id": commandAssignInactive, "expected_revision": 9, "capability_key": "design_delivery", "note": "不应成功", "start_date": "2026-09-25",
	}, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("inactive capability assignment was not rejected: %w", err)
	}

	fmt.Println("[10/12] 发布 revision 9 生命周期快照，并证明 revision 4 快照未变")
	var snapshotTwo snapshotSummary
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/snapshots", map[string]any{
		"command_id": commandSnapshotTwo, "expected_revision": 9,
	}, http.StatusCreated, &snapshotTwo); err != nil {
		return err
	}
	if snapshotTwo.GroupRevision != 9 {
		return fmt.Errorf("second snapshot revision=%d, want 9", snapshotTwo.GroupRevision)
	}
	twoPayload, twoDigest, err := client.getSnapshot(ctx, team.ID, snapshotTwo.GroupRevision)
	if err != nil {
		return err
	}
	if err := verifySnapshot(twoPayload, twoDigest, snapshotTwo, 1, 0, "lead_designer", "林清和（负责人）", 3); err != nil {
		return fmt.Errorf("second snapshot: %w", err)
	}
	if strings.Contains(string(twoPayload), iamAliceSubject) {
		return errors.New("roster snapshot leaked the external IAM Subject link")
	}
	unchangedPayload, unchangedDigest, err := client.getSnapshot(ctx, team.ID, snapshotOne.GroupRevision)
	if err != nil {
		return err
	}
	if string(unchangedPayload) != string(onePayload) || unchangedDigest != oneDigest {
		return errors.New("first published snapshot changed after later membership mutations")
	}

	fmt.Println("[11/12] 结束最后一名成员并归档群组，拒绝归档后写入")
	var endedAlice membership
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships/"+aliceMembership.ID+"/end", map[string]any{
		"command_id": commandEndAlice, "expected_group_revision": 9, "expected_revision": 2, "end_date": "2026-09-25",
	}, http.StatusOK, &endedAlice); err != nil {
		return err
	}
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/archive", map[string]any{
		"command_id": commandArchive, "expected_revision": 10,
	}, http.StatusOK, &team); err != nil {
		return err
	}
	if team.Status != "archived" || team.Revision != 11 {
		return fmt.Errorf("unexpected archived group: %+v", team)
	}
	if err := client.postJSON(ctx, "/api/v1/groups/"+team.ID+"/memberships", map[string]any{
		"command_id": commandArchivedJoin, "expected_revision": team.Revision, "person_id": bob.ID, "role_key": "designer", "start_date": "2026-09-25",
	}, http.StatusConflict, nil); err != nil {
		return fmt.Errorf("archived group write was not rejected: %w", err)
	}

	fmt.Println("[12/12] 核对人员、目录、历史、幂等重放与浏览器页面")
	var details groupDetails
	if err := client.getJSON(ctx, "/api/v1/groups/"+team.ID, &details); err != nil {
		return err
	}
	if err := verifyFinalDetails(details, aliceMembership.ID, bobMembership.ID, assignment.ID); err != nil {
		return err
	}
	var persons []person
	if err := client.getJSON(ctx, "/api/v1/persons", &persons); err != nil {
		return err
	}
	var definitions []definition
	if err := client.getJSON(ctx, "/api/v1/catalog", &definitions); err != nil {
		return err
	}
	if err := verifyLifecycle(persons, definitions, alice.ID, bob.ID); err != nil {
		return err
	}
	var replayed group
	if err := client.postJSON(ctx, "/api/v1/groups", map[string]any{
		"command_id": commandGroup, "group_type_key": "design_delivery_team", "name": "青岚设计交付组", "purpose": "完成星禾办公空间项目设计交付",
	}, http.StatusCreated, &replayed); err != nil {
		return err
	}
	if replayed.ID != team.ID || replayed.Status != "archived" || replayed.Revision != 11 {
		return fmt.Errorf("group command replay did not return current canonical group: %+v", replayed)
	}
	dashboard, err := client.getText(ctx, "/")
	if err != nil {
		return err
	}
	groupPage, err := client.getText(ctx, "/groups/"+team.ID)
	if err != nil {
		return err
	}
	for _, expected := range []string{"青岚设计交付组", "林清和（负责人）", "周予安", "design_delivery", "已关联 IAM", "inactive"} {
		if !strings.Contains(dashboard+groupPage, expected) {
			return fmt.Errorf("browser pages are missing %q", expected)
		}
	}
	for _, expected := range []string{"archived", "lead_designer", "ended", "revision 4", "revision 9", snapshotOne.SHA256, snapshotTwo.SHA256} {
		if !strings.Contains(groupPage, expected) {
			return fmt.Errorf("group page is missing %q", expected)
		}
	}

	fmt.Printf("PASS: People & Groups 独立 Demo group=%s status=%s revision=%d snapshots=%d,%d alice=%s bob=%s\n", team.ID, team.Status, team.Revision, snapshotOne.GroupRevision, snapshotTwo.GroupRevision, alice.ID, bob.ID)
	fmt.Println("      人员资料、IAM Subject 外部引用、人员/目录停用、角色变更、成员/能力结束、不可变快照、归档和冲突保护均已验证。")
	fmt.Println("      命令重放未制造重复事实；名册快照不泄露 IAM Subject，旧目录与人员历史保持可读。")
	fmt.Println("      浏览器入口：" + cfg.baseURL + "/groups/" + team.ID)
	return nil
}

func loadConfig() (config, error) {
	cfg := config{
		baseURL:     strings.TrimRight(envOr("DEMO_PEOPLE_URL", "http://127.0.0.1:28084"), "/"),
		accessToken: os.Getenv("DEMO_TEMPLATE_ACCESS_TOKEN"),
	}
	if len(cfg.accessToken) < 24 {
		return config{}, errors.New("a 24+ byte DEMO_TEMPLATE_ACCESS_TOKEN is required")
	}
	return cfg, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func newDemoClient(baseURL string) (*demoClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &demoClient{baseURL: baseURL, http: &http.Client{
		Timeout:       10 * time.Second,
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func verifyAnonymousRedirect(ctx context.Context, baseURL string) error {
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("anonymous GET /: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSeeOther || response.Header.Get("Location") != "/login" {
		return fmt.Errorf("anonymous GET / status=%d location=%q", response.StatusCode, response.Header.Get("Location"))
	}
	return nil
}

func (c *demoClient) verifyWrongToken(ctx context.Context) error {
	page, err := c.getText(ctx, "/login")
	if err != nil {
		return err
	}
	body, _, err := c.postForm(ctx, "/login", url.Values{
		"csrf_token": {hiddenValue(page, "csrf_token")}, "access_token": {"definitely-wrong-template-token"},
	}, http.StatusUnauthorized)
	if err != nil {
		return err
	}
	if !strings.Contains(body, "login_failed") || !strings.Contains(body, "模板访问令牌无效") {
		return errors.New("wrong-token response did not contain the expected authentication failure")
	}
	return nil
}

func (c *demoClient) login(ctx context.Context, token string) error {
	page, err := c.getText(ctx, "/login")
	if err != nil {
		return err
	}
	_, location, err := c.postForm(ctx, "/login", url.Values{
		"csrf_token": {hiddenValue(page, "csrf_token")}, "access_token": {token},
	}, http.StatusSeeOther)
	if err != nil {
		return err
	}
	if location != "/" {
		return fmt.Errorf("login redirected to %q", location)
	}
	return nil
}

func (c *demoClient) postJSON(ctx context.Context, path string, payload any, expected int, destination any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != expected {
		return fmt.Errorf("POST %s status=%d want=%d body=%s", path, response.StatusCode, expected, strings.TrimSpace(string(body)))
	}
	if destination != nil && expected < 300 {
		if err := json.Unmarshal(body, destination); err != nil {
			return fmt.Errorf("decode POST %s: %w", path, err)
		}
	}
	return nil
}

func (c *demoClient) getJSON(ctx context.Context, path string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		return fmt.Errorf("GET %s status=%d body=%s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(response.Body).Decode(destination)
}

func (c *demoClient) getSnapshot(ctx context.Context, groupID string, revision int64) ([]byte, string, error) {
	path := fmt.Sprintf("/api/v1/groups/%s/snapshots/%d", groupID, revision)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, "", err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, "", err
	}
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("GET %s status=%d body=%s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, response.Header.Get("X-Content-SHA256"), nil
}

func (c *demoClient) getText(ctx context.Context, path string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return "", err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", path, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s status=%d body=%s", path, response.StatusCode, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}

func (c *demoClient) postForm(ctx context.Context, path string, values url.Values, expected int) (string, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(values.Encode()))
	if err != nil {
		return "", "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.http.Do(request)
	if err != nil {
		return "", "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", "", err
	}
	if response.StatusCode != expected {
		return "", "", fmt.Errorf("POST %s status=%d want=%d body=%s", path, response.StatusCode, expected, strings.TrimSpace(string(body)))
	}
	return string(body), response.Header.Get("Location"), nil
}

func verifySnapshot(payload []byte, headerDigest string, summary snapshotSummary, members, capabilities int, role, personName string, personRevision int64) error {
	digest := sha256.Sum256(payload)
	actualDigest := hex.EncodeToString(digest[:])
	if headerDigest != actualDigest || summary.SHA256 != actualDigest {
		return fmt.Errorf("digest mismatch header=%q summary=%q actual=%q", headerDigest, summary.SHA256, actualDigest)
	}
	var snapshot rosterSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return err
	}
	if snapshot.SchemaVersion != "1" || snapshot.SnapshotID != summary.ID || snapshot.Group.Revision != summary.GroupRevision || snapshot.Group.Status != "active" {
		return fmt.Errorf("unexpected snapshot envelope: %+v", snapshot)
	}
	if len(snapshot.Members) != members || len(snapshot.Capabilities) != capabilities {
		return fmt.Errorf("snapshot counts members=%d capabilities=%d, want %d/%d", len(snapshot.Members), len(snapshot.Capabilities), members, capabilities)
	}
	if members > 0 && !containsRole(snapshot, role) {
		return fmt.Errorf("snapshot does not contain role %q", role)
	}
	if personName != "" && !containsPerson(snapshot, personName, personRevision) {
		return fmt.Errorf("snapshot does not contain person %q at revision %d", personName, personRevision)
	}
	return nil
}

func containsRole(snapshot rosterSnapshot, role string) bool {
	for _, member := range snapshot.Members {
		if member.RoleKey == role {
			return true
		}
	}
	return false
}

func containsPerson(snapshot rosterSnapshot, displayName string, revision int64) bool {
	for _, member := range snapshot.Members {
		if member.Person.DisplayName == displayName && member.Person.Revision == revision {
			return true
		}
	}
	return false
}

func verifyFinalDetails(details groupDetails, aliceID, bobID, capabilityID string) error {
	if details.Group.Status != "archived" || details.Group.Revision != 11 || len(details.Snapshots) != 2 {
		return fmt.Errorf("unexpected final group details: %+v", details)
	}
	foundAlice, foundBob := false, false
	for _, item := range details.Memberships {
		switch item.ID {
		case aliceID:
			foundAlice = item.Status == "ended" && item.RoleKey == "lead_designer" && item.Revision == 3
		case bobID:
			foundBob = item.Status == "ended" && item.RoleKey == "designer" && item.Revision == 2
		}
	}
	if !foundAlice || !foundBob {
		return fmt.Errorf("membership history is incomplete: %+v", details.Memberships)
	}
	foundCapability := false
	for _, item := range details.Capabilities {
		if item.ID == capabilityID && item.Status == "ended" && item.Revision == 2 {
			foundCapability = true
		}
	}
	if !foundCapability {
		return fmt.Errorf("capability history is incomplete: %+v", details.Capabilities)
	}
	return nil
}

func verifyLifecycle(persons []person, definitions []definition, aliceID, bobID string) error {
	foundAlice, foundBob := false, false
	for _, item := range persons {
		switch item.ID {
		case aliceID:
			foundAlice = item.DisplayName == "林清和（负责人）" && item.Status == "active" && item.IAMSubjectID == iamAliceSubject && item.Revision == 3
		case bobID:
			foundBob = item.DisplayName == "周予安" && item.Status == "inactive" && item.IAMSubjectID == "" && item.Revision == 2
		}
	}
	if !foundAlice || !foundBob {
		return fmt.Errorf("person lifecycle is incomplete: %+v", persons)
	}
	foundInactiveCapability := false
	for _, item := range definitions {
		if item.Kind == "capability" && item.Key == "design_delivery" && item.Status == "inactive" && item.Revision == 2 {
			foundInactiveCapability = true
		}
	}
	if !foundInactiveCapability {
		return fmt.Errorf("definition lifecycle is incomplete: %+v", definitions)
	}
	return nil
}

var hiddenPattern = regexp.MustCompile(`<input[^>]+name="([^"]+)"[^>]+value="([^"]*)"[^>]*>`)

func hiddenValue(page, name string) string {
	for _, match := range hiddenPattern.FindAllStringSubmatch(page, -1) {
		if match[1] == name {
			return html.UnescapeString(match[2])
		}
	}
	return ""
}
