// ===================== State =====================
let accounts = [];
let webhooks = [];
let rules = [];
let filterRules = [];
let logs = [];
let lastChecks = {};

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

// ===================== Navigation =====================
document.querySelectorAll('.nav-tab').forEach(tab => {
    tab.addEventListener('click', () => {
        document.querySelectorAll('.nav-tab').forEach(t => t.classList.remove('active'));
        document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
        tab.classList.add('active');
        document.getElementById(tab.dataset.page).classList.add('active');

        // Load data for the page
        switch (tab.dataset.page) {
            case 'accounts': loadAccounts(); break;
            case 'webhooks': loadWebhooks(); break;
            case 'filters': loadFilters(); break;
            case 'rules': loadRules(); break;
            case 'history': loadHistory(); break;
        }
    });
});

// ===================== API Calls =====================
async function api(method, path, data) {
    const opts = { method };
    if (data) {
        opts.headers = { 'Content-Type': 'application/json' };
        opts.body = JSON.stringify(data);
    }
    const res = await fetch(path, opts);
    return res.json();
}

// ===================== Dashboard =====================
async function loadStats() {
    try {
        const stats = await api('GET', '/api/stats');
        document.getElementById('stat-pending').textContent = stats?.pending || 0;
        document.getElementById('stat-sent').textContent = stats?.sent || 0;
        document.getElementById('stat-failed').textContent = stats?.failed || 0;

        const accData = await api('GET', '/api/accounts');
        document.getElementById('stat-accounts').textContent = (accData && Array.isArray(accData)) ? accData.length : 0;

        // 更新最后检查时间状态
        if (stats?.last_checks) {
            lastChecks = stats.last_checks;
            const times = Object.values(lastChecks);
            if (times.length > 0) {
                times.sort().reverse();
                const timeStr = times[0].split(' ')[1] || times[0];
                document.getElementById('global-last-check').innerHTML = `<span class="status-dot"></span>最后检查于 ${timeStr}`;
            }

            if (document.getElementById('accounts').classList.contains('active')) {
                renderAccounts();
            }
        }
    } catch (e) {
        console.error('Failed to load stats:', e);
    }
}

async function loadLogs() {
    try {
        logs = await api('GET', '/api/logs');
        if (!logs || !Array.isArray(logs)) logs = [];
        renderLogs();
    } catch (e) {
        console.error('Failed to load logs:', e);
        logs = [];
        renderLogs();
    }
}

