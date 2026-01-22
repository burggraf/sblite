# Storage Policies Dashboard Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Storage Policies section within Settings → Storage to manage RLS policies for storage_objects, organized by bucket with policy templates.

**Architecture:** Extend the existing `storageSettings` state with a `policies` sub-object. Reuse existing policy APIs (`/_/api/policies`) and bucket APIs (`/_/api/storage/buckets`). Render a two-panel layout (bucket list + policy list) within the Storage settings section.

**Tech Stack:** Vanilla JavaScript (dashboard app.js), CSS (dashboard style.css)

---

### Task 1: Add Storage Policies State

**Files:**
- Modify: `internal/dashboard/static/app.js:62-79` (storageSettings state)

**Step 1: Add policies sub-state to storageSettings**

Find the `storageSettings` object in the state (around line 62) and add the policies sub-state:

```javascript
storageSettings: {
    backend: 'local',
    localPath: './storage',
    s3: {
        endpoint: '',
        region: '',
        bucket: '',
        accessKey: '',
        secretKey: '',
        pathStyle: false
    },
    loading: false,
    testing: false,
    testResult: null,
    saving: false,
    dirty: false,
    originalBackend: 'local',
    // ADD THIS:
    policies: {
        loading: false,
        list: [],
        buckets: [],
        selectedBucket: null,
        showModal: false,
        modalStep: 'template', // 'template' or 'form'
        editingPolicy: null,
        template: null,
        error: null
    }
},
```

**Step 2: Verify change**

Open the dashboard in browser, check browser console for errors. No errors expected.

**Step 3: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add storage policies state structure"
```

---

### Task 2: Add Load Storage Policies Function

**Files:**
- Modify: `internal/dashboard/static/app.js` (after loadStorageSettings function, around line 2948)

**Step 1: Add loadStoragePolicies function**

Add this function after `loadStorageSettings()`:

```javascript
async loadStoragePolicies() {
    const sp = this.state.settings.storageSettings.policies;
    sp.loading = true;
    this.render();

    try {
        // Load buckets
        const bucketsRes = await fetch('/_/api/storage/buckets');
        if (bucketsRes.ok) {
            sp.buckets = await bucketsRes.json();
        }

        // Load all storage_objects policies
        const policiesRes = await fetch('/_/api/policies?table=storage_objects');
        if (policiesRes.ok) {
            const data = await policiesRes.json();
            sp.list = data.policies || [];
        }

        // Auto-select first bucket if none selected
        if (!sp.selectedBucket && sp.buckets.length > 0) {
            sp.selectedBucket = sp.buckets[0].id;
        }
    } catch (e) {
        console.error('Failed to load storage policies:', e);
        sp.error = 'Failed to load storage policies';
    }

    sp.loading = false;
    this.render();
},

selectStorageBucket(bucketId) {
    this.state.settings.storageSettings.policies.selectedBucket = bucketId;
    this.render();
},

getStoragePoliciesForBucket(bucketId) {
    const { list } = this.state.settings.storageSettings.policies;
    if (bucketId === '__all__') {
        return list;
    }
    // Filter policies that reference this bucket
    return list.filter(p => {
        const expr = (p.using_expr || '') + (p.check_expr || '');
        return expr.includes(`'${bucketId}'`) || expr.includes(`"${bucketId}"`);
    });
},

getStoragePolicyCountForBucket(bucketId) {
    return this.getStoragePoliciesForBucket(bucketId).length;
},
```

**Step 2: Update toggleSettingsSection to load policies when storage expands**

Find `toggleSettingsSection` (around line 2907) and add the policies load:

```javascript
toggleSettingsSection(section) {
    this.state.settings.expandedSections[section] = !this.state.settings.expandedSections[section];
    // Load storage settings when section is expanded
    if (section === 'storage' && this.state.settings.expandedSections.storage) {
        this.loadStorageSettings();
        this.loadStoragePolicies();  // ADD THIS LINE
    }
    // ... rest of function
},
```

**Step 3: Verify**

Open dashboard → Settings → Storage. Check Network tab shows requests to `/api/storage/buckets` and `/api/policies?table=storage_objects`.

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add storage policies data loading"
```

---

### Task 3: Render Storage Policies Section

