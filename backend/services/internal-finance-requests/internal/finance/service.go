package finance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type Store interface {
	Ping(context.Context) error
	Create(context.Context, Actor, CreateInput, string) (Request, error)
	Update(context.Context, Actor, string, UpdateInput, string) (Request, error)
	Submit(context.Context, Actor, string, SubmitInput, string) (Request, error)
	Withdraw(context.Context, Actor, string, WithdrawInput, string) (Request, error)
	PreparePayment(context.Context, Actor, string, ConfirmPaymentInput, string) (Request, PaymentDelivery, error)
	CompletePayment(context.Context, Actor, string, string, MoneyPaymentResult) (Request, error)
	MarkPaymentFailed(context.Context, Actor, string, string, string) error
	MarkPaymentUnknown(context.Context, Actor, string, string) error
	RecordSensitiveAccess(context.Context, Actor, string, string, string) error
	CreateCategory(context.Context, Actor, string, CategoryCreateInput, string) (ExpenseCategory, error)
	UpdateCategory(context.Context, Actor, ExpenseCategory, CategoryUpdateInput, string) (ExpenseCategory, error)
	ListCategories(context.Context, string) ([]ExpenseCategory, error)
	Get(context.Context, string, string) (Request, error)
	List(context.Context, string, ListFilter) ([]Request, error)
}

type MoneyPayments interface {
	Record(context.Context, MoneyPaymentRequest) (MoneyPaymentResult, error)
}

type Service struct {
	store    Store
	payments MoneyPayments
}

var platformCategories = []ExpenseCategory{
	{ID: "10000000-0000-4000-8000-000000000001", Key: "travel", DisplayName: "差旅费", Source: "platform", Enabled: true, Revision: 1},
	{ID: "10000000-0000-4000-8000-000000000002", Key: "meals", DisplayName: "餐饮费", Source: "platform", Enabled: true, Revision: 1},
	{ID: "10000000-0000-4000-8000-000000000003", Key: "transport", DisplayName: "交通费", Source: "platform", Enabled: true, Revision: 1},
	{ID: "10000000-0000-4000-8000-000000000004", Key: "accommodation", DisplayName: "住宿费", Source: "platform", Enabled: true, Revision: 1},
	{ID: "10000000-0000-4000-8000-000000000005", Key: "office", DisplayName: "办公费", Source: "platform", Enabled: true, Revision: 1},
	{ID: "10000000-0000-4000-8000-000000000006", Key: "communication", DisplayName: "通讯费", Source: "platform", Enabled: true, Revision: 1},
	{ID: "10000000-0000-4000-8000-000000000007", Key: "service", DisplayName: "服务费", Source: "platform", Enabled: true, Revision: 1},
	{ID: "10000000-0000-4000-8000-000000000008", Key: "other", DisplayName: "其他", Source: "platform", Enabled: true, Revision: 1},
}

func NewService(store Store) (*Service, error) { return NewServiceWithPayments(store, nil) }
func NewServiceWithPayments(store Store, payments MoneyPayments) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("store is required")
	}
	return &Service{store: store, payments: payments}, nil
}
func (s *Service) Ping(ctx context.Context) error { return s.store.Ping(ctx) }

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Request, error) {
	normalizeCreate(&input)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if err := validateCreate(input); err != nil {
		return Request{}, err
	}
	if len(input.Details.Lines) > 0 {
		if err := normalizeDetails(input.RequestType, &input.Details, false); err != nil {
			return Request{}, err
		}
		input.AmountMinor = totalLines(input.Details.Lines)
	}
	value, err := s.store.Create(ctx, actor, input, hash(input))
	return redactRequest(value, false), err
}

func (s *Service) Update(ctx context.Context, actor Actor, id string, input UpdateInput) (Request, error) {
	id, input.CommandID, input.Title = normalizeID(id), normalizeID(input.CommandID), strings.TrimSpace(input.Title)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.Title == "" || len([]rune(input.Title)) > 200 {
		return Request{}, fmt.Errorf("invalid update")
	}
	current, err := s.store.Get(ctx, actor.TenantID, id)
	if err != nil {
		return Request{}, err
	}
	if len(input.Details.Lines) > 0 {
		if err = normalizeDetails(current.RequestType, &input.Details, false); err != nil {
			return Request{}, err
		}
		input.AmountMinor = totalLines(input.Details.Lines)
	}
	if input.AmountMinor < 1 {
		return Request{}, fmt.Errorf("invalid update amount")
	}
	value, err := s.store.Update(ctx, actor, id, input, hash(input))
	return redactRequest(value, false), err
}