function renderLogs() {
    const container = document.getElementById('logs');
    if (!logs || logs.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                    <polyline points="14 2 14 8 20 8"/>
                    <line x1="16" y1="13" x2="8" y2="13"/>
                    <line x1="16" y1="17" x2="8" y2="17"/>
                </svg>
                <h3>暂无日志</h3>
                <p>系统运行日志将在此显示</p>
            </div>
        `;
        return;
    }

    const iconMap = {
        info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>',
        success: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>',
        error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>',
        warning: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
    };

    container.innerHTML = logs.map(log => `
        <div class="log-item">
            <div class="log-icon ${log.type}">${iconMap[log.type] || iconMap.info}</div>
            <div class="log-content">
                <div class="log-time">${log.time}</div>
                <div class="log-message">${log.message}</div>
            </div>
        </div>
    `).join('');
}

function clearLogs() {
    logs = [];
    renderLogs();
}

// ===================== Accounts =====================
async function loadAccounts() {
    try {
        accounts = await api('GET', '/api/accounts');
        if (!accounts || !Array.isArray(accounts)) accounts = [];
        renderAccounts();
    } catch (e) {
        console.error('Failed to load accounts:', e);
        accounts = [];
        renderAccounts();
    }
}

function renderAccounts() {
    const tbody = document.getElementById('accounts-table');
    if (!accounts || accounts.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="5" style="text-align: center; padding: 40px; color: var(--text-secondary);">
                    暂无邮箱账号，点击上方按钮添加
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = accounts.map(acc => {
        const checkTime = lastChecks[acc.id];
        const checkDisplay = checkTime ? checkTime.split(' ')[1] : '尚未检查';
        return `
        <tr>
            <td>
                <strong>${escapeHtml(acc.name)}</strong>
                <div style="font-size: 0.75rem; color: var(--text-tertiary); margin-top: 4px; display: flex; align-items: center; gap: 4px;">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="width: 12px; height: 12px;"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                    ${checkDisplay}
                </div>
            </td>
            <td>${escapeHtml(acc.email_user)}</td>
            <td>${escapeHtml(acc.imap_server)}</td>
            <td>
                <span class="tag ${acc.enabled ? 'tag-success' : 'tag-neutral'}">
                    ${acc.enabled ? '已启用' : '已禁用'}
                </span>
            </td>
            <td>
                <button class="btn btn-secondary btn-sm" onclick="testAccountConnectionInList('${acc.id}')">测试</button>
                <button class="btn btn-secondary btn-sm" onclick="editAccount('${acc.id}')">编辑</button>
                <button class="btn btn-danger btn-sm" onclick="deleteAccount('${acc.id}')">删除</button>
            </td>
        </tr>
    `}).join('');
}

function openAccountModal(data = null) {
    document.getElementById('folder-checkboxes').style.display = 'none';
    document.getElementById('folder-checkboxes').innerHTML = '';
    document.getElementById('account-modal-title').textContent = data ? '编辑邮箱账号' : '添加邮箱账号';
    document.getElementById('account-id').value = data?.id || '';
    document.getElementById('account-name').value = data?.name || '';
    document.getElementById('account-email').value = data?.email_user || '';
    // 解析IMAP服务器地址
    const imapServer = data?.imap_server || '';
    if (imapServer.includes(':')) {
        const [host, port] = imapServer.split(':');
        document.getElementById('account-imap-host').value = host;
        document.getElementById('account-imap-port').value = port;
    } else {
        document.getElementById('account-imap-host').value = imapServer;
        document.getElementById('account-imap-port').value = '993';
    }
    document.getElementById('account-pass').value = data?.email_pass || '';
    document.getElementById('account-interval').value = data?.check_interval || 60;
    document.getElementById('account-folders').value = (data?.folders || ['INBOX']).join(', ');
    document.getElementById('account-enabled').checked = data?.enabled !== false;
    document.getElementById('account-modal').classList.add('active');
}

function editAccount(id) {
    const acc = accounts.find(a => a.id === id);
    if (acc) openAccountModal(acc);
}

function getAccountFormData() {
    const id = document.getElementById('account-id').value;
    const foldersList = document.getElementById('account-folders').value
        .split(',')
        .map(s => s.trim())
        .filter(s => s);

    const imapHost = document.getElementById('account-imap-host').value.trim();
    const imapPort = document.getElementById('account-imap-port').value || '993';
    const data = {
        id: id || undefined,
        name: document.getElementById('account-name').value,
        email_user: document.getElementById('account-email').value,
        imap_server: imapHost + ':' + imapPort,
        email_pass: document.getElementById('account-pass').value,
        check_interval: parseInt(document.getElementById('account-interval').value) || 60,
        folders: foldersList.length > 0 ? foldersList : ['INBOX'],
        enabled: document.getElementById('account-enabled').checked
    };
    return data;
}

async function saveAccount() {
    const id = document.getElementById('account-id').value;
    const data = getAccountFormData();

    try {
        if (id) {
            await api('PUT', `/api/accounts/${id}`, data);
        } else {
            await api('POST', '/api/accounts', data);
        }
        closeModal('account-modal');
        loadAccounts();
        loadStats();
    } catch (e) {
        alert('保存失败: ' + e.message);
    }
}

async function testAccountFolders() {
    const btn = document.getElementById('btn-test-folders');
    const originalText = btn.textContent;
    btn.textContent = '获取中...';
    btn.disabled = true;

    try {
        const data = getAccountFormData();
        const res = await api('POST', '/api/test-imap', data);
        if (res.error) {
            alert('获取失败: ' + res.error);
        } else if (res.folders) {
            const container = document.getElementById('folder-checkboxes');
            container.style.display = 'flex';
            container.innerHTML = '';

            const currentSelected = document.getElementById('account-folders').value
                .split(',')
                .map(s => s.trim())
                .filter(s => s);

            res.folders.forEach(folder => {
                const isChecked = currentSelected.includes(folder);
                const label = document.createElement('label');
                label.className = 'form-checkbox';
                label.style.margin = '0';
                label.style.padding = '6px 10px';
                label.style.background = 'var(--bg-primary)';
                label.style.borderRadius = '4px';
                label.style.border = '1px solid var(--border-color)';

                const input = document.createElement('input');
                input.type = 'checkbox';
                input.value = folder;
                input.checked = isChecked;
                input.onchange = (e) => {
                    const selected = new Set(
                        document.getElementById('account-folders').value
                            .split(',')
                            .map(s => s.trim())
                            .filter(s => s)
                    );
                    if (e.target.checked) {
                        selected.add(e.target.value);
                    } else {
                        selected.delete(e.target.value);
                    }
                    document.getElementById('account-folders').value = Array.from(selected).join(', ');
                };

                const span = document.createElement('span');
                span.textContent = folder;

                label.appendChild(input);
                label.appendChild(span);
                container.appendChild(label);
            });
        }
    } catch (e) {
        alert('网络或服务异常: ' + e.message);
    } finally {
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

async function testAccountConnectionModal() {
    try {
        const data = getAccountFormData();
        const res = await api('POST', '/api/test-imap', data);
        if (res.error) {
            alert('连接测试失败: ' + res.error);
        } else {
            alert('当前配置连接测试成功！');
        }
    } catch (e) {
        alert('网络异常: ' + e.message);
    }
}

async function testAccountConnectionInList(id) {
    const acc = accounts.find(a => a.id === id);
    if (!acc) return;
    try {
        const res = await api('POST', '/api/test-imap', acc);
        if (res.error) {
            alert('连接测试失败: ' + res.error);
        } else {
            alert('账号连接测试成功！');
        }
    } catch (e) {
        alert('网络异常: ' + e.message);
    }
}

async function deleteAccount(id) {
    if (!confirm('确定要删除此邮箱账号吗？')) return;
    try {
        await api('DELETE', `/api/accounts/${id}`);
        loadAccounts();
        loadStats();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

// ===================== Webhooks =====================
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
    if (!webhooks || webhooks.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="5" style="text-align: center; padding: 40px; color: var(--text-secondary);">
                    暂无 Webhook，点击上方按钮添加
                </td>
            </tr>
        `;
        return;
    }

    const typeNames = {
        feishu: '飞书',
        dingtalk: '钉钉',
        wecom: '企业微信',
        slack: 'Slack',
        discord: 'Discord',
        custom: '自定义'
    };

    tbody.innerHTML = webhooks.map(wh => `
        <tr>
            <td><strong>${escapeHtml(wh.name)}</strong></td>
            <td><span class="tag tag-info">${typeNames[wh.type] || wh.type}</span></td>
            <td style="max-width: 200px; overflow: hidden; text-overflow: ellipsis;">
                ${escapeHtml(wh.url.substring(0, 40))}${wh.url.length > 40 ? '...' : ''}
            </td>
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
    `).join('');
}

