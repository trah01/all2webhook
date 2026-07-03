const filterTemplates = [
    {
        name: '空白模板',
        type: 'content',
        mode: 'blacklist',
        patterns: []
    },
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
        name: '屏蔽自动发送人',
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
        name: '只转发 Webhook 通知',
        type: 'source',
        mode: 'whitelist',
        patterns: ['webhook']
    },
    {
        name: '屏蔽 Webhook 测试通知',
        type: 'all',
        mode: 'blacklist',
        patterns: ['test', '测试', 'debug', 'heartbeat']
    }
];