**Files:**
- Modify: `internal/dashboard/static/app.js` (renderStorageSettingsSection, around line 3786)

**Step 1: Add renderStoragePoliciesSection function**

Add this function after `renderStorageSettingsSection`:

```javascript
renderStoragePoliciesSection() {
    const sp = this.state.settings.storageSettings.policies;

    if (sp.loading) {
        return '<div class="loading">Loading storage policies...</div>';
    }

    const selectedPolicies = this.getStoragePoliciesForBucket(sp.selectedBucket || '__all__');

    return `
        <div class="settings-subsection storage-policies-section">
            <h4>Storage Policies</h4>
            <p class="text-muted">Manage access control for storage objects by bucket.</p>

            <div class="storage-policies-layout">
                <div class="storage-bucket-panel">
                    <div class="panel-header">Buckets</div>
                    <div class="bucket-list">
                        <div class="bucket-list-item ${sp.selectedBucket === '__all__' ? 'active' : ''}"
                             onclick="App.selectStorageBucket('__all__')">
                            <span class="bucket-name">All Buckets</span>
                            <span class="policy-badge">${sp.list.length}</span>
                        </div>
                        ${sp.buckets.map(b => `
                            <div class="bucket-list-item ${sp.selectedBucket === b.id ? 'active' : ''}"
                                 onclick="App.selectStorageBucket('${b.id}')">
                                <span class="bucket-name">${b.id}</span>
                                <span class="policy-badge">${this.getStoragePolicyCountForBucket(b.id)}</span>
                            </div>
                        `).join('')}
                    </div>
                </div>
                <div class="storage-policy-panel">
                    <div class="panel-header">
                        <span>${sp.selectedBucket === '__all__' ? 'All Policies' : sp.selectedBucket}</span>
                        <button class="btn btn-primary btn-sm" onclick="App.showStoragePolicyModal()">+ New Policy</button>
                    </div>
                    <div class="storage-policies-list">
                        ${selectedPolicies.length === 0
                            ? '<div class="empty-state">No policies for this bucket</div>'
                            : selectedPolicies.map(p => this.renderStoragePolicyCard(p)).join('')}
                    </div>
                    ${this.renderStorageHelperReference()}
                </div>
            </div>
        </div>
    `;
},

renderStoragePolicyCard(policy) {
    return `
        <div class="policy-card ${!policy.enabled ? 'disabled' : ''}">
            <div class="policy-header">
                <div class="policy-title">
                    <span class="policy-name">${policy.policy_name}</span>
                    <span class="policy-command">${policy.command}</span>
                </div>
                <div class="policy-actions">
                    <label class="policy-toggle">
                        <input type="checkbox" ${policy.enabled ? 'checked' : ''}
                            onchange="App.toggleStoragePolicyEnabled(${policy.id}, this.checked)">
                        <span class="toggle-label">${policy.enabled ? 'Enabled' : 'Disabled'}</span>
                    </label>
                    <button class="btn-icon" onclick="App.showEditStoragePolicyModal(${policy.id})">Edit</button>
                    <button class="btn-icon" onclick="App.confirmDeleteStoragePolicy(${policy.id}, '${policy.policy_name}')">Delete</button>
                </div>
            </div>
            ${policy.using_expr ? `
                <div class="policy-expr">
                    <span class="expr-label">USING:</span>
                    <code>${this.truncate(policy.using_expr, 80)}</code>
                </div>
            ` : ''}
            ${policy.check_expr ? `
                <div class="policy-expr">
                    <span class="expr-label">CHECK:</span>
                    <code>${this.truncate(policy.check_expr, 80)}</code>
                </div>
            ` : ''}
        </div>
    `;
},

renderStorageHelperReference() {
    return `
        <details class="helper-reference">
            <summary>Helper Functions Reference</summary>
            <div class="helper-list">
                <div class="helper-item">
                    <code>storage.filename(name)</code>
                    <span>Returns filename without path. <em>'uploads/photo.jpg'</em> → <em>'photo.jpg'</em></span>
                </div>
                <div class="helper-item">
                    <code>storage.foldername(name)</code>
                    <span>Returns folder path. <em>'user123/photos/img.png'</em> → <em>'user123/photos'</em></span>
                </div>
                <div class="helper-item">
                    <code>storage.extension(name)</code>
                    <span>Returns file extension. <em>'document.pdf'</em> → <em>'pdf'</em></span>
                </div>
                <div class="helper-item">
                    <code>auth.uid()</code>
                    <span>Current user's ID, or NULL if anonymous.</span>
                </div>
            </div>
        </details>
    `;
},
```