func (s *Service) Submit(ctx context.Context, actor Actor, id string, input SubmitInput) (Request, error) {
	id, input.CommandID = normalizeID(id), normalizeID(input.CommandID)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 {
		return Request{}, fmt.Errorf("invalid submit")
	}
	value, err := s.store.Get(ctx, actor.TenantID, id)
	if err != nil {
		return Request{}, err
	}
	if value.ApplicantSubjectID != actor.ID {
		return Request{}, ErrNotFound
	}
	if value.Revision != input.ExpectedRevision {
		return Request{}, ErrRevisionConflict
	}
	if value.Status != "draft" && value.Status != "withdrawn" {
		return Request{}, ErrStateConflict
	}
	value.Details.MissingEvidenceAcknowledged = input.MissingEvidenceAcknowledged
	if err = normalizeDetails(value.RequestType, &value.Details, true); err != nil {
		return Request{}, err
	}
	if value.RequestType == "payment" && !hasEvidence(value.Details) && !input.MissingEvidenceAcknowledged {
		return Request{}, fmt.Errorf("invalid submit: payment without Evidence requires acknowledgement")
	}
	value, err = s.store.Submit(ctx, actor, id, input, hash(input))
	return redactRequest(value, false), err
}

func (s *Service) Withdraw(ctx context.Context, actor Actor, id string, input WithdrawInput) (Request, error) {
	id, input.CommandID, input.Reason = normalizeID(id), normalizeID(input.CommandID), strings.TrimSpace(input.Reason)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.Reason == "" || len([]rune(input.Reason)) > 1000 {
		return Request{}, fmt.Errorf("invalid withdrawal")
	}
	value, err := s.store.Withdraw(ctx, actor, id, input, hash(input))
	return redactRequest(value, false), err
}

func (s *Service) ConfirmPayment(ctx context.Context, actor Actor, id string, input ConfirmPaymentInput) (Request, error) {
	id, input.CommandID = normalizeID(id), normalizeID(input.CommandID)
	input.OccurredOn, input.Note, input.OverageReason = strings.TrimSpace(input.OccurredOn), strings.TrimSpace(input.Note), strings.TrimSpace(input.OverageReason)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.ActualAmountMinor < 1 || len([]rune(input.Note)) > 500 || len([]rune(input.OverageReason)) > 1000 {
		return Request{}, fmt.Errorf("invalid payment")
	}
	if parsed, err := time.Parse("2006-01-02", input.OccurredOn); err != nil || parsed.Format("2006-01-02") != input.OccurredOn {
		return Request{}, fmt.Errorf("invalid payment date")
	}
	value, delivery, err := s.store.PreparePayment(ctx, actor, id, input, hash(input))
	if err != nil {
		return Request{}, err
	}
	if delivery.Status == "succeeded" {
		return redactRequest(value, false), nil
	}
	if delivery.Status == "failed" {
		return Request{}, ErrPaymentRejected
	}
	if s.payments == nil {
		_ = s.store.MarkPaymentUnknown(ctx, actor, id, input.CommandID)
		return Request{}, ErrDependency
	}
	result, err := s.payments.Record(ctx, MoneyPaymentRequest{
		SchemaVersion: "1", CommandID: input.CommandID, TenantID: actor.TenantID, ApplicationID: value.ID,
		ApplicationRevision: delivery.SourceRequestRevision, ApplicationDigest: requestDigest(value), RequestType: value.RequestType,
		Title: value.Title, TotalPayableMinor: value.AmountMinor, ActualAmountMinor: input.ActualAmountMinor,
		OccurredOn: input.OccurredOn, Note: input.Note, Payee: value.Details.Payee,
	})
	if err != nil {
		if errors.Is(err, ErrPaymentRejected) {
			if markErr := s.store.MarkPaymentFailed(ctx, actor, id, input.CommandID, "MONEY_REJECTED"); markErr != nil {
				return Request{}, markErr
			}
			return Request{}, ErrPaymentRejected
		}
		_ = s.store.MarkPaymentUnknown(ctx, actor, id, input.CommandID)
		return Request{}, fmt.Errorf("%w: %v", ErrPaymentUnknown, err)
	}
	value, err = s.store.CompletePayment(ctx, actor, id, input.CommandID, result)
	return redactRequest(value, false), err
}

