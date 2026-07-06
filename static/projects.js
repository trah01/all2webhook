// ===================== Inbound Projects =====================
let projects = [];

async function loadProjects() {
    try {
        projects = await api('GET', '/api/projects');
        if (!Array.isArray(projects)) projects = [];
        renderProjects();
    } catch (e) {
        console.error('Failed to load projects:', e);
        projects = [];
        renderProjects();
    }
}

function renderProjects() {
    const tbody = document.getElementById('projects-table');
    if (!tbody) return;
    if (!projects.length) {
        tbody.innerHTML = `
            <tr>
                <td colspan="5" style="text-align: center; padding: 40px; color: var(--text-secondary);">
                    暂无接收入口，点击上方按钮添加
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = projects.map(project => {
        const projectURL = escapeHtml(project.url || '');
        return `
        <tr>
            <td>
                <strong>${escapeHtml(project.name)}</strong>
                <div class="muted-line">${escapeHtml(project.id)}</div>
            </td>
            <td>
                <code class="inline-code" title="${projectURL}">${projectURL}</code>
            </td>
            <td>
                <span class="tag ${project.enabled ? 'tag-success' : 'tag-neutral'}">
                    ${project.enabled ? '已启用' : '已禁用'}
                </span>
            </td>
            <td>${formatDate(project.created_at)}</td>
            <td>
                <button class="btn btn-secondary btn-sm" onclick="copyProjectURL('${project.id}')">复制 URL</button>
                <button class="btn btn-secondary btn-sm" onclick="openProjectModal('${project.id}')">编辑</button>
                <button class="btn btn-secondary btn-sm" onclick="rotateProjectSecret('${project.id}')">轮换密钥</button>
                <button class="btn btn-danger btn-sm" onclick="deleteProject('${project.id}')">删除</button>
            </td>
        </tr>
    `;
    }).join('');
}

function openProjectModal(id = '') {
    const project = projects.find(item => item.id === id);
    document.getElementById('project-modal-title').textContent = project ? '编辑接收入口' : '添加接收入口';
    document.getElementById('project-id').value = project?.id || '';
    document.getElementById('project-name').value = project?.name || '';
    document.getElementById('project-enabled').checked = project?.enabled !== false;
    document.getElementById('project-modal').classList.add('active');
}

async function saveProject() {
    const id = document.getElementById('project-id').value;
    const data = {
        name: document.getElementById('project-name').value,
        enabled: document.getElementById('project-enabled').checked
    };
    try {
        if (id) {
            await api('PUT', `/api/projects/${id}`, data);
        } else {
            await api('POST', '/api/projects', data);
        }
        closeModal('project-modal');
        await loadProjects();
        if (typeof loadRules === 'function') loadRules();
    } catch (e) {
        await showAppAlert('保存失败: ' + e.message, { type: 'error', title: '保存失败' });
    }
}

async function rotateProjectSecret(id) {
    if (!(await showAppConfirm('轮换后旧接收 URL 会立即失效，确定继续吗？', { title: '轮换密钥', confirmText: '轮换' }))) return;
    try {
        const result = await api('POST', `/api/projects/${id}/rotate`);
        if (result.error) {
            await showAppAlert(result.error, { type: 'error', title: '轮换失败' });
            return;
        }
        await loadProjects();
        await showAppAlert('新的接收 URL 已生成', { type: 'success', title: '轮换完成' });
    } catch (e) {
        await showAppAlert('轮换失败: ' + e.message, { type: 'error', title: '轮换失败' });
    }
}

async function deleteProject(id) {
    if (!(await showAppConfirm('确定删除此接收入口吗？相关接收 URL 将不可用。', { title: '删除接收入口', confirmText: '删除' }))) return;
    try {
        await api('DELETE', `/api/projects/${id}`);
        await loadProjects();
        if (typeof loadRules === 'function') loadRules();
    } catch (e) {
        await showAppAlert('删除失败: ' + e.message, { type: 'error', title: '删除失败' });
    }
}

async function copyProjectURL(id) {
    const project = projects.find(item => item.id === id);
    if (!project?.url) return;
    const copied = await copyTextToClipboard(project.url);
    await showAppAlert(copied ? '接收 URL 已复制' : '复制失败，请手动复制表格中的 URL', {
        type: copied ? 'success' : 'error',
        title: copied ? '已复制' : '复制失败'
    });
}
