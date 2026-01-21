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
            // Filtering
            filters: [],
            showFilters: false,
            // Sorting
            sort: { column: null, direction: null }, // direction: 'asc', 'desc', or null
            // FTS indexes
            ftsIndexes: [],
            ftsLoading: false,
        },
        modal: {
            type: null,  // 'createTable', 'addRow', 'editRow', 'schema', 'addColumn', 'userDetail'
            data: {}
        },
        users: {
            list: [],
            page: 1,
            pageSize: 25,
            totalUsers: 0,
            loading: false,
            filter: 'all',  // 'all', 'regular', 'anonymous'
        },
        policies: {
            tables: [],           // Tables with RLS info
            selectedTable: null,  // Currently selected table
            list: [],             // Policies for selected table
            loading: false,
        },
        settings: {
            server: null,
            auth: null,
            authConfig: { require_email_confirmation: true },
            templates: [],
            loading: false,
            expandedSections: { server: true, apiKeys: false, auth: false, oauth: false, templates: false, export: false },
            editingTemplate: null,
            oauth: {
                providers: {},
                redirectUrls: [],
            },
            apiKeys: null,
        },
        logs: {
            config: null,
            list: [],
            total: 0,
            page: 1,
            pageSize: 50,
            filters: { level: 'all', since: '', until: '', search: '', user_id: '', request_id: '' },
            expandedLog: null,
            tailLines: [],
            loading: false,
            // New console buffer state
            activeTab: 'console',  // 'console', 'database', 'file'
            consoleLines: [],
            consoleTotal: 0,
            consoleBufferSize: 0,
            consoleEnabled: false,
            autoRefresh: false,
            autoRefreshInterval: null,
        },
        apiConsole: {
            method: 'GET',
            url: '/rest/v1/',
            headers: [],
            body: '',
            response: null,
            loading: false,
            history: [],
            activeTab: 'body',
            showHistory: false,
            apiKeys: null,          // { anon_key, service_role_key }
            selectedKeyType: 'anon', // 'anon' or 'service_role'
            autoInjectKey: true,    // Auto-inject apikey header
        },
        sqlBrowser: {
            query: 'SELECT * FROM ',
            results: null,
            loading: false,
            error: null,
            history: [],
            page: 1,
            pageSize: 50,
            sort: { column: null, direction: null },
            showHistory: false,
            showTablePicker: false,
            tables: [],
            postgresMode: false,
        },
        functions: {
            list: [],
            selected: null,
            status: null,  // Edge runtime status
            loading: false,
            config: null,  // Selected function config
            secrets: [],
            secretsLoading: false,
            showSecrets: false,
            testConsole: {
                method: 'POST',
                body: '{}',
                headers: [],
                response: null,
                loading: false,
            },
            editor: {
                currentFile: null,      // Currently open file path
                content: '',            // File content in editor
                originalContent: '',    // For dirty detection
                isDirty: false,         // Has unsaved changes
                tree: null,             // File tree structure
                expandedFolders: {},    // Which folders are expanded
                isExpanded: false,      // Full-width mode
                monacoEditor: null,     // Monaco editor instance
                loading: false          // Loading state
            }
        },
        storage: {
            buckets: [],
            selectedBucket: null,
            objects: [],
            currentPath: '',
            viewMode: 'grid',
            selectedFiles: [],
            uploading: [],
            loading: false,
            offset: 0,
            hasMore: false,
            pageSize: 100
        },
        apiDocs: {
            page: 'intro',          // 'intro', 'auth', 'users-management', 'tables-intro', 'rpc-intro'
            resource: null,         // table name when viewing table docs
            rpc: null,              // function name when viewing RPC docs
            language: 'javascript', // 'javascript' or 'bash'
            tables: [],             // cached table list with schemas
            functions: [],          // cached RPC function list
            loading: false,
            selectedTable: null,    // detailed table info
            selectedFunction: null, // detailed function info
        },
    },

    async init() {
        this.loadTheme();
        this.loadSqlBrowserPreferences();
        await this.checkAuth();
        if (this.state.authenticated) {
            await this.loadTables();
        }
        this.render();
    },

    loadSqlBrowserPreferences() {
        const postgresMode = localStorage.getItem('sql_postgres_mode');
        if (postgresMode === 'true') {
            this.state.sqlBrowser.postgresMode = true;
        }
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
                await this.loadTables();
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
        this.state.tables.filters = [];
        this.state.tables.showFilters = false;
        this.state.tables.sort = { column: null, direction: null };
        await this.loadTableSchema(name);
        await this.loadTableData();
    },

    async loadTableSchema(name) {
        try {
            const res = await fetch(`/_/api/tables/${name}`);
            if (res.ok) {
                this.state.tables.schema = await res.json();
            }
            // Also load FTS indexes
            await this.loadFTSIndexes(name);
        } catch (e) {
            this.state.error = 'Failed to load schema';
        }
    },

    async loadTableData() {
        const { selected, page, pageSize, filters, sort } = this.state.tables;
        if (!selected) return;

        this.state.tables.loading = true;
        this.render();

        try {
            const offset = (page - 1) * pageSize;
            const params = new URLSearchParams();
            params.set('limit', pageSize);
            params.set('offset', offset);

            // Add filters
            filters.forEach(f => {
                if (f.column && f.operator && f.value !== '') {
                    params.append(f.column, `${f.operator}.${f.value}`);
                }
            });

            // Add sort
            if (sort.column && sort.direction) {
                params.set('order', `${sort.column}.${sort.direction}`);
            }

            const res = await fetch(`/_/api/data/${selected}?${params.toString()}`);
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

    // Filter methods
    toggleFilterPanel() {
        this.state.tables.showFilters = !this.state.tables.showFilters;
        this.render();
    },

    addFilter() {
        const { schema } = this.state.tables;
        const firstColumn = schema?.columns?.[0]?.name || '';
        this.state.tables.filters.push({
            column: firstColumn,
            operator: 'eq',
            value: ''
        });
        this.render();
    },

    removeFilter(index) {
        this.state.tables.filters.splice(index, 1);
        this.state.tables.page = 1;
        this.loadTableData();
    },

    updateFilter(index, field, value) {
        this.state.tables.filters[index][field] = value;
        this.render();
    },

    applyFilters() {
        this.state.tables.page = 1;
        this.loadTableData();
    },

    clearFilters() {
        this.state.tables.filters = [];
        this.state.tables.page = 1;
        this.loadTableData();
    },

    // Sort methods
    toggleSort(column) {
        const { sort } = this.state.tables;
        if (sort.column === column) {
            // Cycle: asc -> desc -> null
            if (sort.direction === 'asc') {
                this.state.tables.sort = { column, direction: 'desc' };
            } else if (sort.direction === 'desc') {
                this.state.tables.sort = { column: null, direction: null };
            } else {
                this.state.tables.sort = { column, direction: 'asc' };
            }
        } else {
            this.state.tables.sort = { column, direction: 'asc' };
        }
        this.state.tables.page = 1;
        this.loadTableData();
    },

    navigate(view) {
        this.state.currentView = view;
        if (view === 'users') {
            this.loadUsers();
        } else if (view === 'policies') {
            this.loadPoliciesTables();
        } else if (view === 'settings') {
            this.loadSettings();
        } else if (view === 'logs') {
            this.loadLogs();
        } else if (view === 'apiConsole') {
            this.initApiConsole();
        } else if (view === 'sqlBrowser') {
            this.initSqlBrowser();
        } else if (view === 'functions') {
            this.loadFunctions();
        } else if (view === 'storage') {
            this.loadBuckets();
        } else if (view === 'apiDocs') {
            this.initApiDocs();
        } else {
            this.render();
        }
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
                            <a class="nav-item ${this.state.currentView === 'policies' ? 'active' : ''}"
                               onclick="App.navigate('policies')">Policies</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Auth</div>
                            <a class="nav-item ${this.state.currentView === 'users' ? 'active' : ''}"
                               onclick="App.navigate('users')">Users</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Storage</div>
                            <a class="nav-item ${this.state.currentView === 'storage' ? 'active' : ''}"
                               onclick="App.navigate('storage')">Buckets</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Edge Functions</div>
                            <a class="nav-item ${this.state.currentView === 'functions' ? 'active' : ''}"
                               onclick="App.navigate('functions')">Functions</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">Documentation</div>
                            <a class="nav-item ${this.state.currentView === 'apiDocs' ? 'active' : ''}"
                               onclick="App.navigate('apiDocs')">API Docs</a>
                        </div>

                        <div class="nav-section">
                            <div class="nav-section-title">System</div>
                            <a class="nav-item ${this.state.currentView === 'settings' ? 'active' : ''}"
                               onclick="App.navigate('settings')">Settings</a>
                            <a class="nav-item ${this.state.currentView === 'logs' ? 'active' : ''}"
                               onclick="App.navigate('logs')">Logs</a>
                            <a class="nav-item ${this.state.currentView === 'apiConsole' ? 'active' : ''}"
                               onclick="App.navigate('apiConsole')">API Console</a>
                            <a class="nav-item ${this.state.currentView === 'sqlBrowser' ? 'active' : ''}"
                               onclick="App.navigate('sqlBrowser')">SQL Browser</a>
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
                return this.renderUsersView();
            case 'policies':
                return this.renderPoliciesView();
            case 'settings':
                return this.renderSettingsView();
            case 'logs':
                return this.renderLogsView();
            case 'apiConsole':
                return this.renderApiConsoleView();
            case 'sqlBrowser':
                return this.renderSqlBrowserView();
            case 'functions':
                return this.renderFunctionsView();
            case 'storage':
                return this.renderStorageView();
            case 'apiDocs':
                return this.renderApiDocsView();
            default:
                return '<div class="card">Select a section from the sidebar</div>';
        }
    },

    renderTablesView() {
        return `
            <div class="card-title">Tables</div>
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
        const { selectedRows, filters, showFilters } = this.state.tables;
        const hasActiveFilters = filters.some(f => f.column && f.operator && f.value !== '');

        return `
            <div class="table-toolbar">
                <h2>${this.state.tables.selected}</h2>
                <div class="toolbar-actions">
                    <button class="btn ${showFilters || hasActiveFilters ? 'btn-primary' : 'btn-secondary'} btn-sm"
                        onclick="App.toggleFilterPanel()">
                        Filter${hasActiveFilters ? ` (${filters.filter(f => f.value !== '').length})` : ''}
                    </button>
                    ${selectedRows.size > 0 ? `
                        <button class="btn btn-secondary btn-sm" style="color: var(--error)"
                            onclick="App.deleteSelectedRows()">Delete (${selectedRows.size})</button>
                    ` : ''}
                    <button class="btn btn-secondary btn-sm" onclick="App.showAddRowModal()">+ Add Row</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.showSchemaModal()">Schema</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.confirmDeleteTable()">Delete Table</button>
                </div>
            </div>
            ${showFilters ? this.renderFilterPanel() : ''}
            ${this.renderDataGrid()}
            ${this.renderPagination()}
        `;
    },

    renderFilterPanel() {
        const { filters, schema } = this.state.tables;
        const columns = schema?.columns || [];
        const operators = [
            { value: 'eq', label: 'equals' },
            { value: 'neq', label: 'not equals' },
            { value: 'gt', label: 'greater than' },
            { value: 'gte', label: 'greater or equal' },
            { value: 'lt', label: 'less than' },
            { value: 'lte', label: 'less or equal' },
            { value: 'like', label: 'like' },
            { value: 'ilike', label: 'ilike (case-insensitive)' },
            { value: 'is', label: 'is (null/true/false)' },
        ];

        return `
            <div class="filter-panel">
                <div class="filter-rows">
                    ${filters.length === 0 ? `
                        <div class="filter-empty">No filters applied. Click "Add filter" to start filtering.</div>
                    ` : filters.map((f, i) => `
                        <div class="filter-row">
                            <select class="form-input filter-column" onchange="App.updateFilter(${i}, 'column', this.value)">
                                ${columns.map(c => `
                                    <option value="${c.name}" ${f.column === c.name ? 'selected' : ''}>${c.name}</option>
                                `).join('')}
                            </select>
                            <select class="form-input filter-operator" onchange="App.updateFilter(${i}, 'operator', this.value)">
                                ${operators.map(o => `
                                    <option value="${o.value}" ${f.operator === o.value ? 'selected' : ''}>${o.label}</option>
                                `).join('')}
                            </select>
                            <input type="text" class="form-input filter-value" value="${f.value}"
                                placeholder="value" onchange="App.updateFilter(${i}, 'value', this.value)"
                                onkeydown="if(event.key==='Enter')App.applyFilters()">
                            <button class="btn-icon filter-remove" onclick="App.removeFilter(${i})">&times;</button>
                        </div>
                    `).join('')}
                </div>
                <div class="filter-actions">
                    <button class="btn btn-secondary btn-sm" onclick="App.addFilter()">+ Add filter</button>
                    ${filters.length > 0 ? `
                        <button class="btn btn-primary btn-sm" onclick="App.applyFilters()">Apply</button>
                        <button class="btn btn-secondary btn-sm" onclick="App.clearFilters()">Clear all</button>
                    ` : ''}
                </div>
            </div>
        `;
    },

    // Data grid rendering
    renderDataGrid() {
        const { schema, data, selectedRows, sort } = this.state.tables;
        if (!schema || !schema.columns) return '';

        const columns = schema.columns;
        const primaryKey = columns.find(c => c.primary)?.name || columns[0]?.name;

        const getSortIndicator = (colName) => {
            if (sort.column !== colName) return '<span class="sort-indicator"></span>';
            if (sort.direction === 'asc') return '<span class="sort-indicator sort-asc">▲</span>';
            if (sort.direction === 'desc') return '<span class="sort-indicator sort-desc">▼</span>';
            return '<span class="sort-indicator"></span>';
        };

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
                                <th class="sortable-header" onclick="App.toggleSort('${col.name}')">
                                    <div class="header-content">
                                        <span class="header-name">${col.name}</span>
                                        ${getSortIndicator(col.name)}
                                    </div>
                                    <span class="col-type">${col.type}</span>
                                </th>
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
        const isSelected = this.state.tables.selectedRows.has(String(rowId));
        const { editingCell } = this.state.tables;

        return `
            <tr class="${isSelected ? 'selected' : ''}">
                <td class="checkbox-col">
                    <input type="checkbox" ${isSelected ? 'checked' : ''}
                        onchange="App.toggleRow('${rowId}', this.checked)">
                </td>
                ${columns.map(col => {
                    const isEditing = String(editingCell?.rowId) === String(rowId) && editingCell?.column === col.name;
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
            data.forEach(row => this.state.tables.selectedRows.add(String(row[primaryKey])));
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
            case 'schema':
                content = this.renderSchemaModal();
                break;
            case 'addColumn':
                content = this.renderAddColumnModal();
                break;
            case 'userDetail':
                content = this.renderUserDetailModal();
                break;
            case 'createUser':
                content = this.renderCreateUserModal();
                break;
            case 'inviteUser':
                content = this.renderInviteUserModal();
                break;
            case 'createPolicy':
            case 'editPolicy':
                content = this.renderPolicyModal();
                break;
            case 'regenerateSecret':
                content = this.renderRegenerateSecretModal();
                break;
            case 'confirmDestructive':
                content = this.renderConfirmDestructiveModal();
                break;
            case 'createFTSIndex':
                content = this.renderCreateFTSIndexModal();
                break;
            case 'ftsSearch':
                content = this.renderFTSSearchModal();
                break;
            case 'createFunction':
                content = this.renderCreateFunctionModal();
                break;
            case 'addSecret':
                content = this.renderAddSecretModal();
                break;
            case 'createBucket':
                content = this.renderCreateBucketModal();
                break;
            case 'bucketSettings':
                content = this.renderBucketSettingsModal();
                break;
            case 'filePreview':
                content = this.renderFilePreviewModal();
                break;
        }

        // Use larger modal for policy editing and FTS search
        const isLargeModal = type === 'createPolicy' || type === 'editPolicy' || type === 'ftsSearch';
        const isFilePreview = type === 'filePreview';
        const modalClass = isFilePreview ? 'modal modal-file-preview' : (isLargeModal ? 'modal modal-large' : 'modal');

        return `
            <div class="modal-overlay${isFilePreview ? ' modal-overlay-dark' : ''}" onclick="App.closeModal()">
                <div class="${modalClass}" onclick="event.stopPropagation()">
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
        const row = data.find(r => String(r[primaryKey]) === String(rowId));

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

    // Schema management
    showSchemaModal() {
        this.state.modal = { type: 'schema', data: {} };
        this.render();
    },

    showAddColumnModal() {
        this.state.modal = {
            type: 'addColumn',
            data: { name: '', type: 'text', nullable: true }
        };
        this.render();
    },

    async addColumn() {
        const { data } = this.state.modal;
        const { selected } = this.state.tables;

        try {
            const res = await fetch(`/_/api/tables/${selected}/columns`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });

            if (res.ok) {
                this.closeModal();
                await this.loadTableSchema(selected);
                await this.loadTableData();
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to add column';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to add column';
            this.render();
        }
    },

    async renameColumn(oldName) {
        const newName = prompt('New column name:', oldName);
        if (!newName || newName === oldName) return;

        const { selected } = this.state.tables;

        try {
            const res = await fetch(`/_/api/tables/${selected}/columns/${oldName}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ new_name: newName })
            });

            if (res.ok) {
                await this.loadTableSchema(selected);
                await this.loadTableData();
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to rename column';
            }
        } catch (e) {
            this.state.error = 'Failed to rename column';
        }
        this.render();
    },

    async dropColumn(colName) {
        if (!confirm(`Drop column "${colName}"? Data in this column will be lost.`)) return;

        const { selected } = this.state.tables;

        try {
            const res = await fetch(`/_/api/tables/${selected}/columns/${colName}`, {
                method: 'DELETE'
            });

            if (res.ok) {
                await this.loadTableSchema(selected);
                await this.loadTableData();
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to drop column';
            }
        } catch (e) {
            this.state.error = 'Failed to drop column';
        }
        this.render();
    },

    // FTS Index Management
    async loadFTSIndexes(tableName) {
        try {
            const res = await fetch(`/_/api/tables/${tableName}/fts`);
            if (res.ok) {
                const data = await res.json();
                this.state.tables.ftsIndexes = data.indexes || [];
            } else {
                this.state.tables.ftsIndexes = [];
            }
        } catch (e) {
            this.state.tables.ftsIndexes = [];
        }
    },

    showCreateFTSIndexModal() {
        const { schema } = this.state.tables;
        // Get text columns for FTS indexing
        const textColumns = schema.columns.filter(c => c.type === 'text').map(c => c.name);
        this.state.modal = {
            type: 'createFTSIndex',
            data: {
                name: '',
                columns: [],
                tokenizer: 'unicode61',
                availableColumns: textColumns
            }
        };
        this.render();
    },

    async createFTSIndex() {
        const { data } = this.state.modal;
        const { selected } = this.state.tables;

        if (!data.name) {
            this.state.error = 'Index name is required';
            this.render();
            return;
        }

        if (data.columns.length === 0) {
            this.state.error = 'At least one column is required';
            this.render();
            return;
        }

        try {
            const res = await fetch(`/_/api/tables/${selected}/fts`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    name: data.name,
                    columns: data.columns,
                    tokenizer: data.tokenizer
                })
            });

            if (res.ok) {
                this.closeModal();
                await this.loadFTSIndexes(selected);
                this.showSchemaModal();
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to create FTS index';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to create FTS index';
            this.render();
        }
    },

    async deleteFTSIndex(indexName) {
        if (!confirm(`Delete FTS index "${indexName}"? This will disable full-text search on these columns.`)) return;

        const { selected } = this.state.tables;

        try {
            const res = await fetch(`/_/api/tables/${selected}/fts/${indexName}`, {
                method: 'DELETE'
            });

            if (res.ok) {
                await this.loadFTSIndexes(selected);
                this.render();
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to delete FTS index';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to delete FTS index';
            this.render();
        }
    },

    async rebuildFTSIndex(indexName) {
        const { selected } = this.state.tables;

        try {
            const res = await fetch(`/_/api/tables/${selected}/fts/${indexName}/rebuild`, {
                method: 'POST'
            });

            if (res.ok) {
                this.state.error = null;
                alert('Index rebuilt successfully');
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to rebuild FTS index';
            }
        } catch (e) {
            this.state.error = 'Failed to rebuild FTS index';
        }
        this.render();
    },

    showFTSSearchModal(indexName) {
        const { selected, ftsIndexes } = this.state.tables;
        const index = ftsIndexes.find(i => i.index_name === indexName);

        this.state.modal = {
            type: 'ftsSearch',
            data: {
                indexName: indexName,
                index: index,
                query: '',
                queryType: 'plain',
                results: null,
                loading: false,
                error: null
            }
        };
        this.render();
    },

    async testFTSSearch() {
        const { data } = this.state.modal;
        const { selected } = this.state.tables;

        if (!data.query) {
            return;
        }

        data.loading = true;
        data.error = null;
        this.render();

        try {
            const res = await fetch(`/_/api/tables/${selected}/fts/test`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    index_name: data.indexName,
                    query: data.query,
                    query_type: data.queryType,
                    limit: 10
                })
            });

            const result = await res.json();
            data.loading = false;

            if (result.success) {
                data.results = result.results;
                data.ftsQuery = result.fts_query;
                data.error = null;
            } else {
                data.results = null;
                data.error = result.error;
            }
        } catch (e) {
            data.loading = false;
            data.error = 'Search failed';
        }
        this.render();
    },

    toggleFTSColumn(colName) {
        const { data } = this.state.modal;
        const idx = data.columns.indexOf(colName);
        if (idx === -1) {
            data.columns.push(colName);
        } else {
            data.columns.splice(idx, 1);
        }
        this.render();
    },

    renderSchemaModal() {
        const { schema, ftsIndexes } = this.state.tables;

        return `
            <div class="modal-header">
                <h3>Schema: ${schema.name}</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <h4 style="margin-bottom: 0.5rem;">Columns</h4>
                <table class="schema-table">
                    <thead>
                        <tr><th>Column</th><th>Type</th><th>Nullable</th><th>Primary</th><th></th></tr>
                    </thead>
                    <tbody>
                        ${schema.columns.map(col => `
                            <tr>
                                <td>${col.name}</td>
                                <td>${col.type}</td>
                                <td>${col.nullable ? 'Yes' : 'No'}</td>
                                <td>${col.primary ? 'Yes' : ''}</td>
                                <td>
                                    <button class="btn-icon" onclick="App.renameColumn('${col.name}')">Rename</button>
                                    ${!col.primary ? `<button class="btn-icon" onclick="App.dropColumn('${col.name}')">Drop</button>` : ''}
                                </td>
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
                <div style="margin-top: 0.5rem;">
                    <button class="btn btn-secondary btn-sm" onclick="App.showAddColumnModal()">+ Add Column</button>
                </div>

                <hr style="margin: 1rem 0; border-color: var(--border);">

                <h4 style="margin-bottom: 0.5rem;">Full-Text Search Indexes</h4>
                ${ftsIndexes.length === 0 ? `
                    <p style="color: var(--text-muted); font-size: 0.875rem;">No FTS indexes. Create one to enable full-text search.</p>
                ` : `
                    <table class="schema-table">
                        <thead>
                            <tr><th>Index Name</th><th>Columns</th><th>Tokenizer</th><th></th></tr>
                        </thead>
                        <tbody>
                            ${ftsIndexes.map(idx => `
                                <tr>
                                    <td>${idx.index_name}</td>
                                    <td>${idx.columns.join(', ')}</td>
                                    <td>${idx.tokenizer}</td>
                                    <td>
                                        <button class="btn-icon" onclick="App.showFTSSearchModal('${idx.index_name}')" title="Test Search">Test</button>
                                        <button class="btn-icon" onclick="App.rebuildFTSIndex('${idx.index_name}')" title="Rebuild Index">Rebuild</button>
                                        <button class="btn-icon" onclick="App.deleteFTSIndex('${idx.index_name}')" title="Delete Index">Delete</button>
                                    </td>
                                </tr>
                            `).join('')}
                        </tbody>
                    </table>
                `}
                <div style="margin-top: 0.5rem;">
                    <button class="btn btn-secondary btn-sm" onclick="App.showCreateFTSIndexModal()">+ Create FTS Index</button>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-primary" onclick="App.closeModal()">Done</button>
            </div>
        `;
    },

    renderAddColumnModal() {
        const { data } = this.state.modal;
        const types = ['uuid', 'text', 'integer', 'boolean', 'timestamptz', 'jsonb', 'numeric', 'bytea'];

        return `
            <div class="modal-header">
                <h3>Add Column</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="form-group">
                    <label class="form-label">Column Name</label>
                    <input type="text" class="form-input" value="${data.name}"
                        onchange="App.updateModalData('name', this.value)">
                </div>
                <div class="form-group">
                    <label class="form-label">Type</label>
                    <select class="form-input" onchange="App.updateModalData('type', this.value)">
                        ${types.map(t => `<option value="${t}" ${data.type === t ? 'selected' : ''}>${t}</option>`).join('')}
                    </select>
                </div>
                <div class="form-group">
                    <label><input type="checkbox" ${data.nullable ? 'checked' : ''}
                        onchange="App.updateModalData('nullable', this.checked)"> Nullable</label>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.showSchemaModal()">Back</button>
                <button class="btn btn-primary" onclick="App.addColumn()">Add Column</button>
            </div>
        `;
    },

    renderCreateFTSIndexModal() {
        const { data } = this.state.modal;
        const tokenizers = [
            { value: 'unicode61', label: 'Unicode61 (Default)', desc: 'Unicode-aware, multi-language support' },
            { value: 'porter', label: 'Porter Stemming', desc: 'English stemming (run/running/runs match)' },
            { value: 'ascii', label: 'ASCII', desc: 'Simple ASCII tokenizer' },
            { value: 'trigram', label: 'Trigram', desc: 'Character trigrams for fuzzy/substring matching' }
        ];

        return `
            <div class="modal-header">
                <h3>Create FTS Index</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="form-group">
                    <label class="form-label">Index Name</label>
                    <input type="text" class="form-input" value="${data.name}" placeholder="e.g., search"
                        onchange="App.updateModalData('name', this.value)">
                </div>
                <div class="form-group">
                    <label class="form-label">Columns to Index</label>
                    ${data.availableColumns.length === 0 ? `
                        <p style="color: var(--text-muted); font-size: 0.875rem;">No text columns available. FTS indexes require text columns.</p>
                    ` : `
                        <div style="display: flex; flex-wrap: wrap; gap: 0.5rem;">
                            ${data.availableColumns.map(col => `
                                <label class="checkbox-label" style="display: inline-flex; align-items: center; gap: 0.25rem;">
                                    <input type="checkbox" ${data.columns.includes(col) ? 'checked' : ''}
                                        onchange="App.toggleFTSColumn('${col}')"> ${col}
                                </label>
                            `).join('')}
                        </div>
                    `}
                </div>
                <div class="form-group">
                    <label class="form-label">Tokenizer</label>
                    <select class="form-input" onchange="App.updateModalData('tokenizer', this.value)">
                        ${tokenizers.map(t => `<option value="${t.value}" ${data.tokenizer === t.value ? 'selected' : ''}>${t.label}</option>`).join('')}
                    </select>
                    <p style="color: var(--text-muted); font-size: 0.75rem; margin-top: 0.25rem;">
                        ${tokenizers.find(t => t.value === data.tokenizer)?.desc || ''}
                    </p>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.showSchemaModal()">Back</button>
                <button class="btn btn-primary" onclick="App.createFTSIndex()" ${data.columns.length === 0 ? 'disabled' : ''}>Create Index</button>
            </div>
        `;
    },

    renderFTSSearchModal() {
        const { data } = this.state.modal;
        const queryTypes = [
            { value: 'plain', label: 'Plain', desc: 'All terms must match (AND)' },
            { value: 'phrase', label: 'Phrase', desc: 'Exact phrase match' },
            { value: 'websearch', label: 'Websearch', desc: 'Google-like syntax (OR, -negation, "quotes")' },
            { value: 'fts', label: 'FTS Query', desc: "PostgreSQL tsquery syntax ('term' & 'term')" }
        ];

        return `
            <div class="modal-header">
                <h3>Test FTS Search: ${data.indexName}</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="form-group">
                    <label class="form-label">Indexed Columns: <span style="font-weight: normal;">${data.index?.columns?.join(', ') || ''}</span></label>
                </div>
                <div class="form-group">
                    <label class="form-label">Search Query</label>
                    <input type="text" class="form-input" value="${data.query}" placeholder="Enter search terms..."
                        onchange="App.updateModalData('query', this.value)"
                        onkeydown="if(event.key === 'Enter') App.testFTSSearch()">
                </div>
                <div class="form-group">
                    <label class="form-label">Query Type</label>
                    <select class="form-input" onchange="App.updateModalData('queryType', this.value)">
                        ${queryTypes.map(t => `<option value="${t.value}" ${data.queryType === t.value ? 'selected' : ''}>${t.label} - ${t.desc}</option>`).join('')}
                    </select>
                </div>
                <div style="margin-bottom: 1rem;">
                    <button class="btn btn-primary" onclick="App.testFTSSearch()" ${data.loading ? 'disabled' : ''}>
                        ${data.loading ? 'Searching...' : 'Search'}
                    </button>
                </div>

                ${data.error ? `
                    <div class="alert alert-danger" style="margin-bottom: 1rem;">${data.error}</div>
                ` : ''}

                ${data.ftsQuery ? `
                    <div style="margin-bottom: 1rem; padding: 0.5rem; background: var(--bg-secondary); border-radius: 4px;">
                        <small style="color: var(--text-muted);">FTS5 Query: <code>${data.ftsQuery}</code></small>
                    </div>
                ` : ''}

                ${data.results !== null ? `
                    <div class="form-label">Results (${data.results.length})</div>
                    ${data.results.length === 0 ? `
                        <p style="color: var(--text-muted);">No results found</p>
                    ` : `
                        <div style="max-height: 300px; overflow: auto;">
                            <table class="data-table">
                                <thead>
                                    <tr>${Object.keys(data.results[0] || {}).map(k => `<th>${k}</th>`).join('')}</tr>
                                </thead>
                                <tbody>
                                    ${data.results.map(row => `
                                        <tr>${Object.values(row).map(v => `<td>${v !== null ? String(v).substring(0, 100) : '<null>'}</td>`).join('')}</tr>
                                    `).join('')}
                                </tbody>
                            </table>
                        </div>
                    `}
                ` : ''}
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.showSchemaModal()">Back</button>
            </div>
        `;
    },

    // User management methods

    async loadUsers() {
        this.state.users.loading = true;
        this.render();

        try {
            const { page, pageSize, filter } = this.state.users;
            const offset = (page - 1) * pageSize;
            const res = await fetch(`/_/api/users?limit=${pageSize}&offset=${offset}&filter=${filter || 'all'}`);
            if (res.ok) {
                const data = await res.json();
                this.state.users.list = data.users;
                this.state.users.totalUsers = data.total;
            }
        } catch (e) {
            this.state.error = 'Failed to load users';
        }
        this.state.users.loading = false;
        this.render();
    },

    renderUsersView() {
        const { loading, list, page, pageSize, totalUsers } = this.state.users;

        if (loading) {
            return '<div class="loading">Loading...</div>';
        }

        const totalPages = Math.ceil(totalUsers / pageSize);

        return `
            <div class="card-title">Users</div>
            <div class="users-view">
                <div class="table-toolbar">
                    <h2>Users</h2>
                    <div class="toolbar-actions">
                        <select class="form-input" style="width: auto;"
                                onchange="App.setUserFilter(this.value)">
                            <option value="all" ${this.state.users.filter === 'all' ? 'selected' : ''}>All Users</option>
                            <option value="regular" ${this.state.users.filter === 'regular' ? 'selected' : ''}>Regular</option>
                            <option value="anonymous" ${this.state.users.filter === 'anonymous' ? 'selected' : ''}>Anonymous</option>
                        </select>
                        <button class="btn btn-primary btn-sm" onclick="App.showCreateUserModal()">+ Create User</button>
                        <button class="btn btn-secondary btn-sm" onclick="App.showInviteUserModal()">Invite User</button>
                        <span class="text-muted">${totalUsers} user${totalUsers !== 1 ? 's' : ''}</span>
                    </div>
                </div>

                <div class="data-grid-container">
                    <table class="data-grid">
                        <thead>
                            <tr>
                                <th>Email</th>
                                <th>Created</th>
                                <th>Last Sign In</th>
                                <th>Confirmed</th>
                                <th class="actions-col"></th>
                            </tr>
                        </thead>
                        <tbody>
                            ${list.length === 0
                                ? '<tr><td colspan="5" class="empty-state">No users</td></tr>'
                                : list.map(user => this.renderUserRow(user)).join('')}
                        </tbody>
                    </table>
                </div>

                ${this.renderUsersPagination()}
            </div>
        `;
    },

    renderUserRow(user) {
        const confirmed = user.email_confirmed_at ? '✓' : '—';
        const createdAt = user.created_at ? this.formatDate(user.created_at) : '—';
        const lastSignIn = user.last_sign_in_at ? this.formatDate(user.last_sign_in_at) : 'Never';

        return `
            <tr>
                <td>
                    ${user.is_anonymous ? `
                        <span class="text-muted">(anonymous)</span>
                        <span class="badge badge-muted">Anon</span>
                    ` : this.escapeHtml(user.email || '')}
                </td>
                <td>${createdAt}</td>
                <td>${lastSignIn}</td>
                <td class="${user.email_confirmed_at ? 'text-success' : 'text-muted'}">${confirmed}</td>
                <td class="actions-col">
                    <button class="btn-icon" onclick="App.showUserModal('${user.id}')">View</button>
                    <button class="btn-icon" onclick="App.confirmDeleteUser('${user.id}', '${user.email || ''}')">Delete</button>
                </td>
            </tr>
        `;
    },

    formatDate(dateStr) {
        if (!dateStr) return '—';
        const date = new Date(dateStr);
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
    },

    renderUsersPagination() {
        const { page, pageSize, totalUsers } = this.state.users;
        const totalPages = Math.ceil(totalUsers / pageSize);

        return `
            <div class="pagination">
                <div class="pagination-info">
                    ${totalUsers} users | Page ${page} of ${totalPages || 1}
                </div>
                <div class="pagination-controls">
                    <select onchange="App.changeUsersPageSize(this.value)">
                        <option value="25" ${pageSize === 25 ? 'selected' : ''}>25</option>
                        <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                        <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                    </select>
                    <button class="btn btn-secondary btn-sm" onclick="App.prevUsersPage()" ${page <= 1 ? 'disabled' : ''}>Prev</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.nextUsersPage()" ${page >= totalPages ? 'disabled' : ''}>Next</button>
                </div>
            </div>
        `;
    },

    changeUsersPageSize(size) {
        this.state.users.pageSize = parseInt(size);
        this.state.users.page = 1;
        this.loadUsers();
    },

    setUserFilter(filter) {
        this.state.users.filter = filter;
        this.state.users.page = 1;
        this.loadUsers();
    },

    prevUsersPage() {
        if (this.state.users.page > 1) {
            this.state.users.page--;
            this.loadUsers();
        }
    },

    nextUsersPage() {
        const totalPages = Math.ceil(this.state.users.totalUsers / this.state.users.pageSize);
        if (this.state.users.page < totalPages) {
            this.state.users.page++;
            this.loadUsers();
        }
    },

    async showUserModal(userId) {
        try {
            const res = await fetch(`/_/api/users/${userId}`);
            if (res.ok) {
                const user = await res.json();
                this.state.modal = { type: 'userDetail', data: user };
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to load user';
            this.render();
        }
    },

    async confirmDeleteUser(userId, email) {
        // Find the user in the list to check if they're anonymous
        const user = this.state.users.list.find(u => u.id === userId);

        let confirmMessage;
        if (user && user.is_anonymous) {
            // Enhanced dialog for anonymous users
            const truncatedId = userId.length > 20 ? userId.substring(0, 20) + '...' : userId;
            const createdDate = user.created_at ? new Date(user.created_at).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric'
            }) : 'Unknown';

            confirmMessage = `Delete Anonymous User?\n\n` +
                `This will permanently delete this anonymous user and all associated data.\n\n` +
                `User ID: ${truncatedId}\n` +
                `Created: ${createdDate}\n\n` +
                `This action cannot be undone.`;
        } else {
            // Regular user confirmation
            confirmMessage = `Delete user "${email}"? This will also delete their sessions and cannot be undone.`;
        }

        if (!confirm(confirmMessage)) return;

        try {
            const res = await fetch(`/_/api/users/${userId}`, { method: 'DELETE' });
            if (res.ok) {
                await this.loadUsers();
            } else {
                this.state.error = 'Failed to delete user';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to delete user';
            this.render();
        }
    },

    async updateUser() {
        const { data } = this.state.modal;
        const userId = data.id;

        const updateData = {
            raw_user_meta_data: data.raw_user_meta_data,
            email_confirmed: data.email_confirmed_at !== null
        };

        try {
            const res = await fetch(`/_/api/users/${userId}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(updateData)
            });

            if (res.ok) {
                this.closeModal();
                await this.loadUsers();
            } else {
                const err = await res.json();
                this.state.error = err.error || 'Failed to update user';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to update user';
            this.render();
        }
    },

    updateUserField(field, value) {
        this.state.modal.data[field] = value;
        this.render();
    },

    toggleUserEmailConfirmed() {
        const { data } = this.state.modal;
        if (data.email_confirmed_at) {
            data.email_confirmed_at = null;
        } else {
            data.email_confirmed_at = new Date().toISOString();
        }
        this.render();
    },

    renderUserDetailModal() {
        const { data } = this.state.modal;
        const confirmed = data.email_confirmed_at !== null;

        return `
            <div class="modal-header">
                <h3>User Details</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="form-group">
                    <label class="form-label">ID</label>
                    <input type="text" class="form-input" value="${data.id}" disabled>
                </div>
                <div class="form-group">
                    <label class="form-label">Email</label>
                    <input type="text" class="form-input" value="${data.email}" disabled>
                </div>
                <div class="form-group">
                    <label class="form-label">Created</label>
                    <input type="text" class="form-input" value="${this.formatDate(data.created_at)}" disabled>
                </div>
                <div class="form-group">
                    <label class="form-label">Last Sign In</label>
                    <input type="text" class="form-input" value="${data.last_sign_in_at ? this.formatDate(data.last_sign_in_at) : 'Never'}" disabled>
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" ${confirmed ? 'checked' : ''} onchange="App.toggleUserEmailConfirmed()">
                        Email Confirmed
                    </label>
                </div>
                <div class="form-group">
                    <label class="form-label">User Metadata (JSON)</label>
                    <textarea class="form-input" rows="4"
                        onchange="App.updateUserField('raw_user_meta_data', this.value)">${data.raw_user_meta_data || '{}'}</textarea>
                </div>
                <div class="form-group">
                    <label class="form-label">App Metadata (JSON)</label>
                    <textarea class="form-input" rows="3" disabled>${data.raw_app_meta_data || '{}'}</textarea>
                    <small class="text-muted">App metadata is read-only</small>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.updateUser()">Save Changes</button>
            </div>
        `;
    },

    // Create User modal methods
    showCreateUserModal() {
        this.state.modal = {
            type: 'createUser',
            data: { email: '', password: '', autoConfirm: true, error: null }
        };
        this.render();
    },

    updateCreateUserField(field, value) {
        this.state.modal.data[field] = value;
    },

    async createUser() {
        const { email, password, autoConfirm } = this.state.modal.data;

        // Validate
        if (!email || !email.includes('@')) {
            this.state.modal.data.error = 'Please enter a valid email address';
            this.render();
            return;
        }
        if (!password || password.length < 6) {
            this.state.modal.data.error = 'Password must be at least 6 characters';
            this.render();
            return;
        }

        try {
            const res = await fetch('/_/api/users', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email, password, auto_confirm: autoConfirm })
            });

            if (res.ok) {
                this.closeModal();
                await this.loadUsers();
            } else {
                const err = await res.json();
                this.state.modal.data.error = err.error || 'Failed to create user';
                this.render();
            }
        } catch (e) {
            this.state.modal.data.error = 'Failed to create user';
            this.render();
        }
    },

    renderCreateUserModal() {
        const { email, password, autoConfirm, error } = this.state.modal.data;

        return `
            <div class="modal-header">
                <h3>Create User</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                ${error ? `<div class="message message-error">${error}</div>` : ''}
                <div class="form-group">
                    <label class="form-label">Email</label>
                    <input type="email" class="form-input" value="${email}" placeholder="user@example.com"
                        onchange="App.updateCreateUserField('email', this.value)">
                </div>
                <div class="form-group">
                    <label class="form-label">Password</label>
                    <input type="password" class="form-input" value="${password}" placeholder="Minimum 6 characters"
                        onchange="App.updateCreateUserField('password', this.value)">
                </div>
                <div class="form-group">
                    <label>
                        <input type="checkbox" ${autoConfirm ? 'checked' : ''}
                            onchange="App.updateCreateUserField('autoConfirm', this.checked)">
                        Auto-confirm email
                    </label>
                    <small class="text-muted" style="display: block; margin-top: 4px;">
                        Skip email verification and mark user as confirmed immediately
                    </small>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.createUser()">Create User</button>
            </div>
        `;
    },

    // Invite User modal methods
    showInviteUserModal() {
        this.state.modal = {
            type: 'inviteUser',
            data: { email: '', error: null, success: false, inviteLink: '' }
        };
        this.render();
    },

    updateInviteUserField(field, value) {
        this.state.modal.data[field] = value;
    },

    async sendInvite() {
        const { email } = this.state.modal.data;

        // Validate
        if (!email || !email.includes('@')) {
            this.state.modal.data.error = 'Please enter a valid email address';
            this.render();
            return;
        }

        try {
            const res = await fetch('/_/api/users/invite', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ email })
            });

            const data = await res.json();
            if (res.ok) {
                this.state.modal.data.success = true;
                this.state.modal.data.inviteLink = data.invite_link;
                this.state.modal.data.error = null;
                this.render();
            } else {
                this.state.modal.data.error = data.error || 'Failed to send invite';
                this.render();
            }
        } catch (e) {
            this.state.modal.data.error = 'Failed to send invite';
            this.render();
        }
    },

    async copyInviteLink() {
        const { inviteLink } = this.state.modal.data;
        try {
            await navigator.clipboard.writeText(inviteLink);
            // Show brief feedback
            const btn = document.querySelector('.copy-link-btn');
            if (btn) {
                const original = btn.textContent;
                btn.textContent = 'Copied!';
                setTimeout(() => { btn.textContent = original; }, 1500);
            }
        } catch (e) {
            // Fallback: select the text
            const input = document.querySelector('.invite-link-input');
            if (input) {
                input.select();
            }
        }
    },

    renderInviteUserModal() {
        const { email, error, success, inviteLink } = this.state.modal.data;

        if (success) {
            return `
                <div class="modal-header">
                    <h3>Invite Sent</h3>
                    <button class="btn-icon" onclick="App.closeModal()">&times;</button>
                </div>
                <div class="modal-body">
                    <div class="message message-success">
                        Invitation created for ${email}
                    </div>
                    <div class="form-group">
                        <label class="form-label">Invite Link</label>
                        <div style="display: flex; gap: 8px;">
                            <input type="text" class="form-input invite-link-input" value="${inviteLink}" readonly
                                style="flex: 1; font-size: 12px;">
                            <button class="btn btn-secondary copy-link-btn" onclick="App.copyInviteLink()">Copy Link</button>
                        </div>
                        <small class="text-muted" style="display: block; margin-top: 4px;">
                            Share this link with the user. The link expires in 7 days.
                        </small>
                    </div>
                </div>
                <div class="modal-footer">
                    <button class="btn btn-primary" onclick="App.closeModal(); App.loadUsers();">Done</button>
                </div>
            `;
        }

        return `
            <div class="modal-header">
                <h3>Invite User</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                ${error ? `<div class="message message-error">${error}</div>` : ''}
                <div class="form-group">
                    <label class="form-label">Email</label>
                    <input type="email" class="form-input" value="${email}" placeholder="user@example.com"
                        onchange="App.updateInviteUserField('email', this.value)">
                </div>
                <small class="text-muted">
                    An invitation will be created. The user will need to use the invite link to set their password.
                </small>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.sendInvite()">Send Invite</button>
            </div>
        `;
    },

    // ========================================================================
    // RLS Policy Management
    // ========================================================================

    async loadPoliciesTables() {
        this.state.policies.loading = true;
        this.render();

        try {
            // Load tables list
            const tablesRes = await fetch('/_/api/tables');
            if (tablesRes.ok) {
                const tables = await tablesRes.json();

                // For each table, get RLS status
                const tablesWithRLS = await Promise.all(tables.map(async (t) => {
                    const rlsRes = await fetch(`/_/api/tables/${t.name}/rls`);
                    if (rlsRes.ok) {
                        const rlsData = await rlsRes.json();
                        return { ...t, rls_enabled: rlsData.rls_enabled, policy_count: rlsData.policy_count };
                    }
                    return { ...t, rls_enabled: false, policy_count: 0 };
                }));

                this.state.policies.tables = tablesWithRLS;
            }
        } catch (e) {
            this.state.error = 'Failed to load tables';
        }
        this.state.policies.loading = false;
        this.render();
    },

    async selectPolicyTable(tableName) {
        this.state.policies.selectedTable = tableName;
        this.state.policies.loading = true;
        this.render();

        try {
            const res = await fetch(`/_/api/policies?table=${tableName}`);
            if (res.ok) {
                const data = await res.json();
                this.state.policies.list = data.policies || [];
            }
        } catch (e) {
            this.state.error = 'Failed to load policies';
        }
        this.state.policies.loading = false;
        this.render();
    },

    async toggleTableRLS(tableName, enabled) {
        try {
            const res = await fetch(`/_/api/tables/${tableName}/rls`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled })
            });
            if (res.ok) {
                // Update local state
                const table = this.state.policies.tables.find(t => t.name === tableName);
                if (table) table.rls_enabled = enabled;
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to update RLS state';
            this.render();
        }
    },

    renderPoliciesView() {
        const { tables, selectedTable, list, loading } = this.state.policies;

        if (loading && tables.length === 0) {
            return '<div class="loading">Loading...</div>';
        }

        return `
            <div class="card-title">RLS Policies</div>
            <div class="policies-layout">
                <div class="policy-tables-panel">
                    <div class="panel-header">
                        <span>Tables</span>
                    </div>
                    <div class="table-list">
                        ${tables.length === 0
                            ? '<div class="empty-state">No tables yet</div>'
                            : tables.map(t => `
                                <div class="table-list-item ${selectedTable === t.name ? 'active' : ''}"
                                     onclick="App.selectPolicyTable('${t.name}')">
                                    <div class="table-item-info">
                                        <span class="table-name">${t.name}</span>
                                        ${t.policy_count > 0 ? `<span class="policy-badge">${t.policy_count}</span>` : ''}
                                    </div>
                                    <label class="rls-toggle" onclick="event.stopPropagation()">
                                        <input type="checkbox" ${t.rls_enabled ? 'checked' : ''}
                                            onchange="App.toggleTableRLS('${t.name}', this.checked)">
                                        <span class="toggle-label">RLS</span>
                                    </label>
                                </div>
                            `).join('')}
                    </div>
                </div>
                <div class="policy-content-panel">
                    ${selectedTable ? this.renderPolicyContent() : '<div class="empty-state">Select a table to manage policies</div>'}
                </div>
            </div>
        `;
    },

    renderPolicyContent() {
        const { selectedTable, list, loading } = this.state.policies;
        const table = this.state.policies.tables.find(t => t.name === selectedTable);
        const rlsEnabled = table?.rls_enabled || false;

        if (loading) {
            return '<div class="loading">Loading...</div>';
        }

        return `
            <div class="table-toolbar">
                <h2>${selectedTable}</h2>
                <div class="toolbar-actions">
                    <button class="btn btn-primary btn-sm" onclick="App.showCreatePolicyModal()">+ New Policy</button>
                </div>
            </div>
            ${!rlsEnabled ? `
                <div class="message message-warning" style="margin-bottom: 1rem;">
                    RLS is disabled for this table. All rows are accessible to all users.
                    <button class="btn btn-secondary btn-sm" style="margin-left: 1rem;"
                        onclick="App.toggleTableRLS('${selectedTable}', true)">Enable RLS</button>
                </div>
            ` : ''}
            ${rlsEnabled && list.length === 0 ? `
                <div class="message message-error" style="margin-bottom: 1rem;">
                    RLS is enabled but no policies exist. All access is currently denied.
                </div>
            ` : ''}
            <div class="policies-list">
                ${list.length === 0
                    ? '<div class="empty-state">No policies for this table. Click "+ New Policy" to create one.</div>'
                    : list.map(p => this.renderPolicyCard(p)).join('')}
            </div>
        `;
    },

    renderPolicyCard(policy) {
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
                                onchange="App.togglePolicyEnabled(${policy.id}, this.checked)">
                            <span class="toggle-label">${policy.enabled ? 'Enabled' : 'Disabled'}</span>
                        </label>
                        <button class="btn-icon" onclick="App.showEditPolicyModal(${policy.id})">Edit</button>
                        <button class="btn-icon" onclick="App.confirmDeletePolicy(${policy.id}, '${policy.policy_name}')">Delete</button>
                    </div>
                </div>
                ${policy.using_expr ? `
                    <div class="policy-expr">
                        <span class="expr-label">USING:</span>
                        <code>${this.truncate(policy.using_expr, 100)}</code>
                    </div>
                ` : ''}
                ${policy.check_expr ? `
                    <div class="policy-expr">
                        <span class="expr-label">CHECK:</span>
                        <code>${this.truncate(policy.check_expr, 100)}</code>
                    </div>
                ` : ''}
            </div>
        `;
    },

    truncate(str, max) {
        if (!str) return '';
        return str.length > max ? str.substring(0, max) + '...' : str;
    },

    async togglePolicyEnabled(policyId, enabled) {
        try {
            const res = await fetch(`/_/api/policies/${policyId}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled })
            });
            if (res.ok) {
                const policy = this.state.policies.list.find(p => p.id === policyId);
                if (policy) policy.enabled = enabled;
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to update policy';
            this.render();
        }
    },

    async confirmDeletePolicy(policyId, policyName) {
        const { list } = this.state.policies;
        const table = this.state.policies.tables.find(t => t.name === this.state.policies.selectedTable);

        let message = `Delete policy "${policyName}"? This cannot be undone.`;
        if (list.length === 1 && table?.rls_enabled) {
            message += '\n\nWarning: This is the only policy on this table. Deleting it will deny all access.';
        }

        if (!confirm(message)) return;

        try {
            const res = await fetch(`/_/api/policies/${policyId}`, { method: 'DELETE' });
            if (res.ok) {
                this.state.policies.list = list.filter(p => p.id !== policyId);
                // Update policy count
                if (table) table.policy_count = Math.max(0, (table.policy_count || 1) - 1);
                this.render();
            } else {
                this.state.error = 'Failed to delete policy';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to delete policy';
            this.render();
        }
    },

    // Policy Modal Methods
    showCreatePolicyModal() {
        this.state.modal = {
            type: 'createPolicy',
            data: {
                table_name: this.state.policies.selectedTable,
                policy_name: '',
                command: 'SELECT',
                using_expr: '',
                check_expr: '',
                enabled: true,
                error: null,
                testResult: null,
                testUserId: '',
                showTest: false
            }
        };
        this.render();
    },

    async showEditPolicyModal(policyId) {
        try {
            const res = await fetch(`/_/api/policies/${policyId}`);
            if (res.ok) {
                const policy = await res.json();
                this.state.modal = {
                    type: 'editPolicy',
                    data: {
                        ...policy,
                        error: null,
                        testResult: null,
                        testUserId: '',
                        showTest: false
                    }
                };
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to load policy';
            this.render();
        }
    },

    updatePolicyField(field, value) {
        this.state.modal.data[field] = value;
        this.state.modal.data.testResult = null; // Clear test result on change
        // Only update SQL preview, don't re-render entire modal (which would lose input focus)
        const previewEl = document.querySelector('.sql-preview');
        if (previewEl) {
            previewEl.textContent = this.generatePolicySQL();
        }
        // Clear test result display
        const testResultEl = document.querySelector('.test-result');
        if (testResultEl) {
            testResultEl.remove();
        }
    },

    applyPolicyTemplate(template) {
        const templates = {
            'own_data': { using: 'auth.uid() = user_id', check: 'auth.uid() = user_id' },
            'authenticated_read': { using: 'auth.uid() IS NOT NULL', check: '' },
            'public_read': { using: 'true', check: '' },
            'own_modify': { using: 'auth.uid() = user_id', check: 'auth.uid() = user_id' }
        };
        const t = templates[template];
        if (t) {
            this.state.modal.data.using_expr = t.using;
            this.state.modal.data.check_expr = t.check;
            this.state.modal.data.testResult = null;
            this.render();
        }
    },

    togglePolicyTest() {
        this.state.modal.data.showTest = !this.state.modal.data.showTest;
        this.render();
    },

    async runPolicyTest() {
        const { table_name, using_expr, check_expr, testUserId } = this.state.modal.data;

        try {
            const res = await fetch('/_/api/policies/test', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    table: table_name,
                    using_expr: using_expr,
                    check_expr: check_expr,
                    user_id: testUserId || null
                })
            });
            const result = await res.json();
            this.state.modal.data.testResult = result;
            this.render();
        } catch (e) {
            this.state.modal.data.testResult = { success: false, error: 'Failed to run test' };
            this.render();
        }
    },

    async savePolicy() {
        const { type, data } = this.state.modal;
        const isNew = type === 'createPolicy';

        // Validate
        if (!data.policy_name) {
            this.state.modal.data.error = 'Policy name is required';
            this.render();
            return;
        }
        if (!data.using_expr && !data.check_expr) {
            this.state.modal.data.error = 'At least one expression (USING or CHECK) is required';
            this.render();
            return;
        }

        try {
            let res;
            if (isNew) {
                res = await fetch('/_/api/policies', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        table_name: data.table_name,
                        policy_name: data.policy_name,
                        command: data.command,
                        using_expr: data.using_expr || null,
                        check_expr: data.check_expr || null,
                        enabled: data.enabled
                    })
                });
            } else {
                res = await fetch(`/_/api/policies/${data.id}`, {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        policy_name: data.policy_name,
                        command: data.command,
                        using_expr: data.using_expr || null,
                        check_expr: data.check_expr || null,
                        enabled: data.enabled
                    })
                });
            }

            if (res.ok) {
                this.closeModal();
                await this.selectPolicyTable(data.table_name);
                // Update policy count
                if (isNew) {
                    const table = this.state.policies.tables.find(t => t.name === data.table_name);
                    if (table) table.policy_count = (table.policy_count || 0) + 1;
                }
            } else {
                const err = await res.json();
                this.state.modal.data.error = err.error || 'Failed to save policy';
                this.render();
            }
        } catch (e) {
            this.state.modal.data.error = 'Failed to save policy';
            this.render();
        }
    },

    generatePolicySQL() {
        const { table_name, policy_name, command, using_expr, check_expr } = this.state.modal.data;
        let sql = `CREATE POLICY "${policy_name || 'policy_name'}"\nON ${table_name || 'table_name'}\nFOR ${command}`;
        if (using_expr) {
            sql += `\nUSING (${using_expr})`;
        }
        if (check_expr) {
            sql += `\nWITH CHECK (${check_expr})`;
        }
        return sql + ';';
    },

    renderPolicyModal() {
        const { type, data } = this.state.modal;
        const isNew = type === 'createPolicy';
        const showUsing = ['SELECT', 'UPDATE', 'DELETE', 'ALL'].includes(data.command);
        const showCheck = ['INSERT', 'UPDATE', 'ALL'].includes(data.command);

        return `
            <div class="modal-header">
                <h3>${isNew ? 'Create Policy' : 'Edit Policy'}</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body policy-modal-body">
                ${data.error ? `<div class="message message-error">${data.error}</div>` : ''}

                <div class="policy-form">
                    <div class="form-group">
                        <label class="form-label">Policy Name</label>
                        <input type="text" class="form-input" value="${data.policy_name}"
                            placeholder="e.g., Users can view own data"
                            oninput="App.updatePolicyField('policy_name', this.value)">
                    </div>

                    <div class="form-group">
                        <label class="form-label">Command</label>
                        <select class="form-input" onchange="App.updatePolicyField('command', this.value)">
                            <option value="SELECT" ${data.command === 'SELECT' ? 'selected' : ''}>SELECT</option>
                            <option value="INSERT" ${data.command === 'INSERT' ? 'selected' : ''}>INSERT</option>
                            <option value="UPDATE" ${data.command === 'UPDATE' ? 'selected' : ''}>UPDATE</option>
                            <option value="DELETE" ${data.command === 'DELETE' ? 'selected' : ''}>DELETE</option>
                            <option value="ALL" ${data.command === 'ALL' ? 'selected' : ''}>ALL</option>
                        </select>
                    </div>

                    <div class="form-group">
                        <label class="form-label">Use Template</label>
                        <select class="form-input" onchange="App.applyPolicyTemplate(this.value); this.value='';">
                            <option value="">Select a template...</option>
                            <option value="own_data">Users can access their own data</option>
                            <option value="authenticated_read">Authenticated users can read</option>
                            <option value="public_read">Anyone can read</option>
                            <option value="own_modify">Users can modify their own data</option>
                        </select>
                    </div>

                    ${showUsing ? `
                        <div class="form-group">
                            <label class="form-label">USING Expression</label>
                            <div class="policy-expr-editor">
                                <pre class="policy-expr-highlight" id="using-highlight" aria-hidden="true">${this.highlightSql(data.using_expr || '')}</pre>
                                <textarea class="policy-expr-input"
                                    id="using-input"
                                    spellcheck="false"
                                    placeholder="auth.uid() = user_id"
                                    oninput="App.updatePolicyField('using_expr', this.value); document.getElementById('using-highlight').innerHTML = App.highlightSql(this.value);">${this.escapeHtml(data.using_expr || '')}</textarea>
                            </div>
                            <small class="text-muted">Filters which existing rows can be accessed</small>
                        </div>
                    ` : ''}

                    ${showCheck ? `
                        <div class="form-group">
                            <label class="form-label">CHECK Expression</label>
                            <div class="policy-expr-editor">
                                <pre class="policy-expr-highlight" id="check-highlight" aria-hidden="true">${this.highlightSql(data.check_expr || '')}</pre>
                                <textarea class="policy-expr-input"
                                    id="check-input"
                                    spellcheck="false"
                                    placeholder="auth.uid() = user_id"
                                    oninput="App.updatePolicyField('check_expr', this.value); document.getElementById('check-highlight').innerHTML = App.highlightSql(this.value);">${this.escapeHtml(data.check_expr || '')}</textarea>
                            </div>
                            <small class="text-muted">Validates new or modified data</small>
                        </div>
                    ` : ''}

                    <div class="form-group">
                        <label>
                            <input type="checkbox" ${data.enabled ? 'checked' : ''}
                                onchange="App.updatePolicyField('enabled', this.checked)">
                            Policy enabled
                        </label>
                    </div>
                </div>

                <div class="policy-preview">
                    <label class="form-label">SQL Preview</label>
                    <pre class="sql-preview">${this.highlightSql(this.generatePolicySQL())}</pre>
                </div>

                <div class="policy-test-section">
                    <button class="btn btn-secondary btn-sm" onclick="App.togglePolicyTest()">
                        ${data.showTest ? '▼ Hide Test' : '▶ Test Policy'}
                    </button>
                    ${data.showTest ? this.renderPolicyTestPanel() : ''}
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.savePolicy()">${isNew ? 'Create Policy' : 'Save Changes'}</button>
            </div>
        `;
    },

    renderPolicyTestPanel() {
        const { testUserId, testResult } = this.state.modal.data;

        return `
            <div class="policy-test-panel">
                <div class="form-group">
                    <label class="form-label">Test as User</label>
                    <select class="form-input" onchange="App.updatePolicyField('testUserId', this.value)">
                        <option value="">Anonymous (no user)</option>
                        ${this.state.users.list.map(u => `
                            <option value="${u.id}" ${testUserId === u.id ? 'selected' : ''}>${u.email}</option>
                        `).join('')}
                    </select>
                    <small class="text-muted">Select a user to test auth.uid(), auth.email(), etc.</small>
                </div>
                <button class="btn btn-secondary btn-sm" onclick="App.loadUsersForTest().then(() => App.runPolicyTest())">Run Test</button>
                ${testResult ? `
                    <div class="test-result ${testResult.success ? 'test-success' : 'test-error'}">
                        ${testResult.success
                            ? `<span class="test-icon">✓</span> Policy would allow access to ${testResult.row_count} row${testResult.row_count !== 1 ? 's' : ''}`
                            : `<span class="test-icon">✗</span> Error: ${testResult.error}`}
                        <details>
                            <summary>Show SQL</summary>
                            <pre class="sql-preview">${testResult.executed_sql || 'N/A'}</pre>
                        </details>
                    </div>
                ` : ''}
            </div>
        `;
    },

    async loadUsersForTest() {
        if (this.state.users.list.length === 0) {
            try {
                const res = await fetch('/_/api/users?limit=100');
                if (res.ok) {
                    const data = await res.json();
                    this.state.users.list = data.users || [];
                }
            } catch (e) {
                // Ignore error, test will work with anonymous
            }
        }
    },

    // ========================================================================
    // Settings View Methods
    // ========================================================================

    async loadSettings() {
        this.state.settings.loading = true;
        this.render();

        try {
            const [serverRes, authRes, templatesRes, oauthRes, redirectUrlsRes, apiKeysRes, authConfigRes] = await Promise.all([
                fetch('/_/api/settings/server'),
                fetch('/_/api/settings/auth'),
                fetch('/_/api/settings/templates'),
                fetch('/_/api/settings/oauth'),
                fetch('/_/api/settings/oauth/redirect-urls'),
                fetch('/_/api/apikeys'),
                fetch('/_/api/settings/auth-config')
            ]);

            if (serverRes.ok) {
                this.state.settings.server = await serverRes.json();
            }
            if (authRes.ok) {
                this.state.settings.auth = await authRes.json();
            }
            if (templatesRes.ok) {
                this.state.settings.templates = await templatesRes.json();
            }
            if (oauthRes.ok) {
                this.state.settings.oauth.providers = await oauthRes.json();
            }
            if (redirectUrlsRes.ok) {
                const data = await redirectUrlsRes.json();
                this.state.settings.oauth.redirectUrls = data.urls || [];
            }
            if (apiKeysRes.ok) {
                this.state.settings.apiKeys = await apiKeysRes.json();
            }
            if (authConfigRes.ok) {
                this.state.settings.authConfig = await authConfigRes.json();
            }
        } catch (e) {
            this.state.error = 'Failed to load settings';
        }

        this.state.settings.loading = false;
        this.render();
    },

    toggleSettingsSection(section) {
        this.state.settings.expandedSections[section] = !this.state.settings.expandedSections[section];
        this.render();
    },

    startEditingTemplate(type) {
        const template = this.state.settings.templates.find(t => t.type === type);
        if (template) {
            this.state.settings.editingTemplate = { ...template };
            this.render();
        }
    },

    cancelEditingTemplate() {
        this.state.settings.editingTemplate = null;
        this.render();
    },

    updateTemplateField(field, value) {
        if (this.state.settings.editingTemplate) {
            this.state.settings.editingTemplate[field] = value;
        }
    },

    async saveTemplate() {
        const template = this.state.settings.editingTemplate;
        if (!template) return;

        try {
            const res = await fetch(`/_/api/settings/templates/${template.type}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    subject: template.subject,
                    body_html: template.body_html,
                    body_text: template.body_text
                })
            });

            if (res.ok) {
                // Update in list
                const idx = this.state.settings.templates.findIndex(t => t.type === template.type);
                if (idx >= 0) {
                    this.state.settings.templates[idx] = { ...template, updated_at: new Date().toISOString() };
                }
                this.state.settings.editingTemplate = null;
            } else {
                const err = await res.json();
                alert(err.error || 'Failed to save template');
            }
        } catch (e) {
            alert('Failed to save template');
        }
        this.render();
    },

    async resetTemplate(type) {
        if (!confirm(`Reset ${type} template to default? Your changes will be lost.`)) return;

        try {
            const res = await fetch(`/_/api/settings/templates/${type}/reset`, { method: 'POST' });
            if (res.ok) {
                const data = await res.json();
                const idx = this.state.settings.templates.findIndex(t => t.type === type);
                if (idx >= 0) {
                    this.state.settings.templates[idx] = {
                        ...this.state.settings.templates[idx],
                        subject: data.subject,
                        body_html: data.body_html,
                        body_text: data.body_text,
                        updated_at: new Date().toISOString()
                    };
                }
                if (this.state.settings.editingTemplate?.type === type) {
                    this.state.settings.editingTemplate = null;
                }
                this.render();
            }
        } catch (e) {
            alert('Failed to reset template');
        }
    },

    showRegenerateSecretModal() {
        this.state.modal = {
            type: 'regenerateSecret',
            data: { confirmation: '', error: null }
        };
        this.render();
    },

    async regenerateSecret() {
        const { confirmation } = this.state.modal.data;

        try {
            const res = await fetch('/_/api/settings/auth/regenerate-secret', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ confirmation })
            });

            const data = await res.json();
            if (res.ok) {
                this.closeModal();
                await this.loadSettings();
                alert('JWT secret regenerated. All user sessions have been invalidated.');
            } else {
                this.state.modal.data.error = data.error;
                this.render();
            }
        } catch (e) {
            this.state.modal.data.error = 'Failed to regenerate secret';
            this.render();
        }
    },

    renderRegenerateSecretModal() {
        const { confirmation, error } = this.state.modal.data;

        return `
            <div class="modal-header">
                <h3>Regenerate JWT Secret</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <div class="message message-warning">
                    <strong>Warning:</strong> This will invalidate ALL user sessions. Users will need to log in again.
                </div>
                ${error ? `<div class="message message-error">${error}</div>` : ''}
                <div class="form-group">
                    <label class="form-label">Type REGENERATE to confirm</label>
                    <input type="text" class="form-input" value="${confirmation}"
                        placeholder="REGENERATE"
                        oninput="App.state.modal.data.confirmation = this.value">
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-danger" onclick="App.regenerateSecret()">Regenerate Secret</button>
            </div>
        `;
    },

    async exportSchema() {
        window.location.href = '/_/api/export/schema';
    },

    async exportData(format) {
        const tables = this.state.tables.list.map(t => t.name).join(',');
        if (!tables) {
            alert('No tables to export');
            return;
        }
        window.location.href = `/_/api/export/data?tables=${encodeURIComponent(tables)}&format=${format}`;
    },

    async exportBackup() {
        window.location.href = '/_/api/export/backup';
    },

    renderSettingsView() {
        const { server, auth, templates, loading, expandedSections, editingTemplate, oauth, apiKeys } = this.state.settings;

        if (loading) {
            return '<div class="loading">Loading settings...</div>';
        }

        return `
            <div class="card-title">Settings</div>
            <div class="settings-view">
                ${this.renderServerInfoSection(server, expandedSections.server)}
                ${this.renderApiKeysSection(apiKeys, expandedSections.apiKeys)}
                ${this.renderAuthSection(auth, expandedSections.auth)}
                ${this.renderOAuthSection(oauth, expandedSections.oauth)}
                ${this.renderTemplatesSection(templates, expandedSections.templates, editingTemplate)}
                ${this.renderExportSection(expandedSections.export)}
            </div>
        `;
    },

    renderServerInfoSection(server, expanded) {
        return `
            <div class="settings-section">
                <div class="section-header" onclick="App.toggleSettingsSection('server')">
                    <span class="section-toggle">${expanded ? '▼' : '▶'}</span>
                    <h3>Server Information</h3>
                </div>
                ${expanded ? `
                    <div class="section-content">
                        <div class="info-grid">
                            <div class="info-item">
                                <label>Version</label>
                                <span>${server?.version || 'Unknown'}</span>
                            </div>
                            <div class="info-item">
                                <label>Host</label>
                                <span>${server?.host || 'Unknown'}:${server?.port || 'Unknown'}</span>
                            </div>
                            <div class="info-item">
                                <label>Database</label>
                                <span>${server?.db_path || 'Unknown'}</span>
                            </div>
                            <div class="info-item">
                                <label>Log Mode</label>
                                <span>${server?.log_mode || 'console'}</span>
                            </div>
                            <div class="info-item">
                                <label>Uptime</label>
                                <span>${server?.uptime_human || 'Unknown'}</span>
                            </div>
                            <div class="info-item">
                                <label>Memory</label>
                                <span>${server?.memory_mb || 0} MB (${server?.memory_sys_mb || 0} MB sys)</span>
                            </div>
                            <div class="info-item">
                                <label>Goroutines</label>
                                <span>${server?.goroutines || 0}</span>
                            </div>
                            <div class="info-item">
                                <label>Go Version</label>
                                <span>${server?.go_version || 'Unknown'}</span>
                            </div>
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    },

    renderApiKeysSection(apiKeys, expanded) {
        return `
            <div class="settings-section">
                <div class="section-header" onclick="App.toggleSettingsSection('apiKeys')">
                    <span class="section-toggle">${expanded ? '▼' : '▶'}</span>
                    <h3>API Keys</h3>
                </div>
                ${expanded ? `
                    <div class="section-content">
                        <p class="text-muted" style="margin-bottom: 1rem;">
                            Use these keys to connect with <code>@supabase/supabase-js</code> or make API requests.
                        </p>
                        <div class="api-key-item">
                            <label>anon (public) key</label>
                            <div class="api-key-value">
                                <input type="text" class="form-input mono" readonly
                                    value="${apiKeys?.anon_key || ''}"
                                    onclick="this.select()">
                                <button class="btn btn-secondary btn-sm" onclick="App.copyApiKey('anon')">Copy</button>
                            </div>
                            <small class="text-muted">Safe to use in browsers. Subject to Row Level Security policies.</small>
                        </div>
                        <div class="api-key-item" style="margin-top: 1rem;">
                            <label>service_role (secret) key</label>
                            <div class="api-key-value">
                                <input type="password" class="form-input mono" readonly
                                    id="service-role-key-input"
                                    value="${apiKeys?.service_role_key || ''}"
                                    onclick="this.select()">
                                <button class="btn btn-secondary btn-sm" onclick="App.toggleServiceRoleKey()">Show</button>
                                <button class="btn btn-secondary btn-sm" onclick="App.copyApiKey('service_role')">Copy</button>
                            </div>
                            <small class="text-muted">Keep secret! Bypasses Row Level Security. Never expose in browsers.</small>
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    },

    copyApiKey(keyType) {
        const apiKeys = this.state.settings.apiKeys;
        const key = keyType === 'service_role' ? apiKeys?.service_role_key : apiKeys?.anon_key;
        if (key) {
            navigator.clipboard.writeText(key).then(() => {
                alert('API key copied to clipboard');
            }).catch(() => {
                alert('Failed to copy to clipboard');
            });
        }
    },

    toggleServiceRoleKey() {
        const input = document.getElementById('service-role-key-input');
        if (input) {
            input.type = input.type === 'password' ? 'text' : 'password';
        }
    },

    renderAuthSection(auth, expanded) {
        const authConfig = this.state.settings.authConfig || { require_email_confirmation: true };

        return `
            <div class="settings-section">
                <div class="section-header" onclick="App.toggleSettingsSection('auth')">
                    <span class="section-toggle">${expanded ? '▼' : '▶'}</span>
                    <h3>Authentication</h3>
                </div>
                ${expanded ? `
                    <div class="section-content">
                        <div class="setting-group">
                            <label class="form-label">Site URL</label>
                            <div style="display: flex; gap: 8px; align-items: center;">
                                <input type="text" class="form-input" style="flex: 1;"
                                       value="${authConfig.site_url || ''}"
                                       placeholder="http://localhost:8080"
                                       id="site-url-input">
                                <button class="btn btn-primary btn-sm" onclick="App.saveSiteURL()">Save</button>
                            </div>
                            <p class="text-muted" style="margin-top: 4px;">
                                Base URL for authentication email links (verification, password reset, magic links).
                                This should be your API server URL.
                            </p>
                        </div>
                        <hr style="margin: 16px 0; border: none; border-top: 1px solid var(--border-color);">
                        <div class="setting-group">
                            <label class="setting-toggle">
                                <input type="checkbox"
                                       ${authConfig.require_email_confirmation ? 'checked' : ''}
                                       onchange="App.toggleEmailConfirmation(this.checked)">
                                <span>Require email confirmation for new signups</span>
                            </label>
                            <p class="text-muted" style="margin-top: 4px; margin-left: 24px;">
                                When enabled, users must verify their email address before signing in.
                            </p>
                        </div>
                        <div class="setting-group">
                            <label class="setting-toggle">
                                <input type="checkbox"
                                       ${authConfig.allow_anonymous ? 'checked' : ''}
                                       onchange="App.toggleAnonymousSignin(this.checked)">
                                <span>Allow anonymous sign-in</span>
                            </label>
                            <p class="text-muted" style="margin-top: 4px; margin-left: 24px;">
                                When enabled, users can sign in without email or password.
                                ${authConfig.anonymous_user_count !== undefined ?
                                  `<br>Anonymous users: <strong>${authConfig.anonymous_user_count}</strong>` : ''}
                            </p>
                        </div>
                        <hr style="margin: 16px 0; border: none; border-top: 1px solid var(--border-color);">
                        <div class="info-grid">
                            <div class="info-item">
                                <label>JWT Secret</label>
                                <span class="mono">${auth?.jwt_secret_masked || 'Not set'}</span>
                            </div>
                            <div class="info-item">
                                <label>Secret Source</label>
                                <span>${auth?.jwt_secret_source || 'Unknown'}</span>
                            </div>
                            <div class="info-item">
                                <label>Access Token Expiry</label>
                                <span>${auth?.access_token_expiry || '1 hour'}</span>
                            </div>
                            <div class="info-item">
                                <label>Refresh Token Expiry</label>
                                <span>${auth?.refresh_token_expiry || '1 week'}</span>
                            </div>
                        </div>
                        ${auth?.can_regenerate ? `
                            <div class="section-actions">
                                <button class="btn btn-danger btn-sm" onclick="App.showRegenerateSecretModal()">
                                    Regenerate JWT Secret
                                </button>
                                <small class="text-muted">Warning: This will invalidate all user sessions</small>
                            </div>
                        ` : `
                            <div class="message message-info">
                                JWT secret is set via environment variable and cannot be changed from the dashboard.
                            </div>
                        `}
                    </div>
                ` : ''}
            </div>
        `;
    },

    renderOAuthSection(oauth, expanded) {
        const providers = oauth?.providers || {};
        const redirectUrls = oauth?.redirectUrls || [];

        return `
            <div class="settings-section">
                <div class="section-header" onclick="App.toggleSettingsSection('oauth')">
                    <span class="section-toggle">${expanded ? '▼' : '▶'}</span>
                    <h3>OAuth Providers</h3>
                </div>
                ${expanded ? `
                    <div class="section-content">
                        <div class="oauth-providers">
                            ${this.renderOAuthProvider('google', 'Google', providers.google)}
                            ${this.renderOAuthProvider('github', 'GitHub', providers.github)}
                        </div>

                        <div class="oauth-redirect-urls">
                            <h4>Allowed Redirect URLs</h4>
                            <p class="text-muted oauth-redirect-help">
                                ${redirectUrls.length === 0
                                    ? 'No redirect URLs configured. All URLs are allowed in development mode.'
                                    : 'Only these URLs will be allowed as OAuth callback destinations.'}
                            </p>
                            <div class="redirect-urls-list">
                                ${redirectUrls.map(url => `
                                    <div class="redirect-url-item">
                                        <span class="redirect-url-value">${this.escapeHtml(url)}</span>
                                        <button class="btn btn-secondary btn-sm" onclick="App.removeRedirectUrl('${this.escapeHtml(url)}')">
                                            Remove
                                        </button>
                                    </div>
                                `).join('')}
                            </div>
                            <div class="add-redirect-url-form">
                                <input type="url" id="new-redirect-url" class="form-input"
                                       placeholder="https://example.com/auth/callback">
                                <button class="btn btn-primary btn-sm" onclick="App.addRedirectUrl()">
                                    Add URL
                                </button>
                            </div>
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    },

    renderOAuthProvider(providerId, providerName, config) {
        const enabled = config?.enabled || false;
        const clientId = config?.client_id || '';
        const hasSecret = config?.client_secret_set || false;

        return `
            <div class="oauth-provider-card ${enabled ? 'enabled' : ''}">
                <div class="oauth-provider-header">
                    <h4>${providerName}</h4>
                    <label class="oauth-toggle">
                        <input type="checkbox"
                               ${enabled ? 'checked' : ''}
                               onchange="App.toggleOAuthProvider('${providerId}', this.checked)">
                        <span>${enabled ? 'Enabled' : 'Disabled'}</span>
                    </label>
                </div>
                <div class="oauth-provider-fields">
                    <div class="form-group">
                        <label class="form-label">Client ID</label>
                        <input type="text" class="form-input"
                               value="${this.escapeHtml(clientId)}"
                               placeholder="Enter client ID"
                               onchange="App.updateOAuthProviderField('${providerId}', 'client_id', this.value)">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Client Secret</label>
                        <input type="password" class="form-input"
                               placeholder="${hasSecret ? '••••••••••••••••' : 'Enter client secret'}"
                               onchange="App.updateOAuthProviderField('${providerId}', 'client_secret', this.value)">
                        ${hasSecret ? '<small class="text-muted">Secret is set. Enter a new value to change it.</small>' : ''}
                    </div>
                </div>
            </div>
        `;
    },

    async toggleEmailConfirmation(required) {
        try {
            const res = await fetch('/_/api/settings/auth-config', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ require_email_confirmation: required })
            });

            if (res.ok) {
                this.state.settings.authConfig.require_email_confirmation = required;
                this.render();
            } else {
                const data = await res.json();
                this.state.error = data.error || 'Failed to update setting';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to update setting';
            this.render();
        }
    },

    async toggleAnonymousSignin(enabled) {
        try {
            const res = await fetch('/_/api/settings/auth-config', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ allow_anonymous: enabled })
            });

            if (!res.ok) throw new Error('Failed to update setting');

            this.showToast(
                enabled ? 'Anonymous sign-in enabled' : 'Anonymous sign-in disabled',
                'success'
            );

            // Reload settings to refresh count
            await this.loadSettings();
        } catch (err) {
            this.showToast(err.message, 'error');
            // Revert checkbox on error
            this.render();
        }
    },

    async saveSiteURL() {
        const input = document.getElementById('site-url-input');
        const siteURL = input?.value?.trim() || '';

        try {
            const res = await fetch('/_/api/settings/auth-config', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ site_url: siteURL })
            });

            if (!res.ok) throw new Error('Failed to save Site URL');

            this.state.settings.authConfig.site_url = siteURL;
            this.showToast('Site URL saved', 'success');
        } catch (err) {
            this.showToast(err.message, 'error');
        }
    },

    async toggleOAuthProvider(provider, enabled) {
        try {
            const res = await fetch('/_/api/settings/oauth', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ [provider]: { enabled } })
            });

            if (res.ok) {
                // Update local state
                if (!this.state.settings.oauth.providers[provider]) {
                    this.state.settings.oauth.providers[provider] = {};
                }
                this.state.settings.oauth.providers[provider].enabled = enabled;
                this.render();
            } else {
                const data = await res.json();
                this.state.error = data.error || 'Failed to update provider';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to update provider';
            this.render();
        }
    },

    async updateOAuthProviderField(provider, field, value) {
        if (!value.trim()) return;

        try {
            const res = await fetch('/_/api/settings/oauth', {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ [provider]: { [field]: value } })
            });

            if (res.ok) {
                // Update local state
                if (!this.state.settings.oauth.providers[provider]) {
                    this.state.settings.oauth.providers[provider] = {};
                }
                if (field === 'client_id') {
                    this.state.settings.oauth.providers[provider].client_id = value;
                } else if (field === 'client_secret') {
                    this.state.settings.oauth.providers[provider].client_secret_set = true;
                }
                this.render();
            } else {
                const data = await res.json();
                this.state.error = data.error || 'Failed to update provider';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to update provider';
            this.render();
        }
    },

    async addRedirectUrl() {
        const input = document.getElementById('new-redirect-url');
        const url = input.value.trim();

        if (!url) {
            this.state.error = 'Please enter a URL';
            this.render();
            return;
        }

        try {
            new URL(url); // Validate URL format
        } catch (e) {
            this.state.error = 'Please enter a valid URL';
            this.render();
            return;
        }

        try {
            const res = await fetch('/_/api/settings/oauth/redirect-urls', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url })
            });

            if (res.ok) {
                this.state.settings.oauth.redirectUrls.push(url);
                input.value = '';
                this.state.error = null;
                this.render();
            } else {
                const data = await res.json();
                this.state.error = data.error || 'Failed to add redirect URL';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to add redirect URL';
            this.render();
        }
    },

    async removeRedirectUrl(url) {
        try {
            const res = await fetch('/_/api/settings/oauth/redirect-urls', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ url })
            });

            if (res.ok) {
                this.state.settings.oauth.redirectUrls = this.state.settings.oauth.redirectUrls.filter(u => u !== url);
                this.state.error = null;
                this.render();
            } else {
                const data = await res.json();
                this.state.error = data.error || 'Failed to remove redirect URL';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to remove redirect URL';
            this.render();
        }
    },

    renderTemplatesSection(templates, expanded, editingTemplate) {
        return `
            <div class="settings-section">
                <div class="section-header" onclick="App.toggleSettingsSection('templates')">
                    <span class="section-toggle">${expanded ? '▼' : '▶'}</span>
                    <h3>Email Templates</h3>
                </div>
                ${expanded ? `
                    <div class="section-content">
                        <div class="templates-list">
                            ${templates.map(t => this.renderTemplateItem(t, editingTemplate)).join('')}
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    },

    renderTemplateItem(template, editingTemplate) {
        const isEditing = editingTemplate?.type === template.type;

        if (isEditing) {
            return `
                <div class="template-item editing">
                    <div class="template-header">
                        <strong>${template.type}</strong>
                    </div>
                    <div class="template-form">
                        <div class="form-group">
                            <label class="form-label">Subject</label>
                            <input type="text" class="form-input" value="${editingTemplate.subject}"
                                oninput="App.updateTemplateField('subject', this.value)">
                        </div>
                        <div class="form-group">
                            <label class="form-label">HTML Body</label>
                            <textarea class="form-input code-input" rows="6"
                                oninput="App.updateTemplateField('body_html', this.value)">${editingTemplate.body_html}</textarea>
                        </div>
                        <div class="form-group">
                            <label class="form-label">Text Body</label>
                            <textarea class="form-input code-input" rows="4"
                                oninput="App.updateTemplateField('body_text', this.value)">${editingTemplate.body_text || ''}</textarea>
                        </div>
                        <div class="template-actions">
                            <button class="btn btn-primary btn-sm" onclick="App.saveTemplate()">Save</button>
                            <button class="btn btn-secondary btn-sm" onclick="App.cancelEditingTemplate()">Cancel</button>
                            <button class="btn btn-danger btn-sm" onclick="App.resetTemplate('${template.type}')">Reset to Default</button>
                        </div>
                    </div>
                </div>
            `;
        }

        return `
            <div class="template-item">
                <div class="template-header">
                    <strong>${template.type}</strong>
                    <span class="text-muted">${template.subject}</span>
                </div>
                <div class="template-actions">
                    <button class="btn btn-secondary btn-sm" onclick="App.startEditingTemplate('${template.type}')">Edit</button>
                </div>
            </div>
        `;
    },

    renderExportSection(expanded) {
        return `
            <div class="settings-section">
                <div class="section-header" onclick="App.toggleSettingsSection('export')">
                    <span class="section-toggle">${expanded ? '▼' : '▶'}</span>
                    <h3>Export & Backup</h3>
                </div>
                ${expanded ? `
                    <div class="section-content">
                        <div class="export-buttons">
                            <div class="export-item">
                                <button class="btn btn-primary" onclick="App.exportSchema()">
                                    Export PostgreSQL Schema
                                </button>
                                <small>Download SQL file for migration to Supabase</small>
                            </div>
                            <div class="export-item">
                                <button class="btn btn-secondary" onclick="App.exportData('json')">
                                    Export Data (JSON)
                                </button>
                                <button class="btn btn-secondary" onclick="App.exportData('csv')">
                                    Export Data (CSV)
                                </button>
                                <small>Export all table data</small>
                            </div>
                            <div class="export-item">
                                <button class="btn btn-secondary" onclick="App.exportBackup()">
                                    Download Database Backup
                                </button>
                                <small>Download the entire SQLite database file</small>
                            </div>
                        </div>
                    </div>
                ` : ''}
            </div>
        `;
    },

    // ========================================================================
    // Logs View Methods
    // ========================================================================

    async loadLogs() {
        this.state.logs.loading = true;
        this.render();

        try {
            // Load log config
            const configRes = await fetch('/_/api/logs/config');
            if (configRes.ok) {
                this.state.logs.config = await configRes.json();
            }

            // Always load console buffer
            await this.loadConsoleBuffer();

            // Load mode-specific logs
            if (this.state.logs.activeTab === 'database' && this.state.logs.config?.mode === 'database') {
                await this.queryLogs();
            } else if (this.state.logs.activeTab === 'file' && this.state.logs.config?.mode === 'file') {
                await this.tailLogs();
            }
        } catch (e) {
            this.state.error = 'Failed to load logs';
        }

        this.state.logs.loading = false;
        this.render();
    },

    async loadConsoleBuffer() {
        try {
            const res = await fetch('/_/api/logs/buffer?lines=500');
            if (res.ok) {
                const data = await res.json();
                this.state.logs.consoleLines = data.lines || [];
                this.state.logs.consoleTotal = data.total || 0;
                this.state.logs.consoleBufferSize = data.buffer_size || 0;
                this.state.logs.consoleEnabled = data.enabled !== false;
            }
        } catch (e) {
            this.state.logs.consoleLines = [];
            this.state.logs.consoleEnabled = false;
        }
    },

    setLogsTab(tab) {
        this.state.logs.activeTab = tab;
        this.stopAutoRefresh();
        this.loadLogs();
    },

    toggleAutoRefresh() {
        if (this.state.logs.autoRefresh) {
            this.stopAutoRefresh();
        } else {
            this.startAutoRefresh();
        }
        this.render();
    },

    startAutoRefresh() {
        this.state.logs.autoRefresh = true;
        this.state.logs.autoRefreshInterval = setInterval(() => {
            this.loadConsoleBuffer().then(() => this.render());
        }, 5000);
    },

    stopAutoRefresh() {
        this.state.logs.autoRefresh = false;
        if (this.state.logs.autoRefreshInterval) {
            clearInterval(this.state.logs.autoRefreshInterval);
            this.state.logs.autoRefreshInterval = null;
        }
    },

    async queryLogs() {
        const { filters, page, pageSize } = this.state.logs;
        const params = new URLSearchParams();

        if (filters.level && filters.level !== 'all') params.set('level', filters.level);
        if (filters.since) params.set('since', filters.since);
        if (filters.until) params.set('until', filters.until);
        if (filters.search) params.set('search', filters.search);
        if (filters.user_id) params.set('user_id', filters.user_id);
        if (filters.request_id) params.set('request_id', filters.request_id);
        params.set('limit', pageSize.toString());
        params.set('offset', ((page - 1) * pageSize).toString());

        try {
            const res = await fetch(`/_/api/logs?${params}`);
            if (res.ok) {
                const data = await res.json();
                this.state.logs.list = data.logs || [];
                this.state.logs.total = data.total || 0;
            }
        } catch (e) {
            this.state.logs.list = [];
        }
    },

    async tailLogs() {
        try {
            const res = await fetch('/_/api/logs/tail?lines=100');
            if (res.ok) {
                const data = await res.json();
                this.state.logs.tailLines = data.lines || [];
            }
        } catch (e) {
            this.state.logs.tailLines = [];
        }
    },

    updateLogFilter(field, value) {
        this.state.logs.filters[field] = value;
        this.state.logs.page = 1;
    },

    async applyLogFilters() {
        this.state.logs.loading = true;
        this.render();
        await this.queryLogs();
        this.state.logs.loading = false;
        this.render();
    },

    clearLogFilters() {
        this.state.logs.filters = { level: 'all', since: '', until: '', search: '', user_id: '', request_id: '' };
        this.state.logs.page = 1;
        this.applyLogFilters();
    },

    async refreshLogs() {
        await this.loadLogs();
    },

    setLogsPage(page) {
        this.state.logs.page = page;
        this.applyLogFilters();
    },

    toggleLogExpand(logId) {
        if (this.state.logs.expandedLog === logId) {
            this.state.logs.expandedLog = null;
        } else {
            this.state.logs.expandedLog = logId;
        }
        this.render();
    },

    renderLogsView() {
        const { config, loading, activeTab } = this.state.logs;

        if (loading) {
            return '<div class="loading">Loading logs...</div>';
        }

        const dbEnabled = config?.mode === 'database';
        const fileEnabled = config?.mode === 'file';

        return `
            <div class="card-title">Logs</div>
            <div class="logs-view">
                <div class="logs-tabs">
                    <button class="tab-btn ${activeTab === 'console' ? 'active' : ''}"
                            onclick="App.setLogsTab('console')">Console</button>
                    <button class="tab-btn ${activeTab === 'database' ? 'active' : ''} ${!dbEnabled ? 'disabled' : ''}"
                            onclick="App.setLogsTab('database')" ${!dbEnabled ? 'disabled' : ''}>Database</button>
                    <button class="tab-btn ${activeTab === 'file' ? 'active' : ''} ${!fileEnabled ? 'disabled' : ''}"
                            onclick="App.setLogsTab('file')" ${!fileEnabled ? 'disabled' : ''}>File</button>
                </div>
                ${activeTab === 'console' ? this.renderConsoleLogs() :
                  activeTab === 'database' ? this.renderDatabaseLogs() :
                  this.renderFileLogs()}
            </div>
        `;
    },

    renderDatabaseLogs() {
        const { list, total, page, pageSize, filters, expandedLog } = this.state.logs;
        const totalPages = Math.ceil(total / pageSize);

        return `
            <div class="logs-toolbar">
                <div class="logs-filters">
                    <select class="form-input" onchange="App.updateLogFilter('level', this.value)">
                        <option value="all" ${filters.level === 'all' ? 'selected' : ''}>All Levels</option>
                        <option value="debug" ${filters.level === 'debug' ? 'selected' : ''}>Debug</option>
                        <option value="info" ${filters.level === 'info' ? 'selected' : ''}>Info</option>
                        <option value="warn" ${filters.level === 'warn' ? 'selected' : ''}>Warn</option>
                        <option value="error" ${filters.level === 'error' ? 'selected' : ''}>Error</option>
                    </select>
                    <input type="text" class="form-input" placeholder="Search message..."
                        value="${filters.search}" oninput="App.updateLogFilter('search', this.value)">
                    <input type="text" class="form-input" placeholder="User ID"
                        value="${filters.user_id}" oninput="App.updateLogFilter('user_id', this.value)">
                    <input type="text" class="form-input" placeholder="Request ID"
                        value="${filters.request_id}" oninput="App.updateLogFilter('request_id', this.value)">
                    <button class="btn btn-primary btn-sm" onclick="App.applyLogFilters()">Filter</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.clearLogFilters()">Clear</button>
                </div>
                <div class="logs-actions">
                    <span class="text-muted">${total} logs</span>
                    <button class="btn btn-secondary btn-sm" onclick="App.refreshLogs()">Refresh</button>
                </div>
            </div>

            <div class="logs-table-container">
                <table class="logs-table">
                    <thead>
                        <tr>
                            <th>Timestamp</th>
                            <th>Level</th>
                            <th>Message</th>
                            <th>Source</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${list.length === 0 ? `
                            <tr><td colspan="4" class="empty-state">No logs match your filters</td></tr>
                        ` : list.map(log => this.renderLogRow(log, expandedLog)).join('')}
                    </tbody>
                </table>
            </div>

            ${totalPages > 1 ? `
                <div class="pagination">
                    <button class="btn btn-secondary btn-sm" ${page <= 1 ? 'disabled' : ''} onclick="App.setLogsPage(${page - 1})">Previous</button>
                    <span>Page ${page} of ${totalPages}</span>
                    <button class="btn btn-secondary btn-sm" ${page >= totalPages ? 'disabled' : ''} onclick="App.setLogsPage(${page + 1})">Next</button>
                </div>
            ` : ''}
        `;
    },

    renderLogRow(log, expandedLog) {
        const isExpanded = expandedLog === log.id;
        const levelClass = `level-${log.level.toLowerCase()}`;

        return `
            <tr class="log-row ${isExpanded ? 'expanded' : ''}" onclick="App.toggleLogExpand(${log.id})">
                <td class="log-timestamp">${new Date(log.timestamp).toLocaleString()}</td>
                <td><span class="log-level ${levelClass}">${log.level}</span></td>
                <td class="log-message">${this.escapeHtml(log.message)}</td>
                <td class="log-source">${log.source || ''}</td>
            </tr>
            ${isExpanded ? `
                <tr class="log-details-row">
                    <td colspan="4">
                        <div class="log-details">
                            ${log.request_id ? `<div><strong>Request ID:</strong> ${log.request_id}</div>` : ''}
                            ${log.user_id ? `<div><strong>User ID:</strong> ${log.user_id}</div>` : ''}
                            ${log.extra ? `<div><strong>Extra:</strong> <pre>${JSON.stringify(log.extra, null, 2)}</pre></div>` : ''}
                        </div>
                    </td>
                </tr>
            ` : ''}
        `;
    },

    renderConsoleLogs() {
        const { consoleLines, consoleTotal, consoleBufferSize, consoleEnabled, autoRefresh } = this.state.logs;

        if (!consoleEnabled) {
            return `
                <div class="message message-info">
                    <p>Console log buffer is disabled.</p>
                    <p>Start server with <code>--log-buffer-lines=500</code> to enable.</p>
                </div>
            `;
        }

        return `
            <div class="logs-toolbar">
                <div class="logs-info">
                    Showing ${consoleLines.length} of ${consoleTotal} lines (buffer: ${consoleBufferSize})
                </div>
                <div class="logs-actions">
                    <label class="auto-refresh-label">
                        <input type="checkbox" ${autoRefresh ? 'checked' : ''}
                               onchange="App.toggleAutoRefresh()">
                        Auto-refresh (5s)
                    </label>
                    <button class="btn btn-secondary btn-sm" onclick="App.loadConsoleBuffer().then(() => App.render())">
                        Refresh
                    </button>
                </div>
            </div>
            <div class="console-output">
                <pre>${consoleLines.map(l => this.escapeHtml(l)).join('')}</pre>
            </div>
        `;
    },

    renderFileLogs() {
        const { config, tailLines } = this.state.logs;

        if (config?.mode !== 'file') {
            return `
                <div class="message message-info">
                    <p>File logging is not enabled.</p>
                    <p>Start server with <code>--log-mode=file</code> to enable.</p>
                </div>
            `;
        }

        return `
            <div class="logs-toolbar">
                <div class="logs-info">
                    Log file: ${config.file_path}
                </div>
                <div class="logs-actions">
                    <button class="btn btn-secondary btn-sm" onclick="App.tailLogs().then(() => App.render())">
                        Refresh
                    </button>
                </div>
            </div>
            <div class="console-output">
                <pre>${tailLines.map(l => this.escapeHtml(l)).join('\n')}</pre>
            </div>
        `;
    },

    // API Console methods

    apiConsoleTemplates: {
        // Auth templates
        'auth-signup': {
            method: 'POST',
            url: '/auth/v1/signup',
            headers: [{ key: 'Content-Type', value: 'application/json' }],
            body: JSON.stringify({ email: 'user@example.com', password: 'password123' }, null, 2)
        },
        'auth-signin': {
            method: 'POST',
            url: '/auth/v1/token?grant_type=password',
            headers: [{ key: 'Content-Type', value: 'application/json' }],
            body: JSON.stringify({ email: 'user@example.com', password: 'password123' }, null, 2)
        },
        'auth-refresh': {
            method: 'POST',
            url: '/auth/v1/token?grant_type=refresh_token',
            headers: [{ key: 'Content-Type', value: 'application/json' }],
            body: JSON.stringify({ refresh_token: 'your-refresh-token' }, null, 2)
        },
        'auth-user': {
            method: 'GET',
            url: '/auth/v1/user',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        },
        'auth-update-user': {
            method: 'PUT',
            url: '/auth/v1/user',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }, { key: 'Content-Type', value: 'application/json' }],
            body: JSON.stringify({ data: { display_name: 'John Doe' } }, null, 2)
        },
        'auth-logout': {
            method: 'POST',
            url: '/auth/v1/logout',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        },
        // REST templates
        'rest-select': {
            method: 'GET',
            url: '/rest/v1/table_name?select=*',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        },
        'rest-select-filter': {
            method: 'GET',
            url: '/rest/v1/table_name?select=*&column=eq.value',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        },
        'rest-insert': {
            method: 'POST',
            url: '/rest/v1/table_name',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }, { key: 'Content-Type', value: 'application/json' }, { key: 'Prefer', value: 'return=representation' }],
            body: JSON.stringify({ column1: 'value1', column2: 'value2' }, null, 2)
        },
        'rest-update': {
            method: 'PATCH',
            url: '/rest/v1/table_name?id=eq.1',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }, { key: 'Content-Type', value: 'application/json' }, { key: 'Prefer', value: 'return=representation' }],
            body: JSON.stringify({ column1: 'new_value' }, null, 2)
        },
        'rest-delete': {
            method: 'DELETE',
            url: '/rest/v1/table_name?id=eq.1',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        },
        // Admin templates
        'admin-list-tables': {
            method: 'GET',
            url: '/admin/v1/tables',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        },
        'admin-create-table': {
            method: 'POST',
            url: '/admin/v1/tables',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }, { key: 'Content-Type', value: 'application/json' }],
            body: JSON.stringify({ name: 'new_table', columns: [{ name: 'id', type: 'uuid', primary: true }, { name: 'name', type: 'text' }] }, null, 2)
        },
        'admin-get-schema': {
            method: 'GET',
            url: '/admin/v1/tables/table_name',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        },
        'admin-drop-table': {
            method: 'DELETE',
            url: '/admin/v1/tables/table_name',
            headers: [{ key: 'Authorization', value: 'Bearer <access_token>' }],
            body: ''
        }
    },

    async initApiConsole() {
        // Load history from localStorage
        const saved = localStorage.getItem('sblite_api_console_history');
        if (saved) {
            try {
                this.state.apiConsole.history = JSON.parse(saved);
            } catch (e) {
                this.state.apiConsole.history = [];
            }
        }
        // Initialize with default header if empty
        if (this.state.apiConsole.headers.length === 0) {
            this.state.apiConsole.headers = [
                { key: 'Content-Type', value: 'application/json' }
            ];
        }
        // Load API keys if not already loaded
        if (!this.state.apiConsole.apiKeys) {
            await this.loadApiKeys();
        }
        this.render();
    },

    saveApiConsoleHistory() {
        const history = this.state.apiConsole.history.slice(0, 20); // Keep last 20
        localStorage.setItem('sblite_api_console_history', JSON.stringify(history));
    },

    applyApiConsoleTemplate(templateId) {
        const template = this.apiConsoleTemplates[templateId];
        if (!template) return;

        this.state.apiConsole.method = template.method;
        this.state.apiConsole.url = template.url;
        this.state.apiConsole.headers = template.headers.map(h => ({ ...h }));
        this.state.apiConsole.body = template.body;
        this.render();
    },

    updateApiConsoleMethod(method) {
        this.state.apiConsole.method = method;
        this.render();
    },

    updateApiConsoleUrl(url) {
        this.state.apiConsole.url = url;
        // Don't re-render on every keystroke
    },

    updateApiConsoleBody(body) {
        this.state.apiConsole.body = body;
        // Don't re-render on every keystroke
    },

    async loadApiKeys() {
        try {
            const res = await fetch('/_/api/apikeys');
            if (res.ok) {
                this.state.apiConsole.apiKeys = await res.json();
            }
        } catch (e) {
            console.error('Failed to load API keys:', e);
        }
    },

    setApiKeyType(type) {
        this.state.apiConsole.selectedKeyType = type;
        this.render();
    },

    toggleAutoInjectKey() {
        this.state.apiConsole.autoInjectKey = !this.state.apiConsole.autoInjectKey;
        this.render();
    },

    addApiConsoleHeader() {
        this.state.apiConsole.headers.push({ key: '', value: '' });
        this.render();
    },

    updateApiConsoleHeader(index, field, value) {
        this.state.apiConsole.headers[index][field] = value;
        // Don't re-render on every keystroke
    },

    removeApiConsoleHeader(index) {
        this.state.apiConsole.headers.splice(index, 1);
        this.render();
    },

    async sendApiConsoleRequest() {
        const { method, url, headers, body, apiKeys, selectedKeyType, autoInjectKey } = this.state.apiConsole;

        // Get current URL value from input (in case it wasn't saved to state)
        const urlInput = document.getElementById('api-console-url');
        const bodyInput = document.getElementById('api-console-body');
        const currentUrl = urlInput ? urlInput.value : url;
        const currentBody = bodyInput ? bodyInput.value : body;

        this.state.apiConsole.url = currentUrl;
        this.state.apiConsole.body = currentBody;
        this.state.apiConsole.loading = true;
        this.state.apiConsole.response = null;
        this.render();

        const startTime = performance.now();

        try {
            const fetchHeaders = {};
            headers.forEach(h => {
                if (h.key && h.value) {
                    fetchHeaders[h.key] = h.value;
                }
            });

            // Auto-inject apikey header for REST and Auth API requests
            if (autoInjectKey && apiKeys && (currentUrl.includes('/rest/v1/') || currentUrl.includes('/auth/v1/'))) {
                const apiKey = selectedKeyType === 'service_role' ? apiKeys.service_role_key : apiKeys.anon_key;
                if (apiKey && !fetchHeaders['apikey']) {
                    fetchHeaders['apikey'] = apiKey;
                }
            }

            const fetchOptions = {
                method: method,
                headers: fetchHeaders,
            };

            if (['POST', 'PUT', 'PATCH'].includes(method) && currentBody) {
                fetchOptions.body = currentBody;
            }

            const res = await fetch(currentUrl, fetchOptions);
            const endTime = performance.now();

            // Get response headers
            const responseHeaders = {};
            res.headers.forEach((value, key) => {
                responseHeaders[key] = value;
            });

            // Get response body
            let responseBody = '';
            const contentType = res.headers.get('content-type') || '';
            if (contentType.includes('application/json')) {
                try {
                    const json = await res.json();
                    responseBody = JSON.stringify(json, null, 2);
                } catch (e) {
                    responseBody = await res.text();
                }
            } else {
                responseBody = await res.text();
            }

            this.state.apiConsole.response = {
                status: res.status,
                statusText: res.statusText,
                headers: responseHeaders,
                body: responseBody,
                time: Math.round(endTime - startTime)
            };

            // Add to history
            const historyEntry = {
                id: Date.now().toString(),
                method: method,
                url: currentUrl,
                headers: headers.map(h => ({ ...h })),
                body: currentBody,
                status: res.status,
                timestamp: Date.now()
            };

            // Don't add duplicate consecutive requests
            const lastEntry = this.state.apiConsole.history[0];
            if (!lastEntry || lastEntry.method !== method || lastEntry.url !== currentUrl) {
                this.state.apiConsole.history.unshift(historyEntry);
                this.saveApiConsoleHistory();
            }

        } catch (e) {
            this.state.apiConsole.response = {
                status: 0,
                statusText: 'Network Error',
                headers: {},
                body: e.message,
                time: Math.round(performance.now() - startTime)
            };
        }

        this.state.apiConsole.loading = false;
        this.render();
    },

    loadFromApiConsoleHistory(id) {
        const entry = this.state.apiConsole.history.find(h => h.id === id);
        if (!entry) return;

        this.state.apiConsole.method = entry.method;
        this.state.apiConsole.url = entry.url;
        this.state.apiConsole.headers = entry.headers.map(h => ({ ...h }));
        this.state.apiConsole.body = entry.body;
        this.state.apiConsole.showHistory = false;
        this.render();
    },

    clearApiConsoleHistory() {
        this.state.apiConsole.history = [];
        localStorage.removeItem('sblite_api_console_history');
        this.state.apiConsole.showHistory = false;
        this.render();
    },

    toggleApiConsoleHistory() {
        this.state.apiConsole.showHistory = !this.state.apiConsole.showHistory;
        this.render();
    },

    setApiConsoleResponseTab(tab) {
        this.state.apiConsole.activeTab = tab;
        this.render();
    },

    copyApiConsoleResponse() {
        const { response } = this.state.apiConsole;
        if (response && response.body) {
            navigator.clipboard.writeText(response.body);
        }
    },

    formatTimeAgo(timestamp) {
        const seconds = Math.floor((Date.now() - timestamp) / 1000);
        if (seconds < 60) return 'just now';
        const minutes = Math.floor(seconds / 60);
        if (minutes < 60) return `${minutes} min ago`;
        const hours = Math.floor(minutes / 60);
        if (hours < 24) return `${hours} hr ago`;
        const days = Math.floor(hours / 24);
        return `${days} day${days > 1 ? 's' : ''} ago`;
    },

    getStatusClass(status) {
        if (status >= 200 && status < 300) return 'status-success';
        if (status >= 400 && status < 500) return 'status-warning';
        return 'status-error';
    },

    renderApiConsoleView() {
        const { method, url, headers, body, response, loading, history, activeTab, showHistory } = this.state.apiConsole;
        const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'];
        const showBody = ['POST', 'PUT', 'PATCH'].includes(method);

        return `
            <div class="card-title">
                API Console
                <div class="api-console-history-toggle">
                    <button class="btn btn-secondary btn-sm" onclick="App.toggleApiConsoleHistory()">
                        History (${history.length})
                    </button>
                    ${showHistory ? this.renderApiConsoleHistoryDropdown() : ''}
                </div>
            </div>
            <div class="api-console-view">
                <div class="api-console-split">
                    <div class="api-console-request">
                        <h4>Request</h4>

                        <div class="api-console-url-bar">
                            <select class="form-input api-console-method" onchange="App.updateApiConsoleMethod(this.value)">
                                ${methods.map(m => `<option value="${m}" ${method === m ? 'selected' : ''}>${m}</option>`).join('')}
                            </select>
                            <input type="text" id="api-console-url" class="form-input api-console-url-input"
                                value="${this.escapeHtml(url)}"
                                onchange="App.updateApiConsoleUrl(this.value)"
                                placeholder="/rest/v1/table_name">
                        </div>

                        <div class="api-console-templates">
                            <label class="form-label">Templates:</label>
                            <select class="form-input" onchange="if(this.value) App.applyApiConsoleTemplate(this.value); this.value='';">
                                <option value="">Select a template...</option>
                                <optgroup label="Auth">
                                    <option value="auth-signup">Sign Up</option>
                                    <option value="auth-signin">Sign In</option>
                                    <option value="auth-refresh">Refresh Token</option>
                                    <option value="auth-user">Get User</option>
                                    <option value="auth-update-user">Update User</option>
                                    <option value="auth-logout">Logout</option>
                                </optgroup>
                                <optgroup label="REST">
                                    <option value="rest-select">Select All</option>
                                    <option value="rest-select-filter">Select with Filter</option>
                                    <option value="rest-insert">Insert Row</option>
                                    <option value="rest-update">Update Row</option>
                                    <option value="rest-delete">Delete Row</option>
                                </optgroup>
                                <optgroup label="Admin">
                                    <option value="admin-list-tables">List Tables</option>
                                    <option value="admin-create-table">Create Table</option>
                                    <option value="admin-get-schema">Get Schema</option>
                                    <option value="admin-drop-table">Drop Table</option>
                                </optgroup>
                            </select>
                        </div>

                        ${this.renderApiKeySettings()}

                        <div class="api-console-headers">
                            <div class="api-console-section-header">
                                <label class="form-label">Headers</label>
                                <button class="btn btn-secondary btn-sm" onclick="App.addApiConsoleHeader()">+ Add</button>
                            </div>
                            ${headers.map((h, i) => `
                                <div class="api-console-header-row">
                                    <input type="text" class="form-input" placeholder="Header name"
                                        value="${this.escapeHtml(h.key)}"
                                        onchange="App.updateApiConsoleHeader(${i}, 'key', this.value)">
                                    <input type="text" class="form-input" placeholder="Value"
                                        value="${this.escapeHtml(h.value)}"
                                        onchange="App.updateApiConsoleHeader(${i}, 'value', this.value)">
                                    <button class="btn-icon" onclick="App.removeApiConsoleHeader(${i})">&times;</button>
                                </div>
                            `).join('')}
                        </div>

                        ${showBody ? `
                            <div class="api-console-body">
                                <label class="form-label">Body (JSON)</label>
                                <textarea id="api-console-body" class="form-input api-console-body-input"
                                    rows="8" placeholder='{"key": "value"}'
                                    onchange="App.updateApiConsoleBody(this.value)">${this.escapeHtml(body)}</textarea>
                            </div>
                        ` : ''}

                        <button class="btn btn-primary api-console-send"
                            onclick="App.sendApiConsoleRequest()"
                            ${loading ? 'disabled' : ''}>
                            ${loading ? 'Sending...' : 'Send Request'}
                        </button>
                    </div>

                    <div class="api-console-response">
                        <h4>Response</h4>
                        ${response ? this.renderApiConsoleResponse() : `
                            <div class="api-console-empty">
                                Send a request to see the response
                            </div>
                        `}
                    </div>
                </div>
            </div>
        `;
    },

    renderApiConsoleResponse() {
        const { response, activeTab } = this.state.apiConsole;
        const statusClass = this.getStatusClass(response.status);

        return `
            <div class="api-console-response-status ${statusClass}">
                <span class="status-code">${response.status} ${response.statusText}</span>
                <span class="status-time">${response.time}ms</span>
            </div>

            <div class="api-console-response-tabs">
                <button class="tab ${activeTab === 'body' ? 'active' : ''}"
                    onclick="App.setApiConsoleResponseTab('body')">Body</button>
                <button class="tab ${activeTab === 'headers' ? 'active' : ''}"
                    onclick="App.setApiConsoleResponseTab('headers')">Headers</button>
                <button class="btn btn-secondary btn-sm copy-btn" onclick="App.copyApiConsoleResponse()">Copy</button>
            </div>

            <div class="api-console-response-content">
                ${activeTab === 'body' ? `
                    <pre class="api-console-json">${this.syntaxHighlightJson(response.body)}</pre>
                ` : `
                    <div class="api-console-headers-list">
                        ${Object.entries(response.headers).map(([key, value]) => `
                            <div class="header-item">
                                <span class="header-key">${this.escapeHtml(key)}:</span>
                                <span class="header-value">${this.escapeHtml(value)}</span>
                            </div>
                        `).join('')}
                    </div>
                `}
            </div>
        `;
    },

    renderApiKeySettings() {
        const { apiKeys, selectedKeyType, autoInjectKey } = this.state.apiConsole;

        if (!apiKeys) {
            return `
                <div class="api-console-auth-settings">
                    <label class="form-label">Authentication</label>
                    <div class="api-key-loading">Loading API keys...</div>
                </div>
            `;
        }

        return `
            <div class="api-console-auth-settings">
                <label class="form-label">Authentication</label>
                <div class="api-key-controls">
                    <label class="checkbox-label">
                        <input type="checkbox" ${autoInjectKey ? 'checked' : ''} onchange="App.toggleAutoInjectKey()">
                        Auto-inject apikey header
                    </label>
                    ${autoInjectKey ? `
                        <select class="form-input api-key-select" onchange="App.setApiKeyType(this.value)">
                            <option value="anon" ${selectedKeyType === 'anon' ? 'selected' : ''}>anon key</option>
                            <option value="service_role" ${selectedKeyType === 'service_role' ? 'selected' : ''}>service_role key</option>
                        </select>
                        <span class="api-key-hint">Will be added to /rest/v1/ and /auth/v1/ requests</span>
                    ` : ''}
                </div>
            </div>
        `;
    },

    renderApiConsoleHistoryDropdown() {
        const { history } = this.state.apiConsole;

        return `
            <div class="api-console-history-dropdown">
                ${history.length === 0 ? `
                    <div class="history-empty">No history yet</div>
                ` : `
                    ${history.map(h => `
                        <div class="history-item ${this.getStatusClass(h.status)}" onclick="App.loadFromApiConsoleHistory('${h.id}')">
                            <span class="history-method">${h.method}</span>
                            <span class="history-url">${this.escapeHtml(h.url.substring(0, 30))}${h.url.length > 30 ? '...' : ''}</span>
                            <span class="history-status">${h.status}</span>
                            <span class="history-time">${this.formatTimeAgo(h.timestamp)}</span>
                        </div>
                    `).join('')}
                    <div class="history-actions">
                        <button class="btn btn-secondary btn-sm" onclick="App.clearApiConsoleHistory()">Clear History</button>
                    </div>
                `}
            </div>
        `;
    },

    syntaxHighlightJson(json) {
        if (!json) return '';
        const escaped = this.escapeHtml(json);
        return escaped
            .replace(/"([^"]+)":/g, '<span class="json-key">"$1"</span>:')
            .replace(/: "([^"]*)"/g, ': <span class="json-string">"$1"</span>')
            .replace(/: (\d+)/g, ': <span class="json-number">$1</span>')
            .replace(/: (true|false)/g, ': <span class="json-boolean">$1</span>')
            .replace(/: (null)/g, ': <span class="json-null">$1</span>');
    },

    // SQL Browser methods

    sqlKeywords: [
        'SELECT', 'FROM', 'WHERE', 'AND', 'OR', 'NOT', 'IN', 'LIKE', 'BETWEEN',
        'ORDER', 'BY', 'ASC', 'DESC', 'LIMIT', 'OFFSET', 'GROUP', 'HAVING',
        'JOIN', 'LEFT', 'RIGHT', 'INNER', 'OUTER', 'ON', 'AS', 'DISTINCT',
        'INSERT', 'INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE', 'CREATE', 'TABLE',
        'DROP', 'ALTER', 'ADD', 'COLUMN', 'INDEX', 'PRIMARY', 'KEY', 'FOREIGN',
        'REFERENCES', 'NULL', 'NOT NULL', 'DEFAULT', 'UNIQUE', 'CHECK',
        'TRUNCATE', 'BEGIN', 'COMMIT', 'ROLLBACK', 'TRANSACTION',
        'CASE', 'WHEN', 'THEN', 'ELSE', 'END', 'CAST', 'COALESCE', 'NULLIF',
        'EXISTS', 'ALL', 'ANY', 'UNION', 'INTERSECT', 'EXCEPT',
        'COUNT', 'SUM', 'AVG', 'MIN', 'MAX', 'PRAGMA'
    ],

    async initSqlBrowser() {
        // Load history from localStorage
        const saved = localStorage.getItem('sblite_sql_browser_history');
        if (saved) {
            try {
                this.state.sqlBrowser.history = JSON.parse(saved);
            } catch (e) {
                this.state.sqlBrowser.history = [];
            }
        }

        // Load table list for autocomplete
        try {
            const res = await fetch('/_/api/tables');
            if (res.ok) {
                const tables = await res.json();
                this.state.sqlBrowser.tables = tables;
            }
        } catch (e) {
            console.error('Failed to load tables for autocomplete', e);
        }

        this.render();
    },

    saveSqlBrowserHistory() {
        const history = this.state.sqlBrowser.history.slice(0, 30);
        localStorage.setItem('sblite_sql_browser_history', JSON.stringify(history));
    },

    updateSqlQuery(query) {
        this.state.sqlBrowser.query = query;
        // Don't re-render on every keystroke
    },

    async runSqlQuery() {
        // Get current query from textarea
        const textarea = document.getElementById('sql-editor-input');
        const query = textarea ? textarea.value : this.state.sqlBrowser.query;
        this.state.sqlBrowser.query = query;

        if (!query.trim()) {
            this.state.sqlBrowser.error = 'Query cannot be empty';
            this.render();
            return;
        }

        // Check for destructive queries
        const upperQuery = query.toUpperCase().trim();
        const isDestructive = upperQuery.startsWith('DELETE') ||
                              upperQuery.startsWith('DROP') ||
                              upperQuery.startsWith('TRUNCATE') ||
                              upperQuery.startsWith('ALTER');

        if (isDestructive) {
            const confirmed = await this.confirmDestructiveQuery(query);
            if (!confirmed) return;
        }

        this.state.sqlBrowser.loading = true;
        this.state.sqlBrowser.error = null;
        this.state.sqlBrowser.results = null;
        this.state.sqlBrowser.page = 1;
        this.render();

        try {
            const res = await fetch('/_/api/sql', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    query,
                    postgres_mode: this.state.sqlBrowser.postgresMode
                })
            });

            const data = await res.json();

            if (data.error) {
                this.state.sqlBrowser.error = data.error;
            } else {
                this.state.sqlBrowser.results = data;

                // Add to history
                const historyEntry = {
                    id: Date.now().toString(),
                    query: query,
                    rowCount: data.row_count,
                    type: data.type,
                    timestamp: Date.now()
                };

                // Don't add duplicate consecutive queries
                const lastEntry = this.state.sqlBrowser.history[0];
                if (!lastEntry || lastEntry.query !== query) {
                    this.state.sqlBrowser.history.unshift(historyEntry);
                    this.saveSqlBrowserHistory();
                }
            }
        } catch (e) {
            this.state.sqlBrowser.error = 'Failed to execute query: ' + e.message;
        }

        this.state.sqlBrowser.loading = false;
        this.render();
    },

    async confirmDestructiveQuery(query) {
        const upperQuery = query.toUpperCase().trim();
        const needsTyping = upperQuery.startsWith('DROP') || upperQuery.startsWith('TRUNCATE');
        const isDeleteAll = upperQuery.startsWith('DELETE') && !upperQuery.includes('WHERE');

        return new Promise((resolve) => {
            this.state.modal = {
                type: 'confirmDestructive',
                data: {
                    query,
                    needsTyping,
                    isDeleteAll,
                    confirmText: '',
                    resolve
                }
            };
            this.render();
        });
    },

    confirmDestructiveAction() {
        const { needsTyping, confirmText, resolve } = this.state.modal.data;
        if (needsTyping && confirmText !== 'CONFIRM') {
            this.state.error = 'Please type CONFIRM to proceed';
            this.render();
            return;
        }
        this.closeModal();
        resolve(true);
    },

    cancelDestructiveAction() {
        const { resolve } = this.state.modal.data;
        this.closeModal();
        resolve(false);
    },

    updateDestructiveConfirmText(text) {
        this.state.modal.data.confirmText = text;
        // Don't re-render on every keystroke
    },

    clearSqlQuery() {
        this.state.sqlBrowser.query = '';
        this.state.sqlBrowser.results = null;
        this.state.sqlBrowser.error = null;
        this.render();
    },

    loadFromSqlHistory(id) {
        const entry = this.state.sqlBrowser.history.find(h => h.id === id);
        if (!entry) return;

        this.state.sqlBrowser.query = entry.query;
        this.state.sqlBrowser.showHistory = false;
        this.render();
    },

    clearSqlHistory() {
        this.state.sqlBrowser.history = [];
        localStorage.removeItem('sblite_sql_browser_history');
        this.state.sqlBrowser.showHistory = false;
        this.render();
    },

    toggleSqlHistory() {
        this.state.sqlBrowser.showHistory = !this.state.sqlBrowser.showHistory;
        this.render();
    },

    toggleSqlTablePicker() {
        this.state.sqlBrowser.showTablePicker = !this.state.sqlBrowser.showTablePicker;
        this.render();
    },

    togglePostgresMode(enabled) {
        this.state.sqlBrowser.postgresMode = enabled;
        // Save preference to localStorage
        localStorage.setItem('sql_postgres_mode', enabled ? 'true' : 'false');
        this.render();
    },

    insertTableName(tableName) {
        const textarea = document.getElementById('sql-editor-input');
        if (textarea) {
            const start = textarea.selectionStart;
            const end = textarea.selectionEnd;
            const text = textarea.value;
            textarea.value = text.substring(0, start) + tableName + text.substring(end);
            textarea.selectionStart = textarea.selectionEnd = start + tableName.length;
            textarea.focus();
            this.state.sqlBrowser.query = textarea.value;
        }
        this.state.sqlBrowser.showTablePicker = false;
        this.render();
    },

    setSqlPage(page) {
        this.state.sqlBrowser.page = page;
        this.render();
    },

    sortSqlResults(column) {
        const { sort } = this.state.sqlBrowser;
        if (sort.column === column) {
            // Toggle direction or clear
            if (sort.direction === 'asc') {
                this.state.sqlBrowser.sort = { column, direction: 'desc' };
            } else if (sort.direction === 'desc') {
                this.state.sqlBrowser.sort = { column: null, direction: null };
            }
        } else {
            this.state.sqlBrowser.sort = { column, direction: 'asc' };
        }
        this.state.sqlBrowser.page = 1;
        this.render();
    },

    exportSqlResults(format) {
        const { results } = this.state.sqlBrowser;
        if (!results || !results.rows || results.rows.length === 0) return;

        let content, filename, mimeType;

        if (format === 'csv') {
            const headers = results.columns.join(',');
            const rows = results.rows.map(row =>
                row.map(cell => {
                    if (cell === null) return '';
                    const str = String(cell);
                    return str.includes(',') || str.includes('"') || str.includes('\n')
                        ? '"' + str.replace(/"/g, '""') + '"'
                        : str;
                }).join(',')
            );
            content = headers + '\n' + rows.join('\n');
            filename = 'query_results.csv';
            mimeType = 'text/csv';
        } else {
            content = JSON.stringify(results.rows.map(row => {
                const obj = {};
                results.columns.forEach((col, i) => obj[col] = row[i]);
                return obj;
            }), null, 2);
            filename = 'query_results.json';
            mimeType = 'application/json';
        }

        const blob = new Blob([content], { type: mimeType });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        a.click();
        URL.revokeObjectURL(url);
    },

    highlightSql(sql) {
        if (!sql) return '';
        let result = this.escapeHtml(sql);

        // Highlight strings (single quotes)
        result = result.replace(/'([^']*)'/g, '<span class="sql-string">\'$1\'</span>');

        // Highlight numbers
        result = result.replace(/\b(\d+(?:\.\d+)?)\b/g, '<span class="sql-number">$1</span>');

        // Highlight keywords (case insensitive)
        this.sqlKeywords.forEach(keyword => {
            const regex = new RegExp('\\b(' + keyword + ')\\b', 'gi');
            result = result.replace(regex, '<span class="sql-keyword">$1</span>');
        });

        // Highlight comments
        result = result.replace(/--(.*)$/gm, '<span class="sql-comment">--$1</span>');

        return result;
    },

    renderSqlBrowserView() {
        const { query, results, loading, error, history, showHistory, showTablePicker, tables, page, pageSize, sort } = this.state.sqlBrowser;

        return `
            <div class="card-title">
                SQL Browser
                <div class="sql-browser-actions">
                    <button class="btn btn-secondary btn-sm" onclick="App.toggleSqlHistory()">
                        History (${history.length})
                    </button>
                    ${showHistory ? this.renderSqlHistoryDropdown() : ''}
                </div>
            </div>
            <div class="sql-browser-view">
                <div class="sql-editor-container">
                    <div class="sql-editor-header">
                        <span>SQL Editor</span>
                        <div class="sql-editor-toolbar">
                            <div class="postgres-mode-toggle">
                                <label class="toggle-label">
                                    <input type="checkbox"
                                           ${this.state.sqlBrowser.postgresMode ? 'checked' : ''}
                                           onchange="App.togglePostgresMode(this.checked)">
                                    <span class="toggle-text">PostgreSQL Mode</span>
                                </label>
                            </div>
                            <div class="table-picker-container">
                                <button class="btn btn-secondary btn-sm" onclick="App.toggleSqlTablePicker()">
                                    Tables ▼
                                </button>
                                ${showTablePicker ? this.renderTablePicker() : ''}
                            </div>
                        </div>
                    </div>
                    <div class="sql-editor-wrapper">
                        <pre class="sql-editor-highlight" aria-hidden="true">${this.highlightSql(query)}</pre>
                        <textarea
                            id="sql-editor-input"
                            class="sql-editor-input"
                            spellcheck="false"
                            placeholder="Enter SQL query..."
                            onkeydown="if(event.ctrlKey && event.key === 'Enter') { event.preventDefault(); App.runSqlQuery(); }"
                            oninput="App.updateSqlQuery(this.value); document.querySelector('.sql-editor-highlight').innerHTML = App.highlightSql(this.value);"
                        >${this.escapeHtml(query)}</textarea>
                    </div>
                    <div class="sql-editor-footer">
                        <div class="sql-editor-buttons">
                            <button class="btn btn-primary" onclick="App.runSqlQuery()" ${loading ? 'disabled' : ''}>
                                ${loading ? 'Running...' : 'Run Query'} <span class="shortcut">Ctrl+Enter</span>
                            </button>
                            <button class="btn btn-secondary" onclick="App.clearSqlQuery()">Clear</button>
                        </div>
                        ${results && results.rows && results.rows.length > 0 ? `
                            <div class="sql-export-buttons">
                                <button class="btn btn-secondary btn-sm" onclick="App.exportSqlResults('csv')">Export CSV</button>
                                <button class="btn btn-secondary btn-sm" onclick="App.exportSqlResults('json')">Export JSON</button>
                            </div>
                        ` : ''}
                    </div>
                </div>

                <div class="sql-results-container">
                    ${error ? `
                        <div class="sql-error">
                            <strong>Error:</strong> ${this.escapeHtml(error)}
                        </div>
                    ` : ''}

                    ${results ? this.renderSqlResults() : `
                        <div class="sql-results-empty">
                            Run a query to see results
                        </div>
                    `}
                </div>
            </div>
        `;
    },

    renderSqlResults() {
        const { results, page, pageSize, sort } = this.state.sqlBrowser;

        const translationInfo = results.was_translated ? `
            <div class="sql-translation-info">
                ✓ Translated from PostgreSQL syntax
                <details>
                    <summary>View translated query</summary>
                    <pre class="sql-translated-query">${this.escapeHtml(results.translated_query || '')}</pre>
                </details>
            </div>
        ` : '';

        if (results.type !== 'SELECT' && results.type !== 'PRAGMA') {
            return `
                <div class="sql-results-message">
                    <div class="sql-success">Query executed successfully</div>
                    <div class="sql-affected">${results.affected_rows || 0} row${results.affected_rows !== 1 ? 's' : ''} affected</div>
                    <div class="sql-time">${results.execution_time_ms}ms</div>
                    ${translationInfo}
                </div>
            `;
        }

        if (!results.rows || results.rows.length === 0) {
            return `
                <div class="sql-results-message">
                    <div>Query returned 0 rows</div>
                    <div class="sql-time">${results.execution_time_ms}ms</div>
                    ${translationInfo}
                </div>
            `;
        }

        // Sort rows if needed
        let sortedRows = [...results.rows];
        if (sort.column !== null) {
            const colIndex = results.columns.indexOf(sort.column);
            if (colIndex !== -1) {
                sortedRows.sort((a, b) => {
                    const aVal = a[colIndex];
                    const bVal = b[colIndex];
                    if (aVal === null) return 1;
                    if (bVal === null) return -1;
                    if (typeof aVal === 'number' && typeof bVal === 'number') {
                        return sort.direction === 'asc' ? aVal - bVal : bVal - aVal;
                    }
                    const aStr = String(aVal);
                    const bStr = String(bVal);
                    return sort.direction === 'asc' ? aStr.localeCompare(bStr) : bStr.localeCompare(aStr);
                });
            }
        }

        // Paginate
        const totalPages = Math.ceil(sortedRows.length / pageSize);
        const startIdx = (page - 1) * pageSize;
        const pageRows = sortedRows.slice(startIdx, startIdx + pageSize);

        return `
            <div class="sql-results-header">
                <span>${results.row_count} row${results.row_count !== 1 ? 's' : ''}, ${results.execution_time_ms}ms</span>
                <span>Page ${page} of ${totalPages}</span>
            </div>
            ${translationInfo}
            <div class="sql-results-table-wrapper">
                <table class="sql-results-table">
                    <thead>
                        <tr>
                            ${results.columns.map(col => `
                                <th onclick="App.sortSqlResults('${col}')" class="sortable">
                                    ${this.escapeHtml(col)}
                                    ${sort.column === col ? (sort.direction === 'asc' ? ' ▲' : ' ▼') : ''}
                                </th>
                            `).join('')}
                        </tr>
                    </thead>
                    <tbody>
                        ${pageRows.map(row => `
                            <tr>
                                ${row.map(cell => `
                                    <td title="${this.escapeHtml(String(cell ?? ''))}">
                                        ${cell === null
                                            ? '<span class="sql-null">null</span>'
                                            : this.escapeHtml(String(cell).substring(0, 100)) + (String(cell).length > 100 ? '...' : '')}
                                    </td>
                                `).join('')}
                            </tr>
                        `).join('')}
                    </tbody>
                </table>
            </div>
            ${totalPages > 1 ? `
                <div class="sql-pagination">
                    <button class="btn btn-secondary btn-sm" onclick="App.setSqlPage(1)" ${page === 1 ? 'disabled' : ''}>First</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.setSqlPage(${page - 1})" ${page === 1 ? 'disabled' : ''}>Prev</button>
                    ${this.renderSqlPageNumbers(page, totalPages)}
                    <button class="btn btn-secondary btn-sm" onclick="App.setSqlPage(${page + 1})" ${page === totalPages ? 'disabled' : ''}>Next</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.setSqlPage(${totalPages})" ${page === totalPages ? 'disabled' : ''}>Last</button>
                </div>
            ` : ''}
        `;
    },

    renderSqlPageNumbers(current, total) {
        const pages = [];
        const maxVisible = 5;

        if (total <= maxVisible) {
            for (let i = 1; i <= total; i++) pages.push(i);
        } else {
            pages.push(1);
            if (current > 3) pages.push('...');
            for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
                pages.push(i);
            }
            if (current < total - 2) pages.push('...');
            pages.push(total);
        }

        return pages.map(p => {
            if (p === '...') return '<span class="page-ellipsis">...</span>';
            return `<button class="btn btn-sm ${p === current ? 'btn-primary' : 'btn-secondary'}" onclick="App.setSqlPage(${p})">${p}</button>`;
        }).join('');
    },

    renderSqlHistoryDropdown() {
        const { history } = this.state.sqlBrowser;

        return `
            <div class="sql-history-dropdown">
                ${history.length === 0 ? `
                    <div class="history-empty">No history yet</div>
                ` : `
                    ${history.map(h => `
                        <div class="history-item" onclick="App.loadFromSqlHistory('${h.id}')">
                            <span class="history-query">${this.escapeHtml(h.query.substring(0, 40))}${h.query.length > 40 ? '...' : ''}</span>
                            <span class="history-meta">${h.rowCount ?? '-'} rows</span>
                            <span class="history-time">${this.formatTimeAgo(h.timestamp)}</span>
                        </div>
                    `).join('')}
                    <div class="history-actions">
                        <button class="btn btn-secondary btn-sm" onclick="App.clearSqlHistory()">Clear History</button>
                    </div>
                `}
            </div>
        `;
    },

    renderTablePicker() {
        const { tables } = this.state.sqlBrowser;

        return `
            <div class="table-picker-dropdown">
                ${tables.length === 0 ? `
                    <div class="picker-empty">No tables found</div>
                ` : `
                    ${tables.map(t => `
                        <div class="picker-item" onclick="App.insertTableName('${t.name}')">
                            ${this.escapeHtml(t.name)}
                        </div>
                    `).join('')}
                `}
            </div>
        `;
    },

    renderConfirmDestructiveModal() {
        const { query, needsTyping, isDeleteAll, confirmText } = this.state.modal.data;

        return `
            <div class="modal-header">
                <h3>⚠️ Confirm Destructive Query</h3>
                <button class="btn-icon" onclick="App.cancelDestructiveAction()">&times;</button>
            </div>
            <div class="modal-body">
                <p class="warning-text">This query may modify or delete data:</p>
                <pre class="destructive-query">${this.escapeHtml(query)}</pre>
                ${isDeleteAll ? `
                    <p class="danger-text">⚠️ Warning: This DELETE has no WHERE clause and will delete ALL rows!</p>
                ` : ''}
                ${needsTyping ? `
                    <p>Type <strong>CONFIRM</strong> to proceed:</p>
                    <input type="text" class="form-input" placeholder="Type CONFIRM"
                        onchange="App.updateDestructiveConfirmText(this.value)"
                        oninput="App.state.modal.data.confirmText = this.value">
                ` : ''}
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.cancelDestructiveAction()">Cancel</button>
                <button class="btn btn-danger" onclick="App.confirmDestructiveAction()">
                    ${needsTyping ? 'Execute' : 'Yes, Execute'}
                </button>
            </div>
        `;
    },

    // =====================
    // Edge Functions Methods
    // =====================

    async loadFunctions() {
        this.state.functions.loading = true;
        this.render();

        try {
            // Load functions list, status, and API keys in parallel
            const requests = [
                fetch('/_/api/functions'),
                fetch('/_/api/functions/status')
            ];

            // Also load API keys if not already loaded (for test console)
            if (!this.state.apiConsole.apiKeys) {
                requests.push(fetch('/_/api/apikeys'));
            }

            const responses = await Promise.all(requests);
            const [functionsRes, statusRes] = responses;

            if (functionsRes.ok) {
                const data = await functionsRes.json();
                // API returns { functions: [...], enabled: bool }
                this.state.functions.list = data.functions || [];
                // Also update status from this response if available
                if (data.enabled !== undefined) {
                    this.state.functions.status = this.state.functions.status || {};
                    this.state.functions.status.enabled = data.enabled;
                }
            } else if (functionsRes.status === 404) {
                // Functions not enabled
                this.state.functions.list = [];
            }

            if (statusRes.ok) {
                const statusData = await statusRes.json();
                this.state.functions.status = { ...this.state.functions.status, ...statusData };
            } else if (statusRes.status === 404) {
                this.state.functions.status = { enabled: false };
            }

            // Handle API keys response
            if (responses.length > 2 && responses[2].ok) {
                this.state.apiConsole.apiKeys = await responses[2].json();
            }
        } catch (e) {
            this.state.error = 'Failed to load functions';
        }

        this.state.functions.loading = false;
        this.render();
    },

    async loadFunctionsSecrets() {
        this.state.functions.secretsLoading = true;
        this.render();

        try {
            const res = await fetch('/_/api/secrets');
            if (res.ok) {
                const data = await res.json();
                // API returns { secrets: [...], enabled: bool }
                this.state.functions.secrets = data.secrets || [];
            } else {
                this.state.functions.secrets = [];
            }
        } catch (e) {
            this.state.functions.secrets = [];
        }

        this.state.functions.secretsLoading = false;
        this.render();
    },

    async selectFunction(name) {
        this.state.functions.selected = name;
        this.state.functions.config = null;
        this.state.functions.editor = {
            currentFile: null,
            content: '',
            originalContent: '',
            isDirty: false,
            tree: null,
            expandedFolders: {},
            isExpanded: false,
            monacoEditor: null,
            loading: true
        };

        this.render();

        // Load function config
        try {
            const res = await fetch(`/_/api/functions/${name}/config`);
            if (res.ok) {
                this.state.functions.config = await res.json();
            }
        } catch (err) {
            console.error('Failed to load function config:', err);
        }

        // Load file tree
        await this.loadFunctionFiles(name);

        // Initialize Monaco after render
        setTimeout(() => this.initMonacoEditor(), 100);
    },

    deselectFunction() {
        this.destroyMonacoEditor();
        this.state.functions.selected = null;
        this.state.functions.config = null;
        this.state.functions.editor = {
            currentFile: null,
            content: '',
            originalContent: '',
            isDirty: false,
            tree: null,
            expandedFolders: {},
            isExpanded: false,
            monacoEditor: null,
            loading: false
        };
        this.render();
    },

    showCreateFunctionModal() {
        this.state.modal = {
            type: 'createFunction',
            data: { name: '', template: 'default', error: null }
        };
        this.render();
    },

    async createFunction() {
        const { name, template } = this.state.modal.data;

        if (!name || !name.match(/^[a-z][a-z0-9-]*$/)) {
            this.state.modal.data.error = 'Function name must start with a letter and contain only lowercase letters, numbers, and hyphens';
            this.render();
            return;
        }

        try {
            const res = await fetch(`/_/api/functions/${name}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ template })
            });

            if (res.ok) {
                this.closeModal();
                await this.loadFunctions();
                this.selectFunction(name);
            } else {
                const err = await res.json();
                this.state.modal.data.error = err.error || err.message || 'Failed to create function';
                this.render();
            }
        } catch (e) {
            this.state.modal.data.error = 'Failed to create function';
            this.render();
        }
    },

    async deleteFunction(name) {
        if (!confirm(`Delete function "${name}"? This cannot be undone.`)) return;

        try {
            const res = await fetch(`/_/api/functions/${name}`, { method: 'DELETE' });
            if (res.ok) {
                if (this.state.functions.selected === name) {
                    this.state.functions.selected = null;
                    this.state.functions.config = null;
                }
                await this.loadFunctions();
            } else {
                this.state.error = 'Failed to delete function';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to delete function';
            this.render();
        }
    },

    async toggleFunctionJWT(name, enabled) {
        try {
            const res = await fetch(`/_/api/functions/${name}/config`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ verify_jwt: enabled })
            });

            if (res.ok) {
                this.state.functions.config = await res.json();
                // Also update in list
                const fn = this.state.functions.list.find(f => f.name === name);
                if (fn) fn.verify_jwt = enabled;
                this.render();
            } else {
                this.state.error = 'Failed to update function config';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to update function config';
            this.render();
        }
    },

    showAddSecretModal() {
        this.state.modal = {
            type: 'addSecret',
            data: { name: '', value: '', error: null }
        };
        this.render();
    },

    async addSecret() {
        const { name, value } = this.state.modal.data;

        if (!name || !name.match(/^[A-Z][A-Z0-9_]*$/)) {
            this.state.modal.data.error = 'Secret name must be uppercase and start with a letter (e.g., API_KEY)';
            this.render();
            return;
        }

        if (!value) {
            this.state.modal.data.error = 'Secret value is required';
            this.render();
            return;
        }

        try {
            const res = await fetch('/_/api/secrets', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ name, value })
            });

            if (res.ok) {
                this.closeModal();
                await this.loadFunctionsSecrets();
            } else {
                const err = await res.json();
                this.state.modal.data.error = err.error || err.message || 'Failed to add secret';
                this.render();
            }
        } catch (e) {
            this.state.modal.data.error = 'Failed to add secret';
            this.render();
        }
    },

    async deleteSecret(name) {
        if (!confirm(`Delete secret "${name}"? This cannot be undone.`)) return;

        try {
            const res = await fetch(`/_/api/secrets/${name}`, { method: 'DELETE' });
            if (res.ok) {
                await this.loadFunctionsSecrets();
            } else {
                this.state.error = 'Failed to delete secret';
                this.render();
            }
        } catch (e) {
            this.state.error = 'Failed to delete secret';
            this.render();
        }
    },

    // Function test console methods
    updateFunctionTestMethod(method) {
        this.state.functions.testConsole.method = method;
        this.updateTestConsole();
    },

    updateFunctionTestBody(body) {
        this.state.functions.testConsole.body = body;
    },

    addFunctionTestHeader() {
        this.state.functions.testConsole.headers.push({ key: '', value: '' });
        this.updateTestConsole();
    },

    updateFunctionTestHeader(index, field, value) {
        this.state.functions.testConsole.headers[index][field] = value;
    },

    removeFunctionTestHeader(index) {
        this.state.functions.testConsole.headers.splice(index, 1);
        this.updateTestConsole();
    },

    async invokeFunctionTest() {
        const { selected } = this.state.functions;
        const { method, body, headers } = this.state.functions.testConsole;

        if (!selected) return;

        this.state.functions.testConsole.loading = true;
        this.state.functions.testConsole.response = null;
        this.updateTestConsole();

        try {
            // Build headers
            const reqHeaders = {
                'Content-Type': 'application/json'
            };

            // Add API key (get from apiConsole state if available)
            if (this.state.apiConsole.apiKeys?.anon_key) {
                reqHeaders['apikey'] = this.state.apiConsole.apiKeys.anon_key;
                reqHeaders['Authorization'] = `Bearer ${this.state.apiConsole.apiKeys.anon_key}`;
            }

            // Add custom headers
            headers.forEach(h => {
                if (h.key && h.value) {
                    reqHeaders[h.key] = h.value;
                }
            });

            const startTime = Date.now();
            const fetchOptions = {
                method,
                headers: reqHeaders
            };

            if (method !== 'GET' && method !== 'HEAD' && body) {
                fetchOptions.body = body;
            }

            const res = await fetch(`/functions/v1/${selected}`, fetchOptions);
            const elapsed = Date.now() - startTime;

            let responseBody;
            const contentType = res.headers.get('content-type');
            if (contentType && contentType.includes('application/json')) {
                responseBody = await res.json();
            } else {
                responseBody = await res.text();
            }

            this.state.functions.testConsole.response = {
                status: res.status,
                statusText: res.statusText,
                headers: Object.fromEntries(res.headers.entries()),
                body: responseBody,
                elapsed
            };
        } catch (e) {
            this.state.functions.testConsole.response = {
                error: e.message || 'Request failed',
                status: 0,
                statusText: 'Error'
            };
        }

        this.state.functions.testConsole.loading = false;
        this.updateTestConsole();
    },

    updateTestConsole() {
        const container = document.querySelector('.test-dropdown');
        if (container) {
            container.innerHTML = this.renderFunctionTestConsole();
        }
    },

    // =====================
    // Editor State Methods
    // =====================

    async loadFunctionFiles(name) {
        this.state.functions.editor.loading = true;
        this.render();

        try {
            const res = await fetch(`/_/api/functions/${name}/files`);
            if (res.ok) {
                const tree = await res.json();
                this.state.functions.editor.tree = tree;

                // Auto-open index.ts if exists
                const indexFile = this.findFileInTree(tree, 'index.ts');
                if (indexFile) {
                    await this.openFunctionFile('index.ts');
                }
            }
        } catch (err) {
            console.error('Failed to load function files:', err);
        }

        this.state.functions.editor.loading = false;
        this.render();
    },

    findFileInTree(node, filename) {
        if (node.type === 'file' && node.name === filename) return node;
        if (node.children) {
            for (const child of node.children) {
                const found = this.findFileInTree(child, filename);
                if (found) return found;
            }
        }
        return null;
    },

    async openFunctionFile(path) {
        const name = this.state.functions.selected;
        if (!name) return;

        // Check for unsaved changes
        if (this.state.functions.editor.isDirty) {
            if (!confirm('You have unsaved changes. Discard them?')) return;
        }

        try {
            const res = await fetch(`/_/api/functions/${name}/files/${path}`);
            if (res.ok) {
                const data = await res.json();
                this.state.functions.editor.currentFile = path;
                this.state.functions.editor.content = data.content;
                this.state.functions.editor.originalContent = data.content;
                this.state.functions.editor.isDirty = false;

                // Update Monaco editor if it exists
                if (this.state.functions.editor.monacoEditor) {
                    this.state.functions.editor.monacoEditor.setValue(data.content);
                    this.setMonacoLanguage(path);
                }

                this.render();
            }
        } catch (err) {
            console.error('Failed to open file:', err);
        }
    },

    async saveFunctionFile() {
        const { selected } = this.state.functions;
        const { currentFile, monacoEditor } = this.state.functions.editor;
        if (!selected || !currentFile) return;

        const content = monacoEditor ? monacoEditor.getValue() : this.state.functions.editor.content;

        try {
            const res = await fetch(`/_/api/functions/${selected}/files/${currentFile}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ content })
            });

            if (res.ok) {
                this.state.functions.editor.originalContent = content;
                this.state.functions.editor.content = content;
                this.state.functions.editor.isDirty = false;
                // Update UI without full render to preserve Monaco
                this.updateEditorHeader();

                // Auto-restart runtime to apply changes
                await this.restartFunctionsRuntime();
            }
        } catch (err) {
            console.error('Failed to save file:', err);
            alert('Failed to save file');
        }
    },

    async restartFunctionsRuntime() {
        const { selected } = this.state.functions;
        try {
            const res = await fetch(`/_/api/functions/${selected}/restart`, { method: 'POST' });
            if (res.ok) {
                await this.loadFunctionsStatus();
                // Update status indicator without full render
                this.updateRuntimeStatus();
            } else {
                alert('Failed to restart runtime');
            }
        } catch (err) {
            console.error('Failed to restart runtime:', err);
        }
    },

    updateRuntimeStatus() {
        const indicator = document.querySelector('.runtime-status');
        if (indicator && this.state.functions.status) {
            const isRunning = this.state.functions.status.status === 'running';
            indicator.className = `runtime-status ${isRunning ? 'status-running' : 'status-stopped'}`;
            indicator.innerHTML = `
                <span class="status-indicator"></span>
                <span>Runtime: ${isRunning ? 'Running' : 'Stopped'}</span>
            `;
        }
    },

    async loadFunctionsStatus() {
        try {
            const res = await fetch('/_/api/functions/status');
            if (res.ok) {
                const statusData = await res.json();
                this.state.functions.status = { ...this.state.functions.status, ...statusData };
            }
        } catch (err) {
            console.error('Failed to load functions status:', err);
        }
    },

    setMonacoLanguage(path) {
        const editor = this.state.functions.editor.monacoEditor;
        if (!editor) return;

        const ext = path.split('.').pop();
        const languageMap = {
            'ts': 'typescript',
            'tsx': 'typescript',
            'js': 'javascript',
            'jsx': 'javascript',
            'mjs': 'javascript',
            'json': 'json',
            'html': 'html',
            'css': 'css',
            'md': 'markdown',
            'txt': 'plaintext'
        };

        const language = languageMap[ext] || 'plaintext';
        monaco.editor.setModelLanguage(editor.getModel(), language);
    },

    initMonacoEditor() {
        const container = document.getElementById('monaco-editor-container');
        if (!container || this.state.functions.editor.monacoEditor) return;

        // Wait for Monaco to load
        if (typeof monaco === 'undefined') {
            require(['vs/editor/editor.main'], () => this.initMonacoEditor());
            return;
        }

        const isDark = this.state.theme === 'dark';

        const editor = monaco.editor.create(container, {
            value: this.state.functions.editor.content || '// Select a file to edit',
            language: 'typescript',
            theme: isDark ? 'vs-dark' : 'vs',
            automaticLayout: true,
            minimap: { enabled: true },
            fontSize: 14,
            tabSize: 2,
            scrollBeyondLastLine: false,
            wordWrap: 'on'
        });

        this.state.functions.editor.monacoEditor = editor;

        // Track changes for dirty state
        editor.onDidChangeModelContent(() => {
            const current = editor.getValue();
            const original = this.state.functions.editor.originalContent;
            const wasDirty = this.state.functions.editor.isDirty;
            const isDirty = current !== original;

            if (wasDirty !== isDirty) {
                this.state.functions.editor.isDirty = isDirty;
                this.state.functions.editor.content = current;
                // Update just the header, not full render
                this.updateEditorHeader();
            }
        });

        // Keyboard shortcut for save
        editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
            this.saveFunctionFile();
        });
    },

    updateEditorHeader() {
        const { currentFile, isDirty } = this.state.functions.editor;
        const fileSpan = document.querySelector('.editor-header .current-file');
        const actionsDiv = document.querySelector('.editor-header .editor-actions');

        if (fileSpan) {
            fileSpan.textContent = currentFile ? `${currentFile}${isDirty ? ' \u25cf' : ''}` : 'No file selected';
        }
        if (actionsDiv) {
            actionsDiv.innerHTML = isDirty && currentFile
                ? `<button class="btn btn-primary btn-sm" onclick="App.saveFunctionFile()">Save</button>`
                : '';
        }

        // Update file tree item
        const activeItem = document.querySelector('.file-tree-item.file.active .file-name');
        if (activeItem && currentFile) {
            const filename = currentFile.split('/').pop();
            activeItem.textContent = `${filename}${isDirty ? ' \u25cf' : ''}`;
        }
    },

    destroyMonacoEditor() {
        if (this.state.functions.editor.monacoEditor) {
            this.state.functions.editor.monacoEditor.dispose();
            this.state.functions.editor.monacoEditor = null;
        }
    },

    toggleEditorExpand() {
        this.state.functions.editor.isExpanded = !this.state.functions.editor.isExpanded;
        // Destroy Monaco before render (render will destroy the container)
        this.destroyMonacoEditor();
        this.render();
        // Reinitialize Monaco after render
        setTimeout(() => this.initMonacoEditor(), 100);
    },

    toggleFolder(path) {
        const expanded = this.state.functions.editor.expandedFolders;
        expanded[path] = !expanded[path];
        this.render();
    },

    // Functions view rendering
    renderFunctionsView() {
        const { loading, status, list, selected, config, secrets, secretsLoading } = this.state.functions;

        if (loading) {
            return '<div class="loading">Loading...</div>';
        }

        // Check if functions are enabled
        if (!status || !status.enabled) {
            return this.renderFunctionsDisabledView();
        }

        // Two-column layout like tables view
        return `
            <div class="card-title">Edge Functions</div>
            <div class="functions-layout">
                <div class="functions-list-panel">
                    ${this.renderFunctionsList()}
                </div>
                <div class="functions-detail-panel">
                    ${selected ? this.renderFunctionDetail() : this.renderFunctionsEmptyState()}
                </div>
            </div>
        `;
    },

    renderFunctionsDisabledView() {
        return `
            <div class="card-title">Edge Functions</div>
            <div class="card">
                <div class="empty-state">
                    <div class="empty-icon">⚡</div>
                    <h3>Edge Functions Not Enabled</h3>
                    <p>Edge functions are not currently enabled on this server.</p>
                    <p>To enable edge functions, start the server with:</p>
                    <pre class="code-block">sblite serve --functions</pre>
                    <p class="text-muted">Edge functions allow you to run serverless TypeScript/JavaScript code.</p>
                </div>
            </div>
        `;
    },

    renderFunctionsList() {
        const { list, selected, status } = this.state.functions;

        return `
            <div class="panel-header">
                <span class="panel-title">Functions</span>
                <button class="btn btn-primary btn-sm" onclick="App.showCreateFunctionModal()">+ New</button>
            </div>
            <div class="runtime-status ${status?.running ? 'status-running' : 'status-stopped'}">
                <span class="status-indicator"></span>
                <span>Runtime: ${status?.running ? 'Running' : 'Stopped'}</span>
            </div>
            <div class="functions-items">
                ${list.length === 0 ? `
                    <div class="empty-list">No functions yet. Create one to get started.</div>
                ` : list.map(fn => `
                    <div class="function-item ${selected === fn.name ? 'active' : ''}"
                         onclick="App.selectFunction('${fn.name}')">
                        <span class="function-name">${this.escapeHtml(fn.name)}</span>
                        <span class="function-badges">
                            ${fn.verify_jwt === false ? '<span class="badge badge-warning">No JWT</span>' : ''}
                        </span>
                    </div>
                `).join('')}
            </div>
            <div class="panel-section">
                <div class="panel-section-header" onclick="App.toggleSecretsPanel()">
                    <span>Secrets</span>
                    <span class="section-toggle">${this.state.functions.showSecrets ? '−' : '+'}</span>
                </div>
                ${this.state.functions.showSecrets ? this.renderSecretsPanel() : ''}
            </div>
        `;
    },

    toggleSecretsPanel() {
        this.state.functions.showSecrets = !this.state.functions.showSecrets;
        if (this.state.functions.showSecrets && this.state.functions.secrets.length === 0) {
            this.loadFunctionsSecrets();
        }
        this.render();
    },

    renderSecretsPanel() {
        const { secrets, secretsLoading } = this.state.functions;

        if (secretsLoading) {
            return '<div class="panel-loading">Loading secrets...</div>';
        }

        return `
            <div class="secrets-panel">
                <div class="secrets-list">
                    ${secrets.length === 0 ? `
                        <div class="empty-list">No secrets configured</div>
                    ` : secrets.map(s => `
                        <div class="secret-item">
                            <span class="secret-name">${this.escapeHtml(s.name)}</span>
                            <button class="btn btn-danger btn-xs" onclick="event.stopPropagation(); App.deleteSecret('${s.name}')" title="Delete">×</button>
                        </div>
                    `).join('')}
                </div>
                <button class="btn btn-secondary btn-sm" onclick="App.showAddSecretModal()" style="margin-top: 8px; width: 100%">
                    + Add Secret
                </button>
            </div>
        `;
    },

    renderFunctionsEmptyState() {
        return `
            <div class="empty-state">
                <div class="empty-icon">👈</div>
                <h3>Select a Function</h3>
                <p>Choose a function from the list to view details and test it.</p>
            </div>
        `;
    },

    renderFunctionDetail() {
        const { selected, config } = this.state.functions;
        const { tree, currentFile, isDirty, isExpanded, loading } = this.state.functions.editor;
        const fn = this.state.functions.list.find(f => f.name === selected);

        if (!fn) return '';

        if (loading) {
            return `<div class="function-detail loading">Loading files...</div>`;
        }

        return `
            <div class="function-detail ${isExpanded ? 'expanded' : ''}">
                <div class="function-detail-header">
                    <div class="header-left">
                        <button class="btn btn-link" onclick="App.deselectFunction()">← Back</button>
                        <h2>${this.escapeHtml(selected)}</h2>
                    </div>
                    <div class="header-right">
                        <div class="dropdown">
                            <button class="btn btn-secondary btn-sm dropdown-toggle" onclick="this.nextElementSibling.classList.toggle('show')">
                                Config ▼
                            </button>
                            <div class="dropdown-menu">
                                <label class="dropdown-item">
                                    <input type="checkbox" ${fn.verify_jwt !== false ? 'checked' : ''}
                                        onchange="App.toggleFunctionJWT('${selected}', this.checked)">
                                    Require JWT
                                </label>
                            </div>
                        </div>
                        <div class="dropdown">
                            <button class="btn btn-secondary btn-sm dropdown-toggle" onclick="this.nextElementSibling.classList.toggle('show')">
                                Test ▼
                            </button>
                            <div class="dropdown-menu test-dropdown">
                                ${this.renderFunctionTestConsole ? this.renderFunctionTestConsole() : '<div class="p-2">Test console</div>'}
                            </div>
                        </div>
                        <button class="btn btn-secondary btn-sm" onclick="App.toggleEditorExpand()" title="${isExpanded ? 'Exit fullscreen' : 'Fullscreen'}">
                            ${isExpanded ? '⤢' : '⤡'}
                        </button>
                        <button class="btn btn-danger btn-sm" onclick="App.deleteFunction('${selected}')">Delete</button>
                    </div>
                </div>

                <div class="editor-container">
                    ${!isExpanded ? `
                        <div class="file-tree-panel">
                            <div class="file-tree-header">
                                <span>Files</span>
                                <div class="file-tree-actions">
                                    <button class="btn btn-xs" onclick="App.createNewFile()" title="New File">+ File</button>
                                    <button class="btn btn-xs" onclick="App.createNewFolder()" title="New Folder">+ Folder</button>
                                </div>
                            </div>
                            <div class="file-tree">
                                ${tree ? this.renderFileTree(tree) : '<div class="empty">No files</div>'}
                            </div>
                        </div>
                    ` : ''}

                    <div class="editor-panel">
                        <div class="editor-header">
                            <span class="current-file">
                                ${currentFile ? this.escapeHtml(currentFile) : 'No file selected'}
                                ${isDirty ? ' ●' : ''}
                            </span>
                            <div class="editor-actions">
                                ${isDirty && currentFile ? `
                                    <button class="btn btn-primary btn-sm" onclick="App.saveFunctionFile()">Save</button>
                                ` : ''}
                            </div>
                        </div>
                        <div id="monaco-editor-container" class="editor-content"></div>
                    </div>
                </div>
            </div>

            <!-- Context Menu -->
            <div id="file-context-menu" class="context-menu" style="display: none;">
                <div class="ctx-item ctx-new-file" onclick="App.createNewFile()">New File</div>
                <div class="ctx-item ctx-new-folder" onclick="App.createNewFolder()">New Folder</div>
                <div class="ctx-item" onclick="App.renameFile()">Rename</div>
                <div class="ctx-item ctx-delete" onclick="App.deleteFile()">Delete</div>
            </div>
        `;
    },

    renderFunctionTestConsole() {
        const { selected } = this.state.functions;
        const { method, body, headers, response, loading } = this.state.functions.testConsole;

        return `
            <div class="function-test-console">
                <div class="test-controls">
                    <select class="form-input method-select" value="${method}"
                            onchange="App.updateFunctionTestMethod(this.value)">
                        <option value="GET" ${method === 'GET' ? 'selected' : ''}>GET</option>
                        <option value="POST" ${method === 'POST' ? 'selected' : ''}>POST</option>
                        <option value="PUT" ${method === 'PUT' ? 'selected' : ''}>PUT</option>
                        <option value="PATCH" ${method === 'PATCH' ? 'selected' : ''}>PATCH</option>
                        <option value="DELETE" ${method === 'DELETE' ? 'selected' : ''}>DELETE</option>
                    </select>
                    <button class="btn btn-primary" onclick="App.invokeFunctionTest()" ${loading ? 'disabled' : ''}>
                        ${loading ? 'Invoking...' : 'Invoke'}
                    </button>
                </div>

                <div class="test-headers">
                    <label>Headers</label>
                    ${headers.map((h, i) => `
                        <div class="header-row">
                            <input type="text" class="form-input" placeholder="Key" value="${this.escapeHtml(h.key)}"
                                onchange="App.updateFunctionTestHeader(${i}, 'key', this.value)">
                            <input type="text" class="form-input" placeholder="Value" value="${this.escapeHtml(h.value)}"
                                onchange="App.updateFunctionTestHeader(${i}, 'value', this.value)">
                            <button class="btn btn-danger btn-xs" onclick="App.removeFunctionTestHeader(${i})">×</button>
                        </div>
                    `).join('')}
                    <button class="btn btn-secondary btn-xs" onclick="App.addFunctionTestHeader()">+ Add Header</button>
                </div>

                ${method !== 'GET' && method !== 'HEAD' ? `
                    <div class="test-body">
                        <label>Request Body (JSON)</label>
                        <textarea class="form-input code-textarea" rows="4"
                            onchange="App.updateFunctionTestBody(this.value)">${this.escapeHtml(body)}</textarea>
                    </div>
                ` : ''}

                ${response ? this.renderFunctionTestResponse() : ''}
            </div>
        `;
    },

    renderFunctionTestResponse() {
        const { response } = this.state.functions.testConsole;

        if (response.error) {
            return `
                <div class="test-response error">
                    <div class="response-header">
                        <span class="status-badge error">Error</span>
                    </div>
                    <pre class="response-body">${this.escapeHtml(response.error)}</pre>
                </div>
            `;
        }

        const isSuccess = response.status >= 200 && response.status < 300;
        const bodyStr = typeof response.body === 'object'
            ? JSON.stringify(response.body, null, 2)
            : String(response.body);

        return `
            <div class="test-response">
                <div class="response-header">
                    <span class="status-badge ${isSuccess ? 'success' : 'error'}">${response.status} ${response.statusText}</span>
                    <span class="response-time">${response.elapsed}ms</span>
                </div>
                <pre class="response-body">${this.escapeHtml(bodyStr)}</pre>
            </div>
        `;
    },

    renderCreateFunctionModal() {
        const { name, template, error } = this.state.modal.data;

        return `
            <div class="modal-header">
                <h3>Create Function</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                ${error ? `<div class="message message-error">${this.escapeHtml(error)}</div>` : ''}
                <div class="form-group">
                    <label class="form-label">Function Name</label>
                    <input type="text" class="form-input" placeholder="my-function"
                        value="${this.escapeHtml(name || '')}"
                        oninput="App.state.modal.data.name = this.value"
                        pattern="[a-z][a-z0-9-]*">
                    <small class="text-muted">Lowercase letters, numbers, and hyphens only. Must start with a letter.</small>
                </div>
                <div class="form-group">
                    <label class="form-label">Template</label>
                    <select class="form-input" onchange="App.state.modal.data.template = this.value">
                        <option value="default" ${template === 'default' ? 'selected' : ''}>Default - Basic JSON response</option>
                        <option value="supabase" ${template === 'supabase' ? 'selected' : ''}>Supabase - With Supabase client</option>
                        <option value="cors" ${template === 'cors' ? 'selected' : ''}>CORS - With CORS headers</option>
                    </select>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.createFunction()">Create</button>
            </div>
        `;
    },

    renderAddSecretModal() {
        const { name, value, error } = this.state.modal.data;

        return `
            <div class="modal-header">
                <h3>Add Secret</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                ${error ? `<div class="message message-error">${this.escapeHtml(error)}</div>` : ''}
                <div class="form-group">
                    <label class="form-label">Secret Name</label>
                    <input type="text" class="form-input" placeholder="API_KEY"
                        value="${this.escapeHtml(name || '')}"
                        oninput="App.state.modal.data.name = this.value.toUpperCase()"
                        style="text-transform: uppercase">
                    <small class="text-muted">Uppercase letters, numbers, and underscores only.</small>
                </div>
                <div class="form-group">
                    <label class="form-label">Secret Value</label>
                    <input type="password" class="form-input" placeholder="Enter secret value"
                        value="${this.escapeHtml(value || '')}"
                        oninput="App.state.modal.data.value = this.value">
                    <small class="text-muted">The value will be encrypted at rest. Requires server restart to take effect.</small>
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
                <button class="btn btn-primary" onclick="App.addSecret()">Add Secret</button>
            </div>
        `;
    },

    // File tree rendering
    renderFileTree(node, path = '') {
        const currentPath = path ? `${path}/${node.name}` : node.name;
        const isRoot = path === '';
        const isExpanded = isRoot || this.state.functions.editor.expandedFolders[currentPath];

        if (node.type === 'file') {
            const isActive = this.state.functions.editor.currentFile === currentPath;
            const isDirty = isActive && this.state.functions.editor.isDirty;
            return `
                <div class="file-tree-item file ${isActive ? 'active' : ''}"
                     onclick="App.openFunctionFile('${currentPath}')"
                     oncontextmenu="App.showFileContextMenu(event, '${currentPath}', 'file')">
                    <span class="file-icon">${this.getFileIcon(node.name)}</span>
                    <span class="file-name">${this.escapeHtml(node.name)}${isDirty ? ' ●' : ''}</span>
                </div>
            `;
        }

        // Directory
        const children = node.children || [];
        return `
            <div class="file-tree-item dir ${isRoot ? 'root' : ''}">
                <div class="dir-header" onclick="App.toggleFolder('${currentPath}')"
                     oncontextmenu="App.showFileContextMenu(event, '${currentPath}', 'dir')">
                    <span class="expand-icon">${isExpanded ? '▼' : '▶'}</span>
                    <span class="dir-name">${this.escapeHtml(node.name)}</span>
                </div>
                ${isExpanded ? `
                    <div class="dir-children">
                        ${children.map(child => this.renderFileTree(child, isRoot ? '' : currentPath)).join('')}
                    </div>
                ` : ''}
            </div>
        `;
    },

    // Storage methods

    async loadBuckets() {
        this.state.storage.loading = true;
        this.render();
        try {
            const res = await fetch('/_/api/storage/buckets');
            if (!res.ok) throw new Error('Failed to load buckets');
            this.state.storage.buckets = await res.json();
        } catch (err) {
            this.state.error = err.message;
            this.state.storage.buckets = [];
        } finally {
            this.state.storage.loading = false;
            this.render();
        }
    },

    renderStorageView() {
        const { buckets, selectedBucket, loading } = this.state.storage;

        if (loading && buckets.length === 0) {
            return '<div class="loading">Loading buckets...</div>';
        }

        return `
            <div class="storage-layout">
                <div class="storage-sidebar">
                    <div class="storage-sidebar-header">
                        <h3>Buckets</h3>
                        <button class="btn btn-primary btn-sm" onclick="App.showCreateBucketModal()">
                            + New
                        </button>
                    </div>
                    <div class="bucket-list">
                        ${buckets.length === 0 ? `
                            <p class="text-muted" style="padding: 1rem;">No buckets yet</p>
                        ` : buckets.map(bucket => `
                            <div class="bucket-item ${selectedBucket?.id === bucket.id ? 'selected' : ''}"
                                 onclick="App.selectBucket('${bucket.id}')">
                                <span class="bucket-name">${this.escapeHtml(bucket.name)}</span>
                                <div class="bucket-actions">
                                    <span class="bucket-badge ${bucket.public ? 'public' : 'private'}">
                                        ${bucket.public ? 'Public' : 'Private'}
                                    </span>
                                    <button class="btn-icon" onclick="event.stopPropagation(); App.showBucketSettingsModal('${bucket.id}')" title="Settings">&#9881;</button>
                                </div>
                            </div>
                        `).join('')}
                    </div>
                </div>
                <div class="storage-main">
                    ${selectedBucket ? this.renderFileBrowser() : `
                        <div class="storage-empty">
                            <p>Select a bucket to browse files</p>
                        </div>
                    `}
                </div>
            </div>
        `;
    },

    async selectBucket(bucketId) {
        const bucket = this.state.storage.buckets.find(b => b.id === bucketId);
        this.state.storage.selectedBucket = bucket;
        this.state.storage.currentPath = '';
        this.state.storage.selectedFiles = [];
        await this.loadObjects();
    },

    showCreateBucketModal() {
        this.state.modal = {
            type: 'createBucket',
            data: { name: '', isPublic: false, sizeLimit: '', mimeTypes: '', error: null }
        };
        this.render();
    },

    async createBucket(event) {
        if (event) event.preventDefault();
        const { name, isPublic, sizeLimit, mimeTypes } = this.state.modal.data;

        try {
            const body = { name, public: isPublic };
            if (sizeLimit) body.file_size_limit = parseInt(sizeLimit) * 1024 * 1024;
            if (mimeTypes) body.allowed_mime_types = mimeTypes.split(',').map(t => t.trim());

            const res = await fetch('/_/api/storage/buckets', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });
            if (!res.ok) {
                const err = await res.json();
                throw new Error(err.message || err.error || 'Failed to create bucket');
            }
            this.closeModal();
            await this.loadBuckets();
        } catch (err) {
            this.state.modal.data.error = err.message;
            this.render();
        }
    },

    renderCreateBucketModal() {
        const { name, isPublic, sizeLimit, mimeTypes, error } = this.state.modal.data;

        return `
            <div class="modal-header">
                <h3>Create Bucket</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                ${error ? `<div class="message message-error">${this.escapeHtml(error)}</div>` : ''}
                <form onsubmit="App.createBucket(event)">
                    <div class="form-group">
                        <label class="form-label">Bucket Name</label>
                        <input type="text" class="form-input" id="bucket-name" required
                               pattern="[a-z0-9-]+" title="Lowercase letters, numbers, and hyphens only"
                               value="${this.escapeHtml(name)}"
                               oninput="App.state.modal.data.name = this.value">
                    </div>
                    <div class="form-group">
                        <label class="checkbox-label">
                            <input type="checkbox" id="bucket-public" ${isPublic ? 'checked' : ''}
                                   onchange="App.state.modal.data.isPublic = this.checked">
                            Public bucket (files accessible without authentication)
                        </label>
                    </div>
                    <div class="form-group">
                        <label class="form-label">File Size Limit (MB, optional)</label>
                        <input type="number" class="form-input" id="bucket-size-limit" min="1"
                               value="${sizeLimit}"
                               oninput="App.state.modal.data.sizeLimit = this.value">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Allowed MIME Types (optional)</label>
                        <input type="text" class="form-input" id="bucket-mime-types"
                               placeholder="image/*, application/pdf"
                               value="${this.escapeHtml(mimeTypes)}"
                               oninput="App.state.modal.data.mimeTypes = this.value">
                        <small class="text-muted">Comma-separated list</small>
                    </div>
                    <div class="modal-footer">
                        <button type="button" class="btn" onclick="App.closeModal()">Cancel</button>
                        <button type="submit" class="btn btn-primary">Create Bucket</button>
                    </div>
                </form>
            </div>
        `;
    },

    showBucketSettingsModal(bucketId) {
        const bucket = this.state.storage.buckets.find(b => b.id === bucketId);
        if (!bucket) return;

        this.state.modal = {
            type: 'bucketSettings',
            bucket: bucket
        };
        this.render();
    },

    renderBucketSettingsModal() {
        const bucket = this.state.modal.bucket;
        const mimeTypes = bucket.allowed_mime_types ? bucket.allowed_mime_types.join(', ') : '';
        const sizeLimit = bucket.file_size_limit ? Math.round(bucket.file_size_limit / 1024 / 1024) : '';

        return `
            <div class="modal-header">
                <h3>Bucket Settings: ${this.escapeHtml(bucket.name)}</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body">
                <form onsubmit="App.updateBucket(event, '${bucket.id}')">
                    <div class="form-group">
                        <label class="checkbox-label">
                            <input type="checkbox" id="bucket-public" ${bucket.public ? 'checked' : ''}>
                            Public bucket
                        </label>
                    </div>
                    <div class="form-group">
                        <label class="form-label">File Size Limit (MB)</label>
                        <input type="number" class="form-input" id="bucket-size-limit" value="${sizeLimit}" min="1">
                    </div>
                    <div class="form-group">
                        <label class="form-label">Allowed MIME Types</label>
                        <input type="text" class="form-input" id="bucket-mime-types" value="${this.escapeHtml(mimeTypes)}" placeholder="image/*, application/pdf">
                    </div>
                    <hr style="margin: 1rem 0">
                    <div class="danger-zone">
                        <p class="text-muted">Danger Zone</p>
                        <button type="button" class="btn btn-danger btn-sm" onclick="App.emptyBucket('${bucket.id}')">Empty Bucket</button>
                        <button type="button" class="btn btn-danger btn-sm" onclick="App.deleteBucket('${bucket.id}')">Delete Bucket</button>
                    </div>
                    <div class="modal-actions">
                        <button type="button" class="btn" onclick="App.closeModal()">Cancel</button>
                        <button type="submit" class="btn btn-primary">Save Changes</button>
                    </div>
                </form>
            </div>
        `;
    },

    async updateBucket(event, bucketId) {
        event.preventDefault();
        const isPublic = document.getElementById('bucket-public').checked;
        const sizeLimit = document.getElementById('bucket-size-limit').value;
        const mimeTypes = document.getElementById('bucket-mime-types').value;

        try {
            const body = { public: isPublic };
            if (sizeLimit) body.file_size_limit = parseInt(sizeLimit) * 1024 * 1024;
            if (mimeTypes) body.allowed_mime_types = mimeTypes.split(',').map(t => t.trim());

            const res = await fetch(`/_/api/storage/buckets/${bucketId}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body)
            });
            if (!res.ok) throw new Error('Failed to update bucket');

            this.closeModal();
            this.showToast('Bucket updated', 'success');
            await this.loadBuckets();
        } catch (err) {
            this.showToast(err.message, 'error');
        }
    },

    async emptyBucket(bucketId) {
        const confirmed = confirm('Remove all files from this bucket? This cannot be undone.');
        if (!confirmed) return;

        try {
            const res = await fetch(`/_/api/storage/buckets/${bucketId}/empty`, { method: 'POST' });
            if (!res.ok) throw new Error('Failed to empty bucket');
            this.showToast('Bucket emptied', 'success');
            await this.loadObjects();
        } catch (err) {
            this.showToast(err.message, 'error');
        }
    },

    async deleteBucket(bucketId) {
        const bucket = this.state.storage.buckets.find(b => b.id === bucketId);
        const confirmed = confirm(`Delete bucket "${bucket?.name}"? The bucket must be empty.`);
        if (!confirmed) return;

        try {
            const res = await fetch(`/_/api/storage/buckets/${bucketId}`, { method: 'DELETE' });
            if (!res.ok) {
                const err = await res.json();
                throw new Error(err.message || 'Failed to delete bucket');
            }
            this.closeModal();
            this.state.storage.selectedBucket = null;
            this.showToast('Bucket deleted', 'success');
            await this.loadBuckets();
        } catch (err) {
            this.showToast(err.message, 'error');
        }
    },

    renderFileBrowser() {
        const { selectedBucket, objects, currentPath, viewMode, selectedFiles, loading } = this.state.storage;

        // Separate folders and files
        const folders = [];
        const files = [];
        const seenFolders = new Set();

        for (const obj of objects) {
            const relativePath = obj.name.slice(currentPath.length);
            const slashIndex = relativePath.indexOf('/');
            if (slashIndex > 0) {
                const folderName = relativePath.slice(0, slashIndex);
                if (!seenFolders.has(folderName)) {
                    seenFolders.add(folderName);
                    folders.push({ name: folderName, isFolder: true });
                }
            } else if (relativePath && !relativePath.endsWith('/')) {
                // Hide .gitkeep placeholder files used for empty folders
                if (relativePath === '.gitkeep') continue;
                files.push({ ...obj, displayName: relativePath, isFolder: false });
            }
        }

        const allItems = [...folders, ...files];

        const { hasMore } = this.state.storage;

        return `
            <div class="file-browser">
                ${this.renderFileBrowserToolbar()}
                <div class="file-browser-content ${viewMode}"
                     ondragover="App.handleDragOver(event)"
                     ondragleave="App.handleDragLeave(event)"
                     ondrop="App.handleDrop(event)">
                    ${loading && !hasMore ? '<div class="loading">Loading...</div>' : ''}
                    ${!loading && allItems.length === 0 ? `
                        <div class="file-browser-empty"
                             ondragover="event.preventDefault(); event.stopPropagation();"
                             ondrop="App.handleDrop(event)">
                            <p>No files in this ${currentPath ? 'folder' : 'bucket'}</p>
                            <p class="text-muted">Drag and drop files here or click Upload</p>
                        </div>
                    ` : ''}
                    ${viewMode === 'grid' ? this.renderFileGrid(allItems) : this.renderFileList(allItems)}
                    ${hasMore ? `
                        <div class="load-more-container">
                            <button class="btn btn-secondary" onclick="App.loadMoreObjects()" ${loading ? 'disabled' : ''}>
                                ${loading ? 'Loading...' : 'Load More'}
                            </button>
                        </div>
                    ` : ''}
                </div>
                ${this.renderUploadProgress()}
            </div>
        `;
    },

    renderFileBrowserToolbar() {
        const { selectedBucket, currentPath, viewMode, selectedFiles } = this.state.storage;
        const pathParts = currentPath.split('/').filter(Boolean);

        return `
            <div class="file-browser-toolbar">
                <div class="toolbar-left">
                    ${currentPath ? `<button class="btn btn-sm" onclick="App.navigateToFolder('..')">&#8592; Back</button>` : ''}
                    <div class="breadcrumb">
                        <span class="breadcrumb-item clickable" onclick="App.navigateToFolder('')">
                            ${this.escapeHtml(selectedBucket.name)}
                        </span>
                        ${pathParts.map((part, i) => `
                            <span class="breadcrumb-sep">/</span>
                            <span class="breadcrumb-item clickable" onclick="App.navigateToFolder('${this.escapeJsString(pathParts.slice(0, i + 1).join('/'))}/')">
                                ${this.escapeHtml(part)}
                            </span>
                        `).join('')}
                    </div>
                </div>
                <div class="toolbar-right">
                    <button class="btn btn-sm" onclick="App.createStorageFolder()">New Folder</button>
                    <button class="btn btn-primary btn-sm" onclick="App.triggerFileUpload()">Upload</button>
                    <input type="file" id="file-upload-input" multiple style="display:none" onchange="App.handleFileSelect(event)">
                    <div class="view-toggle">
                        <button class="btn btn-sm ${viewMode === 'grid' ? 'active' : ''}" onclick="App.setStorageViewMode('grid')" title="Grid view">&#8862;</button>
                        <button class="btn btn-sm ${viewMode === 'list' ? 'active' : ''}" onclick="App.setStorageViewMode('list')" title="List view">&#9776;</button>
                    </div>
                    ${selectedFiles.length > 0 ? `
                        <button class="btn btn-sm" onclick="App.downloadSelectedFiles()">Download (${selectedFiles.length})</button>
                        <button class="btn btn-sm btn-danger" onclick="App.deleteSelectedFiles()">Delete (${selectedFiles.length})</button>
                    ` : ''}
                </div>
            </div>
        `;
    },

    renderFileGrid(items) {
        if (items.length === 0) return '';
        const { selectedBucket, currentPath, selectedFiles } = this.state.storage;

        return `
            <div class="file-grid">
                ${items.map(item => {
                    if (item.isFolder) {
                        return `
                            <div class="file-card folder" ondblclick="App.navigateToFolder('${this.escapeJsString(currentPath + item.name)}/')">
                                <div class="file-icon folder-icon">&#128193;</div>
                                <div class="file-name">${this.escapeHtml(item.name)}</div>
                            </div>
                        `;
                    } else {
                        const isSelected = selectedFiles.includes(item.name);
                        const isImage = this.isImageFile(item.displayName);
                        // Use public URL for public buckets, dashboard download URL for private
                        const thumbUrl = isImage
                            ? (selectedBucket.public
                                ? `/storage/v1/object/public/${selectedBucket.name}/${item.name}`
                                : `/_/api/storage/objects/download?bucket=${encodeURIComponent(selectedBucket.name)}&path=${encodeURIComponent(item.name)}`)
                            : null;

                        return `
                            <div class="file-card ${isSelected ? 'selected' : ''}"
                                 data-filename="${this.escapeHtml(item.name)}">
                                <div class="file-select"
                                     onclick="event.stopPropagation(); App.toggleFileSelection('${this.escapeJsString(item.name)}')">
                                    <input type="checkbox" ${isSelected ? 'checked' : ''} style="pointer-events: none;">
                                </div>
                                <div class="file-content"
                                     onclick="App.openFilePreview('${this.escapeJsString(item.name)}')"
                                     ondblclick="App.downloadFile('${this.escapeJsString(item.name)}')">
                                    ${thumbUrl ? `
                                        <div class="file-thumbnail" style="background-image: url('${thumbUrl}')"></div>
                                    ` : `
                                        <div class="file-icon">${this.getFileIcon(item.displayName)}</div>
                                    `}
                                    <div class="file-name" title="${this.escapeHtml(item.displayName)}">${this.escapeHtml(item.displayName)}</div>
                                    <div class="file-size">${this.formatFileSize(item.size || 0)}</div>
                                </div>
                            </div>
                        `;
                    }
                }).join('')}
            </div>
        `;
    },

    renderFileList(items) {
        if (items.length === 0) return '';
        const { currentPath, selectedFiles } = this.state.storage;

        return `
            <div class="file-list">
                <table class="file-list-table">
                    <thead>
                        <tr>
                            <th class="col-checkbox"></th>
                            <th class="col-name">Name</th>
                            <th class="col-size">Size</th>
                            <th class="col-type">Type</th>
                            <th class="col-modified">Modified</th>
                        </tr>
                    </thead>
                    <tbody>
                        ${items.map(item => {
                            if (item.isFolder) {
                                return `
                                    <tr class="file-row folder" ondblclick="App.navigateToFolder('${this.escapeJsString(currentPath + item.name)}/')">
                                        <td class="col-checkbox"></td>
                                        <td class="col-name">
                                            <span class="file-icon">&#128193;</span>
                                            ${this.escapeHtml(item.name)}
                                        </td>
                                        <td class="col-size">-</td>
                                        <td class="col-type">Folder</td>
                                        <td class="col-modified">-</td>
                                    </tr>
                                `;
                            } else {
                                const isSelected = selectedFiles.includes(item.name);
                                return `
                                    <tr class="file-row ${isSelected ? 'selected' : ''}"
                                        data-filename="${this.escapeHtml(item.name)}">
                                        <td class="col-checkbox"
                                            onclick="event.stopPropagation(); App.toggleFileSelection('${this.escapeJsString(item.name)}')">
                                            <input type="checkbox" ${isSelected ? 'checked' : ''} style="pointer-events: none;">
                                        </td>
                                        <td class="col-name clickable"
                                            onclick="App.openFilePreview('${this.escapeJsString(item.name)}')"
                                            ondblclick="App.downloadFile('${this.escapeJsString(item.name)}')">
                                            <span class="file-icon">${this.getFileIcon(item.displayName)}</span>
                                            ${this.escapeHtml(item.displayName)}
                                        </td>
                                        <td class="col-size">${this.formatFileSize(item.size || 0)}</td>
                                        <td class="col-type">${this.escapeHtml(item.mime_type || '-')}</td>
                                        <td class="col-modified">${item.updated_at ? new Date(item.updated_at).toLocaleDateString() : '-'}</td>
                                    </tr>
                                `;
                            }
                        }).join('')}
                    </tbody>
                </table>
            </div>
        `;
    },

    async loadObjects(loadMore = false) {
        const { selectedBucket, currentPath, pageSize } = this.state.storage;
        if (!selectedBucket) return;

        this.state.storage.loading = true;

        // Reset offset if not loading more
        if (!loadMore) {
            this.state.storage.offset = 0;
        }

        this.render();

        try {
            const res = await fetch('/_/api/storage/objects/list', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    bucket: selectedBucket.name,
                    prefix: currentPath,
                    limit: pageSize + 1, // Request one extra to check if there's more
                    offset: this.state.storage.offset
                })
            });
            if (!res.ok) throw new Error('Failed to load objects');
            let objects = await res.json() || [];

            // Check if there are more items
            if (objects.length > pageSize) {
                this.state.storage.hasMore = true;
                objects = objects.slice(0, pageSize); // Remove the extra item
            } else {
                this.state.storage.hasMore = false;
            }

            if (loadMore) {
                // Append to existing objects
                this.state.storage.objects = [...this.state.storage.objects, ...objects];
            } else {
                this.state.storage.objects = objects;
            }

            // Update offset for next load
            this.state.storage.offset += objects.length;
        } catch (err) {
            this.showToast(err.message, 'error');
            if (!loadMore) {
                this.state.storage.objects = [];
            }
        } finally {
            this.state.storage.loading = false;
            this.render();
        }
    },

    async loadMoreObjects() {
        await this.loadObjects(true);
    },

    // File browser helper methods
    isImageFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico'].includes(ext);
    },

    isVideoFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['mp4', 'mov', 'avi', 'mkv', 'webm'].includes(ext);
    },

    isAudioFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['mp3', 'wav', 'ogg', 'flac', 'm4a', 'aac'].includes(ext);
    },

    isPdfFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ext === 'pdf';
    },

    isTextFile(filename) {
        const ext = filename.split('.').pop().toLowerCase();
        return ['txt', 'md', 'json', 'js', 'ts', 'css', 'html', 'xml', 'yaml', 'yml', 'py', 'go', 'java', 'c', 'cpp', 'h', 'sh', 'sql', 'log'].includes(ext);
    },

    openFilePreview(filename) {
        const { selectedBucket, currentPath, objects } = this.state.storage;
        const item = objects.find(o => o.name === filename);
        if (!item) return;

        const path = currentPath + filename;
        const url = selectedBucket.public
            ? `/storage/v1/object/public/${selectedBucket.name}/${path}`
            : `/_/api/storage/objects/download?bucket=${encodeURIComponent(selectedBucket.name)}&path=${encodeURIComponent(path)}`;

        this.state.modal = {
            type: 'filePreview',
            data: {
                filename: item.displayName || filename,
                fullPath: path,
                url: url,
                size: item.size,
                mimeType: item.mime_type,
                updatedAt: item.updated_at,
                bucket: selectedBucket.name,
                isPublic: selectedBucket.public
            }
        };
        this.render();
    },

    renderFilePreviewModal() {
        const { filename, url, size, mimeType, updatedAt, fullPath, bucket } = this.state.modal.data;
        const isImage = this.isImageFile(filename);
        const isVideo = this.isVideoFile(filename);
        const isAudio = this.isAudioFile(filename);
        const isPdf = this.isPdfFile(filename);

        let previewContent = '';
        if (isImage) {
            previewContent = `<img src="${url}" alt="${this.escapeHtml(filename)}" class="file-preview-image">`;
        } else if (isVideo) {
            previewContent = `<video src="${url}" controls class="file-preview-video"></video>`;
        } else if (isAudio) {
            previewContent = `
                <div class="file-preview-audio">
                    <div class="file-preview-icon">${this.getFileIcon(filename)}</div>
                    <audio src="${url}" controls></audio>
                </div>
            `;
        } else if (isPdf) {
            previewContent = `<iframe src="${url}" class="file-preview-pdf"></iframe>`;
        } else {
            previewContent = `
                <div class="file-preview-generic">
                    <div class="file-preview-icon">${this.getFileIcon(filename)}</div>
                    <div class="file-preview-name">${this.escapeHtml(filename)}</div>
                    <div class="file-preview-hint">Preview not available for this file type</div>
                </div>
            `;
        }

        return `
            <div class="modal-header">
                <h3 title="${this.escapeHtml(fullPath)}">${this.escapeHtml(filename)}</h3>
                <button class="btn-icon" onclick="App.closeModal()">&times;</button>
            </div>
            <div class="modal-body file-preview-body">
                ${previewContent}
            </div>
            <div class="modal-footer file-preview-footer">
                <div class="file-preview-info">
                    <span>${this.formatFileSize(size || 0)}</span>
                    <span>${mimeType || 'Unknown type'}</span>
                    ${updatedAt ? `<span>${new Date(updatedAt).toLocaleString()}</span>` : ''}
                </div>
                <div class="file-preview-actions">
                    <button class="btn btn-secondary" onclick="App.closeModal()">Close</button>
                    <button class="btn btn-primary" onclick="App.downloadFile('${this.escapeJsString(fullPath.split('/').pop())}')">Download</button>
                </div>
            </div>
        `;
    },

    getFileIcon(filename) {
        const ext = filename.split('.').pop();
        const icons = {
            'ts': '📘', 'tsx': '📘',
            'js': '📙', 'jsx': '📙', 'mjs': '📙',
            'json': '📋',
            'html': '🌐',
            'css': '🎨',
            'md': '📝',
            'txt': '📄'
        };
        return icons[ext] || '📄';
    },

    showFileContextMenu(event, path, type) {
        event.preventDefault();
        // Store for context menu actions
        this._contextMenuPath = path;
        this._contextMenuType = type;

        const menu = document.getElementById('file-context-menu');
        if (menu) {
            menu.style.display = 'block';
            menu.style.left = event.pageX + 'px';
            menu.style.top = event.pageY + 'px';

            // Show/hide options based on type
            menu.querySelector('.ctx-new-file').style.display = type === 'dir' ? 'block' : 'none';
            menu.querySelector('.ctx-new-folder').style.display = type === 'dir' ? 'block' : 'none';
        }
    },

    hideContextMenu() {
        const menu = document.getElementById('file-context-menu');
        if (menu) menu.style.display = 'none';
    },

    async createNewFile() {
        const name = this.state.functions.selected;
        if (!name) return;

        const path = this._contextMenuPath || '';
        const filename = prompt('Enter file name (e.g., utils.ts):');
        if (!filename) return;

        // Validate extension
        const ext = filename.split('.').pop();
        const allowed = ['ts', 'js', 'json', 'mjs', 'tsx', 'jsx', 'html', 'css', 'md', 'txt'];
        if (!allowed.includes(ext)) {
            alert(`File type .${ext} not allowed. Use: ${allowed.join(', ')}`);
            return;
        }

        const fullPath = path && this._contextMenuType === 'dir' ? `${path}/${filename}` : filename;

        try {
            const res = await fetch(`/_/api/functions/${name}/files/${fullPath}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ content: '' })
            });

            if (res.ok) {
                await this.loadFunctionFiles(name);
                await this.openFunctionFile(fullPath);
            } else {
                const err = await res.json();
                alert(err.error || 'Failed to create file');
            }
        } catch (err) {
            console.error('Failed to create file:', err);
        }

        this.hideContextMenu();
    },

    async createNewFolder() {
        const name = this.state.functions.selected;
        if (!name) return;

        const path = this._contextMenuPath || '';
        const dirname = prompt('Enter folder name:');
        if (!dirname) return;

        const fullPath = path && this._contextMenuType === 'dir' ? `${path}/${dirname}` : dirname;

        // Create folder by creating a placeholder file
        try {
            const res = await fetch(`/_/api/functions/${name}/files/${fullPath}/.gitkeep`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ content: '' })
            });

            if (res.ok) {
                await this.loadFunctionFiles(name);
            } else {
                alert('Failed to create folder');
            }
        } catch (err) {
            console.error('Failed to create folder:', err);
        }

        this.hideContextMenu();
    },

    async renameFile() {
        const name = this.state.functions.selected;
        const oldPath = this._contextMenuPath;
        if (!name || !oldPath) return;

        const oldName = oldPath.split('/').pop();
        const newName = prompt('Enter new name:', oldName);
        if (!newName || newName === oldName) return;

        const newPath = oldPath.replace(/[^/]+$/, newName);

        try {
            const res = await fetch(`/_/api/functions/${name}/files/rename`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ oldPath, newPath })
            });

            if (res.ok) {
                // Update current file if it was renamed
                if (this.state.functions.editor.currentFile === oldPath) {
                    this.state.functions.editor.currentFile = newPath;
                }
                await this.loadFunctionFiles(name);
            } else {
                alert('Failed to rename');
            }
        } catch (err) {
            console.error('Failed to rename:', err);
        }

        this.hideContextMenu();
    },

    async deleteFile() {
        const name = this.state.functions.selected;
        const path = this._contextMenuPath;
        if (!name || !path) return;

        if (!confirm(`Delete ${path}?`)) return;

        try {
            const res = await fetch(`/_/api/functions/${name}/files/${path}`, {
                method: 'DELETE'
            });

            if (res.ok) {
                // Clear editor if deleted file was open
                if (this.state.functions.editor.currentFile === path) {
                    this.state.functions.editor.currentFile = null;
                    this.state.functions.editor.content = '';
                    this.state.functions.editor.originalContent = '';
                    this.state.functions.editor.isDirty = false;
                    if (this.state.functions.editor.monacoEditor) {
                        this.state.functions.editor.monacoEditor.setValue('// Select a file to edit');
                    }
                }
                await this.loadFunctionFiles(name);
            } else {
                alert('Failed to delete');
            }
        } catch (err) {
            console.error('Failed to delete:', err);
        }

        this.hideContextMenu();
        const ext = filename.split('.').pop().toLowerCase();
        const iconMap = {
            // Documents
            'pdf': '&#128196;',
            'doc': '&#128196;',
            'docx': '&#128196;',
            'txt': '&#128196;',
            'rtf': '&#128196;',
            // Spreadsheets
            'xls': '&#128200;',
            'xlsx': '&#128200;',
            'csv': '&#128200;',
            // Images
            'jpg': '&#128247;',
            'jpeg': '&#128247;',
            'png': '&#128247;',
            'gif': '&#128247;',
            'webp': '&#128247;',
            'svg': '&#128247;',
            'bmp': '&#128247;',
            'ico': '&#128247;',
            // Videos
            'mp4': '&#127909;',
            'mov': '&#127909;',
            'avi': '&#127909;',
            'mkv': '&#127909;',
            'webm': '&#127909;',
            // Audio
            'mp3': '&#127925;',
            'wav': '&#127925;',
            'ogg': '&#127925;',
            'flac': '&#127925;',
            // Code
            'js': '&#128187;',
            'ts': '&#128187;',
            'json': '&#128187;',
            'html': '&#128187;',
            'css': '&#128187;',
            'py': '&#128187;',
            'go': '&#128187;',
            // Archives
            'zip': '&#128230;',
            'tar': '&#128230;',
            'gz': '&#128230;',
            'rar': '&#128230;',
            '7z': '&#128230;',
        };
        return iconMap[ext] || '&#128196;';
    },

    formatFileSize(bytes) {
        if (!bytes || bytes === 0) return '0 B';
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        const i = Math.floor(Math.log(bytes) / Math.log(1024));
        return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i];
    },

    setStorageViewMode(mode) {
        this.state.storage.viewMode = mode;
        this.render();
    },

    async navigateToFolder(path) {
        if (path === '..') {
            // Go up one level
            const parts = this.state.storage.currentPath.split('/').filter(Boolean);
            parts.pop();
            this.state.storage.currentPath = parts.length > 0 ? parts.join('/') + '/' : '';
        } else {
            this.state.storage.currentPath = path;
        }
        this.state.storage.selectedFiles = [];
        await this.loadObjects();
    },

    toggleFileSelection(filename) {
        const idx = this.state.storage.selectedFiles.indexOf(filename);
        if (idx >= 0) {
            this.state.storage.selectedFiles.splice(idx, 1);
        } else {
            this.state.storage.selectedFiles.push(filename);
        }
        this.render();
    },

    async createStorageFolder() {
        const { selectedBucket, currentPath } = this.state.storage;
        if (!selectedBucket) return;

        const folderName = prompt('Enter folder name:');
        if (!folderName || !folderName.trim()) return;

        // Validate folder name (no slashes, dots at start, etc.)
        const trimmedName = folderName.trim();
        if (trimmedName.includes('/') || trimmedName.includes('\\') || trimmedName.startsWith('.')) {
            this.showToast('Invalid folder name', 'error');
            return;
        }

        const folderPath = currentPath + trimmedName + '/';

        try {
            // Create folder by uploading a placeholder .gitkeep file using XHR (same as file uploads)
            const placeholder = new File([''], '.gitkeep', { type: 'application/octet-stream' });
            const formData = new FormData();
            formData.append('bucket', selectedBucket.name);
            formData.append('path', folderPath);
            formData.append('file', placeholder);

            await new Promise((resolve, reject) => {
                const xhr = new XMLHttpRequest();
                xhr.addEventListener('load', () => {
                    if (xhr.status >= 200 && xhr.status < 300) {
                        resolve();
                    } else {
                        let errorMsg = 'Failed to create folder';
                        try {
                            const errData = JSON.parse(xhr.responseText);
                            errorMsg = errData.message || errData.error || errorMsg;
                        } catch {}
                        reject(new Error(errorMsg));
                    }
                });
                xhr.addEventListener('error', () => reject(new Error('Network error')));
                xhr.open('POST', '/_/api/storage/objects/upload');
                xhr.send(formData);
            });

            await this.loadObjects();
            this.showToast(`Folder "${trimmedName}" created`, 'success');
        } catch (err) {
            console.error('Failed to create folder:', err);
            this.showToast(err.message || 'Failed to create folder', 'error');
        }
    },

    triggerFileUpload() {
        document.getElementById('file-upload-input').click();
    },

    handleFileSelect(event) {
        const files = Array.from(event.target.files);
        if (files.length > 0) {
            this.uploadFiles(files);
        }
        event.target.value = '';
    },

    async uploadFiles(files) {
        const { selectedBucket, currentPath } = this.state.storage;
        if (!selectedBucket) return;

        // Capture bucket and path at start to prevent race conditions if user navigates
        const bucketName = selectedBucket.name;
        const uploadPath = currentPath;

        // Create all upload items with unique IDs first
        const uploadItems = files.map((file, index) => ({
            id: Date.now() + '-' + index,
            name: file.name,
            size: file.size,
            progress: 0,
            status: 'uploading',
            file: file
        }));

        // Add all to state and render once
        this.state.storage.uploading.push(...uploadItems);
        this.render();

        // Upload files sequentially (keeps progress tracking simple)
        for (const uploadItem of uploadItems) {
            try {
                const formData = new FormData();
                formData.append('bucket', bucketName);
                formData.append('path', uploadPath);
                formData.append('file', uploadItem.file);

                await this.uploadWithProgress(formData, uploadItem);
                uploadItem.status = 'complete';
                uploadItem.progress = 100;
            } catch (err) {
                uploadItem.status = 'error';
                uploadItem.error = err.message;
            }
            // Clear file reference to free memory
            delete uploadItem.file;
            this.render();
        }

        // Refresh file list after all uploads complete
        await this.loadObjects();

        // Clear completed/errored uploads after delay
        setTimeout(() => {
            const completedIds = uploadItems.map(u => u.id);
            this.state.storage.uploading = this.state.storage.uploading.filter(
                u => !completedIds.includes(u.id) && u.status === 'uploading'
            );
            this.render();
        }, 3000);
    },

    // Throttle render calls during upload progress
    _lastProgressRender: 0,

    uploadWithProgress(formData, uploadItem) {
        return new Promise((resolve, reject) => {
            const xhr = new XMLHttpRequest();
            xhr.upload.addEventListener('progress', (e) => {
                if (e.lengthComputable) {
                    uploadItem.progress = Math.round((e.loaded / e.total) * 100);
                    // Throttle progress renders to max 10 per second
                    const now = Date.now();
                    if (now - this._lastProgressRender > 100) {
                        this._lastProgressRender = now;
                        this.render();
                    }
                }
            });
            xhr.addEventListener('load', () => {
                if (xhr.status >= 200 && xhr.status < 300) {
                    try {
                        resolve(JSON.parse(xhr.responseText));
                    } catch {
                        resolve({});
                    }
                } else {
                    let errorMsg = 'Upload failed';
                    try {
                        const errData = JSON.parse(xhr.responseText);
                        errorMsg = errData.message || errData.error || errorMsg;
                    } catch {}
                    reject(new Error(errorMsg));
                }
            });
            xhr.addEventListener('error', () => reject(new Error('Upload failed - network error')));
            xhr.open('POST', '/_/api/storage/objects/upload');
            xhr.send(formData);
        });
    },

    renderUploadProgress() {
        const { uploading } = this.state.storage;
        if (uploading.length === 0) return '';

        return `
            <div class="upload-progress-panel">
                <div class="upload-progress-header">
                    <span>Uploading ${uploading.length} file${uploading.length > 1 ? 's' : ''}</span>
                    <button class="btn-icon" onclick="App.clearCompletedUploads()">&#10005;</button>
                </div>
                <div class="upload-progress-list">
                    ${uploading.map(item => `
                        <div class="upload-item ${item.status}">
                            <span class="upload-name">${this.escapeHtml(item.name)}</span>
                            <div class="upload-bar">
                                <div class="upload-bar-fill" style="width: ${item.progress}%"></div>
                            </div>
                            <span class="upload-status">
                                ${item.status === 'uploading' ? item.progress + '%' : ''}
                                ${item.status === 'complete' ? '&#10003;' : ''}
                                ${item.status === 'error' ? '&#10007;' : ''}
                            </span>
                        </div>
                    `).join('')}
                </div>
            </div>
        `;
    },

    clearCompletedUploads() {
        this.state.storage.uploading = this.state.storage.uploading.filter(u => u.status === 'uploading');
        this.render();
    },

    handleDragOver(event) {
        event.preventDefault();
        event.currentTarget.classList.add('drag-over');
    },

    handleDragLeave(event) {
        event.currentTarget.classList.remove('drag-over');
    },

    handleDrop(event) {
        event.preventDefault();
        event.stopPropagation(); // Prevent double-handling from nested drop zones
        event.currentTarget.classList.remove('drag-over');
        const files = Array.from(event.dataTransfer.files);
        if (files.length > 0) {
            this.uploadFiles(files);
        }
    },

    downloadFile(filename) {
        const { selectedBucket, currentPath } = this.state.storage;
        const path = currentPath + filename;
        const url = `/_/api/storage/objects/download?bucket=${encodeURIComponent(selectedBucket.name)}&path=${encodeURIComponent(path)}`;

        // Create temporary link and click
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
    },

    async downloadSelectedFiles() {
        const { selectedFiles } = this.state.storage;
        if (selectedFiles.length === 0) return;

        // Download files sequentially with small delay
        for (const filename of selectedFiles) {
            this.downloadFile(filename);
            await new Promise(resolve => setTimeout(resolve, 500));
        }
    },

    async deleteSelectedFiles() {
        const { selectedBucket, currentPath, selectedFiles } = this.state.storage;
        if (selectedFiles.length === 0) return;

        const confirmed = confirm(`Delete ${selectedFiles.length} file(s)? This cannot be undone.`);
        if (!confirmed) return;

        try {
            const paths = selectedFiles.map(f => currentPath + f);
            const res = await fetch('/_/api/storage/objects', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    bucket: selectedBucket.name,
                    paths: paths
                })
            });

            if (!res.ok && res.status !== 207) {
                throw new Error('Failed to delete files');
            }

            this.state.storage.selectedFiles = [];
            this.showToast('Files deleted', 'success');
            await this.loadObjects();
        } catch (err) {
            this.showToast(err.message, 'error');
        }
    },

    selectAllFiles() {
        const { objects, currentPath } = this.state.storage;
        const files = [];
        for (const obj of objects) {
            const relativePath = obj.name.slice(currentPath.length);
            if (relativePath && !relativePath.includes('/') && !relativePath.endsWith('/')) {
                files.push(relativePath);
            }
        }
        this.state.storage.selectedFiles = files;
        this.render();
    },

    clearSelection() {
        this.state.storage.selectedFiles = [];
        this.render();
    },

    showToast(message, type = 'info') {
        // Simple toast implementation - could be enhanced later
        console.log(`[${type}] ${message}`);
        // For now, show as an alert for error messages
        if (type === 'error') {
            this.state.error = message;
            this.render();
            setTimeout(() => {
                this.state.error = null;
                this.render();
            }, 3000);
        }
    },

    copyToClipboard(text) {
        navigator.clipboard.writeText(text).then(() => {
            // Could show a toast notification here
        }).catch(err => {
            console.error('Failed to copy:', err);
        });
    },

    escapeHtml(str) {
        if (!str) return '';
        return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    },

    // Escape string for use in JavaScript string literals (single-quoted)
    escapeJsString(str) {
        if (!str) return '';
        return str.replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\n/g, '\\n').replace(/\r/g, '\\r');
    },

    // ==================== API Docs Methods ====================

    async initApiDocs() {
        this.state.apiDocs.loading = true;
        this.render();

        try {
            // Load tables and functions in parallel
            const [tablesRes, functionsRes] = await Promise.all([
                fetch('/_/api/apidocs/tables'),
                fetch('/_/api/apidocs/functions')
            ]);

            if (tablesRes.ok) {
                this.state.apiDocs.tables = await tablesRes.json();
            }
            if (functionsRes.ok) {
                this.state.apiDocs.functions = await functionsRes.json();
            }
        } catch (e) {
            console.error('Failed to load API docs data:', e);
        }

        this.state.apiDocs.loading = false;
        this.render();
    },

    navigateApiDocs(page, resource = null, rpc = null) {
        this.state.apiDocs.page = page;
        this.state.apiDocs.resource = resource;
        this.state.apiDocs.rpc = rpc;
        this.state.apiDocs.selectedTable = null;
        this.state.apiDocs.selectedFunction = null;

        if (resource) {
            this.loadApiDocsTable(resource);
        } else if (rpc) {
            this.loadApiDocsFunction(rpc);
        } else {
            this.render();
        }
    },

    async loadApiDocsTable(tableName) {
        this.state.apiDocs.loading = true;
        this.render();

        try {
            const res = await fetch(`/_/api/apidocs/tables/${encodeURIComponent(tableName)}`);
            if (res.ok) {
                this.state.apiDocs.selectedTable = await res.json();
            }
        } catch (e) {
            console.error('Failed to load table:', e);
        }

        this.state.apiDocs.loading = false;
        this.render();
    },

    async loadApiDocsFunction(funcName) {
        this.state.apiDocs.loading = true;
        this.render();

        try {
            const res = await fetch(`/_/api/apidocs/functions/${encodeURIComponent(funcName)}`);
            if (res.ok) {
                this.state.apiDocs.selectedFunction = await res.json();
            }
        } catch (e) {
            console.error('Failed to load function:', e);
        }

        this.state.apiDocs.loading = false;
        this.render();
    },

    setApiDocsLanguage(lang) {
        this.state.apiDocs.language = lang;
        this.render();
    },

    async updateTableDescription(tableName, description) {
        try {
            const res = await fetch(`/_/api/apidocs/tables/${encodeURIComponent(tableName)}/description`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ description })
            });
            if (res.ok && this.state.apiDocs.selectedTable) {
                this.state.apiDocs.selectedTable.description = description;
                this.render();
            }
        } catch (e) {
            console.error('Failed to update description:', e);
        }
    },

    async updateColumnDescription(tableName, columnName, description) {
        try {
            const res = await fetch(`/_/api/apidocs/tables/${encodeURIComponent(tableName)}/columns/${encodeURIComponent(columnName)}/description`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ description })
            });
            if (res.ok && this.state.apiDocs.selectedTable) {
                const col = this.state.apiDocs.selectedTable.columns.find(c => c.name === columnName);
                if (col) {
                    col.description = description;
                    this.render();
                }
            }
        } catch (e) {
            console.error('Failed to update column description:', e);
        }
    },

    async updateFunctionDescription(funcName, description) {
        try {
            const res = await fetch(`/_/api/apidocs/functions/${encodeURIComponent(funcName)}/description`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ description })
            });
            if (res.ok && this.state.apiDocs.selectedFunction) {
                this.state.apiDocs.selectedFunction.description = description;
                this.render();
            }
        } catch (e) {
            console.error('Failed to update function description:', e);
        }
    },

    renderApiDocsView() {
        const { page, resource, rpc, tables, functions, loading } = this.state.apiDocs;

        return `
            <div class="api-docs-layout">
                ${this.renderApiDocsSidebar()}
                <div class="api-docs-content">
                    ${loading ? '<div class="loading">Loading...</div>' : this.renderApiDocsPage()}
                </div>
            </div>
        `;
    },

    renderApiDocsSidebar() {
        const { page, resource, rpc, tables, functions } = this.state.apiDocs;

        return `
            <aside class="api-docs-sidebar">
                <div class="api-docs-sidebar-section">
                    <a class="api-docs-nav-item ${page === 'intro' && !resource && !rpc ? 'active' : ''}"
                       onclick="App.navigateApiDocs('intro')">Introduction</a>
                    <a class="api-docs-nav-item ${page === 'auth' ? 'active' : ''}"
                       onclick="App.navigateApiDocs('auth')">Authentication</a>
                    <a class="api-docs-nav-item ${page === 'users-management' ? 'active' : ''}"
                       onclick="App.navigateApiDocs('users-management')">User Management</a>
                </div>

                <div class="api-docs-sidebar-section">
                    <div class="api-docs-sidebar-title">Tables and Views</div>
                    <a class="api-docs-nav-item api-docs-nav-sub ${page === 'tables-intro' ? 'active' : ''}"
                       onclick="App.navigateApiDocs('tables-intro')">Introduction</a>
                    ${tables.map(t => `
                        <a class="api-docs-nav-item api-docs-nav-sub ${resource === t.name ? 'active' : ''}"
                           onclick="App.navigateApiDocs('table', '${this.escapeJsString(t.name)}')">${this.escapeHtml(t.name)}</a>
                    `).join('')}
                </div>

                <div class="api-docs-sidebar-section">
                    <div class="api-docs-sidebar-title">Stored Procedures</div>
                    <a class="api-docs-nav-item api-docs-nav-sub ${page === 'rpc-intro' ? 'active' : ''}"
                       onclick="App.navigateApiDocs('rpc-intro')">Introduction</a>
                    ${functions.map(f => `
                        <a class="api-docs-nav-item api-docs-nav-sub ${rpc === f.name ? 'active' : ''}"
                           onclick="App.navigateApiDocs('rpc', null, '${this.escapeJsString(f.name)}')">${this.escapeHtml(f.name)}</a>
                    `).join('')}
                </div>
            </aside>
        `;
    },

    renderApiDocsPage() {
        const { page, resource, rpc } = this.state.apiDocs;

        if (resource) {
            return this.renderApiDocsTablePage();
        }
        if (rpc) {
            return this.renderApiDocsFunctionPage();
        }

        switch (page) {
            case 'intro':
                return this.renderApiDocsIntro();
            case 'auth':
                return this.renderApiDocsAuth();
            case 'users-management':
                return this.renderApiDocsUserManagement();
            case 'tables-intro':
                return this.renderApiDocsTablesIntro();
            case 'rpc-intro':
                return this.renderApiDocsRpcIntro();
            default:
                return this.renderApiDocsIntro();
        }
    },

    renderApiDocsLanguageTabs() {
        const { language } = this.state.apiDocs;
        return `
            <div class="api-docs-lang-tabs">
                <button class="api-docs-lang-tab ${language === 'javascript' ? 'active' : ''}"
                        onclick="App.setApiDocsLanguage('javascript')">JavaScript</button>
                <button class="api-docs-lang-tab ${language === 'bash' ? 'active' : ''}"
                        onclick="App.setApiDocsLanguage('bash')">Bash</button>
            </div>
        `;
    },

    getApiBaseUrl() {
        return window.location.origin;
    },

    renderApiDocsIntro() {
        const { language } = this.state.apiDocs;
        const baseUrl = this.getApiBaseUrl();

        const jsCode = `import { createClient } from '@supabase/supabase-js'

const supabase = createClient(
  '${baseUrl}',
  'YOUR_ANON_KEY'
)`;

        const bashCode = `# Base URL for API requests
BASE_URL="${baseUrl}"

# Your API key (anon key for client-side, service_role for server-side)
API_KEY="YOUR_API_KEY"`;

        return `
            <div class="api-docs-page">
                ${this.renderApiDocsLanguageTabs()}
                <h1>Connect To Your Project</h1>
                <p>Your sblite project provides a RESTful API using PostgREST conventions.
                   You can access your data using the Supabase client library or direct HTTP requests.</p>

                <h2>Project URL</h2>
                <p>Your project's API is available at:</p>
                <div class="api-docs-code-block">
                    <code>${baseUrl}</code>
                </div>

                <h2>API Keys</h2>
                <p>You'll need an API key for authentication. You can find your keys in the
                   <a onclick="App.navigate('settings')" style="cursor:pointer;color:var(--accent)">Settings</a> page.</p>
                <ul>
                    <li><strong>anon key</strong> - Safe to use in client-side code. Respects Row Level Security.</li>
                    <li><strong>service_role key</strong> - Server-side only. Bypasses Row Level Security.</li>
                </ul>

                <h2>Getting Started</h2>
                <div class="api-docs-code-block">
                    <pre><code>${language === 'javascript' ? this.escapeHtml(jsCode) : this.escapeHtml(bashCode)}</code></pre>
                </div>

                <h2>Making Requests</h2>
                <p>Once configured, you can interact with your database tables, authenticate users,
                   and call stored procedures. See the sections in the sidebar for detailed examples.</p>
            </div>
        `;
    },

    renderApiDocsAuth() {
        const { language } = this.state.apiDocs;
        const baseUrl = this.getApiBaseUrl();

        const jsAnonCode = `// Client-side code with anon key
const supabase = createClient('${baseUrl}', ANON_KEY)

// The anon key is automatically included in requests
const { data } = await supabase.from('posts').select()`;

        const bashAnonCode = `# Include the anon key in requests
curl '${baseUrl}/rest/v1/posts' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Authorization: Bearer YOUR_ANON_KEY"`;

        const jsServiceCode = `// Server-side code with service_role key
// WARNING: Never expose this key in client-side code!
const supabase = createClient('${baseUrl}', SERVICE_ROLE_KEY)

// Bypasses RLS - use with caution
const { data } = await supabase.from('users').select()`;

        const bashServiceCode = `# Server-side only - bypasses RLS
curl '${baseUrl}/rest/v1/users' \\
  -H "apikey: YOUR_SERVICE_ROLE_KEY" \\
  -H "Authorization: Bearer YOUR_SERVICE_ROLE_KEY"`;

        return `
            <div class="api-docs-page">
                ${this.renderApiDocsLanguageTabs()}
                <h1>Authentication</h1>
                <p>sblite uses JWT (JSON Web Tokens) for API authentication.
                   All requests require an API key in the headers.</p>

                <h2>Client API Keys</h2>
                <p>The <code>anon</code> key is safe to use in client-side applications.
                   It respects Row Level Security (RLS) policies.</p>
                <div class="api-docs-code-block">
                    <pre><code>${language === 'javascript' ? this.escapeHtml(jsAnonCode) : this.escapeHtml(bashAnonCode)}</code></pre>
                </div>

                <h2>Service Keys</h2>
                <div class="api-docs-warning">
                    <strong>Warning:</strong> The <code>service_role</code> key bypasses Row Level Security.
                    Never expose this key in client-side code or public repositories.
                </div>
                <p>Use the service role key only in secure server-side environments.</p>
                <div class="api-docs-code-block">
                    <pre><code>${language === 'javascript' ? this.escapeHtml(jsServiceCode) : this.escapeHtml(bashServiceCode)}</code></pre>
                </div>

                <h2>Managing Keys</h2>
                <p>You can view and regenerate your API keys in the
                   <a onclick="App.navigate('settings')" style="cursor:pointer;color:var(--accent)">Settings</a> page.</p>
            </div>
        `;
    },

    renderApiDocsUserManagement() {
        const { language } = this.state.apiDocs;
        const baseUrl = this.getApiBaseUrl();

        const examples = {
            signup: {
                js: `const { data, error } = await supabase.auth.signUp({
  email: 'user@example.com',
  password: 'secure-password'
})`,
                bash: `curl -X POST '${baseUrl}/auth/v1/signup' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"email":"user@example.com","password":"secure-password"}'`
            },
            signin: {
                js: `const { data, error } = await supabase.auth.signInWithPassword({
  email: 'user@example.com',
  password: 'secure-password'
})`,
                bash: `curl -X POST '${baseUrl}/auth/v1/token?grant_type=password' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"email":"user@example.com","password":"secure-password"}'`
            },
            magiclink: {
                js: `const { data, error } = await supabase.auth.signInWithOtp({
  email: 'user@example.com'
})`,
                bash: `curl -X POST '${baseUrl}/auth/v1/otp' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"email":"user@example.com"}'`
            },
            getuser: {
                js: `const { data: { user } } = await supabase.auth.getUser()`,
                bash: `curl '${baseUrl}/auth/v1/user' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN"`
            },
            updateuser: {
                js: `const { data, error } = await supabase.auth.updateUser({
  data: { display_name: 'John Doe' }
})`,
                bash: `curl -X PUT '${baseUrl}/auth/v1/user' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"data":{"display_name":"John Doe"}}'`
            },
            signout: {
                js: `const { error } = await supabase.auth.signOut()`,
                bash: `curl -X POST '${baseUrl}/auth/v1/logout' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Authorization: Bearer USER_ACCESS_TOKEN"`
            },
            recovery: {
                js: `const { data, error } = await supabase.auth.resetPasswordForEmail(
  'user@example.com'
)`,
                bash: `curl -X POST '${baseUrl}/auth/v1/recover' \\
  -H "apikey: YOUR_ANON_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"email":"user@example.com"}'`
            },
            invite: {
                js: `// Admin only - use service_role key
const { data, error } = await supabase.auth.admin.inviteUserByEmail(
  'newuser@example.com'
)`,
                bash: `# Admin only - use service_role key
curl -X POST '${baseUrl}/auth/v1/invite' \\
  -H "apikey: YOUR_SERVICE_ROLE_KEY" \\
  -H "Authorization: Bearer YOUR_SERVICE_ROLE_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"email":"newuser@example.com"}'`
            }
        };

        const renderExample = (title, desc, key) => `
            <h3>${title}</h3>
            <p>${desc}</p>
            <div class="api-docs-code-block">
                <pre><code>${this.escapeHtml(language === 'javascript' ? examples[key].js : examples[key].bash)}</code></pre>
            </div>
        `;

        return `
            <div class="api-docs-page">
                ${this.renderApiDocsLanguageTabs()}
                <h1>User Management</h1>
                <p>sblite provides Supabase-compatible authentication endpoints for managing users.</p>

                ${renderExample('Sign Up', 'Create a new user with email and password.', 'signup')}
                ${renderExample('Sign In with Password', 'Authenticate an existing user.', 'signin')}
                ${renderExample('Sign In with Magic Link', 'Send a magic link email for passwordless authentication.', 'magiclink')}
                ${renderExample('Get User', 'Retrieve the currently authenticated user.', 'getuser')}
                ${renderExample('Update User', 'Update the current user\'s metadata.', 'updateuser')}
                ${renderExample('Sign Out', 'Log out the current user.', 'signout')}
                ${renderExample('Password Recovery', 'Send a password reset email.', 'recovery')}
                ${renderExample('Invite User (Admin)', 'Invite a new user by email. Requires admin privileges.', 'invite')}
            </div>
        `;
    },

    renderApiDocsTablesIntro() {
        const { tables } = this.state.apiDocs;

        return `
            <div class="api-docs-page">
                ${this.renderApiDocsLanguageTabs()}
                <h1>Tables and Views</h1>
                <p>Your database tables are automatically exposed via a RESTful API.
                   Select a table from the sidebar to see its schema and example queries.</p>

                <h2>Available Tables</h2>
                ${tables.length === 0
                    ? '<p>No user tables found. Create a table in the <a onclick="App.navigate(\'tables\')" style="cursor:pointer;color:var(--accent)">Tables</a> view to get started.</p>'
                    : `<ul>${tables.map(t => `<li><a onclick="App.navigateApiDocs('table', '${this.escapeJsString(t.name)}')" style="cursor:pointer;color:var(--accent)">${this.escapeHtml(t.name)}</a></li>`).join('')}</ul>`
                }

                <h2>REST Conventions</h2>
                <p>The API follows PostgREST conventions:</p>
                <ul>
                    <li><code>GET /rest/v1/table</code> - Read rows</li>
                    <li><code>POST /rest/v1/table</code> - Insert rows</li>
                    <li><code>PATCH /rest/v1/table</code> - Update rows</li>
                    <li><code>DELETE /rest/v1/table</code> - Delete rows</li>
                </ul>
            </div>
        `;
    },

    renderApiDocsRpcIntro() {
        const { language, functions } = this.state.apiDocs;
        const baseUrl = this.getApiBaseUrl();

        const jsCode = `const { data, error } = await supabase.rpc('function_name', {
  param1: 'value1',
  param2: 123
})`;

        const bashCode = `curl -X POST '${baseUrl}/rest/v1/rpc/function_name' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"param1":"value1","param2":123}'`;

        return `
            <div class="api-docs-page">
                ${this.renderApiDocsLanguageTabs()}
                <h1>Stored Procedures</h1>
                <p>Call database functions (stored procedures) via RPC.
                   Select a function from the sidebar to see its signature and parameters.</p>

                <h2>Calling Functions</h2>
                <p>Functions are called via the <code>/rest/v1/rpc/</code> endpoint:</p>
                <div class="api-docs-code-block">
                    <pre><code>${language === 'javascript' ? this.escapeHtml(jsCode) : this.escapeHtml(bashCode)}</code></pre>
                </div>

                <h2>Available Functions</h2>
                ${functions.length === 0
                    ? '<p>No stored procedures found. Create a function using the SQL Browser to get started.</p>'
                    : `<ul>${functions.map(f => `<li><a onclick="App.navigateApiDocs('rpc', null, '${this.escapeJsString(f.name)}')" style="cursor:pointer;color:var(--accent)">${this.escapeHtml(f.name)}</a></li>`).join('')}</ul>`
                }
            </div>
        `;
    },

    renderApiDocsTablePage() {
        const { language, selectedTable } = this.state.apiDocs;
        if (!selectedTable) return '<div class="loading">Loading...</div>';

        const { name, description, columns } = selectedTable;
        const baseUrl = this.getApiBaseUrl();

        return `
            <div class="api-docs-page">
                ${this.renderApiDocsLanguageTabs()}
                <h1>${this.escapeHtml(name)}</h1>

                <div class="api-docs-section">
                    <h2>Description</h2>
                    <div class="api-docs-editable">
                        <textarea id="table-desc" class="api-docs-desc-input" rows="2"
                                  placeholder="Add a description for this table...">${this.escapeHtml(description || '')}</textarea>
                        <button class="btn btn-sm" onclick="App.updateTableDescription('${this.escapeJsString(name)}', document.getElementById('table-desc').value)">Save</button>
                    </div>
                </div>

                ${columns.map(col => this.renderApiDocsColumn(name, col)).join('')}

                ${this.renderApiDocsTableOperations(name, columns)}
            </div>
        `;
    },

    renderApiDocsColumn(tableName, col) {
        const { language } = this.state.apiDocs;
        const required = col.required ? '<span class="api-docs-badge required">REQUIRED</span>' : '<span class="api-docs-badge optional">OPTIONAL</span>';

        const jsSelectCode = `let { data, error } = await supabase
  .from('${tableName}')
  .select('${col.name}')`;

        const bashSelectCode = `curl '${this.getApiBaseUrl()}/rest/v1/${tableName}?select=${col.name}' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY"`;

        return `
            <div class="api-docs-column-section">
                <h3>Column: ${this.escapeHtml(col.name)}</h3>
                <div class="api-docs-column-meta">
                    ${required}
                    <span class="api-docs-badge type">TYPE: ${this.escapeHtml(col.type)}</span>
                    <span class="api-docs-badge format">FORMAT: ${this.escapeHtml(col.format)}</span>
                </div>

                <div class="api-docs-editable">
                    <input type="text" id="col-desc-${col.name}" class="api-docs-desc-input-inline"
                           value="${this.escapeHtml(col.description || '')}"
                           placeholder="Add column description...">
                    <button class="btn btn-sm" onclick="App.updateColumnDescription('${this.escapeJsString(tableName)}', '${this.escapeJsString(col.name)}', document.getElementById('col-desc-${col.name}').value)">Save</button>
                </div>

                <div class="api-docs-code-block">
                    <div class="api-docs-code-title">SELECT ${this.escapeHtml(col.name)}</div>
                    <pre><code>${language === 'javascript' ? this.escapeHtml(jsSelectCode) : this.escapeHtml(bashSelectCode)}</code></pre>
                </div>
            </div>
        `;
    },

    renderApiDocsTableOperations(tableName, columns) {
        const { language } = this.state.apiDocs;
        const baseUrl = this.getApiBaseUrl();
        const allColumns = columns.map(c => c.name).join(', ');
        const sampleInsert = columns.filter(c => !c.name.includes('id') && c.required)
            .slice(0, 3)
            .map(c => `${c.name}: ${c.type === 'number' ? '123' : c.type === 'boolean' ? 'true' : `'value'`}`)
            .join(',\n    ');

        const examples = {
            readAll: {
                title: 'Read All Rows',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .select('*')`,
                bash: `curl '${baseUrl}/rest/v1/${tableName}?select=*' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY"`
            },
            readCols: {
                title: 'Read Specific Columns',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .select('${allColumns}')`,
                bash: `curl '${baseUrl}/rest/v1/${tableName}?select=${encodeURIComponent(allColumns)}' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY"`
            },
            pagination: {
                title: 'With Pagination',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .select('*')
  .range(0, 9)`,
                bash: `curl '${baseUrl}/rest/v1/${tableName}?select=*' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Range: 0-9"`
            },
            filter: {
                title: 'With Filtering',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .select('*')
  .eq('column_name', 'value')
  .gt('other_column', 10)`,
                bash: `curl '${baseUrl}/rest/v1/${tableName}?select=*&column_name=eq.value&other_column=gt.10' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY"`
            },
            insertOne: {
                title: 'Insert a Row',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .insert({
    ${sampleInsert || 'column: value'}
  })
  .select()`,
                bash: `curl -X POST '${baseUrl}/rest/v1/${tableName}' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Prefer: return=representation" \\
  -d '{${sampleInsert ? sampleInsert.replace(/\n\s*/g, ' ').replace(/'/g, '"') : '"column":"value"'}}'`
            },
            insertMany: {
                title: 'Insert Many Rows',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .insert([
    { ${sampleInsert ? sampleInsert.split(',')[0] || 'column: value' : 'column: value'} },
    { ${sampleInsert ? sampleInsert.split(',')[0] || 'column: value2' : 'column: value2'} }
  ])
  .select()`,
                bash: `curl -X POST '${baseUrl}/rest/v1/${tableName}' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Prefer: return=representation" \\
  -d '[{"column":"value1"},{"column":"value2"}]'`
            },
            upsert: {
                title: 'Upsert Rows',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .upsert({ id: 1, ${sampleInsert ? sampleInsert.split(',')[0] || 'column: value' : 'column: value'} })
  .select()`,
                bash: `curl -X POST '${baseUrl}/rest/v1/${tableName}' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Prefer: return=representation,resolution=merge-duplicates" \\
  -d '{"id":1,"column":"value"}'`
            },
            update: {
                title: 'Update Rows',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .update({ ${sampleInsert ? sampleInsert.split(',')[0] || 'column: newValue' : 'column: newValue'} })
  .eq('id', 1)
  .select()`,
                bash: `curl -X PATCH '${baseUrl}/rest/v1/${tableName}?id=eq.1' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Prefer: return=representation" \\
  -d '{"column":"newValue"}'`
            },
            delete: {
                title: 'Delete Rows',
                js: `let { data, error } = await supabase
  .from('${tableName}')
  .delete()
  .eq('id', 1)`,
                bash: `curl -X DELETE '${baseUrl}/rest/v1/${tableName}?id=eq.1' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY"`
            }
        };

        const renderOp = (key) => `
            <div class="api-docs-code-block">
                <div class="api-docs-code-title">${examples[key].title}</div>
                <pre><code>${language === 'javascript' ? this.escapeHtml(examples[key].js) : this.escapeHtml(examples[key].bash)}</code></pre>
            </div>
        `;

        return `
            <div class="api-docs-section">
                <h2>Read Rows</h2>
                ${renderOp('readAll')}
                ${renderOp('readCols')}
                ${renderOp('pagination')}
                ${renderOp('filter')}
            </div>

            <div class="api-docs-section">
                <h2>Insert Rows</h2>
                ${renderOp('insertOne')}
                ${renderOp('insertMany')}
                ${renderOp('upsert')}
            </div>

            <div class="api-docs-section">
                <h2>Update Rows</h2>
                ${renderOp('update')}
            </div>

            <div class="api-docs-section">
                <h2>Delete Rows</h2>
                ${renderOp('delete')}
            </div>
        `;
    },

    renderApiDocsFunctionPage() {
        const { language, selectedFunction } = this.state.apiDocs;
        if (!selectedFunction) return '<div class="loading">Loading...</div>';

        const { name, description, return_type, returns_set, arguments: args } = selectedFunction;
        const baseUrl = this.getApiBaseUrl();

        const argsObj = args && args.length > 0
            ? args.map(a => `${a.name}: ${a.type === 'number' ? '123' : a.type === 'boolean' ? 'true' : `'value'`}`).join(',\n    ')
            : '';

        const argsJson = args && args.length > 0
            ? args.map(a => `"${a.name}":${a.type === 'number' ? '123' : a.type === 'boolean' ? 'true' : '"value"'}`).join(',')
            : '';

        const jsCode = argsObj
            ? `let { data, error } = await supabase.rpc('${name}', {
    ${argsObj}
  })`
            : `let { data, error } = await supabase.rpc('${name}')`;

        const bashCode = argsJson
            ? `curl -X POST '${baseUrl}/rest/v1/rpc/${name}' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{${argsJson}}'`
            : `curl -X POST '${baseUrl}/rest/v1/rpc/${name}' \\
  -H "apikey: YOUR_API_KEY" \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{}'`;

        return `
            <div class="api-docs-page">
                ${this.renderApiDocsLanguageTabs()}
                <h1>${this.escapeHtml(name)}</h1>

                <div class="api-docs-section">
                    <h2>Description</h2>
                    <div class="api-docs-editable">
                        <textarea id="func-desc" class="api-docs-desc-input" rows="2"
                                  placeholder="Add a description for this function...">${this.escapeHtml(description || '')}</textarea>
                        <button class="btn btn-sm" onclick="App.updateFunctionDescription('${this.escapeJsString(name)}', document.getElementById('func-desc').value)">Save</button>
                    </div>
                </div>

                <div class="api-docs-section">
                    <h2>Invoke Function</h2>
                    <div class="api-docs-code-block">
                        <pre><code>${language === 'javascript' ? this.escapeHtml(jsCode) : this.escapeHtml(bashCode)}</code></pre>
                    </div>
                </div>

                <div class="api-docs-section">
                    <h2>Return Type</h2>
                    <p><code>${this.escapeHtml(return_type)}</code>${returns_set ? ' (returns multiple rows)' : ''}</p>
                </div>

                ${args && args.length > 0 ? `
                <div class="api-docs-section">
                    <h2>Arguments</h2>
                    ${args.map(arg => `
                        <div class="api-docs-arg">
                            <div class="api-docs-arg-header">
                                <span class="api-docs-arg-name">${this.escapeHtml(arg.name)}</span>
                                ${arg.required ? '<span class="api-docs-badge required">REQUIRED</span>' : '<span class="api-docs-badge optional">OPTIONAL</span>'}
                            </div>
                            <div class="api-docs-column-meta">
                                <span class="api-docs-badge type">TYPE: ${this.escapeHtml(arg.type)}</span>
                                <span class="api-docs-badge format">FORMAT: ${this.escapeHtml(arg.format)}</span>
                            </div>
                        </div>
                    `).join('')}
                </div>
                ` : '<div class="api-docs-section"><h2>Arguments</h2><p>This function takes no arguments.</p></div>'}
            </div>
        `;
    }
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => App.init());
