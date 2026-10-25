ALTER TABLE persons
    ADD COLUMN iam_subject_status text NULL,
    ADD COLUMN iam_subject_revision bigint NULL;

-- Earlier releases allowed an optional UUID without consulting IAM. Preserve
-- that historical value, but mark it unusable until the normal IAM reference
-- command verifies and replaces the snapshot.
UPDATE persons
SET iam_subject_status = 'legacy_unverified', iam_subject_revision = 1
WHERE iam_subject_id IS NOT NULL;

ALTER TABLE persons
    ADD CONSTRAINT persons_iam_subject_snapshot_check CHECK (
        (iam_subject_id IS NULL AND iam_subject_status IS NULL AND iam_subject_revision IS NULL)
        OR
        (iam_subject_id IS NOT NULL AND iam_subject_status IN ('pending', 'active', 'disabled', 'legacy_unverified') AND iam_subject_revision >= 1)
    );

ALTER TABLE person_revisions
    ADD COLUMN iam_subject_status text NULL,
    ADD COLUMN iam_subject_revision bigint NULL;

-- The revision table is immutable during normal service operation. This
-- one-time schema upgrade must annotate legacy snapshots, so suspend only
-- its immutable trigger inside the migration transaction and restore it
-- immediately after the backfill.
ALTER TABLE person_revisions DISABLE TRIGGER person_revisions_immutable;

UPDATE person_revisions
SET iam_subject_status = 'legacy_unverified', iam_subject_revision = 1
WHERE iam_subject_id IS NOT NULL;

ALTER TABLE person_revisions ENABLE TRIGGER person_revisions_immutable;

ALTER TABLE person_revisions
    ADD CONSTRAINT person_revisions_iam_subject_snapshot_check CHECK (
        (iam_subject_id IS NULL AND iam_subject_status IS NULL AND iam_subject_revision IS NULL)
        OR
        (iam_subject_id IS NOT NULL AND iam_subject_status IN ('pending', 'active', 'disabled', 'legacy_unverified') AND iam_subject_revision >= 1)
    );
