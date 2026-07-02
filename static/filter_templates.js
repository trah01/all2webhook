const filterTemplates = [
    {
        name: '屏蔽营销邮件',
        type: 'content',
        mode: 'blacklist',
        patterns: ['unsubscribe', '退订', 'newsletter', 'promotion', '促销', '广告']
    },
    {
        name: '只转发验证码',
        type: 'content',
        mode: 'whitelist',
        patterns: ['验证码', '校验码', '动态码', 'verification code', 'security code', 'auth code']
    },
    {
        name: '屏蔽自动发信人',
        type: 'sender',
        mode: 'blacklist',
        patterns: ['no-reply@', 'noreply@', 'notification@', 'newsletter@']
    },
    {
        name: '只允许指定域名',
        type: 'sender',
        mode: 'whitelist',
        patterns: ['@example.com']
    },
    {
        name: '屏蔽系统噪音',
        type: 'content',
        mode: 'blacklist',
        patterns: ['cron', 'debug', 'heartbeat', 'health check', '监控恢复', '测试通知']
    },
    {
        name: '只转发账单发票',
        type: 'content',
        mode: 'whitelist',
        patterns: ['账单', '发票', 'invoice', 'receipt', 'payment']
    }
];