**Step 2: Add call to renderStoragePoliciesSection in renderStorageSettingsSection**

Find the end of the backend settings in `renderStorageSettingsSection` (after the Save/Cancel buttons, around line 3908) and add:

```javascript
                            ${ss.dirty ? `
                                <div class="form-actions">
                                    <button class="btn btn-secondary" onclick="App.cancelStorageSettings()">Cancel</button>
                                    <button class="btn btn-primary" onclick="App.saveStorageSettings()" ${ss.saving ? 'disabled' : ''}>
                                        ${ss.saving ? 'Saving...' : 'Save Changes'}
                                    </button>
                                </div>
                            ` : ''}

                            <hr class="section-divider">

                            ${this.renderStoragePoliciesSection()}
                        `}
```

**Step 3: Verify**

Open dashboard → Settings → Storage. Should see "Storage Policies" section with bucket list and policy list.

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): render storage policies section"
```

---

### Task 4: Add Storage Policies CSS

**Files:**
- Modify: `internal/dashboard/static/style.css` (after Storage Settings Styles section, around line 2500)

**Step 1: Add storage policies styles**

Add after the existing Storage Settings Styles section:

```css
/* Storage Policies */
.storage-policies-section {
    margin-top: 2rem;
    padding-top: 2rem;
}

.section-divider {
    border: none;
    border-top: 1px solid var(--border);
    margin: 2rem 0;
}

.storage-policies-layout {
    display: grid;
    grid-template-columns: 200px 1fr;
    gap: 1rem;
    margin-top: 1rem;
}

.storage-bucket-panel {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
}

.storage-bucket-panel .panel-header {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border);
    font-weight: 600;
    font-size: 0.875rem;
}

.bucket-list {
    max-height: 300px;
    overflow-y: auto;
}

.bucket-list-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.625rem 1rem;
    cursor: pointer;
    transition: background 0.15s;
}

.bucket-list-item:hover {
    background: var(--hover);
}

.bucket-list-item.active {
    background: var(--primary);
    color: white;
}

.bucket-list-item.active .policy-badge {
    background: white;
    color: var(--primary);
}

.bucket-name {
    font-size: 0.875rem;
}

.storage-policy-panel {
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
}

.storage-policy-panel .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border);
    font-weight: 600;
    font-size: 0.875rem;
}

.storage-policies-list {
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    max-height: 400px;
    overflow-y: auto;
}

.storage-policies-list .empty-state {
    padding: 2rem;
    text-align: center;
    color: var(--text-muted);
}

/* Helper Reference */
.helper-reference {
    margin-top: 1rem;
    padding: 0 1rem 1rem;
}

.helper-reference summary {
    cursor: pointer;
    font-size: 0.875rem;
    color: var(--text-muted);
    padding: 0.5rem 0;
}

.helper-reference summary:hover {
    color: var(--text);
}

.helper-list {
    margin-top: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.helper-item {
    display: flex;
    gap: 1rem;
    align-items: baseline;
    font-size: 0.8125rem;
}

.helper-item code {
    background: var(--code-bg);
    padding: 0.125rem 0.375rem;
    border-radius: 4px;
    font-size: 0.75rem;
    white-space: nowrap;
}

.helper-item em {
    color: var(--text-muted);
    font-style: normal;
}
```

**Step 2: Verify**

Open dashboard → Settings → Storage. Policies section should be styled properly with two-panel layout.

**Step 3: Commit**

```bash
git add internal/dashboard/static/style.css
git commit -m "feat(dashboard): add storage policies CSS"
```

---

### Task 5: Add Storage Policy Modal (Template Selection)

**Files:**
- Modify: `internal/dashboard/static/app.js` (add modal functions and rendering)

**Step 1: Add storage policy modal functions**

Add these functions after the storage policies rendering functions:

