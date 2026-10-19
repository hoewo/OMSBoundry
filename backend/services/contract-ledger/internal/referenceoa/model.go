// Package referenceoa is a versioned, typed customer overlay owned by Contract.
// Its editable intentions are not confirmed Contract instruments/settlements.
package referenceoa

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"

	"omsbizfoundry/contract-ledger/internal/audit"
)

var (
	ErrInvalid  = errors.New("reference OA draft input invalid")
	ErrConflict = errors.New("reference OA draft version or command conflict")
	ErrScope    = errors.New("reference OA draft context mismatch")
	ErrState    = errors.New("reference OA draft state does not allow this operation")
	uuid        = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	decimal     = regexp.MustCompile(`^-?[0-9]{1,18}(\.[0-9]{1,2})?$`)
)

type Fees struct {
	Design       string `json:"design_fee"`
	Survey       string `json:"survey_fee"`
	Consultation string `json:"consultation_fee"`
}
type Amendment struct {
	ID          string `json:"id"`
	Number      string `json:"amendment_number"`
	SignedOn    string `json:"signed_on"`
	ChangeType  string `json:"change_type"`
	Fees        Fees   `json:"fees"`
	RateBPS     *int64 `json:"rate_bps"`
	Description string `json:"description"`
	Cancelled   bool   `json:"cancelled"`
}
type Draft struct {
	SchemaVersion     string      `json:"schema_version"`
	ContractID        string      `json:"contract_id"`
	TenantID          string      `json:"tenant_id"`
	ProjectID         string      `json:"project_id"`
	ChainType         string      `json:"chain_type"`
	Revision          int64       `json:"revision"`
	Fees              Fees        `json:"fees"`
	RateBPS           *int64      `json:"rate_bps"`
	SettlementFollows bool        `json:"settlement_follows_contract_amount"`
	SettlementAmount  *string     `json:"settlement_amount"`
	Amendments        []Amendment `json:"amendments"`
}
type SaveInput struct {
	CommandID                string         `json:"command_id"`
	ExpectedRevision         int64          `json:"expected_revision"`
	ExpectedDraftVersion     int64          `json:"expected_draft_version"`
	ExpectedContractRevision int64          `json:"expected_contract_revision,omitempty"`
	ProjectID                string         `json:"project_id"`
	ChainType                string         `json:"chain_type"`
	Fees                     Fees           `json:"fees"`
	RateBPS                  *int64         `json:"rate_bps"`
	SettlementFollows        bool           `json:"settlement_follows_contract_amount"`
	SettlementAmount         *string        `json:"settlement_amount"`
	Amendments               []Amendment    `json:"amendments"`
	ContractID               string         `json:"-"`
	TenantID                 string         `json:"-"`
	Actor                    audit.ActorRef `json:"-"`
}
type Result struct {
	Draft    Draft `json:"draft"`
	Replayed bool  `json:"replayed"`
}