func (s *Service) ConfirmPayments(ctx context.Context, actor Actor, input BatchPaymentInput) BatchPaymentResult {
	result := BatchPaymentResult{Items: make([]BatchPaymentItemResult, 0, len(input.Items))}
	seen := map[string]bool{}
	for _, item := range input.Items {
		entry := BatchPaymentItemResult{RequestID: normalizeID(item.RequestID)}
		if seen[item.CommandID] {
			entry.ErrorCode, entry.Detail = "DUPLICATE_COMMAND", "批次内 Command ID 重复"
		} else {
			seen[item.CommandID] = true
			value, err := s.ConfirmPayment(ctx, actor, entry.RequestID, item.ConfirmPaymentInput)
			if err != nil {
				entry.ErrorCode, entry.Detail = paymentErrorCode(err), err.Error()
			} else {
				entry.Request = &value
			}
		}
		result.Items = append(result.Items, entry)
	}
	return result
}

func (s *Service) Get(ctx context.Context, tenant, id string) (Request, error) {
	if !uuidPattern.MatchString(tenant) || !uuidPattern.MatchString(id) {
		return Request{}, fmt.Errorf("invalid identity")
	}
	value, err := s.store.Get(ctx, tenant, id)
	return redactRequest(value, false), err
}
func (s *Service) GetSensitive(ctx context.Context, actor Actor, id, purpose string) (Request, error) {
	id, purpose = normalizeID(id), strings.TrimSpace(purpose)
	if err := validateActor(actor); err != nil {
		return Request{}, err
	}
	if !uuidPattern.MatchString(id) || purpose == "" || len([]rune(purpose)) > 200 {
		return Request{}, fmt.Errorf("invalid identity")
	}
	value, err := s.store.Get(ctx, actor.TenantID, id)
	result := "succeeded"
	if errors.Is(err, ErrNotFound) {
		result = "not_found"
	} else if err != nil {
		result = "failed"
	}
	if auditErr := s.store.RecordSensitiveAccess(ctx, actor, id, purpose, result); auditErr != nil {
		return Request{}, auditErr
	}
	return value, err
}
func (s *Service) List(ctx context.Context, tenant string, filter ListFilter) (Page, error) {
	if !uuidPattern.MatchString(tenant) {
		return Page{}, fmt.Errorf("invalid tenant")
	}
	if filter.ApplicantSubjectID != "" && !uuidPattern.MatchString(filter.ApplicantSubjectID) {
		return Page{}, fmt.Errorf("invalid applicant")
	}
	items, err := s.store.List(ctx, tenant, filter)
	for i := range items {
		items[i] = redactRequest(items[i], false)
	}
	return Page{Items: items}, err
}

func (s *Service) ListCategories(ctx context.Context, tenant string) (CategoryPage, error) {
	if !uuidPattern.MatchString(tenant) {
		return CategoryPage{}, fmt.Errorf("invalid tenant")
	}
	stored, err := s.store.ListCategories(ctx, tenant)
	if err != nil {
		return CategoryPage{}, err
	}
	overrides := make(map[string]ExpenseCategory, len(stored))
	for _, category := range stored {
		overrides[category.ID] = category
	}
	items := make([]ExpenseCategory, 0, len(platformCategories)+len(stored))
	for _, category := range platformCategories {
		if override, ok := overrides[category.ID]; ok {
			items = append(items, override)
			delete(overrides, category.ID)
		} else {
			items = append(items, category)
		}
	}
	for _, category := range stored {
		if _, ok := overrides[category.ID]; ok {
			items = append(items, category)
		}
	}
	return CategoryPage{Items: items}, nil
}

