ALTER TABLE file_versions
    ADD COLUMN validation_profile text NOT NULL DEFAULT 'general' CHECK (validation_profile IN ('general','invoice-v1')),
    ADD COLUMN security_status text NOT NULL DEFAULT 'not_required' CHECK (security_status IN ('not_required','passed'));
