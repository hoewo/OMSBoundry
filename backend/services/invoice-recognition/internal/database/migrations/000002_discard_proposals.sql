ALTER TABLE invoice_recognition_proposals
    DROP CONSTRAINT invoice_recognition_proposals_status_check,
    ADD CONSTRAINT invoice_recognition_proposals_status_check
        CHECK (status IN ('needs_review','confirmed','discarded'));

ALTER TABLE invoice_recognition_proposal_revisions
    DROP CONSTRAINT invoice_recognition_proposal_revisions_event_kind_check,
    ADD CONSTRAINT invoice_recognition_proposal_revisions_event_kind_check
        CHECK (event_kind IN ('recognized','confirmed','discarded'));

ALTER TABLE invoice_recognition_commands
    DROP CONSTRAINT invoice_recognition_commands_response_kind_check,
    ADD CONSTRAINT invoice_recognition_commands_response_kind_check
        CHECK (response_kind IN ('proposal','confirmation','discard'));

ALTER TABLE invoice_recognition_audit_events
    DROP CONSTRAINT invoice_recognition_audit_events_action_check,
    ADD CONSTRAINT invoice_recognition_audit_events_action_check
        CHECK (action IN ('invoice_recognized','invoice_recognition_confirmed','invoice_recognition_discarded'));