func (s *Service) CreateCategory(ctx context.Context, actor Actor, input CategoryCreateInput) (ExpenseCategory, error) {
	input.CommandID, input.Key = normalizeID(input.CommandID), strings.ToLower(strings.TrimSpace(input.Key))
	input.DisplayName, input.Reason = strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.Reason)
	if err := validateActor(actor); err != nil {
		return ExpenseCategory{}, err
	}
	if !uuidPattern.MatchString(input.CommandID) || !regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`).MatchString(input.Key) || input.DisplayName == "" || len([]rune(input.DisplayName)) > 80 || input.Reason == "" || len([]rune(input.Reason)) > 200 {
		return ExpenseCategory{}, fmt.Errorf("invalid expense category")
	}
	for _, category := range platformCategories {
		if category.Key == input.Key {
			return ExpenseCategory{}, fmt.Errorf("invalid expense category: platform key is reserved")
		}
	}
	return s.store.CreateCategory(ctx, actor, newUUID(), input, hash(input))
}

func (s *Service) UpdateCategory(ctx context.Context, actor Actor, id string, input CategoryUpdateInput) (ExpenseCategory, error) {
	id, input.CommandID = normalizeID(id), normalizeID(input.CommandID)
	input.DisplayName, input.Reason = strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.Reason)
	if err := validateActor(actor); err != nil {
		return ExpenseCategory{}, err
	}
	if !uuidPattern.MatchString(id) || !uuidPattern.MatchString(input.CommandID) || input.ExpectedRevision < 1 || input.DisplayName == "" || len([]rune(input.DisplayName)) > 80 || input.Reason == "" || len([]rune(input.Reason)) > 200 {
		return ExpenseCategory{}, fmt.Errorf("invalid expense category update")
	}
	for _, category := range platformCategories {
		if category.ID == id {
			if input.DisplayName != category.DisplayName {
				return ExpenseCategory{}, fmt.Errorf("invalid expense category: platform name cannot be changed")
			}
			return s.store.UpdateCategory(ctx, actor, category, input, hash(input))
		}
	}
	return s.store.UpdateCategory(ctx, actor, ExpenseCategory{ID: id, Source: "tenant"}, input, hash(input))
}

func normalizeCreate(input *CreateInput) {
	input.CommandID, input.RequestType, input.Title = normalizeID(input.CommandID), strings.ToLower(strings.TrimSpace(input.RequestType)), strings.TrimSpace(input.Title)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	if input.Currency == "" {
		input.Currency = "CNY"
	}
}
func validateCreate(input CreateInput) error {
	if !uuidPattern.MatchString(input.CommandID) || (input.RequestType != "reimbursement" && input.RequestType != "payment") || input.Title == "" || len([]rune(input.Title)) > 200 || input.AmountMinor < 1 || input.Currency != "CNY" {
		return fmt.Errorf("invalid finance request")
	}
	return nil
}
func validateActor(actor Actor) error {
	if !uuidPattern.MatchString(actor.ID) || !uuidPattern.MatchString(actor.TenantID) || actor.CredentialID == "" {
		return fmt.Errorf("invalid actor")
	}
	return nil
}
func normalizeDetails(requestType string, d *DetailSnapshot, submitting bool) error {
	d.Applicant.ID, d.Applicant.Digest, d.Applicant.DisplayName = normalizeID(d.Applicant.ID), strings.ToLower(strings.TrimSpace(d.Applicant.Digest)), strings.TrimSpace(d.Applicant.DisplayName)
	d.Payee.Kind, d.Payee.EntityKind, d.Payee.Reference.ID, d.Payee.Reference.Digest = strings.ToLower(strings.TrimSpace(d.Payee.Kind)), strings.ToLower(strings.TrimSpace(d.Payee.EntityKind)), normalizeID(d.Payee.Reference.ID), strings.ToLower(strings.TrimSpace(d.Payee.Reference.Digest))
	d.Payee.DisplayName, d.Payee.BankName, d.Payee.AccountNumber, d.Payee.DelegateReason = strings.TrimSpace(d.Payee.DisplayName), strings.TrimSpace(d.Payee.BankName), strings.TrimSpace(d.Payee.AccountNumber), strings.TrimSpace(d.Payee.DelegateReason)
	if d.Payee.AccountNumber != "" {
		d.Payee.AccountLast4 = last4(d.Payee.AccountNumber)
	}
	if submitting && (!validReference(d.Applicant) || d.Applicant.DisplayName == "") {
		return fmt.Errorf("invalid applicant snapshot")
	}
	if len(d.Lines) == 0 {
		return fmt.Errorf("invalid lines: at least one line is required")
	}
	lineIDs := map[string]bool{}
	for i := range d.Lines {
		line := &d.Lines[i]
		line.ID, line.BusinessDate, line.CategoryKey, line.CategoryName, line.Purpose = normalizeID(line.ID), strings.TrimSpace(line.BusinessDate), strings.TrimSpace(line.CategoryKey), strings.TrimSpace(line.CategoryName), strings.TrimSpace(line.Purpose)
		if !uuidPattern.MatchString(line.ID) || lineIDs[line.ID] || line.AmountMinor < 1 || line.Purpose == "" || len([]rune(line.Purpose)) > 500 {
			return fmt.Errorf("invalid line")
		}
		lineIDs[line.ID] = true
		if requestType == "reimbursement" {
			parsed, err := time.Parse("2006-01-02", line.BusinessDate)
			if err != nil || parsed.Format("2006-01-02") != line.BusinessDate || line.CategoryKey == "" || line.CategoryName == "" || line.CategoryRev < 1 {
				return fmt.Errorf("invalid reimbursement line")
			}
		}
		if err := normalizeEvidence(line.Evidence); err != nil {
			return err
		}
	}
	if err := normalizeEvidence(d.Evidence); err != nil {
		return err
	}
	if err := normalizeAttribution(&d.Attribution); err != nil {
		return err
	}
	if requestType == "reimbursement" {
		if d.Payee.Kind != "person" || !validReference(d.Payee.Reference) || d.Payee.DisplayName == "" {
			return fmt.Errorf("invalid reimbursement payee")
		}
		if d.Payee.Reference.ID != d.Applicant.ID && d.Payee.DelegateReason == "" {
			return fmt.Errorf("invalid reimbursement delegate reason")
		}
		if submitting && !hasEvidence(*d) {
			return fmt.Errorf("invalid reimbursement: Evidence is required")
		}
	} else if requestType == "payment" {
		if d.Payee.Kind != "party" && d.Payee.Kind != "one_time" {
			return fmt.Errorf("invalid payment payee kind")
		}
		if d.Payee.Kind == "party" && !validReference(d.Payee.Reference) {
			return fmt.Errorf("invalid payment Party reference")
		}
		if d.Payee.Kind == "one_time" && d.Payee.EntityKind != "organization" && d.Payee.EntityKind != "individual" {
			return fmt.Errorf("invalid one-time payee entity kind")
		}
		if d.Payee.DisplayName == "" || d.Payee.BankName == "" || d.Payee.AccountNumber == "" {
			return fmt.Errorf("invalid payment payee snapshot")
		}
	}
	return nil
}

func normalizeAttribution(value *AttributionSnapshot) error {
	for _, reference := range []*ReferenceSnapshot{value.Department, value.Project, value.Contract} {
		if reference == nil {
			continue
		}
		reference.ID = normalizeID(reference.ID)
		reference.Digest = strings.ToLower(strings.TrimSpace(reference.Digest))
		reference.DisplayName = strings.TrimSpace(reference.DisplayName)
		if !validReference(*reference) || reference.DisplayName == "" || len([]rune(reference.DisplayName)) > 200 {
			return fmt.Errorf("invalid attribution reference")
		}
	}
	return nil
}
func normalizeEvidence(refs []EvidenceRef) error {
	for i := range refs {
		refs[i].FileID, refs[i].Purpose = normalizeID(refs[i].FileID), strings.TrimSpace(refs[i].Purpose)
		if !uuidPattern.MatchString(refs[i].FileID) || refs[i].Version < 1 || refs[i].Purpose == "" || len([]rune(refs[i].Purpose)) > 200 {
			return fmt.Errorf("invalid Evidence reference")
		}
	}
	return nil
}
func validReference(v ReferenceSnapshot) bool {
	return uuidPattern.MatchString(v.ID) && v.Revision >= 1 && digestPattern.MatchString(v.Digest)
}
func hasEvidence(d DetailSnapshot) bool {
	if len(d.Evidence) > 0 {
		return true
	}
	for _, line := range d.Lines {
		if len(line.Evidence) > 0 {
			return true
		}
	}
	return false
}
func totalLines(lines []Line) int64 {
	var total int64
	for _, line := range lines {
		total += line.AmountMinor
	}
	return total
}
func requestDigest(v Request) string {
	v.Payments = nil
	v.AllowedActions = nil
	v.PaymentUnknownCount = 0
	raw, _ := json.Marshal(v)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func hash(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func normalizeID(v string) string { return strings.ToLower(strings.TrimSpace(v)) }
func last4(v string) string {
	r := []rune(v)
	if len(r) <= 4 {
		return string(r)
	}
	return string(r[len(r)-4:])
}
func redactRequest(v Request, sensitive bool) Request {
	if !sensitive {
		v.Details.Payee.AccountNumber = ""
	}
	return v
}
func paymentErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrPaymentUnknown):
		return "PAYMENT_UNKNOWN"
	case errors.Is(err, ErrPaymentRejected):
		return "PAYMENT_REJECTED"
	case errors.Is(err, ErrRevisionConflict):
		return "REVISION_CONFLICT"
	case errors.Is(err, ErrStateConflict):
		return "STATE_CONFLICT"
	case errors.Is(err, ErrCommandConflict):
		return "COMMAND_CONFLICT"
	default:
		return "PAYMENT_FAILED"
	}
}
