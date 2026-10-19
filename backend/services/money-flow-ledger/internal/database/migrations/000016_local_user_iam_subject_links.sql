CREATE TABLE local_user_iam_subject_links (
    local_user_id uuid PRIMARY KEY REFERENCES local_users(id) ON DELETE RESTRICT,
    tenant_id uuid NOT NULL,
    iam_subject_id uuid NOT NULL,
    linked_by_iam_subject_id uuid NOT NULL,
    linked_by_display_name text NOT NULL CHECK (
        linked_by_display_name = btrim(linked_by_display_name)
        AND char_length(linked_by_display_name) BETWEEN 1 AND 100
    ),
    reason text NOT NULL CHECK (
        reason = btrim(reason)
        AND char_length(reason) BETWEEN 1 AND 500
    ),
    linked_at timestamptz NOT NULL,
    CONSTRAINT money_local_user_iam_subject_unique UNIQUE (tenant_id, iam_subject_id)
);

CREATE TRIGGER local_user_iam_subject_links_immutable
BEFORE UPDATE OR DELETE ON local_user_iam_subject_links
FOR EACH ROW EXECUTE FUNCTION reject_immutable_money_flow_fact_change();