// MinorUnits never converts through float64. Overlay money is CNY cents.
func MinorUnits(raw string, signed bool) (*big.Int, error) {
	if !decimal.MatchString(raw) || (!signed && strings.HasPrefix(raw, "-")) {
		return nil, ErrInvalid
	}
	negative := strings.HasPrefix(raw, "-")
	parts := strings.Split(strings.TrimPrefix(raw, "-"), ".")
	fraction := "00"
	if len(parts) == 2 {
		fraction = parts[1] + strings.Repeat("0", 2-len(parts[1]))
	}
	n, ok := new(big.Int).SetString(parts[0]+fraction, 10)
	if !ok {
		return nil, ErrInvalid
	}
	if negative {
		n.Neg(n)
	}
	return n, nil
}
func Amount(n *big.Int) string {
	abs := new(big.Int).Abs(n)
	whole, fraction := new(big.Int), new(big.Int)
	whole.QuoRem(abs, big.NewInt(100), fraction)
	f := fraction.String()
	if len(f) == 1 {
		f = "0" + f
	}
	sign := ""
	if n.Sign() < 0 {
		sign = "-"
	}
	return sign + whole.String() + "." + f
}
func NormalizeFees(value Fees, signed bool) (Fees, string, error) {
	fields := []*string{&value.Design, &value.Survey, &value.Consultation}
	total := new(big.Int)
	for _, field := range fields {
		n, err := MinorUnits(*field, signed)
		if err != nil {
			return Fees{}, "", err
		}
		*field = Amount(n)
		total.Add(total, n)
	}
	if len(new(big.Int).Abs(total).String()) > 20 {
		return Fees{}, "", ErrInvalid
	}
	return value, Amount(total), nil
}
func normalize(in SaveInput) (SaveInput, string, error) {
	for _, v := range []string{in.CommandID, in.ContractID, in.TenantID, in.ProjectID} {
		if !uuid.MatchString(v) {
			return in, "", ErrInvalid
		}
	}
	if in.ExpectedRevision < 0 || in.ExpectedContractRevision < 0 || in.ExpectedDraftVersion < 1 || (in.ChainType != "client_to_qualification" && in.ChainType != "qualification_to_our_company") {
		return in, "", ErrInvalid
	}
	actor, err := audit.NormalizeActor(in.Actor)
	if err != nil || actor.Kind != audit.ActorKindIAMSubject {
		return in, "", ErrInvalid
	}
	in.Actor = actor
	var total string
	in.Fees, total, err = NormalizeFees(in.Fees, false)
	if err != nil {
		return in, "", err
	}
	validRate := func(v *int64) bool { return v == nil || (*v >= 0 && *v <= 10000) }
	if !validRate(in.RateBPS) || len(in.Amendments) > 100 {
		return in, "", ErrInvalid
	}
	// SaveInput is caller-owned. Normalization and the resulting receipt must
	// not share mutable slice elements or nullable rates with that caller.
	cloneRate := func(value *int64) *int64 {
		if value == nil {
			return nil
		}
		copy := *value
		return &copy
	}
	in.RateBPS = cloneRate(in.RateBPS)
	in.Amendments = append([]Amendment(nil), in.Amendments...)
	if in.SettlementFollows {
		if in.SettlementAmount != nil {
			return in, "", ErrInvalid
		}
	} else if in.SettlementAmount != nil {
		n, e := MinorUnits(*in.SettlementAmount, false)
		if e != nil {
			return in, "", e
		}
		s := Amount(n)
		in.SettlementAmount = &s
	}
	ids, numbers := map[string]bool{}, map[string]bool{}
	for i := range in.Amendments {
		a := &in.Amendments[i]
		a.RateBPS = cloneRate(a.RateBPS)
		a.Number = strings.TrimSpace(a.Number)
		a.Description = strings.TrimSpace(a.Description)
		date, e := time.Parse("2006-01-02", a.SignedOn)
		if !uuid.MatchString(a.ID) || ids[a.ID] || a.Number == "" || len([]rune(a.Number)) > 100 || e != nil || date.Format("2006-01-02") != a.SignedOn || !validRate(a.RateBPS) || len([]rune(a.Description)) > 2000 {
			return in, "", ErrInvalid
		}
		ids[a.ID] = true
		if !a.Cancelled {
			key := strings.ToLower(a.Number)
			if numbers[key] {
				return in, "", ErrInvalid
			}
			numbers[key] = true
		}
		if a.ChangeType != "adjustment" && a.ChangeType != "revision" && a.ChangeType != "no_amount_change" {
			return in, "", ErrInvalid
		}
		var sum string
		a.Fees, sum, err = NormalizeFees(a.Fees, a.ChangeType == "adjustment")
		if err != nil {
			return in, "", err
		}
		if a.ChangeType == "no_amount_change" && sum != "0.00" {
			return in, "", ErrInvalid
		}
	}
	if in.Amendments == nil {
		in.Amendments = []Amendment{}
	}
	if _, err = PreviewFees(in.Fees, in.Amendments); err != nil {
		return in, "", err
	}
	return in, total, nil
}
func PreviewFees(base Fees, amendments []Amendment) (Fees, error) {
	values := []string{base.Design, base.Survey, base.Consultation}
	amounts := make([]*big.Int, 3)
	for i, v := range values {
		n, e := MinorUnits(v, false)
		if e != nil {
			return Fees{}, e
		}
		amounts[i] = n
	}
	ordered := append([]Amendment{}, amendments...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].SignedOn != ordered[j].SignedOn {
			return ordered[i].SignedOn < ordered[j].SignedOn
		}
		if ordered[i].Number != ordered[j].Number {
			return ordered[i].Number < ordered[j].Number
		}
		return ordered[i].ID < ordered[j].ID
	})
	for _, a := range ordered {
		if a.Cancelled || a.ChangeType == "no_amount_change" {
			continue
		}
		for i, v := range []string{a.Fees.Design, a.Fees.Survey, a.Fees.Consultation} {
			n, e := MinorUnits(v, a.ChangeType == "adjustment")
			if e != nil {
				return Fees{}, e
			}
			if a.ChangeType == "revision" {
				amounts[i] = n
			} else {
				amounts[i].Add(amounts[i], n)
			}
			if amounts[i].Sign() < 0 {
				return Fees{}, ErrInvalid
			}
		}
	}
	value, _, err := NormalizeFees(Fees{Amount(amounts[0]), Amount(amounts[1]), Amount(amounts[2])}, false)
	return value, err
}
func intentHash(in SaveInput) string {
	// Actor identity belongs to replay intent; display label does not grant rights.
	b, _ := json.Marshal(struct {
		Input                   SaveInput
		Tenant, Contract, Actor string
	}{in, in.TenantID, in.ContractID, in.Actor.ID})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// orderedAmendments fixes the original calendar/number/ID order. It is shared
// by previews and the explicit confirmation gate, never locale-dependent.
func orderedAmendments(values []Amendment) []Amendment {
	result := append([]Amendment{}, values...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].SignedOn != result[j].SignedOn {
			return result[i].SignedOn < result[j].SignedOn
		}
		if result[i].Number != result[j].Number {
			return result[i].Number < result[j].Number
		}
		return result[i].ID < result[j].ID
	})
	return result
}
