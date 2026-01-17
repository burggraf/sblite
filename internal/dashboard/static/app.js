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
        return `
            <div class="table-toolbar">
                <h2>${this.state.tables.selected}</h2>
                <div class="toolbar-actions">
                    <button class="btn btn-secondary btn-sm" onclick="App.showAddRowModal()">+ Add Row</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.showSchemaModal()">Schema</button>
                    <button class="btn btn-secondary btn-sm" onclick="App.confirmDeleteTable()">Delete Table</button>
                </div>
            </div>
            ${this.renderDataGrid()}
            ${this.renderPagination()}
        `;
    },

    // Placeholder methods for data grid and pagination (to be implemented in Task 10)
    renderDataGrid() {
        return '<div class="empty-state">Data grid coming in next task</div>';
    },

    renderPagination() {
        return '';
    },

    // Placeholder methods for modals (to be implemented in later tasks)
    showCreateTableModal() {
        alert('Create table modal coming soon');
    },

    showAddRowModal() {
        alert('Add row modal coming soon');
    },

    showSchemaModal() {
        alert('Schema modal coming soon');
    },

    confirmDeleteTable() {
        alert('Delete table confirmation coming soon');
    }
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => App.init());
