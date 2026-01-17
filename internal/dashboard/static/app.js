// sblite Dashboard Application

const App = {
    state: {
        authenticated: false,
        needsSetup: true,
        theme: 'dark',
        currentView: 'tables',
        loading: true,
        error: null,
        tables: {
            list: [],
            selected: null,
            schema: null,
            data: [],
            page: 1,
            pageSize: 25,
            totalRows: 0,
            selectedRows: new Set(),
            editingCell: null,
            loading: false,
        },
        modal: {
            type: null,  // 'createTable', 'addRow', 'editRow', 'schema', 'addColumn'
            data: {}
        },
    },

    async init() {
        this.loadTheme();
        await this.checkAuth();
        if (this.state.authenticated) {
            await this.loadTables();
        }
        this.render();
    },

    loadTheme() {
        const saved = localStorage.getItem('sblite_theme');
        if (saved) {
            this.state.theme = saved;
        }
        this.applyTheme();
    },

    applyTheme() {
        document.documentElement.setAttribute('data-theme', this.state.theme);
    },

    toggleTheme() {
        this.state.theme = this.state.theme === 'dark' ? 'light' : 'dark';
        localStorage.setItem('sblite_theme', this.state.theme);
        this.applyTheme();
        this.render();
    },

    async checkAuth() {
        try {
            const res = await fetch('/_/api/auth/status');
            const data = await res.json();
            this.state.needsSetup = data.needs_setup;
            this.state.authenticated = data.authenticated;
        } catch (e) {
            this.state.error = 'Failed to connect to server';
        }
        this.state.loading = false;
    },

    async setup(password) {
        try {
            const res = await fetch('/_/api/auth/setup', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ password })
            });
            if (res.ok) {
                this.state.needsSetup = false;
                this.state.authenticated = true;
                this.state.error = null;
                this.render();
            } else {
                const data = await res.json();
                this.state.error = data.error || 'Setup failed';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Connection error';
            this.render();
        }
    },

    async login(password) {
        try {
            const res = await fetch('/_/api/auth/login', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ password })
            });
            if (res.ok) {
                this.state.authenticated = true;
                this.state.error = null;
                this.render();
            } else {
                this.state.error = 'Invalid password';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Connection error';
            this.render();
        }
    },

    async logout() {
        try {
            await fetch('/_/api/auth/logout', { method: 'POST' });
            this.state.authenticated = false;
            this.render();
        } catch (e) {
            this.state.error = 'Logout failed';
            this.render();
        }
    },

    // Table management methods
    async loadTables() {
        try {
            const res = await fetch('/_/api/tables');
            if (res.ok) {
                this.state.tables.list = await res.json();
            }
        } catch (e) {
            this.state.error = 'Failed to load tables';
        }
        this.render();
    },

    async selectTable(name) {
        this.state.tables.selected = name;
        this.state.tables.page = 1;
        this.state.tables.selectedRows = new Set();
        await this.loadTableSchema(name);
        await this.loadTableData();
    },

    async loadTableSchema(name) {
        try {
            const res = await fetch(`/_/api/tables/${name}`);
            if (res.ok) {
                this.state.tables.schema = await res.json();
            }
        } catch (e) {
            this.state.error = 'Failed to load schema';
        }
    },

    async loadTableData() {
        const { selected, page, pageSize } = this.state.tables;
        if (!selected) return;

        this.state.tables.loading = true;
        this.render();

        try {
            const offset = (page - 1) * pageSize;
            const res = await fetch(`/_/api/data/${selected}?limit=${pageSize}&offset=${offset}`);
            if (res.ok) {
                const data = await res.json();
                this.state.tables.data = data.rows;
                this.state.tables.totalRows = data.total;
            }
        } catch (e) {
            this.state.error = 'Failed to load data';
        }
        this.state.tables.loading = false;
        this.render();
    },

    navigate(view) {
        this.state.currentView = view;
        this.render();
    },

    render() {
        const app = document.getElementById('app');

        if (this.state.loading) {
            app.innerHTML = '<div class="loading">Loading...</div>';
            return;
        }

        if (this.state.needsSetup) {
            app.innerHTML = this.renderSetup();
            return;
        }

        if (!this.state.authenticated) {
            app.innerHTML = this.renderLogin();
            return;
        }

        app.innerHTML = this.renderDashboard();
    },

    renderSetup() {
        return `
            <div class="auth-container">
                <div class="card auth-card">
                    <h1 class="auth-title">Welcome to sblite</h1>
                    <p class="auth-subtitle">Set up your dashboard password to get started</p>
                    ${this.state.error ? `<div class="message message-error">${this.state.error}</div>` : ''}
                    <form onsubmit="event.preventDefault(); App.setup(this.password.value)">
                        <div class="form-group">
                            <label class="form-label" for="password">Password</label>
                            <input type="password" id="password" name="password" class="form-input"
                                   placeholder="Minimum 8 characters" minlength="8" required>
                        </div>
                        <div class="form-group">
                            <label class="form-label" for="confirm">Confirm Password</label>
                            <input type="password" id="confirm" name="confirm" class="form-input"
                                   placeholder="Confirm password" required>
                        </div>
                        <button type="submit" class="btn btn-primary" style="width: 100%">
                            Set Password
                        </button>
                    </form>
                </div>
            </div>
        `;
    },

    renderLogin() {
        return `
            <div class="auth-container">
                <div class="card auth-card">
                    <h1 class="auth-title">sblite Dashboard</h1>
                    <p class="auth-subtitle">Enter your password to continue</p>
                    ${this.state.error ? `<div class="message message-error">${this.state.error}</div>` : ''}
                    <form onsubmit="event.preventDefault(); App.login(this.password.value)">
                        <div class="form-group">
                            <label class="form-label" for="password">Password</label>
                            <input type="password" id="password" name="password" class="form-input" required>
                        </div>
                        <button type="submit" class="btn btn-primary" style="width: 100%">
                            Sign In
                        </button>
                    </form>
                </div>
            </div>
        `;
    },

    renderDashboard() {
        const themeIcon = this.state.theme === 'dark' ? '&#9728;' : '&#9789;';

        return `
            <div class="layout">
                <aside class="sidebar">
                    <div class="sidebar-header">
                        <span class="logo">sblite</span>
                        <button class="theme-toggle" onclick="App.toggleTheme()">${themeIcon}</button>
                    </div>

                    <nav>
                        <div class="nav-section">
                            <div class="nav-section-title">Database</div>
                            <a class="nav-item ${this.state.currentView === 'tables' ? 'active' : ''}"
                               onclick="App.navigate('tables')">Tables</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Auth</div>
                            <a class="nav-item ${this.state.currentView === 'users' ? 'active' : ''}"
                               onclick="App.navigate('users')">Users</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Security</div>
                            <a class="nav-item ${this.state.currentView === 'policies' ? 'active' : ''}"
                               onclick="App.navigate('policies')">Policies</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">System</div>
                            <a class="nav-item ${this.state.currentView === 'settings' ? 'active' : ''}"
                               onclick="App.navigate('settings')">Settings</a>
                            <a class="nav-item ${this.state.currentView === 'logs' ? 'active' : ''}"
                               onclick="App.navigate('logs')">Logs</a>
                        </div>
                    </nav>

                    <div style="margin-top: auto; padding-top: 1rem; border-top: 1px solid var(--border)">
                        <a class="nav-item" onclick="App.logout()">Logout</a>
                    </div>
                </aside>

                <main class="main-content">
                    ${this.renderContent()}
                </main>
            </div>
            ${this.renderModals()}
        `;
    },

    renderContent() {
        switch (this.state.currentView) {
            case 'tables':
                return this.renderTablesView();
            case 'users':
                return '<div class="card"><h2 class="card-title">Users</h2><p>User management coming soon</p></div>';
            case 'policies':
                return '<div class="card"><h2 class="card-title">RLS Policies</h2><p>Policy editor coming in Phase 5</p></div>';
            case 'settings':
                return '<div class="card"><h2 class="card-title">Settings</h2><p>Settings panel coming in Phase 6</p></div>';
            case 'logs':
                return '<div class="card"><h2 class="card-title">Logs</h2><p>Log viewer coming in Phase 6</p></div>';
            default:
                return '<div class="card">Select a section from the sidebar</div>';
        }
    },

    renderTablesView() {
        return `
            <div class="tables-layout">
                <div class="table-list-panel">
                    <div class="panel-header">
                        <span>Tables</span>
                        <button class="btn btn-primary btn-sm" onclick="App.showCreateTableModal()">+ New</button>
                    </div>
                    <div class="table-list">
                        ${this.state.tables.list.length === 0
                            ? '<div class="empty-state">No tables yet</div>'
                            : this.state.tables.list.map(t => `
                                <div class="table-list-item ${this.state.tables.selected === t.name ? 'active' : ''}"
                                     onclick="App.selectTable('${t.name}')">
                                    ${t.name}
                                </div>
                            `).join('')}
                    </div>
                </div>
                <div class="table-content-panel">
                    ${this.state.tables.selected ? this.renderTableContent() : '<div class="empty-state">Select a table</div>'}
                </div>
            </div>
        `;
    },

    renderTableContent() {
        if (this.state.tables.loading) {
            return '<div class="loading">Loading...</div>';
        }
        const { selectedRows } = this.state.tables;
        return `
            <div class="table-toolbar">
                <h2>${this.state.tables.selected}</h2>
                <div class="toolbar-actions">
                    ${selectedRows.size > 0 ? `
                        <button class="btn btn-secondary btn-sm" style="color: var(--error)"
                            onclick="App.deleteSelectedRows()">Delete (${selectedRows.size})</button>
                    ` : ''}
                    <button class="btn btn-secondary btn-sm" onclick="App.showAddRowModal()">+ Add Row</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.showSchemaModal()">Schema</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.confirmDeleteTable()">Delete Table</button>
                </div>
            </div>
            ${this.renderDataGrid()}
            ${this.renderPagination()}
        `;
    },

    // Data grid rendering
    renderDataGrid() {
        const { schema, data, selectedRows } = this.state.tables;
        if (!schema || !schema.columns) return '';

        const columns = schema.columns;
        const primaryKey = columns.find(c => c.primary)?.name || columns[0]?.name;

        return `
            <div class="data-grid-container">
                <table class="data-grid">
                    <thead>
                        <tr>
                            <th class="checkbox-col">
                                <input type="checkbox" onchange="App.toggleAllRows(this.checked)"
                                    ${data.length > 0 && selectedRows.size === data.length ? 'checked' : ''}>
                            </th>
                            ${columns.map(col => `
                                <th>${col.name}<span class="col-type">${col.type}</span></th>
                            `).join('')}
                            <th class="actions-col"></th>
                        </tr>
                    </thead>
                    <tbody>
                        ${data.length === 0
                            ? `<tr><td colspan="${columns.length + 2}" class="empty-state">No data</td></tr>`
                            : data.map(row => this.renderDataRow(row, columns, primaryKey)).join('')}
                    </tbody>
                </table>
            </div>
        `;
    },

    renderDataRow(row, columns, primaryKey) {
        const rowId = row[primaryKey];
        const isSelected = this.state.tables.selectedRows.has(rowId);
        const { editingCell } = this.state.tables;

        return `
            <tr class="${isSelected ? 'selected' : ''}">
                <td class="checkbox-col">
                    <input type="checkbox" ${isSelected ? 'checked' : ''}
                        onchange="App.toggleRow('${rowId}', this.checked)">
                </td>
                ${columns.map(col => {
                    const isEditing = editingCell?.rowId === rowId && editingCell?.column === col.name;
                    const value = row[col.name];

                    if (isEditing) {
                        return `
                            <td class="data-cell editing">
                                <input type="text" class="cell-input" value="${value ?? ''}"
                                    onblur="App.saveCellEdit('${rowId}', '${col.name}', this.value)"
                                    onkeydown="App.handleCellKeydown(event, '${rowId}', '${col.name}')">
                            </td>
                        `;
                    }
                    return `
                        <td class="data-cell"
                            onclick="App.startCellEdit('${rowId}', '${col.name}')">
                            ${this.formatCellValue(value, col.type)}
                        </td>
                    `;
                }).join('')}
                <td class="actions-col">
                    <button class="btn-icon" onclick="App.showEditRowModal('${rowId}')">Edit</button>
                    <button class="btn-icon" onclick="App.confirmDeleteRow('${rowId}')">Delete</button>
                </td>
            </tr>
        `;
    },

    formatCellValue(value, type) {
        if (value === null || value === undefined) return '<span class="null-value">NULL</span>';
        if (type === 'boolean') return value ? 'true' : 'false';
        if (type === 'jsonb') return '<span class="json-value">{...}</span>';
        const str = String(value);
        return str.length > 50 ? str.substring(0, 50) + '...' : str;
    },

    toggleRow(rowId, checked) {
        if (checked) {
            this.state.tables.selectedRows.add(rowId);
        } else {
            this.state.tables.selectedRows.delete(rowId);
        }
        this.render();
    },

    toggleAllRows(checked) {
        const { data, schema } = this.state.tables;
        const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

        if (checked) {
            data.forEach(row => this.state.tables.selectedRows.add(row[primaryKey]));
        } else {
            this.state.tables.selectedRows.clear();
        }
        this.render();
    },

    renderPagination() {
        const { page, pageSize, totalRows } = this.state.tables;
        const totalPages = Math.ceil(totalRows / pageSize);

        return `
            <div class="pagination">
                <div class="pagination-info">
                    ${totalRows} rows | Page ${page} of ${totalPages || 1}
                </div>
                <div class="pagination-controls">
                    <select onchange="App.changePageSize(this.value)">
                        <option value="25" ${pageSize === 25 ? 'selected' : ''}>25</option>
                        <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                        <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                    </select>
                    <button class="btn btn-secondary btn-sm" onclick="App.prevPage()" ${page <= 1 ? 'disabled' : ''}>Prev</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.nextPage()" ${page >= totalPages ? 'disabled' : ''}>Next</button>
                </div>
            </div>
        `;
    },

    changePageSize(size) {
        this.state.tables.pageSize = parseInt(size);
        this.state.tables.page = 1;
        this.loadTableData();
    },

    prevPage() {
        if (this.state.tables.page > 1) {
            this.state.tables.page--;
            this.loadTableData();
        }
    },

    nextPage() {
        const totalPages = Math.ceil(this.state.tables.totalRows / this.state.tables.pageSize);
        if (this.state.tables.page < totalPages) {
            this.state.tables.page++;
            this.loadTableData();
        }
    },

    // Inline cell editing
    startCellEdit(rowId, column) {
        this.state.tables.editingCell = { rowId, column };
        this.render();

        // Focus the input after render
        setTimeout(() => {
            const input = document.querySelector('.cell-input');
            if (input) {
                input.focus();
                input.select();
            }
        }, 0);
    },

    cancelCellEdit() {
        this.state.tables.editingCell = null;
        this.render();
    },

    async saveCellEdit(rowId, column, value) {
        const { selected, schema } = this.state.tables;
        const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

        try {
            const res = await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${rowId}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ [column]: value || null })
            });

            if (!res.ok) {
                const err = await res.json();
                this.state.error = err.error || 'Failed to update';
            }
        } catch (e) {
            this.state.error = 'Failed to update';
        }

        this.state.tables.editingCell = null;
        await this.loadTableData();
    },

    handleCellKeydown(e, rowId, column) {
        if (e.key === 'Enter') {
            this.saveCellEdit(rowId, column, e.target.value);
        } else if (e.key === 'Escape') {
            this.cancelCellEdit();
        }
    },

    // Modal methods
    showCreateTableModal() {
        this.state.modal = {
            type: 'createTable',
            data: {
                name: '',
                columns: [{ name: 'id', type: 'uuid', primary: true, nullable: false }]
            }
        };
        this.render();
    },

    closeModal() {
        this.state.modal = { type: null, data: {} };
        this.render();
    },

    updateModalData(field, value) {
        this.state.modal.data[field] = value;
        this.render();
    },

    addColumnToModal() {
        this.state.modal.data.columns.push({ name: '', type: 'text', nullable: true, primary: false });
        this.render();
    },

    removeColumnFromModal(index) {
        this.state.modal.data.columns.splice(index, 1);
        this.render();
    },

    updateModalColumn(index, field, value) {
        this.state.modal.data.columns[index][field] = value;
        this.render();
    },

    async createTable() {
        const { name, columns } = this.state.modal.data;
        if (!name || columns.length === 0) {
            this.state.error = 'Name and at least one column required';
            this.render();
            return;
        }

        try {
            const res = await fetch('/_/api/tables', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, columns })
            });

            if (res.ok) {
                this.closeModal();
                await this.loadTables();
                this.selectTable(name);
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to create table';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to create table';
            this.render();
        }
    },

    renderModals() {
        const { type, data } = this.state.modal;
        if (!type) return '';

        let content = '';
        switch (type) {
            case 'createTable':
                content = this.renderCreateTableModal();
                break;
            case 'addRow':
            case 'editRow':
                content = this.renderRowModal();
                break;
            // Other modal types will be added in later tasks
        }

        return `
            <div class="modal-overlay" onclick="App.closeModal()">
                <div class="modal" onclick="event.stopPropagation()">
                    ${content}
                </div>
            </div>
        `;
    },

    renderCreateTableModal() {
        const { name, columns } = this.state.modal.data;
        const types = ['uuid', 'text', 'integer', 'boolean', 'timestamptz', 'jsonb', 'numeric', 'bytea'];

        return `
            <div class="modal-header">
                <h3>Create Table</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="form-group">
                    <label class="form-label">Table Name</label>
                    <input type="text" class="form-input" value="${name}"
                        onchange="App.updateModalData('name', this.value)" placeholder="my_table">
                </div>

                <div class="form-group">
                    <label class="form-label">Columns</label>
                    ${columns.map((col, i) => `
                        <div class="column-row">
                            <input type="text" class="form-input" value="${col.name}" placeholder="column_name"
                                onchange="App.updateModalColumn(${i}, 'name', this.value)">
                            <select class="form-input" onchange="App.updateModalColumn(${i}, 'type', this.value)">
                                ${types.map(t => `<option value="${t}" ${col.type === t ? 'selected' : ''}>${t}</option>`).join('')}
                            </select>
                            <label><input type="checkbox" ${col.primary ? 'checked' : ''}
                                onchange="App.updateModalColumn(${i}, 'primary', this.checked)"> PK</label>
                            <label><input type="checkbox" ${col.nullable ? 'checked' : ''}
                                onchange="App.updateModalColumn(${i}, 'nullable', this.checked)"> Null</label>
                            <button class="btn-icon" onclick="App.removeColumnFromModal(${i})">&times;</button>
                        </div>
                    `).join('')}
                    <button class="btn btn-secondary btn-sm" onclick="App.addColumnToModal()">+ Add Column</button>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.createTable()">Create Table</button>
            </div>
        `;
    },

    // Row modal methods
    showAddRowModal() {
        const { schema } = this.state.tables;
        if (!schema) return;

        const data = {};
        schema.columns.forEach(col => {
            data[col.name] = col.type === 'uuid' ? crypto.randomUUID() : '';
        });

        this.state.modal = { type: 'addRow', data };
        this.render();
    },

    showEditRowModal(rowId) {
        const { data, schema } = this.state.tables;
        const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;
        const row = data.find(r => r[primaryKey] === rowId);

        if (row) {
            this.state.modal = { type: 'editRow', data: { ...row, _rowId: rowId } };
            this.render();
        }
    },

    async saveRow() {
        const { type, data } = this.state.modal;
        const { selected } = this.state.tables;
        const isNew = type === 'addRow';

        const rowData = { ...data };
        delete rowData._rowId;

        try {
            let res;
            if (isNew) {
                res = await fetch(`/_/api/data/${selected}`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(rowData)
                });
            } else {
                const { schema } = this.state.tables;
                const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;
                res = await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${data._rowId}`, {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(rowData)
                });
            }

            if (res.ok) {
                this.closeModal();
                await this.loadTableData();
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to save';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to save';
            this.render();
        }
    },

    updateRowField(field, value) {
        this.state.modal.data[field] = value;
    },

    renderRowModal() {
        const { type, data } = this.state.modal;
        const { schema } = this.state.tables;
        const isNew = type === 'addRow';

        return `
            <div class="modal-header">
                <h3>${isNew ? 'Add Row' : 'Edit Row'}</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                ${schema.columns.map(col => `
                    <div class="form-group">
                        <label class="form-label">${col.name} <span class="col-type">${col.type}</span></label>
                        <input type="text" class="form-input" value="${data[col.name] ?? ''}"
                            onchange="App.updateRowField('${col.name}', this.value)"
                            ${col.primary && !isNew ? 'disabled' : ''}>
                    </div>
                `).join('')}
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.saveRow()">${isNew ? 'Add' : 'Save'}</button>
            </div>
        `;
    },

    // Delete operations
    async confirmDeleteRow(rowId) {
        if (!confirm('Delete this row?')) return;

        const { selected, schema } = this.state.tables;
        const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

        try {
            const res = await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${rowId}`, {
                method: 'DELETE'
            });

            if (res.ok) {
                await this.loadTableData();
            } else {
                this.state.error = 'Failed to delete';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to delete';
            this.render();
        }
    },

    async deleteSelectedRows() {
        const { selectedRows, selected, schema } = this.state.tables;
        if (selectedRows.size === 0) return;

        if (!confirm(`Delete ${selectedRows.size} row(s)?`)) return;

        const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

        for (const rowId of selectedRows) {
            try {
                await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${rowId}`, {
                    method: 'DELETE'
                });
            } catch (e) {
                // Continue with others
            }
        }

        this.state.tables.selectedRows.clear();
        await this.loadTableData();
    },

    async confirmDeleteTable() {
        const { selected } = this.state.tables;
        if (!confirm(`Delete table "${selected}"? This cannot be undone.`)) return;

        try {
            const res = await fetch(`/_/api/tables/${selected}`, {
                method: 'DELETE'
            });

            if (res.ok) {
                this.state.tables.selected = null;
                this.state.tables.schema = null;
                this.state.tables.data = [];
                await this.loadTables();
            } else {
                this.state.error = 'Failed to delete table';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to delete table';
            this.render();
        }
    },

    // Placeholder for schema modal (Task 15)
    showSchemaModal() {
        alert('Schema modal coming soon');
    }
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => App.init());