```javascript
showStoragePolicyModal(editPolicy = null) {
    const sp = this.state.settings.storageSettings.policies;
    const bucket = sp.selectedBucket === '__all__' ? (sp.buckets[0]?.id || '') : sp.selectedBucket;

    sp.showModal = true;
    sp.modalStep = editPolicy ? 'form' : 'template';
    sp.template = null;
    sp.editingPolicy = editPolicy ? { ...editPolicy } : {
        table_name: 'storage_objects',
        policy_name: '',
        command: 'SELECT',
        using_expr: '',
        check_expr: '',
        enabled: true,
        bucket: bucket
    };
    sp.error = null;
    this.render();
},

async showEditStoragePolicyModal(policyId) {
    try {
        const res = await fetch(`/_/api/policies/${policyId}`);
        if (res.ok) {
            const policy = await res.json();
            this.showStoragePolicyModal(policy);
        }
    } catch (e) {
        console.error('Failed to load policy:', e);
    }
},

closeStoragePolicyModal() {
    const sp = this.state.settings.storageSettings.policies;
    sp.showModal = false;
    sp.editingPolicy = null;
    sp.template = null;
    sp.error = null;
    this.render();
},

selectStoragePolicyTemplate(template) {
    const sp = this.state.settings.storageSettings.policies;
    sp.template = template;

    const bucket = sp.editingPolicy.bucket;
    const templates = {
        'public_read': {
            policy_name: `${bucket}_public_read`,
            command: 'SELECT',
            using_expr: `bucket_id = '${bucket}'`,
            check_expr: ''
        },
        'authenticated_read': {
            policy_name: `${bucket}_authenticated_read`,
            command: 'SELECT',
            using_expr: `bucket_id = '${bucket}' AND auth.uid() IS NOT NULL`,
            check_expr: ''
        },
        'authenticated_upload': {
            policy_name: `${bucket}_authenticated_upload`,
            command: 'INSERT',
            using_expr: '',
            check_expr: `bucket_id = '${bucket}' AND auth.uid() IS NOT NULL`
        },
        'owner_only': {
            policy_name: `${bucket}_owner_access`,
            command: 'ALL',
            using_expr: `bucket_id = '${bucket}' AND storage.foldername(name) = auth.uid()`,
            check_expr: `bucket_id = '${bucket}' AND storage.foldername(name) = auth.uid()`
        },
        'custom': {
            policy_name: '',
            command: 'SELECT',
            using_expr: `bucket_id = '${bucket}'`,
            check_expr: ''
        }
    };

    const t = templates[template];
    if (t) {
        sp.editingPolicy = { ...sp.editingPolicy, ...t };
    }

    sp.modalStep = 'form';
    this.render();
},

updateStoragePolicyField(field, value) {
    const sp = this.state.settings.storageSettings.policies;
    sp.editingPolicy[field] = value;
    // Don't re-render to avoid losing focus, just update preview if visible
    const previewEl = document.querySelector('.storage-policy-preview code');
    if (previewEl) {
        previewEl.textContent = this.generateStoragePolicyPreview();
    }
},

generateStoragePolicyPreview() {
    const p = this.state.settings.storageSettings.policies.editingPolicy;
    if (!p) return '';
    let sql = `CREATE POLICY "${p.policy_name}" ON storage_objects FOR ${p.command}`;
    if (p.using_expr) sql += `\n  USING (${p.using_expr})`;
    if (p.check_expr) sql += `\n  WITH CHECK (${p.check_expr})`;
    return sql + ';';
},
```

**Step 2: Add modal case to renderModal**

Find the `renderModal` switch statement (around line 1030) and add the storage policy cases:

```javascript
case 'storagePolicy':
    content = this.renderStoragePolicyModal();
    break;
```

**Step 3: Update render() to show storage policy modal**

Find where the modal is rendered and add a check for storage policy modal. In the `render()` function, the modal is typically shown based on `this.state.modal`. We need to also check for storage policy modal:

Actually, the storage policy modal uses a different state (`storageSettings.policies.showModal`). Add this at the end of `render()` before the final closing:

