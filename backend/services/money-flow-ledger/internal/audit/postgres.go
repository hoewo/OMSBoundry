package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

func Insert(ctx context.Context, tx pgx.Tx, entry Entry) error {
	details, err := json.Marshal(entry.Details)
	if err != nil {
		return fmt.Errorf("encode audit details: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_entries (
			id, object_type, object_id, action, command_id,
			actor_kind, actor_id, actor_display_name,
			occurred_at, summary, details
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, '')::uuid,
			$6, $7, $8, $9, $10, $11::jsonb
		)
	`,
		entry.ID,
		entry.ObjectType,
		entry.ObjectID,
		entry.Action,
		entry.CommandID,
		entry.Actor.Kind,
		entry.Actor.ID,
		entry.Actor.DisplayName,
		entry.OccurredAt,
		entry.Summary,
		details,
	)
	if err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}
