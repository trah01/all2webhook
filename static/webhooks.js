// ===================== Webhooks =====================
const webhookTypeNames = {
    feishu: '飞书',
    dingtalk: '钉钉',
    wecom: '企业微信',
    slack: 'Slack',
    discord: 'Discord',
    teams: 'Microsoft Teams',
    mattermost: 'Mattermost',
    gotify: 'Gotify',
    bark: 'Bark',
    serverchan: 'Server 酱',
    telegram: 'Telegram',
    ntfy: 'ntfy',
    pushplus: 'PushPlus',
    chanify: 'Chanify',
    pushover: 'Pushover',
    custom: '自定义 Webhook',
    email: '邮件收件人'
};

const webhookPayloadOptions = {
    feishu: [['interactive', '卡片 interactive'], ['text', '文本 text'], ['post', '富文本 post']],
    dingtalk: [['markdown', 'Markdown'], ['text', '文本 text'], ['actioncard', 'ActionCard'], ['feedcard', 'FeedCard']],
    wecom: [['markdown', 'Markdown'], ['text', '文本 text'], ['news', '图文 news']],
    slack: [['blocks', 'Blocks'], ['text', '文本 text'], ['attachments', 'Attachments']],
    discord: [['embeds', 'Embeds'], ['content', '文本 content']],
    teams: [['messagecard', 'MessageCard'], ['adaptivecard', 'Adaptive Card']],
    mattermost: [['markdown', 'Markdown'], ['text', '文本 text']],
    gotify: [['message', 'Message']],
    bark: [['push', 'Push']],
    serverchan: [['send', 'Send']],
    telegram: [['sendMessage', 'sendMessage']],
    ntfy: [['message', 'Message']],
    pushplus: [['markdown', 'Markdown']],
    chanify: [['text', 'Text']],
    pushover: [['message', 'Message']],
    custom: [['json', 'JSON']],
    email: [['email', 'Email']]
};

const webhookTemplateSamples = {
    feishu: '{\n  "msg_type": "interactive",\n  "card": {\n    "config": {"wide_screen_mode": true},\n    "header": {"title": {"tag": "plain_text", "content": "{{subject}}"}, "template": "blue"},\n    "elements": [\n      {"tag": "div", "text": {"tag": "lark_md", "content": "**发送人：** {{from}}\\n**时间：** {{date}}"}},\n      {"tag": "hr"},\n      {"tag": "div", "text": {"tag": "lark_md", "content": "{{body}}"}}\n    ]\n  }\n}',
    dingtalk: '{\n  "msgtype": "markdown",\n  "markdown": {\n    "title": "{{subject}}",\n    "text": "### {{subject}}\\n**发送人:** {{from}}\\n**时间:** {{date}}\\n\\n{{body}}"\n  },\n  "at": {"atMobiles": [], "isAtAll": false}\n}',
    wecom: '{\n  "msgtype": "markdown",\n  "markdown": {\n    "content": "### {{subject}}\\n**发送人:** {{from}}\\n**时间:** {{date}}\\n\\n{{body}}"\n  }\n}',
    slack: '{\n  "text": "{{subject}}",\n  "blocks": [\n    {"type": "header", "text": {"type": "plain_text", "text": "{{subject}}"}},\n    {"type": "section", "fields": [\n      {"type": "mrkdwn", "text": "*发送人:*\\n{{from}}"},\n      {"type": "mrkdwn", "text": "*时间:*\\n{{date}}"}\n    ]},\n    {"type": "section", "text": {"type": "mrkdwn", "text": "{{body}}"}}\n  ]\n}',
    discord: '{\n  "content": "",\n  "embeds": [{\n    "title": "{{subject}}",\n    "description": "{{body}}",\n    "fields": [\n      {"name": "发送人", "value": "{{from}}", "inline": true},\n      {"name": "时间", "value": "{{date}}", "inline": true}\n    ]\n  }],\n  "allowed_mentions": {"parse": []}\n}',
    teams: '{\n  "@type": "MessageCard",\n  "@context": "https://schema.org/extensions",\n  "summary": "{{subject}}",\n  "title": "{{subject}}",\n  "text": "**发送人:** {{from}}\\n\\n**时间:** {{date}}\\n\\n{{body}}",\n  "sections": [{\n    "activityTitle": "All2Webhook",\n    "facts": [\n      {"name": "发送人", "value": "{{from}}"},\n      {"name": "时间", "value": "{{date}}"}\n    ]\n  }]\n}',
    mattermost: '{\n  "username": "All2Webhook",\n  "text": "### {{subject}}\\n**发送人:** {{from}}\\n**时间:** {{date}}\\n\\n{{body}}"\n}',
    gotify: '{\n  "title": "{{subject}}",\n  "message": "{{body}}",\n  "priority": 5,\n  "extras": {"client::display": {"contentType": "text/markdown"}}\n}',
    bark: '{\n  "title": "{{subject}}",\n  "body": "{{body}}",\n  "group": "All2Webhook"\n}',
    serverchan: '{\n  "title": "{{subject}}",\n  "desp": "{{body}}"\n}',
    telegram: '{\n  "chat_id": "你的 Chat ID",\n  "text": "*{{subject}}*\\n\\n{{body}}",\n  "parse_mode": "Markdown",\n  "disable_web_page_preview": true\n}',
    ntfy: '{\n  "title": "{{subject}}",\n  "message": "{{body}}",\n  "priority": 3,\n  "markdown": true\n}',
    pushplus: '{\n  "token": "your_pushplus_token",\n  "title": "{{subject}}",\n  "content": "{{body}}",\n  "template": "markdown"\n}',
    chanify: '{\n  "title": "{{subject}}",\n  "text": "{{body}}"\n}',
    pushover: '{\n  "token": "your_app_token",\n  "user": "your_user_key",\n  "title": "{{subject}}",\n  "message": "{{body}}",\n  "priority": 0\n}',
    custom: '{\n  "subject": "{{subject}}",\n  "from": "{{from}}",\n  "date": "{{date}}",\n  "body": "{{body}}"\n}'
};