```javascript
// Storage policy modal
if (this.state.settings.storageSettings.policies.showModal) {
    const modalOverlay = document.createElement('div');
    modalOverlay.className = 'modal-overlay';
    modalOverlay.innerHTML = `<div class="modal">${this.renderStoragePolicyModal()}</div>`;
    modalOverlay.addEventListener('click', (e) => {
        if (e.target === modalOverlay) App.closeStoragePolicyModal();
    });
    document.body.appendChild(modalOverlay);
}
```

Wait - looking at the existing code, modals are rendered inline. Let me check render():

Actually, add the storage policy modal inline in `renderStorageSettingsSection`, after the `renderStoragePoliciesSection()` call:

```javascript
${this.renderStoragePoliciesSection()}
${ss.policies.showModal ? `
    <div class="modal-overlay" onclick="if(event.target===this)App.closeStoragePolicyModal()">
        <div class="modal">${this.renderStoragePolicyModal()}</div>
    </div>
` : ''}
```

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add storage policy modal functions"
```

---

### Task 6: Render Storage Policy Modal

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add renderStoragePolicyModal function**

```javascript
renderStoragePolicyModal() {
    const sp = this.state.settings.storageSettings.policies;
    const isEdit = sp.editingPolicy && sp.editingPolicy.id;

    if (sp.modalStep === 'template' && !isEdit) {
        return this.renderStoragePolicyTemplateStep();
    }
    return this.renderStoragePolicyFormStep();
},

renderStoragePolicyTemplateStep() {
    const sp = this.state.settings.storageSettings.policies;
    const bucket = sp.editingPolicy?.bucket || 'bucket';

    return `
        <div class="modal-header">
            <h3>New Storage Policy</h3>
            <button class="btn-icon" onclick="App.closeStoragePolicyModal()">&times;</button>
        </div>
        <div class="modal-body">
            <div class="form-group">
                <label class="form-label">Bucket</label>
                <select class="form-input" onchange="App.updateStoragePolicyField('bucket', this.value)">
                    ${sp.buckets.map(b => `
                        <option value="${b.id}" ${sp.editingPolicy?.bucket === b.id ? 'selected' : ''}>${b.id}</option>
                    `).join('')}
                </select>
            </div>

            <label class="form-label">Choose a template</label>
            <div class="template-options">
                <div class="template-option" onclick="App.selectStoragePolicyTemplate('public_read')">
                    <div class="template-name">Public read</div>
                    <div class="template-desc">Anyone can download files from this bucket</div>
                </div>
                <div class="template-option" onclick="App.selectStoragePolicyTemplate('authenticated_read')">
                    <div class="template-name">Authenticated read</div>
                    <div class="template-desc">Only logged-in users can download</div>
                </div>
                <div class="template-option" onclick="App.selectStoragePolicyTemplate('authenticated_upload')">
                    <div class="template-name">Authenticated upload</div>
                    <div class="template-desc">Only logged-in users can upload files</div>
                </div>
                <div class="template-option" onclick="App.selectStoragePolicyTemplate('owner_only')">
                    <div class="template-name">Owner only</div>
                    <div class="template-desc">Users can only access files in their own folder</div>
                </div>
                <div class="template-option" onclick="App.selectStoragePolicyTemplate('custom')">
                    <div class="template-name">Custom policy</div>
                    <div class="template-desc">Write your own USING/CHECK expressions</div>
                </div>
            </div>
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="App.closeStoragePolicyModal()">Cancel</button>
        </div>
    `;
},

