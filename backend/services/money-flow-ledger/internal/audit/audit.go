package audit

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ActorKindLocalUser       = "local_user"
	ActorKindIAMSubject      = "iam_subject"
	ObjectLocalUser          = "local_user"
	ObjectParty              = "party"
	ObjectObligation         = "obligation"
	ObjectCashTransaction    = "cash_transaction"
	ObjectCashReversal       = "cash_transaction_reversal"
	ObjectAllocation         = "allocation"
	ObjectAllocationReversal = "allocation_reversal"
	ObjectInternalTransfer   = "internal_transfer"
	ObjectLedgerExport       = "ledger_export"
	ObjectReceiptSet         = "receipt_set"
)

var (
	actorKindPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
	uuidPattern      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

type ActorRef struct {
	Kind        string
	ID          string
	DisplayName string
}

func (a ActorRef) String() string {
	if a.DisplayName == "" || a.DisplayName == a.ID {
		return a.Kind + ":" + a.ID
	}
	return fmt.Sprintf("%s（%s:%s）", a.DisplayName, a.Kind, a.ID)
}

func NormalizeActor(value ActorRef) (ActorRef, error) {
	kind := strings.ToLower(strings.TrimSpace(value.Kind))
	if !actorKindPattern.MatchString(kind) {
		return ActorRef{}, fmt.Errorf("操作者类型无效")
	}
	id := strings.TrimSpace(value.ID)
	if id == "" || len([]rune(id)) > 100 {
		return ActorRef{}, fmt.Errorf("操作者标识无效")
	}
	if (kind == ActorKindLocalUser || kind == ActorKindIAMSubject) && !uuidPattern.MatchString(id) {
		return ActorRef{}, fmt.Errorf("本地用户操作者标识无效")
	}
	displayName := strings.TrimSpace(value.DisplayName)
	if displayName == "" {
		displayName = id
	}
	if len([]rune(displayName)) > 100 {
		return ActorRef{}, fmt.Errorf("操作者显示名称无效")
	}
	return ActorRef{Kind: kind, ID: strings.ToLower(id), DisplayName: displayName}, nil
}

type Entry struct {
	ID         string
	ObjectType string
	ObjectID   string
	Action     string
	CommandID  string
	Actor      ActorRef
	OccurredAt time.Time
	Summary    string
	Details    map[string]any
}