async function loadWebhooks() {
    try {
        webhooks = await api('GET', '/api/webhooks');
        if (!webhooks || !Array.isArray(webhooks)) webhooks = [];
        renderWebhooks();
    } catch (e) {
        console.error('Failed to load webhooks:', e);
        webhooks = [];
        renderWebhooks();
    }
}

function renderWebhooks() {
    const tbody = document.getElementById('webhooks-table');
    if (!tbody) return;
    if (!webhooks || webhooks.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="5" style="text-align: center; padding: 40px; color: var(--text-secondary);">
                    暂无目标渠道，点击上方按钮添加
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = webhooks.map(wh => {
        let urlDisplay;
        if (wh.type === 'email') {
            const emails = (wh.url || '').split(',').map(s => s.trim()).filter(s => s);
            urlDisplay = emails.slice(0, 3).map(e => `<span class="tag tag-neutral">${escapeHtml(e)}</span>`).join(' ');
            if (emails.length > 3) urlDisplay += ` <span class="tag tag-neutral">+${emails.length - 3}</span>`;
        } else {
            const url = wh.type === 'telegram' && wh.token ? 'Telegram Bot API' : (wh.url || '');
            urlDisplay = escapeHtml(url.substring(0, 46)) + (url.length > 46 ? '...' : '');
        }
        return `
        <tr>
            <td>
                <strong>${escapeHtml(wh.name)}</strong>
                <div class="muted-line">${escapeHtml(wh.payload_type || defaultPayloadType(wh.type))}</div>
            </td>
            <td><span class="tag tag-info">${webhookTypeNames[wh.type] || wh.type}</span></td>
            <td style="max-width: 260px; overflow: hidden; text-overflow: ellipsis;">${urlDisplay}</td>
            <td>
                <span class="tag ${wh.enabled ? 'tag-success' : 'tag-neutral'}">
                    ${wh.enabled ? '已启用' : '已禁用'}
                </span>
            </td>
            <td>
                <button class="btn btn-secondary btn-sm" onclick="testWebhook('${wh.id}')">测试</button>
                <button class="btn btn-secondary btn-sm" onclick="editWebhook('${wh.id}')">编辑</button>
                <button class="btn btn-danger btn-sm" onclick="deleteWebhook('${wh.id}')">删除</button>
            </td>
        </tr>
    `}).join('');
}

function defaultPayloadType(type) {
    return webhookPayloadOptions[type]?.[0]?.[0] || 'json';
}

function openWebhookModal(data = null) {
    document.getElementById('webhook-modal-title').textContent = data ? '编辑目标渠道' : '添加目标渠道';
    document.getElementById('webhook-id').value = data?.id || '';
    document.getElementById('webhook-name').value = data?.name || '';
    document.getElementById('webhook-type').value = data?.type || 'feishu';
    document.getElementById('webhook-url').value = (data?.type !== 'email' ? data?.url : '') || '';
    document.getElementById('webhook-enabled').checked = data?.enabled !== false;
    document.getElementById('webhook-secret').value = data?.secret || '';
    document.getElementById('webhook-token').value = data?.token || '';
    document.getElementById('webhook-chat-id').value = data?.chat_id || '';
    document.getElementById('webhook-username').value = data?.username || '';
    document.getElementById('webhook-icon-url').value = data?.icon_url || '';
    document.getElementById('webhook-link-url').value = data?.link_url || '';
    document.getElementById('webhook-mention-all').checked = data?.mention_all === true;
    document.getElementById('webhook-mention-mobiles').value = data?.mention_mobiles || '';
    document.getElementById('webhook-mention-user-ids').value = data?.mention_user_ids || '';
    document.getElementById('webhook-priority').value = data?.priority || 5;
    document.getElementById('webhook-headers').value = data?.headers || '';
    document.getElementById('webhook-tls-ca-cert').value = data?.tls_ca_cert || '';
    document.getElementById('webhook-tls-client-cert').value = data?.tls_client_cert || '';
    document.getElementById('webhook-tls-client-key').value = data?.tls_client_key || '';
    document.getElementById('webhook-tls-skip-verify').checked = data?.tls_skip_verify === true;
    document.getElementById('webhook-template').value = data?.template || '';

    const recipientsEl = document.getElementById('webhook-email-recipients');
    recipientsEl.value = data?.type === 'email' && data?.url
        ? data.url.split(',').map(s => s.trim()).filter(s => s).join('\n')
        : '';

    populateSmtpAccountSelector(data?.smtp_account_id || '');
    updateWebhookTargetFields(data?.payload_type || '');
    document.getElementById('webhook-modal').classList.add('active');
}

async function populateSmtpAccountSelector(selectedId) {
    const select = document.getElementById('webhook-smtp-account');
    if (!select) return;
    select.innerHTML = '<option value="">自动选择</option>';
    try {
        const accs = await api('GET', '/api/accounts');
        if (Array.isArray(accs)) {
            accs.filter(a => a.type === 'smtp' && a.enabled).forEach(a => {
                const opt = document.createElement('option');
                opt.value = a.id;
                opt.textContent = `${a.name} (${a.email_user})`;
                if (a.id === selectedId) opt.selected = true;
                select.appendChild(opt);
            });
        }
    } catch (e) {
        // 保持默认选项即可
    }
}

function updateWebhookTargetFields(selectedPayloadType = '') {
    const type = document.getElementById('webhook-type')?.value || 'feishu';
    const isEmail = type === 'email';
    const payloadSelect = document.getElementById('webhook-payload-type');
    const currentPayload = selectedPayloadType || payloadSelect.value || defaultPayloadType(type);
    payloadSelect.innerHTML = (webhookPayloadOptions[type] || webhookPayloadOptions.custom)
        .map(([value, label]) => `<option value="${value}">${label}</option>`)
        .join('');
    payloadSelect.value = (webhookPayloadOptions[type] || []).some(([value]) => value === currentPayload)
        ? currentPayload
        : defaultPayloadType(type);

    document.querySelectorAll('.webhook-url-field, .webhook-http-field').forEach(el => {
        el.style.display = isEmail ? 'none' : '';
    });
    document.querySelectorAll('.webhook-email-field').forEach(el => {
        el.style.display = isEmail ? '' : 'none';
    });
    document.querySelectorAll('.webhook-secret-field').forEach(el => {
        el.style.display = ['dingtalk', 'feishu'].includes(type) ? '' : 'none';
    });
    document.querySelectorAll('.webhook-token-field').forEach(el => {
        el.style.display = ['gotify', 'telegram', 'ntfy', 'pushplus', 'pushover'].includes(type) ? '' : 'none';
    });
    document.querySelectorAll('.webhook-priority-field').forEach(el => {
        el.style.display = ['gotify', 'ntfy', 'pushover'].includes(type) ? '' : 'none';
    });
    document.querySelectorAll('.webhook-profile-field').forEach(el => {
        el.style.display = ['slack', 'discord', 'mattermost', 'bark'].includes(type) ? '' : 'none';
    });
    document.querySelectorAll('.webhook-link-field').forEach(el => {
        el.style.display = ['dingtalk', 'wecom', 'teams'].includes(type) ? '' : 'none';
    });
    document.querySelectorAll('.webhook-telegram-field').forEach(el => {
        el.style.display = ['telegram', 'pushover'].includes(type) ? '' : 'none';
    });
    document.querySelectorAll('.webhook-mention-field').forEach(el => {
        el.style.display = ['dingtalk', 'wecom'].includes(type) ? '' : 'none';
    });
    document.querySelectorAll('.webhook-wecom-mention-field').forEach(el => {
        el.style.display = type === 'wecom' ? '' : 'none';
    });

    const label = document.getElementById('webhook-url-label');
    const url = document.getElementById('webhook-url');
    const labels = {
        feishu: ['飞书机器人 Webhook', 'https://open.feishu.cn/open-apis/bot/v2/hook/...'],
        dingtalk: ['钉钉机器人 Webhook', 'https://oapi.dingtalk.com/robot/send?access_token=...'],
        wecom: ['企业微信群机器人 Webhook', 'https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=...'],
        slack: ['Slack Incoming Webhook', 'https://hooks.slack.com/services/...'],
        discord: ['Discord Webhook', 'https://discord.com/api/webhooks/...'],
        teams: ['Microsoft Teams Webhook', 'https://...webhook.office.com/...'],
        mattermost: ['Mattermost Incoming Webhook', 'https://mattermost.example.com/hooks/...'],
        gotify: ['Gotify /message 地址', 'https://gotify.example.com/message'],
        bark: ['Bark 推送地址', 'https://api.day.app/你的key'],
        serverchan: ['Server 酱 send 地址', 'https://sctapi.ftqq.com/SENDKEY.send'],
        telegram: ['Bot API 地址或 Bot Token', 'https://api.telegram.org/botTOKEN/sendMessage'],
        ntfy: ['ntfy Topic 地址', 'https://ntfy.sh/your-topic'],
        pushplus: ['PushPlus send 地址', 'https://www.pushplus.plus/send'],
        chanify: ['Chanify Webhook 地址', 'https://api.chanify.net/v1/sender/...'],
        pushover: ['Pushover messages 地址', 'https://api.pushover.net/1/messages.json'],
        custom: ['目标 URL', 'https://...']
    };
    const [labelText, placeholder] = labels[type] || ['目标 URL', 'https://...'];
    label.textContent = labelText;
    url.placeholder = placeholder;
}

function fillWebhookTemplateSample() {
    const type = document.getElementById('webhook-type')?.value || 'custom';
    document.getElementById('webhook-template').value = webhookTemplateSamples[type] || webhookTemplateSamples.custom;
}

function editWebhook(id) {
    const wh = webhooks.find(w => w.id === id);
    if (wh) openWebhookModal(wh);
}

async function saveWebhook() {
    const id = document.getElementById('webhook-id').value;
    const type = document.getElementById('webhook-type').value;

    let url;
    if (type === 'email') {
        const emails = document.getElementById('webhook-email-recipients').value
            .split('\n')
            .map(s => s.trim())
            .filter(s => s && s.includes('@'));
        if (emails.length === 0) {
            await showAppAlert('请至少填写一个收件人邮箱地址', { type: 'warning', title: '收件人为空' });
            return;
        }
        url = emails.join(',');
    } else {
        url = document.getElementById('webhook-url').value.trim();
    }

    const data = {
        id: id || undefined,
        name: document.getElementById('webhook-name').value.trim(),
        type,
        url,
        enabled: document.getElementById('webhook-enabled').checked,
        template: type === 'email' ? '' : document.getElementById('webhook-template').value.trim(),
        smtp_account_id: type === 'email' ? (document.getElementById('webhook-smtp-account')?.value || '') : '',
        secret: ['dingtalk', 'feishu'].includes(type) ? document.getElementById('webhook-secret').value.trim() : '',
        payload_type: type === 'email' ? '' : document.getElementById('webhook-payload-type').value,
        token: ['gotify', 'telegram', 'ntfy', 'pushplus', 'pushover'].includes(type) ? document.getElementById('webhook-token').value.trim() : '',
        chat_id: ['telegram', 'pushover'].includes(type) ? document.getElementById('webhook-chat-id').value.trim() : '',
        username: ['slack', 'discord', 'mattermost'].includes(type) ? document.getElementById('webhook-username').value.trim() : '',
        icon_url: ['slack', 'discord', 'mattermost', 'bark'].includes(type) ? document.getElementById('webhook-icon-url').value.trim() : '',
        link_url: ['dingtalk', 'wecom', 'teams'].includes(type) ? document.getElementById('webhook-link-url').value.trim() : '',
        mention_all: ['dingtalk', 'wecom'].includes(type) ? document.getElementById('webhook-mention-all').checked : false,
        mention_mobiles: ['dingtalk', 'wecom'].includes(type) ? document.getElementById('webhook-mention-mobiles').value.trim() : '',
        mention_user_ids: type === 'wecom' ? document.getElementById('webhook-mention-user-ids').value.trim() : '',
        priority: ['gotify', 'ntfy', 'pushover'].includes(type) ? (parseInt(document.getElementById('webhook-priority').value, 10) || 0) : 0,
        headers: type === 'email' ? '' : document.getElementById('webhook-headers').value.trim(),
        tls_ca_cert: type === 'email' ? '' : document.getElementById('webhook-tls-ca-cert').value.trim(),
        tls_client_cert: type === 'email' ? '' : document.getElementById('webhook-tls-client-cert').value.trim(),
        tls_client_key: type === 'email' ? '' : document.getElementById('webhook-tls-client-key').value.trim(),
        tls_skip_verify: type === 'email' ? false : document.getElementById('webhook-tls-skip-verify').checked
    };
    if (data.headers && data.headers !== '********') {
        try {
            const parsedHeaders = JSON.parse(data.headers);
            if (!parsedHeaders || Array.isArray(parsedHeaders) || typeof parsedHeaders !== 'object') {
                throw new Error('headers must be object');
            }
        } catch (e) {
            await showAppAlert('自定义请求头必须是合法 JSON 对象', { type: 'warning', title: '请求头格式错误' });
            return;
        }
    }

    try {
        if (id) {
            await api('PUT', `/api/webhooks/${id}`, data);
        } else {
            await api('POST', '/api/webhooks', data);
        }
        closeModal('webhook-modal');
        loadWebhooks();
    } catch (e) {
        await showAppAlert('保存失败: ' + e.message, { type: 'error', title: '保存失败' });
    }
}

async function deleteWebhook(id) {
    if (!(await showAppConfirm('确定要删除此目标渠道吗？', { title: '删除目标渠道', confirmText: '删除' }))) return;
    try {
        await api('DELETE', `/api/webhooks/${id}`);
        loadWebhooks();
    } catch (e) {
        await showAppAlert('删除失败: ' + e.message, { type: 'error', title: '删除失败' });
    }
}

async function testWebhook(id) {
    try {
        const result = await api('POST', `/api/webhooks/${id}/test`);
        if (result.success) {
            await showAppAlert('测试发送成功', { type: 'success', title: '发送成功' });
        } else {
            await showAppAlert('测试发送失败: ' + result.error, { type: 'error', title: '发送失败' });
        }
    } catch (e) {
        await showAppAlert('测试失败: ' + e.message, { type: 'error', title: '测试失败' });
    }
}