renderStoragePolicyFormStep() {
    const sp = this.state.settings.storageSettings.policies;
    const p = sp.editingPolicy;
    const isEdit = p && p.id;
    const showUsing = ['SELECT', 'UPDATE', 'DELETE', 'ALL'].includes(p.command);
    const showCheck = ['INSERT', 'UPDATE', 'ALL'].includes(p.command);

    return `
        <div class="modal-header">
            <h3>${isEdit ? 'Edit Policy' : 'New Storage Policy'}</h3>
            <button class="btn-icon" onclick="App.closeStoragePolicyModal()">&times;</button>
        </div>
        <div class="modal-body">
            ${sp.error ? `<div class="message message-error">${sp.error}</div>` : ''}

            <div class="form-group">
                <label class="form-label">Policy Name</label>
                <input type="text" class="form-input" value="${p.policy_name || ''}"
                    placeholder="e.g., bucket_public_read"
                    oninput="App.updateStoragePolicyField('policy_name', this.value)">
            </div>

            <div class="form-group">
                <label class="form-label">Command</label>
                <select class="form-input" onchange="App.updateStoragePolicyField('command', this.value)">
                    <option value="SELECT" ${p.command === 'SELECT' ? 'selected' : ''}>SELECT</option>
                    <option value="INSERT" ${p.command === 'INSERT' ? 'selected' : ''}>INSERT</option>
                    <option value="UPDATE" ${p.command === 'UPDATE' ? 'selected' : ''}>UPDATE</option>
                    <option value="DELETE" ${p.command === 'DELETE' ? 'selected' : ''}>DELETE</option>
                    <option value="ALL" ${p.command === 'ALL' ? 'selected' : ''}>ALL</option>
                </select>
            </div>

            ${showUsing ? `
                <div class="form-group">
                    <label class="form-label">USING Expression</label>
                    <textarea class="form-input" rows="3"
                        placeholder="bucket_id = 'my-bucket' AND auth.uid() IS NOT NULL"
                        oninput="App.updateStoragePolicyField('using_expr', this.value)">${this.escapeHtml(p.using_expr || '')}</textarea>
                    <small class="text-muted">Filters which existing objects can be accessed</small>
                </div>
            ` : ''}

            ${showCheck ? `
                <div class="form-group">
                    <label class="form-label">CHECK Expression</label>
                    <textarea class="form-input" rows="3"
                        placeholder="bucket_id = 'my-bucket' AND auth.uid() IS NOT NULL"
                        oninput="App.updateStoragePolicyField('check_expr', this.value)">${this.escapeHtml(p.check_expr || '')}</textarea>
                    <small class="text-muted">Validates new or modified objects</small>
                </div>
            ` : ''}

            <div class="form-group">
                <label>
                    <input type="checkbox" ${p.enabled ? 'checked' : ''}
                        onchange="App.updateStoragePolicyField('enabled', this.checked)">
                    Policy enabled
                </label>
            </div>

            <details class="helper-reference">
                <summary>Helper Functions</summary>
                <div class="helper-list">
                    <div class="helper-item">
                        <code>storage.filename(name)</code>
                        <span>Returns filename without path</span>
                    </div>
                    <div class="helper-item">
                        <code>storage.foldername(name)</code>
                        <span>Returns folder path</span>
                    </div>
                    <div class="helper-item">
                        <code>storage.extension(name)</code>
                        <span>Returns file extension</span>
                    </div>
                    <div class="helper-item">
                        <code>auth.uid()</code>
                        <span>Current user's ID or NULL</span>
                    </div>
                </div>
            </details>

            <div class="storage-policy-preview">
                <label class="form-label">SQL Preview</label>
                <pre><code>${this.escapeHtml(this.generateStoragePolicyPreview())}</code></pre>
            </div>
        </div>
        <div class="modal-footer">
            ${!isEdit && sp.template !== 'custom' ? `
                <button class="btn btn-secondary" onclick="App.state.settings.storageSettings.policies.modalStep='template';App.render()">Back</button>
            ` : ''}
            <button class="btn btn-secondary" onclick="App.closeStoragePolicyModal()">Cancel</button>
            <button class="btn btn-primary" onclick="App.saveStoragePolicy()">${isEdit ? 'Save Changes' : 'Create Policy'}</button>
        </div>
    `;
},
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): render storage policy modal with templates"
```

---

### Task 7: Add Storage Policy Modal CSS

**Files:**
- Modify: `internal/dashboard/static/style.css`

**Step 1: Add template options CSS**

```css
/* Storage Policy Modal Templates */
.template-options {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    margin-top: 0.5rem;
}

.template-option {
    padding: 1rem;
    border: 1px solid var(--border);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.15s;
}

.template-option:hover {
    border-color: var(--primary);
    background: var(--hover);
}

.template-name {
    font-weight: 600;
    margin-bottom: 0.25rem;
}

.template-desc {
    font-size: 0.875rem;
    color: var(--text-muted);
}

.storage-policy-preview {
    margin-top: 1rem;
    padding-top: 1rem;
    border-top: 1px solid var(--border);
}

