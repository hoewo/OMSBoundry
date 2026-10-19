package referenceoa

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"omsbizfoundry/contract-ledger/internal/audit"
	"omsbizfoundry/contract-ledger/internal/contract"
)

var ErrEvidenceUnavailable = errors.New("external evidence unavailable")

type FileReference struct {
	Schema       string `json:"schema_version"`
	Source       string `json:"source_application"`
	ResourceType string `json:"resource_type"`
	ID           string `json:"id"`
	FileID       string `json:"file_id"`
	TenantID     string `json:"tenant_id"`
	Version      int    `json:"version"`
	Revision     int    `json:"revision"`
	Status       string `json:"status"`
	Name         string `json:"file_name"`
	Size         int64  `json:"size_bytes"`
	MediaType    string `json:"media_type"`
	SHA256       string `json:"sha256"`
}
type EvidenceValidator interface {
	ValidateFile(context.Context, string, string, int) (FileReference, error)
}
type Material struct {
	ID          string        `json:"id"`
	Kind        string        `json:"kind"`
	AmendmentID string        `json:"amendment_id,omitempty"`
	File        FileReference `json:"file"`
	CreatedAt   time.Time     `json:"created_at"`
	RemovedAt   *time.Time    `json:"removed_at,omitempty"`
}
type Materials struct {
	ContractID string     `json:"contract_id"`
	TenantID   string     `json:"tenant_id"`
	Revision   int64      `json:"revision"`
	Items      []Material `json:"items"`
}
type MaterialInput struct {
	CommandID        string         `json:"command_id"`
	ExpectedRevision int64          `json:"expected_revision"`
	Operation        string         `json:"operation"`
	Kind             string         `json:"kind"`
	AmendmentID      string         `json:"amendment_id,omitempty"`
	FileID           string         `json:"file_id,omitempty"`
	Version          int            `json:"version,omitempty"`
	MaterialID       string         `json:"material_id,omitempty"`
	ContractID       string         `json:"-"`
	TenantID         string         `json:"-"`
	Actor            audit.ActorRef `json:"-"`
}
type MaterialResult struct {
	Materials Materials `json:"materials"`
	Replayed  bool      `json:"replayed"`
}

