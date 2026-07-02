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
        const sourceName = displayRuleSource(rule.source_account);
        const targetIDs = normalizeRuleTargets(rule);
        const targetNames = targetIDs
            .map(id => webhooks.find(w => w.id === id)?.name)
            .filter(Boolean);
        const selectedFilters = (rule.filter_rule_ids || [])
            .map(id => filterRules.find(f => f.id === id))
            .filter(Boolean);
        return `
            <tr>
                <td><strong>${escapeHtml(rule.name)}</strong></td>
                <td>${escapeHtml(sourceName)}</td>
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

function displayWebhookType(type) {
    const names = {
        feishu: '飞书',
        dingtalk: '钉钉',
        wecom: '企业微信',
        slack: 'Slack',
        discord: 'Discord',
        custom: '自定义'
    };
    return names[type] || type || '未知类型';
}

async function openRuleModal(data = null) {
    if (!inboundProjects.length) {
        await loadInboundProjectsForRules();
    }

    const sourceSelect = document.getElementById('rule-source');
    sourceSelect.innerHTML = `
        <option value="all">所有来源</option>
        <optgroup label="邮箱账号">
            ${accounts.map(a => `<option value="${a.id}">${escapeHtml(a.name)}</option>`).join('')}
        </optgroup>
        <optgroup label="接收项目">
            ${inboundProjects.map(p => `<option value="${p.id}">${escapeHtml(p.name)}</option>`).join('')}
        </optgroup>
    `;

    document.getElementById('rule-modal-title').textContent = data ? '编辑转发规则' : '添加转发规则';
    document.getElementById('rule-id').value = data?.id || '';
    document.getElementById('rule-name').value = data?.name || '';
    document.getElementById('rule-source').value = data?.source_account || 'all';
    renderRuleTargetDropdown(normalizeRuleTargets(data || {}));
    renderRuleFilterDropdown(data?.filter_rule_ids || []);
    document.getElementById('rule-enabled').checked = data?.enabled !== false;
    document.getElementById('rule-modal').classList.add('active');
}

function renderRuleTargetDropdown(selectedIDs = []) {
    const menu = document.getElementById('rule-target-menu');
    if (!menu) return;
    const selectedSet = new Set(selectedIDs);
    if (!webhooks.length) {
        menu.innerHTML = '<div class="filter-empty">暂无 Webhook 目标，请先添加</div>';
        updateRuleTargetSummary();
        return;
    }
    menu.innerHTML = webhooks.map(webhook => `
        <label class="filter-option">
            <input type="checkbox" class="rule-target-checkbox" value="${escapeHtml(webhook.id)}"
                ${selectedSet.has(webhook.id) ? 'checked' : ''} onchange="updateRuleTargetSummary()">
            <span class="filter-option-main">
                <span class="filter-option-title">${escapeHtml(webhook.name)}</span>
                <span class="filter-option-meta">${displayWebhookType(webhook.type)} / ${webhook.enabled ? '已启用' : '已禁用'}</span>
            </span>
        </label>
    `).join('');
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
        .map(id => webhooks.find(webhook => webhook.id === id)?.name)
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
    const data = {
        id: id || undefined,
        name: document.getElementById('rule-name').value,
        source_account: document.getElementById('rule-source').value,
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