.storage-policy-preview pre {
    background: var(--code-bg);
    padding: 0.75rem;
    border-radius: 4px;
    font-size: 0.8125rem;
    overflow-x: auto;
}
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/style.css
git commit -m "feat(dashboard): add storage policy modal CSS"
```

---

### Task 8: Add Save/Delete/Toggle Storage Policy Functions

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add saveStoragePolicy function**

```javascript
async saveStoragePolicy() {
    const sp = this.state.settings.storageSettings.policies;
    const p = sp.editingPolicy;
    const isEdit = p && p.id;

    // Validate
    if (!p.policy_name) {
        sp.error = 'Policy name is required';
        this.render();
        return;
    }
    if (!p.using_expr && !p.check_expr) {
        sp.error = 'At least one expression (USING or CHECK) is required';
        this.render();
        return;
    }

    try {
        let res;
        if (isEdit) {
            res = await fetch(`/_/api/policies/${p.id}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    policy_name: p.policy_name,
                    command: p.command,
                    using_expr: p.using_expr || null,
                    check_expr: p.check_expr || null,
                    enabled: p.enabled
                })
            });
        } else {
            res = await fetch('/_/api/policies', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    table_name: 'storage_objects',
                    policy_name: p.policy_name,
                    command: p.command,
                    using_expr: p.using_expr || null,
                    check_expr: p.check_expr || null,
                    enabled: p.enabled
                })
            });
        }

        if (res.ok) {
            this.closeStoragePolicyModal();
            await this.loadStoragePolicies();
        } else {
            const err = await res.json();
            sp.error = err.error || 'Failed to save policy';
            this.render();
        }
    } catch (e) {
        sp.error = 'Failed to save policy';
        this.render();
    }
},

async toggleStoragePolicyEnabled(policyId, enabled) {
    try {
        const res = await fetch(`/_/api/policies/${policyId}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled })
        });
        if (res.ok) {
            const sp = this.state.settings.storageSettings.policies;
            const policy = sp.list.find(p => p.id === policyId);
            if (policy) policy.enabled = enabled;
            this.render();
        }
    } catch (e) {
        console.error('Failed to update policy:', e);
    }
},

async confirmDeleteStoragePolicy(policyId, policyName) {
    if (!confirm(`Delete policy "${policyName}"? This cannot be undone.`)) return;

    try {
        const res = await fetch(`/_/api/policies/${policyId}`, { method: 'DELETE' });
        if (res.ok) {
            await this.loadStoragePolicies();
        }
    } catch (e) {
        console.error('Failed to delete policy:', e);
    }
},
```

**Step 2: Verify full workflow**

1. Open dashboard → Settings → Storage
2. Click "+ New Policy"
3. Select a bucket, choose a template
4. Verify form is pre-filled
5. Create the policy
6. Verify it appears in the list
7. Toggle enabled/disabled
8. Edit the policy
9. Delete the policy

**Step 3: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add storage policy CRUD operations"
```

---

### Task 9: Final Testing and Cleanup

**Step 1: Test all workflows**

1. Load storage section - buckets and policies load
2. Select different buckets - policies filter correctly
3. Create policy with each template - expressions are correct
4. Create custom policy - form works
5. Edit existing policy - form pre-fills
6. Toggle policy enabled/disabled - updates immediately
7. Delete policy - confirms and removes
8. Helper reference - expands and shows functions

**Step 2: Fix any issues found**

Address any bugs or styling issues discovered during testing.

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat(dashboard): complete storage policies management

Add Storage Policies section within Settings → Storage:
- Per-bucket organization with policy counts
- Policy templates (public read, authenticated read/upload, owner only, custom)
- Full CRUD operations using existing policy APIs
- Helper functions reference for storage.filename(), etc.

Closes #XXX"
```

---

## Summary

This plan adds storage policy management to the dashboard in 9 tasks:

1. Add state structure
2. Add data loading functions
3. Render policies section
4. Add CSS styles
5. Add modal functions
6. Render modal with templates
7. Add modal CSS
8. Add CRUD operations
9. Final testing

All changes are in two files:
- `internal/dashboard/static/app.js` - JavaScript logic
- `internal/dashboard/static/style.css` - Styling

No backend changes needed - reuses existing policy and bucket APIs.