func (s *Store) SetEvidenceValidator(v EvidenceValidator) { s.evidence = v }
func (s *Store) MaterialsAvailable() bool                 { return s.evidence != nil }
func validateFile(v FileReference, tenant, id string, version int) error {
	digest, e := hex.DecodeString(v.SHA256)
	if v.Schema != "1" || v.Source != "evidence-service" || v.ResourceType != "evidence-file" || v.ID != id || v.FileID != id || v.TenantID != tenant || v.Version != version || v.Revision != version || v.Status != "available" || v.Size < 0 || v.Size > 1<<53-1 || strings.TrimSpace(v.Name) == "" || len([]rune(v.Name)) > 500 || strings.TrimSpace(v.MediaType) == "" || len(v.MediaType) > 255 || e != nil || len(digest) != 32 {
		return ErrInvalid
	}
	return nil
}
func normalizeMaterial(in MaterialInput) (MaterialInput, error) {
	for _, id := range []string{in.CommandID, in.ContractID, in.TenantID} {
		if !uuid.MatchString(id) {
			return in, ErrInvalid
		}
	}
	if in.ExpectedRevision < 0 || in.ExpectedRevision >= 1<<53-1 {
		return in, ErrInvalid
	}
	if in.Kind != "contract" && in.Kind != "amendment" && in.Kind != "finalization" {
		return in, ErrInvalid
	}
	if (in.Kind == "amendment" && !uuid.MatchString(in.AmendmentID)) || (in.Kind != "amendment" && in.AmendmentID != "") {
		return in, ErrInvalid
	}
	if in.Operation == "append" {
		if !uuid.MatchString(in.FileID) || in.Version < 1 || in.Version > 2147483647 || in.MaterialID != "" {
			return in, ErrInvalid
		}
	} else if in.Operation == "remove" {
		if !uuid.MatchString(in.MaterialID) || in.FileID != "" || in.Version != 0 {
			return in, ErrInvalid
		}
	} else {
		return in, ErrInvalid
	}
	actor, e := audit.NormalizeActor(in.Actor)
	if e != nil || actor.Kind != audit.ActorKindIAMSubject {
		return in, ErrInvalid
	}
	in.Actor = actor
	return in, nil
}
func materialHash(in MaterialInput) string {
	b, _ := json.Marshal(struct {
		Input                   MaterialInput
		Contract, Tenant, Actor string
	}{in, in.ContractID, in.TenantID, in.Actor.ID})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Store) ReadMaterials(ctx context.Context, tenant, id string) (Materials, error) {
	if !uuid.MatchString(tenant) || !uuid.MatchString(id) {
		return Materials{}, ErrInvalid
	}
	value := Materials{ContractID: id, TenantID: tenant, Items: []Material{}}
	var b []byte
	var owner string
	e := s.pool.QueryRow(ctx, `SELECT tenant_id::text,snapshot FROM reference_oa_materials WHERE contract_id=$1`, id).Scan(&owner, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		return value, nil
	}
	if e != nil {
		return value, e
	}
	if owner != tenant {
		return value, ErrScope
	}
	e = json.Unmarshal(b, &value)
	return value, e
}
func (s *Store) materialReplay(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, in MaterialInput, hash string) (MaterialResult, bool, error) {
	var oldHash string
	var b []byte
	e := q.QueryRow(ctx, `SELECT intent_sha256,receipt FROM reference_oa_material_commands WHERE command_id=$1`, in.CommandID).Scan(&oldHash, &b)
	if errors.Is(e, pgx.ErrNoRows) {
		return MaterialResult{}, false, nil
	}
	if e != nil {
		return MaterialResult{}, false, e
	}
	if oldHash != hash {
		return MaterialResult{}, true, ErrConflict
	}
	var v MaterialResult
	e = json.Unmarshal(b, &v)
	v.Replayed = true
	return v, true, e
}
func (s *Store) MutateMaterial(ctx context.Context, input MaterialInput) (MaterialResult, error) {
	in, e := normalizeMaterial(input)
	if e != nil {
		return MaterialResult{}, e
	}
	hash := materialHash(in)
	if v, found, e := s.materialReplay(ctx, s.pool, in, hash); found || e != nil {
		return v, e
	}
	var file FileReference
	if in.Operation == "append" {
		if s.evidence == nil {
			return MaterialResult{}, ErrEvidenceUnavailable
		}
		file, e = s.evidence.ValidateFile(ctx, in.TenantID, in.FileID, in.Version)
		if e != nil {
			return MaterialResult{}, e
		}
		if e = validateFile(file, in.TenantID, in.FileID, in.Version); e != nil {
			return MaterialResult{}, e
		}
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return MaterialResult{}, e
	}
	defer tx.Rollback(context.Background())
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('reference-oa-material:'||$1,0))`, in.CommandID); e != nil {
		return MaterialResult{}, e
	}
	if v, found, e := s.materialReplay(ctx, tx, in, hash); found || e != nil {
		return v, e
	}
	var status string
	e = tx.QueryRow(ctx, `SELECT status FROM contracts WHERE id=$1 FOR UPDATE`, in.ContractID).Scan(&status)
	if errors.Is(e, pgx.ErrNoRows) {
		return MaterialResult{}, ErrScope
	}
	if e != nil {
		return MaterialResult{}, e
	}
	if status == "cancelled" || status == "closed" {
		return MaterialResult{}, ErrState
	}
	var b []byte
	e = tx.QueryRow(ctx, `SELECT snapshot FROM reference_oa_contract_drafts WHERE contract_id=$1`, in.ContractID).Scan(&b)
	if errors.Is(e, pgx.ErrNoRows) {
		return MaterialResult{}, ErrScope
	}
	if e != nil {
		return MaterialResult{}, e
	}
	var d Draft
	if e = json.Unmarshal(b, &d); e != nil {
		return MaterialResult{}, e
	}
	if d.TenantID != in.TenantID {
		return MaterialResult{}, ErrScope
	}
	if in.Kind == "amendment" && in.Operation == "append" {
		found := false
		for _, a := range d.Amendments {
			if a.ID == in.AmendmentID && !a.Cancelled {
				found = true
			}
		}
		if !found {
			return MaterialResult{}, ErrScope
		}
	}
	value := Materials{ContractID: in.ContractID, TenantID: in.TenantID, Items: []Material{}}
	e = tx.QueryRow(ctx, `SELECT snapshot FROM reference_oa_materials WHERE contract_id=$1 FOR UPDATE`, in.ContractID).Scan(&b)
	if e == nil {
		if e = json.Unmarshal(b, &value); e != nil {
			return MaterialResult{}, e
		}
	} else if !errors.Is(e, pgx.ErrNoRows) {
		return MaterialResult{}, e
	}
	if value.TenantID != in.TenantID {
		return MaterialResult{}, ErrScope
	}
	if value.Revision != in.ExpectedRevision {
		return MaterialResult{}, ErrConflict
	}
	now := time.Now().UTC()
	active := 0
	for _, m := range value.Items {
		if m.RemovedAt == nil {
			active++
		}
	}
	if in.Operation == "append" {
		if active >= 100 || len(value.Items) >= 10000 {
			return MaterialResult{}, ErrState
		}
		for _, m := range value.Items {
			if m.RemovedAt == nil && m.Kind == in.Kind && m.AmendmentID == in.AmendmentID && m.File.FileID == in.FileID && m.File.Version == in.Version {
				return MaterialResult{}, ErrConflict
			}
		}
		value.Items = append(value.Items, Material{ID: in.CommandID, Kind: in.Kind, AmendmentID: in.AmendmentID, File: file, CreatedAt: now})
	} else {
		found := false
		for i := range value.Items {
			m := &value.Items[i]
			if m.ID == in.MaterialID && m.Kind == in.Kind && m.AmendmentID == in.AmendmentID && m.RemovedAt == nil {
				m.RemovedAt = &now
				found = true
			}
		}
		if !found {
			return MaterialResult{}, ErrScope
		}
	}
	value.Revision++
	body, e := json.Marshal(value)
	if e != nil {
		return MaterialResult{}, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO reference_oa_materials(contract_id,tenant_id,revision,snapshot) VALUES($1,$2,$3,$4) ON CONFLICT(contract_id) DO UPDATE SET revision=EXCLUDED.revision,snapshot=EXCLUDED.snapshot`, in.ContractID, in.TenantID, value.Revision, body); e != nil {
		return MaterialResult{}, e
	}
	result := MaterialResult{Materials: value}
	receipt, e := json.Marshal(result)
	if e != nil {
		return result, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO reference_oa_material_commands(command_id,contract_id,tenant_id,actor_id,intent_sha256,revision,receipt,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, in.CommandID, in.ContractID, in.TenantID, in.Actor.ID, hash, value.Revision, receipt, now); e != nil {
		return result, e
	}
	aid, e := contract.NewCommandID()
	if e != nil {
		return result, e
	}
	if e = audit.Insert(ctx, tx, audit.Entry{ID: aid, ObjectType: audit.ObjectContract, ObjectID: in.ContractID, Action: "reference_oa_material_" + in.Operation, CommandID: in.CommandID, Actor: in.Actor, OccurredAt: now, Summary: "登记或移出合同文件业务引用", Details: map[string]any{"material_revision": value.Revision, "kind": in.Kind, "amendment_id": in.AmendmentID, "physical_delete": false}}); e != nil {
		return result, e
	}
	e = tx.Commit(ctx)
	return result, e
}
