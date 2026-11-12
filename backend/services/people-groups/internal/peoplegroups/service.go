package peoplegroups

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool           *pgxpool.Pool
	iamSubjectRefs IAMSubjectReferencePort
	now            func() time.Time
	newID          func() (string, error)
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type commandReceipt struct {
	Fingerprint []byte
	ResultType  string
	ResultID    string
	Revision    int64
}

func NewService(pool *pgxpool.Pool, subjectReferences ...IAMSubjectReferencePort) (*Service, error) {
	if pool == nil {
		return nil, fmt.Errorf("database pool is required")
	}
	if len(subjectReferences) > 1 {
		return nil, fmt.Errorf("at most one IAM subject reference port is allowed")
	}
	service := &Service{pool: pool, now: func() time.Time { return time.Now().UTC() }, newID: randomUUID}
	if len(subjectReferences) == 1 && subjectReferences[0] != nil {
		service.iamSubjectRefs = subjectReferences[0]
	}
	return service, nil
}

func (s *Service) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

func (s *Service) CreatePerson(ctx context.Context, actor Actor, input CreatePersonInput) (Person, error) {
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return Person{}, err
	}
	displayName, err := cleanText("display_name", input.DisplayName, 1, 100)
	if err != nil {
		return Person{}, err
	}
	referenceCode, err := cleanOptional("reference_code", input.ReferenceCode, 64)
	if err != nil {
		return Person{}, err
	}
	department, err := cleanOptional("department", input.Department, 100)
	if err != nil {
		return Person{}, err
	}
	positionTitle, err := cleanOptional("position_title", input.PositionTitle, 100)
	if err != nil {
		return Person{}, err
	}
	iamSubjectID, err := cleanUUID("iam_subject_id", input.IAMSubjectID, true)
	if err != nil {
		return Person{}, err
	}
	iamSubjectStatus, iamSubjectRevision, err := s.validateIAMSubject(ctx, actor.TenantID, iamSubjectID)
	if err != nil {
		return Person{}, err
	}
	employmentStartDate := strings.TrimSpace(input.EmploymentStartDate)
	if employmentStartDate == "" {
		employmentStartDate = s.now().Format("2006-01-02")
	}
	if employmentStartDate, err = cleanDate("employment_start_date", employmentStartDate); err != nil {
		return Person{}, err
	}
	fingerprint := fingerprintOf(struct {
		ReferenceCode, DisplayName, Department, PositionTitle, EmploymentStartDate, IAMSubjectID, IAMSubjectStatus string
		IAMSubjectRevision                                                                                         int64
	}{referenceCode, displayName, department, positionTitle, employmentStartDate, iamSubjectID, iamSubjectStatus, iamSubjectRevision})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "person.create", commandID, fingerprint); err != nil {
		return Person{}, err
	} else if found {
		return s.getPerson(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	id, err := s.newID()
	if err != nil {
		return Person{}, err
	}
	now := s.now()
	_, err = tx.Exec(ctx, `INSERT INTO persons(id,tenant_id,reference_code,display_name,department,position_title,status,status_effective_date,iam_subject_id,iam_subject_status,iam_subject_revision,revision,created_at,updated_at) VALUES($1,$2,NULLIF($3,''),$4,$5,$6,'active',$7::date,NULLIF($8,'')::uuid,NULLIF($9,''),NULLIF($10,0),1,$11,$11)`, id, actor.TenantID, referenceCode, displayName, department, positionTitle, employmentStartDate, iamSubjectID, iamSubjectStatus, iamSubjectRevision, now)
	if err != nil {
		return Person{}, translateWriteError(err)
	}
	created := Person{ID: id, TenantID: actor.TenantID, ReferenceCode: referenceCode, DisplayName: displayName, Department: department, PositionTitle: positionTitle, Status: "active", StatusEffectiveDate: employmentStartDate, IAMSubjectID: iamSubjectID, IAMSubjectStatus: iamSubjectStatus, IAMSubjectRevision: iamSubjectRevision, Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := insertPersonRevision(ctx, tx, actor, created, "created", now); err != nil {
		return Person{}, err
	}
	if err := insertEmploymentEvent(ctx, tx, actor, created, "hired", employmentStartDate, "", now); err != nil {
		return Person{}, err
	}
	if err := insertAudit(ctx, tx, actor, "person.created", "person", id, map[string]any{"revision": 1}); err != nil {
		return Person{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "person.create", commandID, fingerprint, "person", id, 1, now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Person{}, translateWriteError(err)
	}
	return s.GetPerson(ctx, actor.TenantID, id)
}

func (s *Service) UpdatePersonProfile(ctx context.Context, actor Actor, personID string, input UpdatePersonProfileInput) (Person, error) {
	return s.updatePerson(ctx, actor, personID, "person.profile", input.CommandID, input.ExpectedRevision, "profile_changed", func(current Person) (Person, []byte, error) {
		displayName, err := cleanText("display_name", input.DisplayName, 1, 100)
		if err != nil {
			return Person{}, nil, err
		}
		referenceCode, err := cleanOptional("reference_code", input.ReferenceCode, 64)
		if err != nil {
			return Person{}, nil, err
		}
		department, err := cleanOptional("department", input.Department, 100)
		if err != nil {
			return Person{}, nil, err
		}
		positionTitle, err := cleanOptional("position_title", input.PositionTitle, 100)
		if err != nil {
			return Person{}, nil, err
		}
		current.DisplayName, current.ReferenceCode, current.Department, current.PositionTitle = displayName, referenceCode, department, positionTitle
		return current, fingerprintOf(struct {
			Expected                                              int64
			ReferenceCode, DisplayName, Department, PositionTitle string
		}{input.ExpectedRevision, referenceCode, displayName, department, positionTitle}), nil
	})
}

func (s *Service) SetPersonStatus(ctx context.Context, actor Actor, personID string, input SetPersonStatusInput) (Person, error) {
	return s.updatePerson(ctx, actor, personID, "person.status", input.CommandID, input.ExpectedRevision, "status_changed", func(current Person) (Person, []byte, error) {
		status := strings.ToLower(strings.TrimSpace(input.Status))
		if status != "active" && status != "inactive" {
			return Person{}, nil, &ValidationError{Field: "status", Message: "status must be active or inactive"}
		}
		if status == current.Status {
			return Person{}, nil, ErrConflict
		}
		effectiveDate, err := cleanDate("effective_date", input.EffectiveDate)
		if err != nil {
			return Person{}, nil, err
		}
		reason, err := cleanOptional("reason", input.Reason, 500)
		if err != nil {
			return Person{}, nil, err
		}
		current.Status = status
		current.StatusEffectiveDate = effectiveDate
		current.employmentReason = reason
		return current, fingerprintOf(struct {
			Expected                      int64
			Status, EffectiveDate, Reason string
		}{input.ExpectedRevision, status, effectiveDate, reason}), nil
	})
}

func (s *Service) SetPersonIAMSubject(ctx context.Context, actor Actor, personID string, input SetIAMSubjectInput) (Person, error) {
	value, err := cleanUUID("iam_subject_id", input.IAMSubjectID, true)
	if err != nil {
		return Person{}, err
	}
	status, revision, err := s.validateIAMSubject(ctx, actor.TenantID, value)
	if err != nil {
		return Person{}, err
	}
	return s.updatePerson(ctx, actor, personID, "person.iam-subject", input.CommandID, input.ExpectedRevision, "iam_subject_changed", func(current Person) (Person, []byte, error) {
		current.IAMSubjectID, current.IAMSubjectStatus, current.IAMSubjectRevision = value, status, revision
		return current, fingerprintOf(struct {
			Expected                       int64
			IAMSubjectID, IAMSubjectStatus string
			IAMSubjectRevision             int64
		}{input.ExpectedRevision, value, status, revision}), nil
	})
}

func (s *Service) validateIAMSubject(ctx context.Context, tenantID, subjectID string) (string, int64, error) {
	if subjectID == "" {
		return "", 0, nil
	}
	if s.iamSubjectRefs == nil {
		return "", 0, ErrSubjectReferenceInvalid
	}
	reference, err := s.iamSubjectRefs.ValidateIAMSubject(ctx, tenantID, subjectID)
	if err != nil {
		return "", 0, err
	}
	if reference.SchemaVersion != "1" || reference.SourceApplication != "identity-access" || reference.ID != subjectID || reference.TenantID != tenantID || reference.Revision < 1 || (reference.Status != "pending" && reference.Status != "active" && reference.Status != "disabled") {
		return "", 0, ErrSubjectReferenceInvalid
	}
	return reference.Status, reference.Revision, nil
}

func (s *Service) updatePerson(ctx context.Context, actor Actor, personID, operation, commandIDRaw string, expected int64, eventKind string, mutate func(Person) (Person, []byte, error)) (Person, error) {
	personID, err := cleanUUID("person_id", personID, false)
	if err != nil {
		return Person{}, err
	}
	commandID, err := validateCommandID(commandIDRaw)
	if err != nil {
		return Person{}, err
	}
	if err := validateRevision(expected, "expected_revision"); err != nil {
		return Person{}, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Person{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	current, err := s.lockPerson(ctx, tx, actor.TenantID, personID)
	if err != nil {
		return Person{}, err
	}
	next, fingerprint, err := mutate(current)
	if err != nil {
		return Person{}, err
	}
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, operation, commandID, fingerprint); err != nil {
		return Person{}, err
	} else if found {
		return s.getPerson(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	if next.ReferenceCode == current.ReferenceCode && next.DisplayName == current.DisplayName && next.Department == current.Department && next.PositionTitle == current.PositionTitle && next.Status == current.Status && next.IAMSubjectID == current.IAMSubjectID && next.IAMSubjectStatus == current.IAMSubjectStatus && next.IAMSubjectRevision == current.IAMSubjectRevision {
		return Person{}, ErrConflict
	}
	if current.Revision != expected {
		return Person{}, ErrConflict
	}
	now := s.now()
	next.Revision = current.Revision + 1
	next.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE persons SET reference_code=NULLIF($1,''),display_name=$2,department=$3,position_title=$4,status=$5,status_effective_date=$6::date,iam_subject_id=NULLIF($7,'')::uuid,iam_subject_status=NULLIF($8,''),iam_subject_revision=NULLIF($9,0),revision=$10,updated_at=$11 WHERE tenant_id=$12 AND id=$13 AND revision=$14`, next.ReferenceCode, next.DisplayName, next.Department, next.PositionTitle, next.Status, next.StatusEffectiveDate, next.IAMSubjectID, next.IAMSubjectStatus, next.IAMSubjectRevision, next.Revision, now, actor.TenantID, personID, expected)
	if err != nil {
		return Person{}, translateWriteError(err)
	}
	if err := insertPersonRevision(ctx, tx, actor, next, eventKind, now); err != nil {
		return Person{}, err
	}
	if eventKind == "status_changed" {
		event := "offboarded"
		if next.Status == "active" {
			event = "rehired"
		}
		if err := insertEmploymentEvent(ctx, tx, actor, next, event, next.StatusEffectiveDate, next.employmentReason, now); err != nil {
			return Person{}, err
		}
	}
	if err := s.bumpGroupsForPerson(ctx, tx, actor, personID, now); err != nil {
		return Person{}, err
	}
	if err := insertAudit(ctx, tx, actor, "person."+eventKind, "person", personID, map[string]any{"revision": next.Revision}); err != nil {
		return Person{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, operation, commandID, fingerprint, "person", personID, next.Revision, now); err != nil {
		return Person{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Person{}, translateWriteError(err)
	}
	return s.GetPerson(ctx, actor.TenantID, personID)
}

func (s *Service) CreateDefinition(ctx context.Context, actor Actor, kind DefinitionKind, input CreateDefinitionInput) (Definition, error) {
	if normalized, err := NormalizeDefinitionKind(string(kind)); err != nil {
		return Definition{}, err
	} else {
		kind = normalized
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return Definition{}, err
	}
	key, err := cleanKey("key", input.Key)
	if err != nil {
		return Definition{}, err
	}
	displayName, err := cleanText("display_name", input.DisplayName, 1, 100)
	if err != nil {
		return Definition{}, err
	}
	description, err := cleanOptional("description", input.Description, 500)
	if err != nil {
		return Definition{}, err
	}
	fingerprint := fingerprintOf(struct {
		Kind                          DefinitionKind
		Key, DisplayName, Description string
	}{kind, key, displayName, description})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Definition{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "definition.create", commandID, fingerprint); err != nil {
		return Definition{}, err
	} else if found {
		return s.getDefinition(ctx, tx, actor.TenantID, kind, receipt.ResultID)
	}
	now := s.now()
	_, err = tx.Exec(ctx, `INSERT INTO definitions(tenant_id,kind,key,display_name,description,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'active',1,$6,$6)`, actor.TenantID, kind, key, displayName, description, now)
	if err != nil {
		return Definition{}, translateWriteError(err)
	}
	if err := insertDefinitionRevision(ctx, tx, actor, Definition{Kind: kind, Key: key, DisplayName: displayName, Description: description, Status: "active", Revision: 1, CreatedAt: now, UpdatedAt: now}, "created", now); err != nil {
		return Definition{}, err
	}
	if err := insertAudit(ctx, tx, actor, "definition.created", "definition", string(kind)+":"+key, map[string]any{"revision": 1}); err != nil {
		return Definition{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "definition.create", commandID, fingerprint, "definition", key, 1, now); err != nil {
		return Definition{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Definition{}, translateWriteError(err)
	}
	return s.getDefinition(ctx, s.pool, actor.TenantID, kind, key)
}

func (s *Service) UpdateDefinition(ctx context.Context, actor Actor, kind DefinitionKind, key string, input UpdateDefinitionInput) (Definition, error) {
	var err error
	if kind, err = NormalizeDefinitionKind(string(kind)); err != nil {
		return Definition{}, err
	}
	if key, err = cleanKey("key", key); err != nil {
		return Definition{}, err
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return Definition{}, err
	}
	if err = validateRevision(input.ExpectedRevision, "expected_revision"); err != nil {
		return Definition{}, err
	}
	displayName, err := cleanText("display_name", input.DisplayName, 1, 100)
	if err != nil {
		return Definition{}, err
	}
	description, err := cleanOptional("description", input.Description, 500)
	if err != nil {
		return Definition{}, err
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status != "active" && status != "inactive" {
		return Definition{}, &ValidationError{Field: "status", Message: "status must be active or inactive"}
	}
	fingerprint := fingerprintOf(struct {
		Expected                              int64
		Kind                                  DefinitionKind
		Key, DisplayName, Description, Status string
	}{input.ExpectedRevision, kind, key, displayName, description, status})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Definition{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "definition.update", commandID, fingerprint); err != nil {
		return Definition{}, err
	} else if found {
		return s.getDefinition(ctx, tx, actor.TenantID, kind, receipt.ResultID)
	}
	current, err := s.getDefinitionForUpdate(ctx, tx, actor.TenantID, kind, key)
	if err != nil {
		return Definition{}, err
	}
	if current.Revision != input.ExpectedRevision {
		return Definition{}, ErrConflict
	}
	if current.DisplayName == displayName && current.Description == description && current.Status == status {
		return Definition{}, ErrConflict
	}
	now := s.now()
	current.DisplayName = displayName
	current.Description = description
	current.Status = status
	current.Revision++
	current.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE definitions SET display_name=$1,description=$2,status=$3,revision=$4,updated_at=$5 WHERE tenant_id=$6 AND kind=$7 AND key=$8 AND revision=$9`, displayName, description, status, current.Revision, now, actor.TenantID, kind, key, input.ExpectedRevision)
	if err != nil {
		return Definition{}, translateWriteError(err)
	}
	if err := insertDefinitionRevision(ctx, tx, actor, current, "changed", now); err != nil {
		return Definition{}, err
	}
	if err := insertAudit(ctx, tx, actor, "definition.changed", "definition", string(kind)+":"+key, map[string]any{"revision": current.Revision, "status": status}); err != nil {
		return Definition{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "definition.update", commandID, fingerprint, "definition", key, current.Revision, now); err != nil {
		return Definition{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Definition{}, translateWriteError(err)
	}
	return s.getDefinition(ctx, s.pool, actor.TenantID, kind, key)
}

func (s *Service) CreateGroup(ctx context.Context, actor Actor, input CreateGroupInput) (Group, error) {
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return Group{}, err
	}
	typeKey, err := cleanKey("group_type_key", input.GroupTypeKey)
	if err != nil {
		return Group{}, err
	}
	name, err := cleanText("name", input.Name, 1, 120)
	if err != nil {
		return Group{}, err
	}
	purpose, err := cleanOptional("purpose", input.Purpose, 1000)
	if err != nil {
		return Group{}, err
	}
	fingerprint := fingerprintOf(struct{ TypeKey, Name, Purpose string }{typeKey, name, purpose})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Group{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "group.create", commandID, fingerprint); err != nil {
		return Group{}, err
	} else if found {
		return s.getGroup(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	if err := requireActiveDefinition(ctx, tx, actor.TenantID, DefinitionGroupType, typeKey); err != nil {
		return Group{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Group{}, err
	}
	now := s.now()
	group := Group{ID: id, TenantID: actor.TenantID, GroupTypeKey: typeKey, Name: name, Purpose: purpose, Status: "active", Revision: 1, CreatedAt: now, UpdatedAt: now}
	_, err = tx.Exec(ctx, `INSERT INTO groups(id,tenant_id,group_type_key,name,purpose,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,'active',1,$6,$6)`, id, actor.TenantID, typeKey, name, purpose, now)
	if err != nil {
		return Group{}, translateWriteError(err)
	}
	if err := insertGroupRevision(ctx, tx, actor, group, "created", now); err != nil {
		return Group{}, err
	}
	if err := insertAudit(ctx, tx, actor, "group.created", "group", id, map[string]any{"revision": 1}); err != nil {
		return Group{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "group.create", commandID, fingerprint, "group", id, 1, now); err != nil {
		return Group{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Group{}, translateWriteError(err)
	}
	return s.GetGroup(ctx, actor.TenantID, id)
}

func (s *Service) UpdateGroup(ctx context.Context, actor Actor, groupID string, input UpdateGroupInput) (Group, error) {
	groupID, err := cleanUUID("group_id", groupID, false)
	if err != nil {
		return Group{}, err
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return Group{}, err
	}
	if err = validateRevision(input.ExpectedRevision, "expected_revision"); err != nil {
		return Group{}, err
	}
	typeKey, err := cleanKey("group_type_key", input.GroupTypeKey)
	if err != nil {
		return Group{}, err
	}
	name, err := cleanText("name", input.Name, 1, 120)
	if err != nil {
		return Group{}, err
	}
	purpose, err := cleanOptional("purpose", input.Purpose, 1000)
	if err != nil {
		return Group{}, err
	}
	fingerprint := fingerprintOf(struct {
		Expected               int64
		TypeKey, Name, Purpose string
	}{input.ExpectedRevision, typeKey, name, purpose})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Group{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "group.update", commandID, fingerprint); err != nil {
		return Group{}, err
	} else if found {
		return s.getGroup(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	current, err := s.lockGroup(ctx, tx, actor.TenantID, groupID)
	if err != nil {
		return Group{}, err
	}
	if current.Status != "active" || current.Revision != input.ExpectedRevision {
		return Group{}, ErrConflict
	}
	if current.GroupTypeKey == typeKey && current.Name == name && current.Purpose == purpose {
		return Group{}, ErrConflict
	}
	if err := requireActiveDefinition(ctx, tx, actor.TenantID, DefinitionGroupType, typeKey); err != nil {
		return Group{}, err
	}
	now := s.now()
	current.GroupTypeKey = typeKey
	current.Name = name
	current.Purpose = purpose
	current.Revision++
	current.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE groups SET group_type_key=$1,name=$2,purpose=$3,revision=$4,updated_at=$5 WHERE tenant_id=$6 AND id=$7 AND revision=$8 AND status='active'`, typeKey, name, purpose, current.Revision, now, actor.TenantID, groupID, input.ExpectedRevision)
	if err != nil {
		return Group{}, translateWriteError(err)
	}
	if err := insertGroupRevision(ctx, tx, actor, current, "profile_changed", now); err != nil {
		return Group{}, err
	}
	if err := insertAudit(ctx, tx, actor, "group.profile_changed", "group", groupID, map[string]any{"revision": current.Revision}); err != nil {
		return Group{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "group.update", commandID, fingerprint, "group", groupID, current.Revision, now); err != nil {
		return Group{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Group{}, translateWriteError(err)
	}
	return s.GetGroup(ctx, actor.TenantID, groupID)
}

func (s *Service) ArchiveGroup(ctx context.Context, actor Actor, groupID string, input GroupCommandInput) (Group, error) {
	return s.groupStateCommand(ctx, actor, groupID, "group.archive", input, "archived")
}

func (s *Service) groupStateCommand(ctx context.Context, actor Actor, groupID, operation string, input GroupCommandInput, status string) (Group, error) {
	groupID, err := cleanUUID("group_id", groupID, false)
	if err != nil {
		return Group{}, err
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return Group{}, err
	}
	if err = validateRevision(input.ExpectedRevision, "expected_revision"); err != nil {
		return Group{}, err
	}
	fingerprint := fingerprintOf(input.ExpectedRevision)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Group{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, operation, commandID, fingerprint); err != nil {
		return Group{}, err
	} else if found {
		return s.getGroup(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	current, err := s.lockGroup(ctx, tx, actor.TenantID, groupID)
	if err != nil {
		return Group{}, err
	}
	if current.Status != "active" || current.Revision != input.ExpectedRevision {
		return Group{}, ErrConflict
	}
	now := s.now()
	current.Status = status
	current.Revision++
	current.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE groups SET status=$1,revision=$2,updated_at=$3 WHERE tenant_id=$4 AND id=$5 AND revision=$6 AND status='active'`, status, current.Revision, now, actor.TenantID, groupID, input.ExpectedRevision)
	if err != nil {
		return Group{}, translateWriteError(err)
	}
	if err := insertGroupRevision(ctx, tx, actor, current, "archived", now); err != nil {
		return Group{}, err
	}
	if err := insertAudit(ctx, tx, actor, "group.archived", "group", groupID, map[string]any{"revision": current.Revision}); err != nil {
		return Group{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, operation, commandID, fingerprint, "group", groupID, current.Revision, now); err != nil {
		return Group{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Group{}, translateWriteError(err)
	}
	return s.GetGroup(ctx, actor.TenantID, groupID)
}

func (s *Service) JoinMembership(ctx context.Context, actor Actor, groupID string, input JoinMembershipInput) (Membership, error) {
	groupID, err := cleanUUID("group_id", groupID, false)
	if err != nil {
		return Membership{}, err
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return Membership{}, err
	}
	if err = validateRevision(input.ExpectedRevision, "expected_revision"); err != nil {
		return Membership{}, err
	}
	personID, err := cleanUUID("person_id", input.PersonID, false)
	if err != nil {
		return Membership{}, err
	}
	roleKey, err := cleanKey("role_key", input.RoleKey)
	if err != nil {
		return Membership{}, err
	}
	startDate, err := cleanDate("start_date", input.StartDate)
	if err != nil {
		return Membership{}, err
	}
	fingerprint := fingerprintOf(struct {
		Expected                     int64
		PersonID, RoleKey, StartDate string
	}{input.ExpectedRevision, personID, roleKey, startDate})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Membership{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "membership.join", commandID, fingerprint); err != nil {
		return Membership{}, err
	} else if found {
		return s.getMembership(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	group, err := s.lockGroup(ctx, tx, actor.TenantID, groupID)
	if err != nil {
		return Membership{}, err
	}
	if group.Status != "active" || group.Revision != input.ExpectedRevision {
		return Membership{}, ErrConflict
	}
	person, err := s.getPerson(ctx, tx, actor.TenantID, personID)
	if err != nil {
		return Membership{}, err
	}
	if person.Status != "active" {
		return Membership{}, ErrConflict
	}
	if err := requireActiveDefinition(ctx, tx, actor.TenantID, DefinitionMembershipRole, roleKey); err != nil {
		return Membership{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Membership{}, err
	}
	now := s.now()
	group.Revision++
	group.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE groups SET revision=$1,updated_at=$2 WHERE tenant_id=$3 AND id=$4 AND revision=$5 AND status='active'`, group.Revision, now, actor.TenantID, groupID, input.ExpectedRevision)
	if err != nil {
		return Membership{}, translateWriteError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO memberships(id,tenant_id,group_id,person_id,role_key,start_date,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6::date,'active',1,$7,$7)`, id, actor.TenantID, groupID, personID, roleKey, startDate, now)
	if err != nil {
		return Membership{}, translateWriteError(err)
	}
	membership := Membership{ID: id, TenantID: actor.TenantID, GroupID: groupID, PersonID: personID, Person: person, RoleKey: roleKey, StartDate: startDate, Status: "active", Revision: 1, GroupRevision: group.Revision, CreatedAt: now, UpdatedAt: now}
	if err := insertMembershipRevision(ctx, tx, actor, membership, "joined", now); err != nil {
		return Membership{}, err
	}
	if err := insertGroupRevision(ctx, tx, actor, group, "member_joined", now); err != nil {
		return Membership{}, err
	}
	if err := insertAudit(ctx, tx, actor, "membership.joined", "membership", id, map[string]any{"group_id": groupID, "group_revision": group.Revision, "person_id": personID}); err != nil {
		return Membership{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "membership.join", commandID, fingerprint, "membership", id, 1, now); err != nil {
		return Membership{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Membership{}, translateWriteError(err)
	}
	return s.getMembership(ctx, s.pool, actor.TenantID, id)
}

func (s *Service) ChangeMembershipRole(ctx context.Context, actor Actor, groupID, membershipID string, input ChangeMembershipRoleInput) (Membership, error) {
	roleKey, err := cleanKey("role_key", input.RoleKey)
	if err != nil {
		return Membership{}, err
	}
	return s.mutateMembership(ctx, actor, groupID, membershipID, "membership.role", input.CommandID, input.ExpectedGroupRevision, input.ExpectedRevision, "role_changed", roleKey, "", func(m *Membership) error {
		if m.RoleKey == roleKey {
			return ErrConflict
		}
		m.RoleKey = roleKey
		return nil
	})
}

func (s *Service) EndMembership(ctx context.Context, actor Actor, groupID, membershipID string, input EndMembershipInput) (Membership, error) {
	endDate, err := cleanDate("end_date", input.EndDate)
	if err != nil {
		return Membership{}, err
	}
	return s.mutateMembership(ctx, actor, groupID, membershipID, "membership.end", input.CommandID, input.ExpectedGroupRevision, input.ExpectedRevision, "ended", "", endDate, func(m *Membership) error {
		if endDate < m.StartDate {
			return &ValidationError{Field: "end_date", Message: "end_date cannot be earlier than start_date"}
		}
		m.EndDate = endDate
		m.Status = "ended"
		return nil
	})
}

func (s *Service) mutateMembership(ctx context.Context, actor Actor, groupID, membershipID, operation, commandIDRaw string, expectedGroup, expectedMembership int64, eventKind, roleKey, endDate string, mutate func(*Membership) error) (Membership, error) {
	groupID, err := cleanUUID("group_id", groupID, false)
	if err != nil {
		return Membership{}, err
	}
	membershipID, err = cleanUUID("membership_id", membershipID, false)
	if err != nil {
		return Membership{}, err
	}
	commandID, err := validateCommandID(commandIDRaw)
	if err != nil {
		return Membership{}, err
	}
	if err = validateRevision(expectedGroup, "expected_group_revision"); err != nil {
		return Membership{}, err
	}
	if err = validateRevision(expectedMembership, "expected_revision"); err != nil {
		return Membership{}, err
	}
	fingerprint := fingerprintOf(struct {
		ExpectedGroup, ExpectedMembership int64
		RoleKey, EndDate                  string
	}{expectedGroup, expectedMembership, roleKey, endDate})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Membership{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, operation, commandID, fingerprint); err != nil {
		return Membership{}, err
	} else if found {
		return s.getMembership(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	group, err := s.lockGroup(ctx, tx, actor.TenantID, groupID)
	if err != nil {
		return Membership{}, err
	}
	if group.Status != "active" || group.Revision != expectedGroup {
		return Membership{}, ErrConflict
	}
	membership, err := s.lockMembership(ctx, tx, actor.TenantID, groupID, membershipID)
	if err != nil {
		return Membership{}, err
	}
	if membership.Status != "active" || membership.Revision != expectedMembership {
		return Membership{}, ErrConflict
	}
	if roleKey != "" {
		if err := requireActiveDefinition(ctx, tx, actor.TenantID, DefinitionMembershipRole, roleKey); err != nil {
			return Membership{}, err
		}
	}
	if err := mutate(&membership); err != nil {
		return Membership{}, err
	}
	now := s.now()
	membership.Revision++
	membership.UpdatedAt = now
	group.Revision++
	group.UpdatedAt = now
	membership.GroupRevision = group.Revision
	_, err = tx.Exec(ctx, `UPDATE groups SET revision=$1,updated_at=$2 WHERE tenant_id=$3 AND id=$4 AND revision=$5 AND status='active'`, group.Revision, now, actor.TenantID, groupID, expectedGroup)
	if err != nil {
		return Membership{}, translateWriteError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE memberships SET role_key=$1,end_date=NULLIF($2,'')::date,status=$3,revision=$4,updated_at=$5 WHERE tenant_id=$6 AND id=$7 AND group_id=$8 AND revision=$9 AND status='active'`, membership.RoleKey, membership.EndDate, membership.Status, membership.Revision, now, actor.TenantID, membershipID, groupID, expectedMembership)
	if err != nil {
		return Membership{}, translateWriteError(err)
	}
	if err := insertMembershipRevision(ctx, tx, actor, membership, eventKind, now); err != nil {
		return Membership{}, err
	}
	groupEvent := "member_role_changed"
	if eventKind == "ended" {
		groupEvent = "member_ended"
	}
	if err := insertGroupRevision(ctx, tx, actor, group, groupEvent, now); err != nil {
		return Membership{}, err
	}
	if err := insertAudit(ctx, tx, actor, "membership."+eventKind, "membership", membershipID, map[string]any{"group_id": groupID, "group_revision": group.Revision, "revision": membership.Revision}); err != nil {
		return Membership{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, operation, commandID, fingerprint, "membership", membershipID, membership.Revision, now); err != nil {
		return Membership{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Membership{}, translateWriteError(err)
	}
	return s.getMembership(ctx, s.pool, actor.TenantID, membershipID)
}

func (s *Service) AssignCapability(ctx context.Context, actor Actor, groupID string, input AssignCapabilityInput) (GroupCapability, error) {
	groupID, err := cleanUUID("group_id", groupID, false)
	if err != nil {
		return GroupCapability{}, err
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return GroupCapability{}, err
	}
	if err = validateRevision(input.ExpectedRevision, "expected_revision"); err != nil {
		return GroupCapability{}, err
	}
	key, err := cleanKey("capability_key", input.CapabilityKey)
	if err != nil {
		return GroupCapability{}, err
	}
	note, err := cleanOptional("note", input.Note, 500)
	if err != nil {
		return GroupCapability{}, err
	}
	startDate, err := cleanDate("start_date", input.StartDate)
	if err != nil {
		return GroupCapability{}, err
	}
	fingerprint := fingerprintOf(struct {
		Expected             int64
		Key, Note, StartDate string
	}{input.ExpectedRevision, key, note, startDate})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return GroupCapability{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "group-capability.assign", commandID, fingerprint); err != nil {
		return GroupCapability{}, err
	} else if found {
		return s.getCapability(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	group, err := s.lockGroup(ctx, tx, actor.TenantID, groupID)
	if err != nil {
		return GroupCapability{}, err
	}
	if group.Status != "active" || group.Revision != input.ExpectedRevision {
		return GroupCapability{}, ErrConflict
	}
	if err := requireActiveDefinition(ctx, tx, actor.TenantID, DefinitionCapability, key); err != nil {
		return GroupCapability{}, err
	}
	id, err := s.newID()
	if err != nil {
		return GroupCapability{}, err
	}
	now := s.now()
	group.Revision++
	group.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE groups SET revision=$1,updated_at=$2 WHERE tenant_id=$3 AND id=$4 AND revision=$5 AND status='active'`, group.Revision, now, actor.TenantID, groupID, input.ExpectedRevision)
	if err != nil {
		return GroupCapability{}, translateWriteError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO group_capability_assignments(id,tenant_id,group_id,capability_key,note,start_date,status,revision,created_at,updated_at) VALUES($1,$2,$3,$4,$5,$6::date,'active',1,$7,$7)`, id, actor.TenantID, groupID, key, note, startDate, now)
	if err != nil {
		return GroupCapability{}, translateWriteError(err)
	}
	assignment := GroupCapability{ID: id, TenantID: actor.TenantID, GroupID: groupID, CapabilityKey: key, Note: note, StartDate: startDate, Status: "active", Revision: 1, GroupRevision: group.Revision, CreatedAt: now, UpdatedAt: now}
	if err := insertCapabilityRevision(ctx, tx, actor, assignment, "assigned", now); err != nil {
		return GroupCapability{}, err
	}
	if err := insertGroupRevision(ctx, tx, actor, group, "capability_assigned", now); err != nil {
		return GroupCapability{}, err
	}
	if err := insertAudit(ctx, tx, actor, "group_capability.assigned", "group_capability", id, map[string]any{"group_id": groupID, "group_revision": group.Revision}); err != nil {
		return GroupCapability{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "group-capability.assign", commandID, fingerprint, "group_capability", id, 1, now); err != nil {
		return GroupCapability{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GroupCapability{}, translateWriteError(err)
	}
	return s.getCapability(ctx, s.pool, actor.TenantID, id)
}

func (s *Service) EndCapability(ctx context.Context, actor Actor, groupID, assignmentID string, input EndCapabilityInput) (GroupCapability, error) {
	groupID, err := cleanUUID("group_id", groupID, false)
	if err != nil {
		return GroupCapability{}, err
	}
	assignmentID, err = cleanUUID("assignment_id", assignmentID, false)
	if err != nil {
		return GroupCapability{}, err
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return GroupCapability{}, err
	}
	if err = validateRevision(input.ExpectedGroupRevision, "expected_group_revision"); err != nil {
		return GroupCapability{}, err
	}
	if err = validateRevision(input.ExpectedRevision, "expected_revision"); err != nil {
		return GroupCapability{}, err
	}
	endDate, err := cleanDate("end_date", input.EndDate)
	if err != nil {
		return GroupCapability{}, err
	}
	fingerprint := fingerprintOf(struct {
		EG, EA int64
		End    string
	}{input.ExpectedGroupRevision, input.ExpectedRevision, endDate})
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return GroupCapability{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "group-capability.end", commandID, fingerprint); err != nil {
		return GroupCapability{}, err
	} else if found {
		return s.getCapability(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	group, err := s.lockGroup(ctx, tx, actor.TenantID, groupID)
	if err != nil {
		return GroupCapability{}, err
	}
	if group.Status != "active" || group.Revision != input.ExpectedGroupRevision {
		return GroupCapability{}, ErrConflict
	}
	assignment, err := s.lockCapability(ctx, tx, actor.TenantID, groupID, assignmentID)
	if err != nil {
		return GroupCapability{}, err
	}
	if assignment.Status != "active" || assignment.Revision != input.ExpectedRevision || endDate < assignment.StartDate {
		return GroupCapability{}, ErrConflict
	}
	now := s.now()
	group.Revision++
	group.UpdatedAt = now
	assignment.Status = "ended"
	assignment.EndDate = endDate
	assignment.Revision++
	assignment.GroupRevision = group.Revision
	assignment.UpdatedAt = now
	_, err = tx.Exec(ctx, `UPDATE groups SET revision=$1,updated_at=$2 WHERE tenant_id=$3 AND id=$4 AND revision=$5 AND status='active'`, group.Revision, now, actor.TenantID, groupID, input.ExpectedGroupRevision)
	if err != nil {
		return GroupCapability{}, translateWriteError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE group_capability_assignments SET end_date=$1::date,status='ended',revision=$2,updated_at=$3 WHERE tenant_id=$4 AND id=$5 AND group_id=$6 AND revision=$7 AND status='active'`, endDate, assignment.Revision, now, actor.TenantID, assignmentID, groupID, input.ExpectedRevision)
	if err != nil {
		return GroupCapability{}, translateWriteError(err)
	}
	if err := insertCapabilityRevision(ctx, tx, actor, assignment, "ended", now); err != nil {
		return GroupCapability{}, err
	}
	if err := insertGroupRevision(ctx, tx, actor, group, "capability_ended", now); err != nil {
		return GroupCapability{}, err
	}
	if err := insertAudit(ctx, tx, actor, "group_capability.ended", "group_capability", assignmentID, map[string]any{"group_id": groupID, "group_revision": group.Revision}); err != nil {
		return GroupCapability{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "group-capability.end", commandID, fingerprint, "group_capability", assignmentID, assignment.Revision, now); err != nil {
		return GroupCapability{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GroupCapability{}, translateWriteError(err)
	}
	return s.getCapability(ctx, s.pool, actor.TenantID, assignmentID)
}

type rosterPayload struct {
	SchemaVersion     string             `json:"schema_version"`
	SnapshotID        string             `json:"snapshot_id"`
	SourceApplication string             `json:"source_application"`
	TenantID          string             `json:"tenant_id"`
	GeneratedAt       time.Time          `json:"generated_at"`
	Group             rosterGroup        `json:"group"`
	Members           []rosterMember     `json:"members"`
	Capabilities      []rosterCapability `json:"capabilities"`
	PublishedBy       Actor              `json:"published_by"`
}
type rosterGroup struct {
	ID           string `json:"id"`
	Revision     int64  `json:"revision"`
	GroupTypeKey string `json:"group_type_key"`
	Name         string `json:"name"`
	Purpose      string `json:"purpose"`
	Status       string `json:"status"`
}
type rosterMember struct {
	MembershipID       string `json:"membership_id"`
	MembershipRevision int64  `json:"membership_revision"`
	RoleKey            string `json:"role_key"`
	StartDate          string `json:"start_date"`
	Person             struct {
		ID            string `json:"id"`
		Revision      int64  `json:"revision"`
		DisplayName   string `json:"display_name"`
		ReferenceCode string `json:"reference_code,omitempty"`
	} `json:"person"`
}
type rosterCapability struct {
	AssignmentID string `json:"assignment_id"`
	Revision     int64  `json:"revision"`
	Key          string `json:"key"`
	Note         string `json:"note"`
	StartDate    string `json:"start_date"`
}

func (s *Service) PublishSnapshot(ctx context.Context, actor Actor, groupID string, input GroupCommandInput) (RosterSnapshot, error) {
	groupID, err := cleanUUID("group_id", groupID, false)
	if err != nil {
		return RosterSnapshot{}, err
	}
	commandID, err := validateCommandID(input.CommandID)
	if err != nil {
		return RosterSnapshot{}, err
	}
	if err = validateRevision(input.ExpectedRevision, "expected_revision"); err != nil {
		return RosterSnapshot{}, err
	}
	fingerprint := fingerprintOf(input.ExpectedRevision)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return RosterSnapshot{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if receipt, found, err := loadReceipt(ctx, tx, actor.TenantID, "snapshot.publish", commandID, fingerprint); err != nil {
		return RosterSnapshot{}, err
	} else if found {
		return s.getSnapshotByID(ctx, tx, actor.TenantID, receipt.ResultID)
	}
	group, err := s.lockGroup(ctx, tx, actor.TenantID, groupID)
	if err != nil {
		return RosterSnapshot{}, err
	}
	if group.Status != "active" || group.Revision != input.ExpectedRevision {
		return RosterSnapshot{}, ErrConflict
	}
	members, err := s.listMemberships(ctx, tx, actor.TenantID, groupID, true)
	if err != nil {
		return RosterSnapshot{}, err
	}
	caps, err := s.listCapabilities(ctx, tx, actor.TenantID, groupID, true)
	if err != nil {
		return RosterSnapshot{}, err
	}
	id, err := s.newID()
	if err != nil {
		return RosterSnapshot{}, err
	}
	now := s.now()
	payload := rosterPayload{SchemaVersion: "1", SnapshotID: id, SourceApplication: "people-groups", TenantID: actor.TenantID, GeneratedAt: now, Group: rosterGroup{ID: group.ID, Revision: group.Revision, GroupTypeKey: group.GroupTypeKey, Name: group.Name, Purpose: group.Purpose, Status: group.Status}, Members: make([]rosterMember, 0, len(members)), Capabilities: make([]rosterCapability, 0, len(caps)), PublishedBy: actor}
	for _, m := range members {
		item := rosterMember{MembershipID: m.ID, MembershipRevision: m.Revision, RoleKey: m.RoleKey, StartDate: m.StartDate}
		item.Person.ID = m.Person.ID
		item.Person.Revision = m.Person.Revision
		item.Person.DisplayName = m.Person.DisplayName
		item.Person.ReferenceCode = m.Person.ReferenceCode
		payload.Members = append(payload.Members, item)
	}
	for _, c := range caps {
		payload.Capabilities = append(payload.Capabilities, rosterCapability{AssignmentID: c.ID, Revision: c.Revision, Key: c.CapabilityKey, Note: c.Note, StartDate: c.StartDate})
	}
	sort.Slice(payload.Members, func(i, j int) bool { return payload.Members[i].MembershipID < payload.Members[j].MembershipID })
	sort.Slice(payload.Capabilities, func(i, j int) bool {
		return payload.Capabilities[i].AssignmentID < payload.Capabilities[j].AssignmentID
	})
	data, err := json.Marshal(payload)
	if err != nil {
		return RosterSnapshot{}, err
	}
	digest := sha256.Sum256(data)
	digestHex := hex.EncodeToString(digest[:])
	_, err = tx.Exec(ctx, `INSERT INTO roster_snapshots(id,tenant_id,group_id,group_revision,payload,sha256,created_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, actor.TenantID, groupID, group.Revision, data, digestHex, now, actor.Kind, actor.ID, actor.DisplayName)
	if err != nil {
		return RosterSnapshot{}, translateWriteError(err)
	}
	if err := insertAudit(ctx, tx, actor, "roster_snapshot.published", "roster_snapshot", id, map[string]any{"group_id": groupID, "group_revision": group.Revision, "sha256": digestHex}); err != nil {
		return RosterSnapshot{}, err
	}
	if err := saveReceipt(ctx, tx, actor.TenantID, "snapshot.publish", commandID, fingerprint, "roster_snapshot", id, group.Revision, now); err != nil {
		return RosterSnapshot{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RosterSnapshot{}, translateWriteError(err)
	}
	return RosterSnapshot{ID: id, TenantID: actor.TenantID, GroupID: groupID, GroupRevision: group.Revision, Payload: data, SHA256: digestHex, CreatedAt: now}, nil
}

func (s *Service) ListPersons(ctx context.Context, tenantID string) ([]Person, error) {
	rows, err := s.pool.Query(ctx, personSelect+` WHERE tenant_id=$1 ORDER BY lower(display_name),id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPersons(rows)
}
func (s *Service) GetPerson(ctx context.Context, tenantID, id string) (Person, error) {
	return s.getPerson(ctx, s.pool, tenantID, id)
}
func (s *Service) GetPersonByIAMSubject(ctx context.Context, tenantID, subjectID string) (Person, error) {
	return scanPerson(s.pool.QueryRow(ctx, personSelect+` WHERE tenant_id=$1 AND iam_subject_id=$2`, tenantID, subjectID))
}
func (s *Service) ListEmploymentEvents(ctx context.Context, tenantID, personID string) ([]EmploymentEvent, error) {
	rows, err := s.pool.Query(ctx, `SELECT person_id::text,sequence,person_revision,event_kind,effective_date::text,reason,department,position_title,occurred_at FROM person_employment_events WHERE tenant_id=$1 AND person_id=$2 ORDER BY sequence`, tenantID, personID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []EmploymentEvent{}
	for rows.Next() {
		var value EmploymentEvent
		if err := rows.Scan(&value.PersonID, &value.Sequence, &value.PersonRevision, &value.EventKind, &value.EffectiveDate, &value.Reason, &value.Department, &value.PositionTitle, &value.OccurredAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Service) ListDefinitions(ctx context.Context, tenantID string) ([]Definition, error) {
	rows, err := s.pool.Query(ctx, definitionSelect+` WHERE tenant_id=$1 ORDER BY kind,key`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []Definition{}
	for rows.Next() {
		value, err := scanDefinition(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Service) ListGroups(ctx context.Context, tenantID string) ([]Group, error) {
	rows, err := s.pool.Query(ctx, groupSelect+` WHERE tenant_id=$1 ORDER BY CASE status WHEN 'active' THEN 0 ELSE 1 END,lower(name),id`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []Group{}
	for rows.Next() {
		value, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Service) GetGroup(ctx context.Context, tenantID, id string) (Group, error) {
	return s.getGroup(ctx, s.pool, tenantID, id)
}
func (s *Service) GetGroupDetails(ctx context.Context, tenantID, id string) (GroupDetails, error) {
	group, err := s.GetGroup(ctx, tenantID, id)
	if err != nil {
		return GroupDetails{}, err
	}
	members, err := s.listMemberships(ctx, s.pool, tenantID, id, false)
	if err != nil {
		return GroupDetails{}, err
	}
	caps, err := s.listCapabilities(ctx, s.pool, tenantID, id, false)
	if err != nil {
		return GroupDetails{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text,group_revision,sha256,created_at FROM roster_snapshots WHERE tenant_id=$1 AND group_id=$2 ORDER BY group_revision DESC`, tenantID, id)
	if err != nil {
		return GroupDetails{}, err
	}
	defer rows.Close()
	snapshots := []SnapshotSummary{}
	for rows.Next() {
		var item SnapshotSummary
		if err := rows.Scan(&item.ID, &item.GroupRevision, &item.SHA256, &item.CreatedAt); err != nil {
			return GroupDetails{}, err
		}
		snapshots = append(snapshots, item)
	}
	return GroupDetails{Group: group, Memberships: members, Capabilities: caps, Snapshots: snapshots}, rows.Err()
}
func (s *Service) GetSnapshot(ctx context.Context, tenantID, groupID string, revision int64) (RosterSnapshot, error) {
	return scanSnapshot(s.pool.QueryRow(ctx, `SELECT id::text,tenant_id::text,group_id::text,group_revision,payload,sha256,created_at FROM roster_snapshots WHERE tenant_id=$1 AND group_id=$2 AND group_revision=$3`, tenantID, groupID, revision))
}

const personSelect = `SELECT id::text,tenant_id::text,COALESCE(reference_code,''),display_name,department,position_title,status,status_effective_date::text,COALESCE(iam_subject_id::text,''),COALESCE(iam_subject_status,''),COALESCE(iam_subject_revision,0),COALESCE((SELECT actor_id::text FROM person_revisions WHERE tenant_id=persons.tenant_id AND person_id=persons.id AND revision=persons.revision),''),COALESCE((SELECT actor_display_name FROM person_revisions WHERE tenant_id=persons.tenant_id AND person_id=persons.id AND revision=persons.revision),''),revision,created_at,updated_at FROM persons`
const definitionSelect = `SELECT kind,key,display_name,description,status,revision,created_at,updated_at FROM definitions`
const groupSelect = `SELECT id::text,tenant_id::text,group_type_key,name,purpose,status,revision,created_at,updated_at FROM groups`

func (s *Service) getPerson(ctx context.Context, q queryer, tenantID, id string) (Person, error) {
	return scanPerson(q.QueryRow(ctx, personSelect+` WHERE tenant_id=$1 AND id=$2`, tenantID, id))
}
func (s *Service) lockPerson(ctx context.Context, tx pgx.Tx, tenantID, id string) (Person, error) {
	return scanPerson(tx.QueryRow(ctx, personSelect+` WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenantID, id))
}
func scanPerson(row pgx.Row) (Person, error) {
	var p Person
	if err := row.Scan(&p.ID, &p.TenantID, &p.ReferenceCode, &p.DisplayName, &p.Department, &p.PositionTitle, &p.Status, &p.StatusEffectiveDate, &p.IAMSubjectID, &p.IAMSubjectStatus, &p.IAMSubjectRevision, &p.LastActorSubjectID, &p.LastActorDisplayName, &p.Revision, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return Person{}, translateReadError(err)
	}
	return p, nil
}
func scanPersons(rows pgx.Rows) ([]Person, error) {
	values := []Person{}
	for rows.Next() {
		value, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Service) getDefinition(ctx context.Context, q queryer, tenant string, kind DefinitionKind, key string) (Definition, error) {
	return scanDefinition(q.QueryRow(ctx, definitionSelect+` WHERE tenant_id=$1 AND kind=$2 AND key=$3`, tenant, kind, key))
}
func (s *Service) getDefinitionForUpdate(ctx context.Context, tx pgx.Tx, tenant string, kind DefinitionKind, key string) (Definition, error) {
	return scanDefinition(tx.QueryRow(ctx, definitionSelect+` WHERE tenant_id=$1 AND kind=$2 AND key=$3 FOR UPDATE`, tenant, kind, key))
}
func scanDefinition(row pgx.Row) (Definition, error) {
	var d Definition
	if err := row.Scan(&d.Kind, &d.Key, &d.DisplayName, &d.Description, &d.Status, &d.Revision, &d.CreatedAt, &d.UpdatedAt); err != nil {
		return Definition{}, translateReadError(err)
	}
	return d, nil
}
func (s *Service) getGroup(ctx context.Context, q queryer, tenant, id string) (Group, error) {
	return scanGroup(q.QueryRow(ctx, groupSelect+` WHERE tenant_id=$1 AND id=$2`, tenant, id))
}
func (s *Service) lockGroup(ctx context.Context, tx pgx.Tx, tenant, id string) (Group, error) {
	return scanGroup(tx.QueryRow(ctx, groupSelect+` WHERE tenant_id=$1 AND id=$2 FOR UPDATE`, tenant, id))
}
func scanGroup(row pgx.Row) (Group, error) {
	var g Group
	if err := row.Scan(&g.ID, &g.TenantID, &g.GroupTypeKey, &g.Name, &g.Purpose, &g.Status, &g.Revision, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return Group{}, translateReadError(err)
	}
	return g, nil
}

func (s *Service) getMembership(ctx context.Context, q queryer, tenant, id string) (Membership, error) {
	rows, err := q.Query(ctx, membershipSelect+` WHERE m.tenant_id=$1 AND m.id=$2`, tenant, id)
	if err != nil {
		return Membership{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Membership{}, ErrNotFound
	}
	m, err := scanMembership(rows)
	if err != nil {
		return Membership{}, err
	}
	return m, nil
}
func (s *Service) lockMembership(ctx context.Context, tx pgx.Tx, tenant, group, id string) (Membership, error) {
	rows, err := tx.Query(ctx, membershipSelect+` WHERE m.tenant_id=$1 AND m.group_id=$2 AND m.id=$3 FOR UPDATE OF m`, tenant, group, id)
	if err != nil {
		return Membership{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return Membership{}, ErrNotFound
	}
	return scanMembership(rows)
}

const membershipSelect = `SELECT m.id::text,m.tenant_id::text,m.group_id::text,m.person_id::text,m.role_key,to_char(m.start_date,'YYYY-MM-DD'),COALESCE(to_char(m.end_date,'YYYY-MM-DD'),''),m.status,m.revision,g.revision,m.created_at,m.updated_at,p.id::text,p.tenant_id::text,COALESCE(p.reference_code,''),p.display_name,p.status,COALESCE(p.iam_subject_id::text,''),COALESCE(p.iam_subject_status,''),COALESCE(p.iam_subject_revision,0),p.revision,p.created_at,p.updated_at FROM memberships m JOIN persons p ON p.tenant_id=m.tenant_id AND p.id=m.person_id JOIN groups g ON g.tenant_id=m.tenant_id AND g.id=m.group_id`

func scanMembership(row pgx.Row) (Membership, error) {
	var m Membership
	if err := row.Scan(&m.ID, &m.TenantID, &m.GroupID, &m.PersonID, &m.RoleKey, &m.StartDate, &m.EndDate, &m.Status, &m.Revision, &m.GroupRevision, &m.CreatedAt, &m.UpdatedAt, &m.Person.ID, &m.Person.TenantID, &m.Person.ReferenceCode, &m.Person.DisplayName, &m.Person.Status, &m.Person.IAMSubjectID, &m.Person.IAMSubjectStatus, &m.Person.IAMSubjectRevision, &m.Person.Revision, &m.Person.CreatedAt, &m.Person.UpdatedAt); err != nil {
		return Membership{}, translateReadError(err)
	}
	return m, nil
}
func (s *Service) listMemberships(ctx context.Context, q queryer, tenant, group string, activeOnly bool) ([]Membership, error) {
	query := membershipSelect + ` WHERE m.tenant_id=$1 AND m.group_id=$2`
	if activeOnly {
		query += ` AND m.status='active'`
	}
	query += ` ORDER BY m.start_date,m.id`
	rows, err := q.Query(ctx, query, tenant, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []Membership{}
	for rows.Next() {
		value, err := scanMembership(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

const capabilitySelect = `SELECT c.id::text,c.tenant_id::text,c.group_id::text,c.capability_key,c.note,to_char(c.start_date,'YYYY-MM-DD'),COALESCE(to_char(c.end_date,'YYYY-MM-DD'),''),c.status,c.revision,g.revision,c.created_at,c.updated_at FROM group_capability_assignments c JOIN groups g ON g.tenant_id=c.tenant_id AND g.id=c.group_id`

func (s *Service) getCapability(ctx context.Context, q queryer, tenant, id string) (GroupCapability, error) {
	return scanCapability(q.QueryRow(ctx, capabilitySelect+` WHERE c.tenant_id=$1 AND c.id=$2`, tenant, id))
}
func (s *Service) lockCapability(ctx context.Context, tx pgx.Tx, tenant, group, id string) (GroupCapability, error) {
	return scanCapability(tx.QueryRow(ctx, capabilitySelect+` WHERE c.tenant_id=$1 AND c.group_id=$2 AND c.id=$3 FOR UPDATE OF c`, tenant, group, id))
}
func scanCapability(row pgx.Row) (GroupCapability, error) {
	var c GroupCapability
	if err := row.Scan(&c.ID, &c.TenantID, &c.GroupID, &c.CapabilityKey, &c.Note, &c.StartDate, &c.EndDate, &c.Status, &c.Revision, &c.GroupRevision, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return GroupCapability{}, translateReadError(err)
	}
	return c, nil
}
func (s *Service) listCapabilities(ctx context.Context, q queryer, tenant, group string, activeOnly bool) ([]GroupCapability, error) {
	query := capabilitySelect + ` WHERE c.tenant_id=$1 AND c.group_id=$2`
	if activeOnly {
		query += ` AND c.status='active'`
	}
	query += ` ORDER BY c.start_date,c.id`
	rows, err := q.Query(ctx, query, tenant, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := []GroupCapability{}
	for rows.Next() {
		value, err := scanCapability(rows)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}
func (s *Service) getSnapshotByID(ctx context.Context, q queryer, tenant, id string) (RosterSnapshot, error) {
	return scanSnapshot(q.QueryRow(ctx, `SELECT id::text,tenant_id::text,group_id::text,group_revision,payload,sha256,created_at FROM roster_snapshots WHERE tenant_id=$1 AND id=$2`, tenant, id))
}
func scanSnapshot(row pgx.Row) (RosterSnapshot, error) {
	var value RosterSnapshot
	if err := row.Scan(&value.ID, &value.TenantID, &value.GroupID, &value.GroupRevision, &value.Payload, &value.SHA256, &value.CreatedAt); err != nil {
		return RosterSnapshot{}, translateReadError(err)
	}
	return value, nil
}

func requireActiveDefinition(ctx context.Context, q queryer, tenant string, kind DefinitionKind, key string) error {
	var status string
	if err := q.QueryRow(ctx, `SELECT status FROM definitions WHERE tenant_id=$1 AND kind=$2 AND key=$3`, tenant, kind, key).Scan(&status); err != nil {
		return translateReadError(err)
	}
	if status != "active" {
		return ErrConflict
	}
	return nil
}
func insertPersonRevision(ctx context.Context, tx pgx.Tx, actor Actor, p Person, event string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO person_revisions(tenant_id,person_id,revision,event_kind,reference_code,display_name,department,position_title,status,status_effective_date,iam_subject_id,iam_subject_status,iam_subject_revision,occurred_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10::date,NULLIF($11,'')::uuid,NULLIF($12,''),NULLIF($13,0),$14,$15,$16,$17)`, actor.TenantID, p.ID, p.Revision, event, p.ReferenceCode, p.DisplayName, p.Department, p.PositionTitle, p.Status, p.StatusEffectiveDate, p.IAMSubjectID, p.IAMSubjectStatus, p.IAMSubjectRevision, at, actor.Kind, actor.ID, actor.DisplayName)
	return err
}
func insertEmploymentEvent(ctx context.Context, tx pgx.Tx, actor Actor, p Person, event, effectiveDate, reason string, at time.Time) error {
	var sequence int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence),0)+1 FROM person_employment_events WHERE tenant_id=$1 AND person_id=$2`, actor.TenantID, p.ID).Scan(&sequence); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO person_employment_events(tenant_id,person_id,sequence,person_revision,event_kind,effective_date,reason,department,position_title,occurred_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,$3,$4,$5,$6::date,$7,$8,$9,$10,$11,$12,$13)`, actor.TenantID, p.ID, sequence, p.Revision, event, effectiveDate, reason, p.Department, p.PositionTitle, at, actor.Kind, actor.ID, actor.DisplayName)
	return err
}
func insertDefinitionRevision(ctx context.Context, tx pgx.Tx, actor Actor, d Definition, event string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO definition_revisions(tenant_id,kind,key,revision,event_kind,display_name,description,status,occurred_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, actor.TenantID, d.Kind, d.Key, d.Revision, event, d.DisplayName, d.Description, d.Status, at, actor.Kind, actor.ID, actor.DisplayName)
	return err
}
func insertGroupRevision(ctx context.Context, tx pgx.Tx, actor Actor, g Group, event string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO group_revisions(tenant_id,group_id,revision,event_kind,group_type_key,name,purpose,status,occurred_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, actor.TenantID, g.ID, g.Revision, event, g.GroupTypeKey, g.Name, g.Purpose, g.Status, at, actor.Kind, actor.ID, actor.DisplayName)
	return err
}
func insertMembershipRevision(ctx context.Context, tx pgx.Tx, actor Actor, m Membership, event string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO membership_revisions(tenant_id,membership_id,revision,event_kind,group_id,person_id,role_key,start_date,end_date,status,group_revision,occurred_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8::date,NULLIF($9,'')::date,$10,$11,$12,$13,$14,$15)`, actor.TenantID, m.ID, m.Revision, event, m.GroupID, m.PersonID, m.RoleKey, m.StartDate, m.EndDate, m.Status, m.GroupRevision, at, actor.Kind, actor.ID, actor.DisplayName)
	return err
}
func insertCapabilityRevision(ctx context.Context, tx pgx.Tx, actor Actor, c GroupCapability, event string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO group_capability_revisions(tenant_id,assignment_id,revision,event_kind,group_id,capability_key,note,start_date,end_date,status,group_revision,occurred_at,actor_kind,actor_id,actor_display_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8::date,NULLIF($9,'')::date,$10,$11,$12,$13,$14,$15)`, actor.TenantID, c.ID, c.Revision, event, c.GroupID, c.CapabilityKey, c.Note, c.StartDate, c.EndDate, c.Status, c.GroupRevision, at, actor.Kind, actor.ID, actor.DisplayName)
	return err
}
func insertAudit(ctx context.Context, tx pgx.Tx, actor Actor, action, objectType, objectID string, details any) error {
	id, err := randomUUID()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(details)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO people_groups_audit_entries(id,tenant_id,action,object_type,object_id,actor_kind,actor_id,actor_display_name,occurred_at,details) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb)`, id, actor.TenantID, action, objectType, objectID, actor.Kind, actor.ID, actor.DisplayName, time.Now().UTC(), raw)
	return err
}

func (s *Service) bumpGroupsForPerson(ctx context.Context, tx pgx.Tx, actor Actor, personID string, at time.Time) error {
	rows, err := tx.Query(ctx, `SELECT g.id::text FROM groups g JOIN memberships m ON m.tenant_id=g.tenant_id AND m.group_id=g.id WHERE g.tenant_id=$1 AND g.status='active' AND m.person_id=$2 AND m.status='active' ORDER BY g.id FOR UPDATE OF g`, actor.TenantID, personID)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		group, err := s.lockGroup(ctx, tx, actor.TenantID, id)
		if err != nil {
			return err
		}
		group.Revision++
		group.UpdatedAt = at
		if _, err := tx.Exec(ctx, `UPDATE groups SET revision=$1,updated_at=$2 WHERE tenant_id=$3 AND id=$4`, group.Revision, at, actor.TenantID, id); err != nil {
			return err
		}
		if err := insertGroupRevision(ctx, tx, actor, group, "member_person_changed", at); err != nil {
			return err
		}
	}
	return nil
}

func loadReceipt(ctx context.Context, tx pgx.Tx, tenant, operation, commandID string, fingerprint []byte) (commandReceipt, bool, error) {
	var r commandReceipt
	err := tx.QueryRow(ctx, `SELECT fingerprint,result_type,result_id,result_revision FROM business_commands WHERE tenant_id=$1 AND operation=$2 AND command_id=$3`, tenant, operation, commandID).Scan(&r.Fingerprint, &r.ResultType, &r.ResultID, &r.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return commandReceipt{}, false, nil
	}
	if err != nil {
		return commandReceipt{}, false, err
	}
	if !equalBytes(r.Fingerprint, fingerprint) {
		return commandReceipt{}, false, ErrConflict
	}
	return r, true, nil
}
func saveReceipt(ctx context.Context, tx pgx.Tx, tenant, operation, commandID string, fingerprint []byte, resultType, resultID string, revision int64, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO business_commands(tenant_id,operation,command_id,fingerprint,result_type,result_id,result_revision,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, tenant, operation, commandID, fingerprint, resultType, resultID, revision, at)
	return translateWriteError(err)
}
func fingerprintOf(value any) []byte {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return sum[:]
}
func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for i := range left {
		diff |= left[i] ^ right[i]
	}
	return diff == 0
}
func translateReadError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
func translateWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503", "23514", "40001":
			return ErrConflict
		}
	}
	return err
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
