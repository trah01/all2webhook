// ===================== State =====================
let accounts = [];
let webhooks = [];
let rules = [];
let filterRules = [];
let logs = [];
let lastChecks = {};

const defaultSenderFilterIDs = new Set(['default_sender_blacklist', 'default_sender_whitelist']);
let currentFilterDefaultSender = false;

// ===================== Navigation =====================
function navigateToPage(page) {
    // Clear all active states
    document.querySelectorAll('.nav-tab').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.nav-tab-dropdown-item').forEach(t => t.classList.remove('active'));
    document.querySelectorAll('.nav-tab-group').forEach(g => g.classList.remove('child-active'));
    document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));

    // Activate the page
    const pageEl = document.getElementById(page);
    if (pageEl) pageEl.classList.add('active');

    // Activate the matching tab or dropdown item
    const directTab = document.querySelector(`.nav-tab[data-page="${page}"]`);
    if (directTab) {
        directTab.classList.add('active');
    }
    const dropdownItem = document.querySelector(`.nav-tab-dropdown-item[data-page="${page}"]`);
    if (dropdownItem) {
        dropdownItem.classList.add('active');
        const group = dropdownItem.closest('.nav-tab-group');
        if (group) group.classList.add('child-active');
    }

    // Load data for the page
    switch (page) {
        case 'accounts': loadAccounts(); break;
        case 'projects': loadProjects(); break;
        case 'webhooks': loadWebhooks(); break;
        case 'filters': loadFilters(); break;
        case 'rules': loadRules(); break;
        case 'history': loadHistory(); break;
    }
}

// Direct tabs (with data-page, not triggers)
document.querySelectorAll('.nav-tab[data-page]').forEach(tab => {
    tab.addEventListener('click', () => navigateToPage(tab.dataset.page));
});

