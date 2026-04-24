const fullTemplateConfig = {
    _comment: 'DS2API built-in template',
    _doc: 'https://github.com/CJackHwang/ds2api',
    keys: ['your-api-key-1', 'your-api-key-2'],
    api_keys: [
        {
            key: 'your-api-key-1',
            name: 'Primary API Key',
            remark: 'for OpenAI clients',
        },
        {
            key: 'your-api-key-2',
            name: 'Backup API Key',
            remark: 'for stress test/debug',
        },
    ],
    accounts: [
        {
            _comment: 'email login',
            name: 'Primary account',
            remark: 'preferred for production',
            email: 'example1@example.com',
            password: 'your-password-1',
        },
        {
            _comment: 'email login - account 2',
            name: 'Backup account',
            email: 'example2@example.com',
            password: 'your-password-2',
        },
        {
            _comment: 'mobile login',
            mobile: '12345678901',
            password: 'your-password-3',
        },
    ],
    model_aliases: {
        'gpt-4o': 'deepseek-chat',
        'gpt-5-codex': 'deepseek-reasoner',
        o3: 'deepseek-reasoner',
    },
    compat: {
        wide_input_strict_output: true,
        strip_reference_markers: true,
    },
    responses: {
        store_ttl_seconds: 900,
    },
    history_split: {
        enabled: true,
        trigger_after_turns: 1,
    },
    embeddings: {
        provider: 'deterministic',
    },
    claude_mapping: {
        fast: 'deepseek-chat',
        slow: 'deepseek-reasoner',
    },
    admin: {
        jwt_expire_hours: 24,
    },
    runtime: {
        account_max_inflight: 2,
        account_max_queue: 0,
        global_max_inflight: 0,
        token_refresh_interval_hours: 6,
    },
    auto_delete: {
        mode: 'none',
    },
}

export function getBatchImportTemplates(t) {
    return {
        full: {
            name: t('batchImport.templates.full.name'),
            desc: t('batchImport.templates.full.desc'),
            config: fullTemplateConfig,
        },
        email_only: {
            name: t('batchImport.templates.emailOnly.name'),
            desc: t('batchImport.templates.emailOnly.desc'),
            config: {
                keys: ['your-api-key'],
                accounts: [
                    { email: 'account1@example.com', password: 'pass1', token: '' },
                    { email: 'account2@example.com', password: 'pass2', token: '' },
                    { email: 'account3@example.com', password: 'pass3', token: '' },
                ],
            },
        },
        mobile_only: {
            name: t('batchImport.templates.mobileOnly.name'),
            desc: t('batchImport.templates.mobileOnly.desc'),
            config: {
                keys: ['your-api-key'],
                accounts: [
                    { mobile: '+8613800000001', password: 'pass1', token: '' },
                    { mobile: '+8613800000002', password: 'pass2', token: '' },
                    { mobile: '+8613800000003', password: 'pass3', token: '' },
                ],
            },
        },
        keys_only: {
            name: t('batchImport.templates.keysOnly.name'),
            desc: t('batchImport.templates.keysOnly.desc'),
            config: {
                keys: ['key-1', 'key-2', 'key-3'],
            },
        },
    }
}