function openWebhookModal(data = null) {
    document.getElementById('webhook-modal-title').textContent = data ? '编辑 Webhook' : '添加 Webhook';
    document.getElementById('webhook-id').value = data?.id || '';
    document.getElementById('webhook-name').value = data?.name || '';
    document.getElementById('webhook-type').value = data?.type || 'feishu';
    document.getElementById('webhook-url').value = data?.url || '';
    document.getElementById('webhook-enabled').checked = data?.enabled !== false;
    document.getElementById('webhook-modal').classList.add('active');
}

function editWebhook(id) {
    const wh = webhooks.find(w => w.id === id);
    if (wh) openWebhookModal(wh);
}

async function saveWebhook() {
    const id = document.getElementById('webhook-id').value;
    const data = {
        id: id || undefined,
        name: document.getElementById('webhook-name').value,
        type: document.getElementById('webhook-type').value,
        url: document.getElementById('webhook-url').value,
        enabled: document.getElementById('webhook-enabled').checked
    };

    try {
        if (id) {
            await api('PUT', `/api/webhooks/${id}`, data);
        } else {
            await api('POST', '/api/webhooks', data);
        }
        closeModal('webhook-modal');
        loadWebhooks();
    } catch (e) {
        alert('保存失败: ' + e.message);
    }
}