// Dropdown items
document.querySelectorAll('.nav-tab-dropdown-item[data-page]').forEach(item => {
    item.addEventListener('click', () => navigateToPage(item.dataset.page));
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
        const limit = document.getElementById('log-limit')?.value || '50';
        logs = await api('GET', `/api/logs?limit=${encodeURIComponent(limit)}`);
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

async function clearLogs() {
    try {
        await api('DELETE', '/api/logs');
        logs = [];
        renderLogs();
        await showAppAlert('实时日志已清空', { type: 'success', title: '已清空' });
    } catch (e) {
        await showAppAlert('清空日志失败: ' + e.message, { type: 'error', title: '清空失败' });
    }
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
        const accountType = acc.type === 'smtp' ? 'smtp' : 'imap';
        const checkDisplay = accountType === 'smtp' ? '用于发送通知' : (checkTime ? checkTime.split(' ')[1] : '尚未检查');
        const serverDisplay = accountType === 'smtp' ? acc.smtp_server : acc.imap_server;
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
            <td>
                ${escapeHtml(serverDisplay || '-')}
                <div class="muted-line">${accountType === 'smtp' ? 'SMTP 发信' : 'IMAP 收件'}</div>
            </td>
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
    document.getElementById('account-type').value = data?.type === 'smtp' ? 'smtp' : 'imap';
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
    const smtpServer = data?.smtp_server || '';
    if (smtpServer.includes(':')) {
        const [host, port] = smtpServer.split(':');
        document.getElementById('account-smtp-host').value = host;
        document.getElementById('account-smtp-port').value = port;
    } else {
        document.getElementById('account-smtp-host').value = smtpServer;
        document.getElementById('account-smtp-port').value = '587';
    }
    document.getElementById('account-pass').value = data?.email_pass || '';
    document.getElementById('account-interval').value = data?.check_interval || 60;
    document.getElementById('account-folders').value = (data?.folders || ['INBOX']).join(', ');
    document.getElementById('account-enabled').checked = data?.enabled !== false;
    updateAccountTypeFields();
    document.getElementById('account-modal').classList.add('active');
}

function updateAccountTypeFields() {
    const type = document.getElementById('account-type')?.value || 'imap';
    document.querySelectorAll('.account-imap-fields').forEach(el => {
        el.style.display = type === 'imap' ? '' : 'none';
    });
    document.querySelectorAll('.account-smtp-fields').forEach(el => {
        el.style.display = type === 'smtp' ? '' : 'none';
    });
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
    const smtpHost = document.getElementById('account-smtp-host').value.trim();
    const smtpPort = document.getElementById('account-smtp-port').value || '587';
    const type = document.getElementById('account-type').value;
    const data = {
        id: id || undefined,
        name: document.getElementById('account-name').value,
        type,
        email_user: document.getElementById('account-email').value,
        imap_server: type === 'imap' && imapHost ? imapHost + ':' + imapPort : '',
        smtp_server: type === 'smtp' && smtpHost ? smtpHost + ':' + smtpPort : '',
        email_pass: document.getElementById('account-pass').value,
        check_interval: type === 'imap' ? (parseInt(document.getElementById('account-interval').value) || 60) : 60,
        folders: type === 'imap' ? (foldersList.length > 0 ? foldersList : ['INBOX']) : [],
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
        await showAppAlert('保存失败: ' + e.message, { type: 'error', title: '保存失败' });
    }
}

async function testAccountFolders() {
    if (document.getElementById('account-type').value !== 'imap') {
        await showAppAlert('SMTP 账号不需要获取 IMAP 文件夹', { type: 'info', title: '无需操作' });
        return;
    }
    const btn = document.getElementById('btn-test-folders');
    const originalText = btn.textContent;
    btn.textContent = '获取中...';
    btn.disabled = true;

    try {
        const data = getAccountFormData();
        const res = await api('POST', '/api/test-imap', data);
        if (res.error) {
            await showAppAlert('获取失败: ' + res.error, { type: 'error', title: '获取失败' });
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
        await showAppAlert('网络或服务异常: ' + e.message, { type: 'error', title: '请求失败' });
    } finally {
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

async function testAccountConnectionModal() {
    try {
        const data = getAccountFormData();
        const res = await api('POST', data.type === 'smtp' ? '/api/test-smtp' : '/api/test-imap', data);
        if (res.error) {
            await showAppAlert('连接测试失败: ' + res.error, { type: 'error', title: '连接失败' });
        } else {
            await showAppAlert(data.type === 'smtp' ? 'SMTP 发信测试成功' : '当前配置连接测试成功', { type: 'success', title: '连接成功' });
        }
    } catch (e) {
        await showAppAlert('网络异常: ' + e.message, { type: 'error', title: '请求失败' });
    }
}

async function testAccountConnectionInList(id) {
    const acc = accounts.find(a => a.id === id);
    if (!acc) return;
    try {
        const type = acc.type === 'smtp' ? 'smtp' : 'imap';
        const res = await api('POST', type === 'smtp' ? '/api/test-smtp' : '/api/test-imap', acc);
        if (res.error) {
            await showAppAlert('连接测试失败: ' + res.error, { type: 'error', title: '连接失败' });
        } else {
            await showAppAlert(type === 'smtp' ? 'SMTP 发信测试成功' : '账号连接测试成功', { type: 'success', title: '连接成功' });
        }
    } catch (e) {
        await showAppAlert('网络异常: ' + e.message, { type: 'error', title: '请求失败' });
    }
}

async function deleteAccount(id) {
    if (!(await showAppConfirm('确定要删除此邮箱账号吗？', { title: '删除邮箱账号', confirmText: '删除' }))) return;
    try {
        await api('DELETE', `/api/accounts/${id}`);
        loadAccounts();
        loadStats();
    } catch (e) {
        await showAppAlert('删除失败: ' + e.message, { type: 'error', title: '删除失败' });
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
                    暂无目标渠道，点击上方按钮添加
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
        custom: '自定义 Webhook',
        email: '邮件收件人'
    };

    tbody.innerHTML = webhooks.map(wh => {
        let urlDisplay;
        if (wh.type === 'email') {
            const emails = wh.url.split(',').map(s => s.trim()).filter(s => s);
            urlDisplay = emails.map(e => `<span class="tag tag-neutral">${escapeHtml(e)}</span>`).join(' ');
            if (emails.length > 3) {
                urlDisplay = emails.slice(0, 3).map(e => `<span class="tag tag-neutral">${escapeHtml(e)}</span>`).join(' ') + ` <span class="tag tag-neutral">+${emails.length - 3}</span>`;
            }
        } else {
            urlDisplay = escapeHtml(wh.url.substring(0, 40)) + (wh.url.length > 40 ? '...' : '');
        }
        return `
        <tr>
            <td><strong>${escapeHtml(wh.name)}</strong></td>
            <td><span class="tag tag-info">${typeNames[wh.type] || wh.type}</span></td>
            <td style="max-width: 240px; overflow: hidden; text-overflow: ellipsis;">
                ${urlDisplay}
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
    `}).join('');
}

function openWebhookModal(data = null) {
    document.getElementById('webhook-modal-title').textContent = data ? '编辑目标渠道' : '添加目标渠道';
    document.getElementById('webhook-id').value = data?.id || '';
    document.getElementById('webhook-name').value = data?.name || '';
    document.getElementById('webhook-type').value = data?.type || 'feishu';
    document.getElementById('webhook-url').value = (data?.type !== 'email' ? data?.url : '') || '';
    document.getElementById('webhook-enabled').checked = data?.enabled !== false;

    // Populate email recipients textarea
    const recipientsEl = document.getElementById('webhook-email-recipients');
    if (data?.type === 'email' && data?.url) {
        // url stores comma-separated emails
        recipientsEl.value = data.url.split(',').map(s => s.trim()).filter(s => s).join('\n');
    } else {
        recipientsEl.value = '';
    }

    // Populate SMTP account selector
    populateSmtpAccountSelector(data?.smtp_account_id || '');

    updateWebhookTargetFields();
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
        // Silently fail, selector will just show "自动选择"
    }
}

function updateWebhookTargetFields() {
    const type = document.getElementById('webhook-type')?.value || 'feishu';
    const isEmail = type === 'email';

    // Toggle URL field vs email fields
    document.querySelectorAll('.webhook-url-field').forEach(el => {
        el.style.display = isEmail ? 'none' : '';
    });
    document.querySelectorAll('.webhook-email-field').forEach(el => {
        el.style.display = isEmail ? '' : 'none';
    });
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
        // Gather emails from textarea, join with comma
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
        url = document.getElementById('webhook-url').value;
    }

    const data = {
        id: id || undefined,
        name: document.getElementById('webhook-name').value,
        type,
        url,
        enabled: document.getElementById('webhook-enabled').checked,
        smtp_account_id: type === 'email' ? (document.getElementById('webhook-smtp-account')?.value || '') : ''
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
            <td>${displayFilterType(rule.type)}</td>
            <td><span class="tag ${rule.mode === 'blacklist' ? 'tag-warning' : 'tag-info'}">${rule.mode === 'blacklist' ? '黑名单' : '白名单'}</span></td>
            <td>${rule.patterns?.length ? rule.patterns.slice(0, 4).map(p => `<span class="tag tag-neutral">${escapeHtml(p)}</span>`).join(' ') : '无'}${rule.patterns?.length > 4 ? ' ...' : ''}</td>
            <td>
                <span class="tag ${rule.enabled ? 'tag-success' : 'tag-neutral'}">
                    ${rule.enabled ? '已启用' : '已禁用'}
                </span>
            </td>
            <td>
                <button class="btn btn-secondary btn-sm" onclick="editFilter('${rule.id}')">编辑</button>
                ${defaultSenderFilterIDs.has(rule.id) ? '' : `<button class="btn btn-danger btn-sm" onclick="deleteFilter('${rule.id}')">删除</button>`}
            </td>
        </tr>
    `).join('');
}

function renderFilterTemplates() {
    const container = document.getElementById('filter-template-list');
    if (!container) return;

    container.innerHTML = filterTemplates.map((template, index) => {
        const typeName = displayFilterType(template.type);
        const modeName = template.mode === 'blacklist' ? '黑名单' : '白名单';
        return `
            <button class="template-card" type="button" onclick="applyFilterTemplate(${index})">
                <span class="template-title">${escapeHtml(template.name)}</span>
                <span class="template-meta">${typeName} / ${modeName} / ${template.patterns.length} 条</span>
            </button>
        `;
    }).join('');
}

function displayFilterType(type) {
    switch (type) {
        case 'content':
            return '标题与内容';
        case 'source':
            return '来源';
        case 'all':
            return '全部字段';
        default:
            return '发送人';
    }
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
    currentFilterDefaultSender = defaultSenderFilterIDs.has(data?.id || '');
    renderFilterTemplates();
    document.getElementById('filter-modal-title').textContent = currentFilterDefaultSender ? (data?.name || '默认发送人名单') : (data ? '编辑过滤规则' : '添加过滤规则');
    document.getElementById('filter-id').value = data?.id || '';
    document.getElementById('filter-name').value = data?.name || '';
    document.getElementById('filter-type').value = data?.type || 'sender';
    document.getElementById('filter-mode').value = data?.mode || 'blacklist';
    document.getElementById('filter-patterns').value = (data?.patterns || []).join('\n');
    document.getElementById('filter-enabled').checked = data?.enabled !== false;
    setDefaultSenderFilterView(currentFilterDefaultSender);
    document.getElementById('filter-modal').classList.add('active');
}

function setDefaultSenderFilterView(isDefaultSender) {
    document.querySelectorAll('[data-filter-advanced]').forEach(element => {
        element.style.display = isDefaultSender ? 'none' : '';
    });
    const patternsLabel = document.getElementById('filter-patterns-label');
    const patternsInput = document.getElementById('filter-patterns');
    if (patternsLabel) {
        patternsLabel.textContent = isDefaultSender ? '发送人匹配内容 (每行一条，可清空)' : '匹配内容 (每行一条，支持邮箱、域名、项目名、关键词)';
    }
    if (patternsInput) {
        patternsInput.rows = isDefaultSender ? 9 : 5;
    }
    ['filter-name', 'filter-type', 'filter-mode', 'filter-patterns', 'filter-enabled'].forEach(id => {
        const element = document.getElementById(id);
        if (element) element.disabled = false;
    });
    const saveButton = document.getElementById('filter-save-btn');
    if (saveButton) saveButton.disabled = false;
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
        await showAppAlert('保存失败: ' + e.message, { type: 'error', title: '保存失败' });
    }
}

async function deleteFilter(id) {
    if (!(await showAppConfirm('确定要删除此过滤规则吗？相关转发规则会自动移除该引用。', { title: '删除过滤规则', confirmText: '删除' }))) return;
    try {
        await api('DELETE', `/api/filters/${id}`);
        loadFilters();
        loadRules();
    } catch (e) {
        await showAppAlert('删除失败: ' + e.message, { type: 'error', title: '删除失败' });
    }
}

async function addSenderToDefaultFilter(sender, mode) {
    sender = (sender || '').trim();
    if (!sender) {
        await showAppAlert('发送人为空，无法加入过滤规则', { type: 'warning', title: '无法操作' });
        return;
    }

    const modeName = mode === 'blacklist' ? '黑名单' : '白名单';
    if (!(await showAppConfirm(`确定将 ${sender} 加入默认发送人${modeName}吗？`, { title: `加入${modeName}`, confirmText: '加入' }))) return;

    try {
        const result = await api('POST', '/api/filters/default-senders', { mode, sender });
        if (result.error) {
            await showAppAlert('操作失败: ' + result.error, { type: 'error', title: '操作失败' });
            return;
        }
        await showAppAlert(result.added ? `已加入默认发送人${modeName}` : `该发送人已在默认发送人${modeName}中`, { type: 'success', title: '操作完成' });
        loadFilters();
    } catch (e) {
        await showAppAlert('操作失败: ' + e.message, { type: 'error', title: '操作失败' });
    }
}

// ===================== History =====================
async function loadHistory() {
    try {
        const status = document.getElementById('history-filter').value;
        const limit = document.getElementById('history-limit')?.value || '50';
        const messages = await api('GET', `/api/messages?status=${encodeURIComponent(status)}&limit=${encodeURIComponent(limit)}`);
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
        failed: '<span class="tag tag-error">发送失败</span>',
        filtered: '<span class="tag tag-neutral">已过滤</span>',
        no_rule: '<span class="tag tag-neutral">无规则</span>'
    };

    container.innerHTML = messages.map(msg => `
        <div class="history-item">
            <div class="history-header">
                <span class="history-subject">${escapeHtml(msg.subject || '(无主题)')}</span>
                <span class="history-actions">
                    ${statusTags[msg.status] || ''}
                    <button class="btn btn-secondary btn-sm" onclick='addSenderToDefaultFilter(${JSON.stringify(msg.from || '')}, "blacklist")'>加入黑名单</button>
                    <button class="btn btn-secondary btn-sm" onclick='addSenderToDefaultFilter(${JSON.stringify(msg.from || '')}, "whitelist")'>加入白名单</button>
                </span>
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

async function clearHistory() {
    try {
        const status = document.getElementById('history-filter').value;
        const result = await api('DELETE', `/api/messages?status=${encodeURIComponent(status)}`);
        await loadHistory();
        loadStats();
        const scope = status ? '当前状态的历史记录' : '全部历史记录';
        await showAppAlert(`${scope}已清空，共删除 ${result.deleted || 0} 条`, { type: 'success', title: '已清空' });
    } catch (e) {
        await showAppAlert('清空历史记录失败: ' + e.message, { type: 'error', title: '清空失败' });
    }
}

// ===================== Utilities =====================
function showAppDialog(options = {}) {
    const dialog = document.getElementById('app-dialog');
    const icon = document.getElementById('app-dialog-icon');
    const title = document.getElementById('app-dialog-title');
    const message = document.getElementById('app-dialog-message');
    const cancelBtn = document.getElementById('app-dialog-cancel');
    const confirmBtn = document.getElementById('app-dialog-confirm');
    const type = options.type || 'info';
    const iconMap = {
        info: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>',
        success: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="20 6 9 17 4 12"/></svg>',
        error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>',
        warning: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>'
    };

    title.textContent = options.title || '提示';
    message.textContent = options.message || '';
    icon.className = `app-dialog-icon ${type}`;
    icon.innerHTML = iconMap[type] || iconMap.info;
    confirmBtn.textContent = options.confirmText || '确定';
    cancelBtn.textContent = options.cancelText || '取消';
    cancelBtn.style.display = options.showCancel ? 'inline-flex' : 'none';
    dialog.classList.add('active');
    dialog.setAttribute('aria-hidden', 'false');

    return new Promise(resolve => {
        const cleanup = result => {
            confirmBtn.removeEventListener('click', onConfirm);
            cancelBtn.removeEventListener('click', onCancel);
            dialog.removeEventListener('click', onOverlay);
            document.removeEventListener('keydown', onKeydown);
            dialog.classList.remove('active');
            dialog.setAttribute('aria-hidden', 'true');
            resolve(result);
        };
        const onConfirm = () => cleanup(true);
        const onCancel = () => cleanup(false);
        const onOverlay = event => {
            if (event.target === dialog) cleanup(false);
        };
        const onKeydown = event => {
            if (event.key === 'Escape') cleanup(false);
        };

        confirmBtn.addEventListener('click', onConfirm);
        cancelBtn.addEventListener('click', onCancel);
        dialog.addEventListener('click', onOverlay);
        document.addEventListener('keydown', onKeydown);
        setTimeout(() => confirmBtn.focus(), 0);
    });
}

function showAppAlert(message, options = {}) {
    showAppToast(message, options);
    return Promise.resolve(true);
}

function showAppToast(message, options = {}) {
    const container = document.getElementById('toast-container');
    if (!container) return;

    const type = options.type || 'info';
    const title = options.title || '提示';
    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.setAttribute('role', type === 'error' ? 'alert' : 'status');
    toast.innerHTML = `
        <div class="toast-indicator" aria-hidden="true"></div>
        <div class="toast-copy">
            <strong>${escapeHtml(title)}</strong>
            <span>${escapeHtml(message)}</span>
        </div>
        <button class="toast-close" type="button" aria-label="关闭通知">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
                stroke-linecap="round" stroke-linejoin="round">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
            </svg>
        </button>
    `;

    const removeToast = () => {
        toast.classList.add('closing');
        setTimeout(() => toast.remove(), 180);
    };
    toast.querySelector('.toast-close')?.addEventListener('click', removeToast);
    container.appendChild(toast);
    setTimeout(removeToast, options.duration || (type === 'error' ? 5200 : 2600));
}

async function copyTextToClipboard(text) {
    if (!text) return false;
    try {
        if (navigator.clipboard?.writeText) {
            await navigator.clipboard.writeText(text);
            return true;
        }
    } catch (e) {
        // 回退到 textarea 复制。
    }

    const textarea = document.createElement('textarea');
    textarea.value = text;
    textarea.setAttribute('readonly', '');
    textarea.style.position = 'fixed';
    textarea.style.left = '-9999px';
    textarea.style.top = '0';
    document.body.appendChild(textarea);
    textarea.select();
    let copied = false;
    try {
        copied = document.execCommand('copy');
    } finally {
        textarea.remove();
    }
    return copied;
}

function showAppConfirm(message, options = {}) {
    return showAppDialog({
        title: options.title || '确认操作',
        message,
        type: options.type || 'warning',
        confirmText: options.confirmText || '确定',
        cancelText: options.cancelText || '取消',
        showCancel: true
    });
}

function closeModal(id) {
    document.getElementById(id).classList.remove('active');
    document.getElementById('rule-source-dropdown')?.classList.remove('open');
    document.getElementById('rule-target-dropdown')?.classList.remove('open');
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
    document.getElementById('rule-source-dropdown')?.classList.remove('open');
    document.getElementById('rule-filter-dropdown')?.classList.remove('open');
    document.getElementById('rule-target-dropdown')?.classList.remove('open');
});

document.getElementById('rule-source-dropdown')?.addEventListener('click', event => {
    event.stopPropagation();
});

document.getElementById('rule-filter-dropdown')?.addEventListener('click', event => {
    event.stopPropagation();
});

document.getElementById('rule-target-dropdown')?.addEventListener('click', event => {
    event.stopPropagation();
});
