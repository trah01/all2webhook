// ===================== Rules =====================
let inboundProjects = [];

async function loadInboundProjectsForRules() {
    try {
        inboundProjects = await api('GET', '/api/projects');
        if (!Array.isArray(inboundProjects)) inboundProjects = [];
    } catch (e) {
        console.error('Failed to load inbound projects:', e);
        inboundProjects = [];
    }
}

async function loadRules() {
    try {
        if (!accounts.length) {
            await loadAccounts();
        }
        if (!webhooks.length) {
            await loadWebhooks();
        }
        if (!filterRules.length) {
            filterRules = await api('GET', '/api/filters');
            if (!filterRules || !Array.isArray(filterRules)) filterRules = [];
        }
        await loadInboundProjectsForRules();
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
        const sourceNames = normalizeRuleSources(rule).map(displayRuleSource);
        const targetIDs = normalizeRuleTargets(rule);
        const targetNames = targetIDs
            .map(displayRuleTarget)
            .filter(Boolean);
        const selectedFilters = (rule.filter_rule_ids || [])
            .map(id => filterRules.find(f => f.id === id))
            .filter(Boolean);
        return `
            <tr>
                <td><strong>${escapeHtml(rule.name)}</strong></td>
                <td>${sourceNames.map(name => `<span class="tag tag-info">${escapeHtml(name)}</span>`).join(' ')}</td>
                <td>${targetNames.length ? targetNames.map(name => `<span class="tag tag-info">${escapeHtml(name)}</span>`).join(' ') : '未知'}</td>
                <td>
                    ${selectedFilters.length ? selectedFilters.map(f => `<span class="tag tag-info">${escapeHtml(f.name)}</span>`).join(' ') : '无'}
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

function displayRuleSource(sourceID) {
    if (sourceID === 'all') return '所有来源';
    const acc = accounts.find(a => a.id === sourceID);
    if (acc) return acc.name || acc.email_user || '邮箱账号';
    const project = inboundProjects.find(p => p.id === sourceID);
    if (project) return project.name || '接收项目';
    return '未知来源';
}

function normalizeRuleTargets(rule) {
    const result = [];
    const seen = new Set();
    [rule.target_webhook, ...(rule.target_webhooks || [])].forEach(id => {
        if (!id || seen.has(id)) return;
        seen.add(id);
        result.push(id);
    });
    return result;
}

function normalizeRuleSources(rule) {
    const result = [];
    const seen = new Set();
    [rule.source_account, ...(rule.source_accounts || [])].forEach(id => {
        if (!id || seen.has(id)) return;
        seen.add(id);
        result.push(id);
    });
    if (result.length === 0 || result.includes('all')) {
        return ['all'];
    }
    return result;
}

function smtpTargetID(accountID) {
    return `smtp:${accountID}`;
}

function getSMTPNotificationAccounts() {
    return accounts.filter(account => account.type === 'smtp' && account.enabled);
}

function displayRuleTarget(targetID) {
    if (targetID?.startsWith('smtp:')) {
        const accountID = targetID.slice(5);
        const account = accounts.find(item => item.id === accountID);
        return account ? (account.name || account.email_user || 'SMTP 发信账号') : '';
    }
    return webhooks.find(webhook => webhook.id === targetID)?.name || '';
}

function displayWebhookType(type) {
    const names = {
        feishu: '飞书',
        dingtalk: '钉钉',
        wecom: '企业微信',
        slack: 'Slack',
        discord: 'Discord',
        custom: '自定义',
        email: '邮件通知'
    };
    return names[type] || type || '未知类型';
}

async function openRuleModal(data = null) {
    if (!inboundProjects.length) {
        await loadInboundProjectsForRules();
    }

    document.getElementById('rule-modal-title').textContent = data ? '编辑转发规则' : '添加转发规则';
    document.getElementById('rule-id').value = data?.id || '';
    document.getElementById('rule-name').value = data?.name || '';
    renderRuleSourceDropdown(normalizeRuleSources(data || { source_account: 'all' }));
    renderRuleTargetDropdown(normalizeRuleTargets(data || {}));
    renderRuleFilterDropdown(data?.filter_rule_ids || []);
    document.getElementById('rule-enabled').checked = data?.enabled !== false;
    document.getElementById('rule-modal').classList.add('active');
}

function renderRuleSourceDropdown(selectedIDs = ['all']) {
    const menu = document.getElementById('rule-source-menu');
    if (!menu) return;
    const selectedSet = new Set(selectedIDs.length ? selectedIDs : ['all']);
    const imapAccounts = accounts.filter(account => (account.type || 'imap') === 'imap');
    const accountOptions = imapAccounts.map(account => `
        <label class="filter-option">
            <input type="checkbox" class="rule-source-checkbox" value="${escapeHtml(account.id)}"
                ${selectedSet.has(account.id) ? 'checked' : ''} onchange="updateRuleSourceSummary(this)">
            <span class="filter-option-main">
                <span class="filter-option-title">${escapeHtml(account.name || account.email_user || '邮箱账号')}</span>
                <span class="filter-option-meta">IMAP 收件 / ${account.enabled ? '已启用' : '已禁用'}</span>
            </span>
        </label>
    `).join('');
    const projectOptions = inboundProjects.map(project => `
        <label class="filter-option">
            <input type="checkbox" class="rule-source-checkbox" value="${escapeHtml(project.id)}"
                ${selectedSet.has(project.id) ? 'checked' : ''} onchange="updateRuleSourceSummary(this)">
            <span class="filter-option-main">
                <span class="filter-option-title">${escapeHtml(project.name || '接收项目')}</span>
                <span class="filter-option-meta">接收项目 / ${project.enabled ? '已启用' : '已禁用'}</span>
            </span>
        </label>
    `).join('');

    menu.innerHTML = `
        <label class="filter-option">
            <input type="checkbox" class="rule-source-checkbox" value="all"
                ${selectedSet.has('all') ? 'checked' : ''} onchange="updateRuleSourceSummary(this)">
            <span class="filter-option-main">
                <span class="filter-option-title">所有来源</span>
                <span class="filter-option-meta">匹配所有 IMAP 收件账号和接收项目</span>
            </span>
        </label>
        ${accountOptions}
        ${projectOptions}
    `;
    updateRuleSourceSummary();
}

function toggleRuleSourceDropdown(event) {
    event.stopPropagation();
    document.getElementById('rule-source-dropdown').classList.toggle('open');
}

function getSelectedRuleSourceIDs() {
    return Array.from(document.querySelectorAll('.rule-source-checkbox:checked'))
        .map(input => input.value);
}

function updateRuleSourceSummary(changedInput = null) {
    const allInput = document.querySelector('.rule-source-checkbox[value="all"]');
    if (changedInput?.value === 'all' && changedInput.checked) {
        document.querySelectorAll('.rule-source-checkbox').forEach(input => {
            if (input.value !== 'all') input.checked = false;
        });
    } else if (changedInput?.value !== 'all' && changedInput?.checked && allInput) {
        allInput.checked = false;
    }
    const specificSelected = Array.from(document.querySelectorAll('.rule-source-checkbox:checked'))
        .filter(input => input.value !== 'all');
    if (specificSelected.length === 0 && allInput) {
        allInput.checked = true;
    }

    const summary = document.getElementById('rule-source-summary');
    if (!summary) return;
    const selectedIDs = getSelectedRuleSourceIDs();
    if (selectedIDs.includes('all')) {
        summary.textContent = '所有来源';
        return;
    }
    const names = selectedIDs.map(displayRuleSource).filter(Boolean);
    summary.textContent = names.length <= 2 ? names.join('、') : `已选择 ${names.length} 个来源`;
}

function renderRuleTargetDropdown(selectedIDs = []) {
    const menu = document.getElementById('rule-target-menu');
    if (!menu) return;
    const selectedSet = new Set(selectedIDs);
    const smtpAccounts = getSMTPNotificationAccounts();
    if (!webhooks.length && !smtpAccounts.length) {
        menu.innerHTML = '<div class="filter-empty">暂无通知渠道，请先添加 Webhook 或 SMTP 发信账号</div>';
        updateRuleTargetSummary();
        return;
    }
    const webhookOptions = webhooks.map(webhook => `
        <label class="filter-option">
            <input type="checkbox" class="rule-target-checkbox" value="${escapeHtml(webhook.id)}"
                ${selectedSet.has(webhook.id) ? 'checked' : ''} onchange="updateRuleTargetSummary()">
            <span class="filter-option-main">
                <span class="filter-option-title">${escapeHtml(webhook.name)}</span>
                <span class="filter-option-meta">${displayWebhookType(webhook.type)} / ${webhook.enabled ? '已启用' : '已禁用'}</span>
            </span>
        </label>
    `).join('');
    const smtpOptions = smtpAccounts.map(account => {
        const targetID = smtpTargetID(account.id);
        return `
            <label class="filter-option">
                <input type="checkbox" class="rule-target-checkbox" value="${escapeHtml(targetID)}"
                    ${selectedSet.has(targetID) ? 'checked' : ''} onchange="updateRuleTargetSummary()">
                <span class="filter-option-main">
                    <span class="filter-option-title">${escapeHtml(account.name || account.email_user || 'SMTP 发信账号')}</span>
                    <span class="filter-option-meta">SMTP 发信 / 发送到 ${escapeHtml(account.email_user || '-')}</span>
                </span>
            </label>
        `;
    }).join('');
    menu.innerHTML = webhookOptions + smtpOptions;
    updateRuleTargetSummary();
}

function toggleRuleTargetDropdown(event) {
    event.stopPropagation();
    document.getElementById('rule-target-dropdown').classList.toggle('open');
}

function getSelectedRuleTargetIDs() {
    return Array.from(document.querySelectorAll('.rule-target-checkbox:checked'))
        .map(input => input.value);
}

function updateRuleTargetSummary() {
    const summary = document.getElementById('rule-target-summary');
    if (!summary) return;

    const selectedIDs = getSelectedRuleTargetIDs();
    if (selectedIDs.length === 0) {
        summary.textContent = '请选择目标渠道';
        return;
    }

    const names = selectedIDs
        .map(displayRuleTarget)
        .filter(Boolean);
    summary.textContent = names.length <= 2 ? names.join('、') : `已选择 ${names.length} 个目标渠道`;
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
        const typeName = rule.type === 'content' ? '通知内容' : '来源';
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
    const targetIDs = getSelectedRuleTargetIDs();
    const sourceIDs = getSelectedRuleSourceIDs();
    const data = {
        id: id || undefined,
        name: document.getElementById('rule-name').value,
        source_account: sourceIDs[0] || 'all',
        source_accounts: sourceIDs,
        target_webhook: targetIDs[0] || '',
        target_webhooks: targetIDs,
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
        await showAppAlert('保存失败: ' + e.message, { type: 'error', title: '保存失败' });
    }
}

async function deleteRule(id) {
    if (!(await showAppConfirm('确定要删除此转发规则吗？', { title: '删除转发规则', confirmText: '删除' }))) return;
    try {
        await api('DELETE', `/api/rules/${id}`);
        loadRules();
    } catch (e) {
        await showAppAlert('删除失败: ' + e.message, { type: 'error', title: '删除失败' });
    }
}