async function deleteWebhook(id) {
    if (!confirm('确定要删除此 Webhook 吗？')) return;
    try {
        await api('DELETE', `/api/webhooks/${id}`);
        loadWebhooks();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

async function testWebhook(id) {
    try {
        const result = await api('POST', `/api/webhooks/${id}/test`);
        if (result.success) {
            alert('测试发送成功！');
        } else {
            alert('测试发送失败: ' + result.error);
        }
    } catch (e) {
        alert('测试失败: ' + e.message);
    }
}

// ===================== Filters =====================
async function loadFilters() {
    try {
        filterRules = await api('GET', '/api/filters');
        if (!filterRules || !Array.isArray(filterRules)) filterRules = [];
        renderFilters();
    } catch (e) {
        console.error('Failed to load filters:', e);
        filterRules = [];
        renderFilters();
    }
}

function renderFilters() {
    const tbody = document.getElementById('filters-table');
    if (!filterRules || filterRules.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="6" style="text-align: center; padding: 40px; color: var(--text-secondary);">
                    暂无过滤规则，点击上方按钮添加
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = filterRules.map(rule => `
        <tr>
            <td><strong>${escapeHtml(rule.name)}</strong></td>
            <td>${rule.type === 'content' ? '邮件内容' : '发信人'}</td>
            <td><span class="tag ${rule.mode === 'blacklist' ? 'tag-warning' : 'tag-info'}">${rule.mode === 'blacklist' ? '黑名单' : '白名单'}</span></td>
            <td>${rule.patterns?.length ? rule.patterns.slice(0, 4).map(p => `<span class="tag tag-neutral">${escapeHtml(p)}</span>`).join(' ') : '无'}${rule.patterns?.length > 4 ? ' ...' : ''}</td>
            <td>
                <span class="tag ${rule.enabled ? 'tag-success' : 'tag-neutral'}">
                    ${rule.enabled ? '已启用' : '已禁用'}
                </span>
            </td>
            <td>
                <button class="btn btn-secondary btn-sm" onclick="editFilter('${rule.id}')">编辑</button>
                <button class="btn btn-danger btn-sm" onclick="deleteFilter('${rule.id}')">删除</button>
            </td>
        </tr>
    `).join('');
}

function renderFilterTemplates() {
    const container = document.getElementById('filter-template-list');
    if (!container) return;

    container.innerHTML = filterTemplates.map((template, index) => {
        const typeName = template.type === 'content' ? '邮件内容' : '发信人';
        const modeName = template.mode === 'blacklist' ? '黑名单' : '白名单';
        return `
            <button class="template-card" type="button" onclick="applyFilterTemplate(${index})">
                <span class="template-title">${escapeHtml(template.name)}</span>
                <span class="template-meta">${typeName} / ${modeName} / ${template.patterns.length} 条</span>
            </button>
        `;
    }).join('');
}

function applyFilterTemplate(index) {
    const template = filterTemplates[index];
    if (!template) return;

    document.getElementById('filter-name').value = template.name;
    document.getElementById('filter-type').value = template.type;
    document.getElementById('filter-mode').value = template.mode;
    document.getElementById('filter-patterns').value = template.patterns.join('\n');
    document.getElementById('filter-enabled').checked = true;
}

function openFilterModal(data = null) {
    renderFilterTemplates();
    document.getElementById('filter-modal-title').textContent = data ? '编辑过滤规则' : '添加过滤规则';
    document.getElementById('filter-id').value = data?.id || '';
    document.getElementById('filter-name').value = data?.name || '';
    document.getElementById('filter-type').value = data?.type || 'sender';
    document.getElementById('filter-mode').value = data?.mode || 'blacklist';
    document.getElementById('filter-patterns').value = (data?.patterns || []).join('\n');
    document.getElementById('filter-enabled').checked = data?.enabled !== false;
    document.getElementById('filter-modal').classList.add('active');
}

function editFilter(id) {
    const filter = filterRules.find(f => f.id === id);
    if (filter) openFilterModal(filter);
}

async function saveFilter() {
    const id = document.getElementById('filter-id').value;
    const data = {
        id: id || undefined,
        name: document.getElementById('filter-name').value,
        type: document.getElementById('filter-type').value,
        mode: document.getElementById('filter-mode').value,
        patterns: document.getElementById('filter-patterns').value
            .split('\n')
            .map(s => s.trim())
            .filter(s => s),
        enabled: document.getElementById('filter-enabled').checked
    };

    try {
        if (id) {
            await api('PUT', `/api/filters/${id}`, data);
        } else {
            await api('POST', '/api/filters', data);
        }
        closeModal('filter-modal');
        loadFilters();
        loadRules();
    } catch (e) {
        alert('保存失败: ' + e.message);
    }
}

async function deleteFilter(id) {
    if (!confirm('确定要删除此过滤规则吗？相关转发规则会自动移除该引用。')) return;
    try {
        await api('DELETE', `/api/filters/${id}`);
        loadFilters();
        loadRules();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

// ===================== Rules =====================
async function loadRules() {
    try {
        if (!filterRules.length) {
            filterRules = await api('GET', '/api/filters');
            if (!filterRules || !Array.isArray(filterRules)) filterRules = [];
        }
        rules = await api('GET', '/api/rules');
        if (!rules || !Array.isArray(rules)) rules = [];
        renderRules();
    } catch (e) {
        console.error('Failed to load rules:', e);
        rules = [];
        renderRules();
    }
}

function renderRules() {
    const tbody = document.getElementById('rules-table');
    if (!rules || rules.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="6" style="text-align: center; padding: 40px; color: var(--text-secondary);">
                    暂无转发规则，点击上方按钮添加
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = rules.map(rule => {
        const acc = accounts.find(a => a.id === rule.source_account);
        const wh = webhooks.find(w => w.id === rule.target_webhook);
        const selectedFilters = (rule.filter_rule_ids || [])
            .map(id => filterRules.find(f => f.id === id))
            .filter(Boolean);
        return `
            <tr>
                <td><strong>${escapeHtml(rule.name)}</strong></td>
                <td>${rule.source_account === 'all' ? '所有账号' : (acc?.name || '未知')}</td>
                <td>${wh?.name || '未知'}</td>
                <td>
                    ${rule.filters?.length ? rule.filters.map(f => `<span class="tag tag-neutral">${escapeHtml(f)}</span>`).join(' ') : '无'}
                    ${selectedFilters.length ? `<div style="margin-top: 6px;">${selectedFilters.map(f => `<span class="tag tag-info">${escapeHtml(f.name)}</span>`).join(' ')}</div>` : ''}
                </td>
                <td>
                    <span class="tag ${rule.enabled ? 'tag-success' : 'tag-neutral'}">
                        ${rule.enabled ? '已启用' : '已禁用'}
                    </span>
                </td>
                <td>
                    <button class="btn btn-secondary btn-sm" onclick="editRule('${rule.id}')">编辑</button>
                    <button class="btn btn-danger btn-sm" onclick="deleteRule('${rule.id}')">删除</button>
                </td>
            </tr>
        `;
    }).join('');
}

function openRuleModal(data = null) {
    // Populate selects
    const sourceSelect = document.getElementById('rule-source');
    sourceSelect.innerHTML = '<option value="all">所有账号</option>' +
        accounts.map(a => `<option value="${a.id}">${escapeHtml(a.name)}</option>`).join('');

    const targetSelect = document.getElementById('rule-target');
    targetSelect.innerHTML = webhooks.map(w => `<option value="${w.id}">${escapeHtml(w.name)}</option>`).join('');

    document.getElementById('rule-modal-title').textContent = data ? '编辑转发规则' : '添加转发规则';
    document.getElementById('rule-id').value = data?.id || '';
    document.getElementById('rule-name').value = data?.name || '';
    document.getElementById('rule-source').value = data?.source_account || 'all';
    document.getElementById('rule-target').value = data?.target_webhook || '';
    document.getElementById('rule-filters').value = (data?.filters || []).join(', ');
    renderRuleFilterDropdown(data?.filter_rule_ids || []);
    document.getElementById('rule-enabled').checked = data?.enabled !== false;
    document.getElementById('rule-modal').classList.add('active');
}

function renderRuleFilterDropdown(selectedIDs = []) {
    const menu = document.getElementById('rule-filter-menu');
    if (!menu) return;

    const selectedSet = new Set(selectedIDs);
    if (!filterRules.length) {
        menu.innerHTML = '<div class="filter-empty">暂无过滤规则，请先在过滤规则页面添加</div>';
        updateRuleFilterSummary();
        return;
    }

    menu.innerHTML = filterRules.map(rule => {
        const typeName = rule.type === 'content' ? '邮件内容' : '发信人';
        const modeName = rule.mode === 'blacklist' ? '黑名单' : '白名单';
        const enabledText = rule.enabled ? '全局启用' : '全局禁用';
        return `
            <label class="filter-option">
                <input type="checkbox" class="rule-filter-checkbox" value="${escapeHtml(rule.id)}"
                    ${selectedSet.has(rule.id) ? 'checked' : ''} onchange="updateRuleFilterSummary()">
                <span class="filter-option-main">
                    <span class="filter-option-title">${escapeHtml(rule.name)}</span>
                    <span class="filter-option-meta">${typeName} / ${modeName} / ${enabledText}</span>
                </span>
            </label>
        `;
    }).join('');
    updateRuleFilterSummary();
}

function toggleRuleFilterDropdown(event) {
    event.stopPropagation();
    document.getElementById('rule-filter-dropdown').classList.toggle('open');
}

function getSelectedRuleFilterIDs() {
    return Array.from(document.querySelectorAll('.rule-filter-checkbox:checked'))
        .map(input => input.value);
}

function updateRuleFilterSummary() {
    const summary = document.getElementById('rule-filter-summary');
    if (!summary) return;

    const selectedIDs = getSelectedRuleFilterIDs();
    if (selectedIDs.length === 0) {
        summary.textContent = '未应用过滤规则';
        return;
    }

    const names = selectedIDs
        .map(id => filterRules.find(rule => rule.id === id)?.name)
        .filter(Boolean);
    summary.textContent = names.length <= 2 ? names.join('、') : `已选择 ${names.length} 个过滤规则`;
}

function editRule(id) {
    const rule = rules.find(r => r.id === id);
    if (rule) openRuleModal(rule);
}

async function saveRule() {
    const id = document.getElementById('rule-id').value;
    const data = {
        id: id || undefined,
        name: document.getElementById('rule-name').value,
        source_account: document.getElementById('rule-source').value,
        target_webhook: document.getElementById('rule-target').value,
        filters: document.getElementById('rule-filters').value
            .split(',')
            .map(s => s.trim())
            .filter(s => s),
        filter_rule_ids: getSelectedRuleFilterIDs(),
        enabled: document.getElementById('rule-enabled').checked
    };

    try {
        if (id) {
            await api('PUT', `/api/rules/${id}`, data);
        } else {
            await api('POST', '/api/rules', data);
        }
        closeModal('rule-modal');
        loadRules();
    } catch (e) {
        alert('保存失败: ' + e.message);
    }
}

async function deleteRule(id) {
    if (!confirm('确定要删除此转发规则吗？')) return;
    try {
        await api('DELETE', `/api/rules/${id}`);
        loadRules();
    } catch (e) {
        alert('删除失败: ' + e.message);
    }
}

// ===================== History =====================
async function loadHistory() {
    try {
        const status = document.getElementById('history-filter').value;
        const messages = await api('GET', `/api/messages?status=${status}`);
        renderHistory(messages || []);
    } catch (e) {
        console.error('Failed to load history:', e);
        renderHistory([]);
    }
}

function renderHistory(messages) {
    const container = document.getElementById('history-list');
    if (!messages || messages.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="10"/>
                    <polyline points="12 6 12 12 16 14"/>
                </svg>
                <h3>暂无转发记录</h3>
                <p>邮件转发记录将在此显示</p>
            </div>
        `;
        return;
    }

    const statusTags = {
        pending: '<span class="tag tag-warning">待发送</span>',
        sent: '<span class="tag tag-success">已发送</span>',
        failed: '<span class="tag tag-error">发送失败</span>'
    };

    container.innerHTML = messages.map(msg => `
        <div class="history-item">
            <div class="history-header">
                <span class="history-subject">${escapeHtml(msg.subject || '(无主题)')}</span>
                ${statusTags[msg.status] || ''}
            </div>
            <div class="history-meta">
                <span>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
                    ${escapeHtml(msg.from)}
                </span>
                <span>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="9" y1="21" x2="9" y2="9"/></svg>
                    ${escapeHtml(msg.source_email)}
                </span>
                <span>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
                    ${msg.target_name || '-'}
                </span>
                <span>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                    ${formatDate(msg.created_at)}
                </span>
            </div>
        </div>
    `).join('');
}

// ===================== Utilities =====================
function closeModal(id) {
    document.getElementById(id).classList.remove('active');
    document.getElementById('rule-filter-dropdown')?.classList.remove('open');
}

function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function formatDate(dateStr) {
    if (!dateStr) return '-';
    const d = new Date(dateStr);
    return d.toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit'
    });
}

// Close modal on overlay click
document.querySelectorAll('.modal-overlay').forEach(overlay => {
    let mouseDownTarget = null;
    overlay.addEventListener('mousedown', (e) => {
        mouseDownTarget = e.target;
    });
    overlay.addEventListener('mouseup', (e) => {
        if (e.target === overlay && mouseDownTarget === overlay) {
            overlay.classList.remove('active');
        }
        mouseDownTarget = null;
    });
});

document.addEventListener('click', () => {
    document.getElementById('rule-filter-dropdown')?.classList.remove('open');
});

document.getElementById('rule-filter-dropdown')?.addEventListener('click', event => {
    event.stopPropagation();
});

// ===================== Initialize =====================
loadStats();
loadLogs();
loadAccounts();
loadWebhooks();
loadFilters();
loadRules();

// Auto refresh
setInterval(loadLogs, 5000);
setInterval(loadStats, 10000);
