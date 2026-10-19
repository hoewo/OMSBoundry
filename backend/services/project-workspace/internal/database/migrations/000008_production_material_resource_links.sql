ALTER TABLE project_resource_links
    DROP CONSTRAINT project_resource_links_target_service_check,
    DROP CONSTRAINT project_resource_links_resource_type_check,
    ADD CONSTRAINT project_resource_links_target_service_check CHECK (
        target_service IN (
            'contract-ledger', 'money-flow-ledger', 'evidence-service',
            'design-operations', 'billing-invoice', 'business-engagement',
            'production-operations'
        )
    ),
    ADD CONSTRAINT project_resource_links_resource_type_check CHECK (
        resource_type IN (
            'contract', 'settlement', 'obligation', 'transaction', 'evidence-file',
            'design-work-item', 'invoice_registration', 'business-engagement',
            'production-work-item'
        )
    );
