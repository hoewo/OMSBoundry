-- Beijing 2026 reference scheme. This is a versioned platform reference,
-- not a tenant declaration of its actual work-injury tier or housing rate.
-- Tenants may use it directly or copy it into a custom scheme and confirm
-- the two variable rates before generating a payroll period.

INSERT INTO contribution_schemes (
    id,tenant_id,scheme_kind,source_scheme_id,source_scheme_revision,title,
    region_code,region_name,effective_month,authority_source,authority_verified_at,
    components,reason,revision,created_at,updated_at
) VALUES (
    '20260000-0004-4000-8000-000000000001',NULL,'standard',NULL,NULL,
    '北京 2026 五险一金参考方案（最低可选档）',
    'CN-BJ','北京市','2026-07',
    'https://rsj.beijing.gov.cn/xxgk/2024zcwj/202608/t20260821_4831461.html | https://www.beijing.gov.cn/fwcj/jiage/zffw1/685cd52595ff563574309b82.html | https://sw.beijing.gov.cn/zwxx/swxx/202605/P020260512520283489932576.pdf',
    '2026-10-26T00:00:00Z',
    '[
      {"key":"pension","base_kind":"social","individual_rate_bps":800,"employer_rate_bps":1600,"base_floor_minor":727000,"base_cap_minor":3634800,"rounding_mode":"half_up"},
      {"key":"medical","base_kind":"social","individual_rate_bps":200,"employer_rate_bps":980,"individual_fixed_minor":300,"base_floor_minor":727000,"base_cap_minor":3634800,"rounding_mode":"half_up"},
      {"key":"unemployment","base_kind":"social","individual_rate_bps":50,"employer_rate_bps":50,"base_floor_minor":727000,"base_cap_minor":3634800,"rounding_mode":"half_up"},
      {"key":"work_injury","base_kind":"social","individual_rate_bps":0,"employer_rate_bps":20,"base_floor_minor":727000,"base_cap_minor":3634800,"rounding_mode":"half_up"},
      {"key":"maternity","base_kind":"social","individual_rate_bps":0,"employer_rate_bps":0,"base_floor_minor":727000,"base_cap_minor":3634800,"rounding_mode":"half_up"},
      {"key":"housing_fund","base_kind":"housing","individual_rate_bps":500,"employer_rate_bps":500,"base_floor_minor":254000,"base_cap_minor":3634800,"rounding_mode":"yuan_half_up"}
    ]'::jsonb,
    '北京市官方公开口径参考；工伤按最低行业基准费率0.2%，住房公积金按最低可选比例5%，租户应确认实际适用档后复制调整',
    1,'2026-10-26T00:00:00Z','2026-10-26T00:00:00Z'
);

INSERT INTO contribution_scheme_revisions (
    scheme_id,tenant_id,revision,command_id,scheme_kind,source_scheme_id,
    source_scheme_revision,title,region_code,region_name,effective_month,
    authority_source,authority_verified_at,components,reason,actor_subject_id,
    credential_id,occurred_at
)
SELECT
    id,tenant_id,revision,'20260000-0004-4000-8000-000000000002',scheme_kind,
    source_scheme_id,source_scheme_revision,title,region_code,region_name,
    effective_month,authority_source,authority_verified_at,components,reason,
    '20260000-0004-4000-8000-000000000003','migration:000004',created_at
FROM contribution_schemes
WHERE id='20260000-0004-4000-8000-000000000001';

INSERT INTO payroll_profile_audit_events (
    id,tenant_id,resource_type,resource_id,person_id,command_id,source_revision,
    result_revision,action,actor_subject_id,credential_id,details,occurred_at
) VALUES (
    '20260000-0004-4000-8000-000000000004',NULL,'contribution_scheme',
    '20260000-0004-4000-8000-000000000001',NULL,
    '20260000-0004-4000-8000-000000000002',0,1,'standard_scheme_created',
    '20260000-0004-4000-8000-000000000003','migration:000004',
    '{"region_code":"CN-BJ","effective_month":"2026-07","reference_kind":"official_public_reference","variable_defaults":{"work_injury_employer_bps":20,"housing_fund_bps":500}}'::jsonb,
    '2026-10-26T00:00:00Z'
);
