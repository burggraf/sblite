// sblite Dashboard Application

const App = {
    state: {
        authenticated: false,
        needsSetup: true,
        theme: 'dark',
        currentView: 'tables',
        loading: true,
        error: null
    },

    async init() {
        this.loadTheme();
        await this.checkAuth();
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
                return '<div class="card"><h2 class="card-title">Tables</h2><p>Table management coming in Phase 3</p></div>';
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
    }
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => App.init());
