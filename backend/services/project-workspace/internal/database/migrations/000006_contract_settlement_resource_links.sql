ALTER TABLE project_resource_links
    DROP CONSTRAINT project_resource_links_resource_type_check,
    ADD CONSTRAINT project_resource_links_resource_type_check CHECK (
        resource_type IN (
            'contract', 'settlement', 'obligation', 'transaction', 'evidence-file',
            'design-work-item', 'invoice_registration'
        )
    );
