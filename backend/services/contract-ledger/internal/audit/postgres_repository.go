package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, limit int) ([]Entry, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, object_type, object_id::text, action,
			COALESCE(command_id::text, ''), actor_kind, actor_id,
			actor_display_name, occurred_at, summary, details
		FROM audit_entries
		ORDER BY occurred_at DESC, id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query audit entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

func ListForObject(ctx context.Context, tx pgx.Tx, objectType, objectID string) ([]Entry, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text, object_type, object_id::text, action,
			COALESCE(command_id::text, ''), actor_kind, actor_id,
			actor_display_name, occurred_at, summary, details
		FROM audit_entries
		WHERE object_type = $1 AND object_id = $2
		ORDER BY occurred_at, id
	`, objectType, objectID)
	if err != nil {
		return nil, fmt.Errorf("query object audit entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

type rowsScanner interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanEntries(rows rowsScanner) ([]Entry, error) {
	entries := make([]Entry, 0)
	for rows.Next() {
		var entry Entry
		var details []byte
		if err := rows.Scan(
			&entry.ID,
			&entry.ObjectType,
			&entry.ObjectID,
			&entry.Action,
			&entry.CommandID,
			&entry.Actor.Kind,
			&entry.Actor.ID,
			&entry.Actor.DisplayName,
			&entry.OccurredAt,
			&entry.Summary,
			&details,
		); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		if err := json.Unmarshal(details, &entry.Details); err != nil {
			return nil, fmt.Errorf("decode audit details: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit entries: %w", err)
	}
	return entries, nil
}
