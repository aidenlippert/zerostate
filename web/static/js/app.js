// ZeroState Frontend Application
// Client-side SPA with routing, authentication, and API integration

// ======================
// Configuration
// ======================
// Use production API on Vercel, local API otherwise
const API_BASE_URL = window.location.hostname.includes('vercel.app')
    ? 'https://zerostate-api.fly.dev/api/v1'
    : window.location.origin + '/api/v1';
const AUTH_TOKEN_KEY = 'zerostate_auth_token';
const USER_DATA_KEY = 'zerostate_user_data';

// ======================
// Authentication Manager
// ======================
class AuthManager {
    constructor() {
        this.token = localStorage.getItem(AUTH_TOKEN_KEY);
        this.userData = JSON.parse(localStorage.getItem(USER_DATA_KEY) || 'null');
    }

    isAuthenticated() {
        return !!this.token;
    }

    getToken() {
        return this.token;
    }

    getUserData() {
        return this.userData;
    }

    async login(email, password) {
        try {
            const response = await fetch(`${API_BASE_URL}/users/login`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({ email, password }),
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.message || 'Login failed');
            }

            const data = await response.json();
            this.token = data.token;
            this.userData = data.user;

            localStorage.setItem(AUTH_TOKEN_KEY, this.token);
            localStorage.setItem(USER_DATA_KEY, JSON.stringify(this.userData));

            return data;
        } catch (error) {
            console.error('Login error:', error);
            throw error;
        }
    }

    async register(fullName, email, password) {
        try {
            const response = await fetch(`${API_BASE_URL}/users/register`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    full_name: fullName,
                    email,
                    password,
                }),
            });

            if (!response.ok) {
                const error = await response.json();
                throw new Error(error.message || 'Registration failed');
            }

            const data = await response.json();
            this.token = data.token;
            this.userData = data.user;

            localStorage.setItem(AUTH_TOKEN_KEY, this.token);
            localStorage.setItem(USER_DATA_KEY, JSON.stringify(this.userData));

            return data;
        } catch (error) {
            console.error('Registration error:', error);
            throw error;
        }
    }

    logout() {
        this.token = null;
        this.userData = null;
        localStorage.removeItem(AUTH_TOKEN_KEY);
        localStorage.removeItem(USER_DATA_KEY);
        window.location.href = '/';
    }
}

// ======================
// API Client
// ======================
class APIClient {
    constructor(authManager) {
        this.authManager = authManager;
    }

    async request(endpoint, options = {}) {
        const headers = {
            'Content-Type': 'application/json',
            ...options.headers,
        };

        if (this.authManager.isAuthenticated()) {
            headers['Authorization'] = `Bearer ${this.authManager.getToken()}`;
        }

        try {
            const response = await fetch(`${API_BASE_URL}${endpoint}`, {
                ...options,
                headers,
            });

            if (!response.ok) {
                if (response.status === 401) {
                    this.authManager.logout();
                    throw new Error('Session expired. Please login again.');
                }
                const error = await response.json();
                throw new Error(error.message || 'Request failed');
            }

            return await response.json();
        } catch (error) {
            console.error('API request error:', error);
            throw error;
        }
    }

    // Agent endpoints
    async getAgents(params = {}) {
        const queryString = new URLSearchParams(params).toString();
        return this.request(`/agents${queryString ? '?' + queryString : ''}`);
    }

    async getAgent(id) {
        return this.request(`/agents/${id}`);
    }

    async registerAgent(agentData) {
        return this.request('/agents/register', {
            method: 'POST',
            body: JSON.stringify(agentData),
        });
    }

    async updateAgent(id, agentData) {
        return this.request(`/agents/${id}`, {
            method: 'PUT',
            body: JSON.stringify(agentData),
        });
    }

    async deleteAgent(id) {
        return this.request(`/agents/${id}`, {
            method: 'DELETE',
        });
    }

    async searchAgents(query) {
        return this.request(`/agents/search?q=${encodeURIComponent(query)}`);
    }

    // Task endpoints
    async getTasks(params = {}) {
        const queryString = new URLSearchParams(params).toString();
        return this.request(`/tasks${queryString ? '?' + queryString : ''}`);
    }

    async getTask(id) {
        return this.request(`/tasks/${id}`);
    }

    async submitTask(taskData) {
        return this.request('/tasks/submit', {
            method: 'POST',
            body: JSON.stringify(taskData),
        });
    }

    async cancelTask(id) {
        return this.request(`/tasks/${id}`, {
            method: 'DELETE',
        });
    }

    async getTaskStatus(id) {
        return this.request(`/tasks/${id}/status`);
    }

    async getTaskResult(id) {
        return this.request(`/tasks/${id}/result`);
    }

    // Orchestrator endpoints
    async getOrchestratorMetrics() {
        return this.request('/orchestrator/metrics');
    }

    async getOrchestratorHealth() {
        return this.request('/orchestrator/health');
    }
}

// ======================
// Router
// ======================
class Router {
    constructor(authManager) {
        this.authManager = authManager;
        this.routes = {
            '/': { page: '/', auth: false },
            '/login': { page: '/login.html', auth: false },
            '/signup': { page: '/signup.html', auth: false },
            '/dashboard': { page: '/dashboard.html', auth: true },
            '/agents': { page: '/agents.html', auth: true },
            '/tasks': { page: '/tasks.html', auth: true },
            '/submit-task': { page: '/submit-task.html', auth: true },
            '/analytics': { page: '/analytics.html', auth: true },
            '/api-keys': { page: '/api-keys.html', auth: true },
            '/settings': { page: '/settings.html', auth: true },
        };

        this.init();
    }

    init() {
        // Handle initial page load
        window.addEventListener('DOMContentLoaded', () => {
            this.handleRoute();
        });

        // Handle back/forward buttons
        window.addEventListener('popstate', () => {
            this.handleRoute();
        });

        // Intercept all link clicks
        document.addEventListener('click', (e) => {
            const link = e.target.closest('a');
            if (link && link.href && link.href.startsWith(window.location.origin)) {
                e.preventDefault();
                const path = link.pathname;
                this.navigate(path);
            }
        });
    }

    handleRoute() {
        const path = window.location.pathname;
        const route = this.routes[path];

        if (!route) {
            // Route not found, redirect to home
            this.navigate('/');
            return;
        }

        // Check authentication
        if (route.auth && !this.authManager.isAuthenticated()) {
            // Redirect to login if authentication required
            this.navigate('/login');
            return;
        }

        // If authenticated user tries to access login/signup, redirect to dashboard
        if (!route.auth && this.authManager.isAuthenticated() && (path === '/login' || path === '/signup')) {
            this.navigate('/dashboard');
            return;
        }
    }

    navigate(path) {
        window.history.pushState({}, '', path);
        window.location.href = path;
    }
}

// ======================
// UI Helpers
// ======================
class UIHelpers {
    static showNotification(message, type = 'info') {
        // Create notification element
        const notification = document.createElement('div');
        notification.className = `fixed top-4 right-4 z-50 p-4 rounded-lg shadow-lg max-w-md animate-slide-in ${
            type === 'success' ? 'bg-green-500' :
            type === 'error' ? 'bg-red-500' :
            type === 'warning' ? 'bg-yellow-500' :
            'bg-blue-500'
        } text-white`;
        notification.innerHTML = `
            <div class="flex items-center gap-3">
                <span class="material-symbols-outlined">
                    ${type === 'success' ? 'check_circle' :
                      type === 'error' ? 'error' :
                      type === 'warning' ? 'warning' :
                      'info'}
                </span>
                <p class="flex-1">${message}</p>
                <button onclick="this.parentElement.parentElement.remove()" class="hover:opacity-80">
                    <span class="material-symbols-outlined">close</span>
                </button>
            </div>
        `;

        document.body.appendChild(notification);

        // Auto-remove after 5 seconds
        setTimeout(() => {
            notification.remove();
        }, 5000);
    }

    static showLoading(element, show = true) {
        if (show) {
            element.disabled = true;
            element.innerHTML = `
                <span class="inline-block animate-spin mr-2">⏳</span>
                Loading...
            `;
        } else {
            element.disabled = false;
        }
    }

    static formatDate(dateString) {
        const date = new Date(dateString);
        const now = new Date();
        const diffMs = now - date;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);

        if (diffMins < 1) return 'Just now';
        if (diffMins < 60) return `${diffMins}m ago`;
        if (diffHours < 24) return `${diffHours}h ago`;
        if (diffDays < 7) return `${diffDays}d ago`;

        return date.toLocaleDateString();
    }

    static formatNumber(num) {
        if (num >= 1000000) {
            return (num / 1000000).toFixed(1) + 'M';
        }
        if (num >= 1000) {
            return (num / 1000).toFixed(1) + 'K';
        }
        return num.toString();
    }
}

// ======================
// Initialize Application
// ======================
const authManager = new AuthManager();
const apiClient = new APIClient(authManager);
const router = new Router(authManager);

// Make available globally for inline event handlers
window.app = {
    authManager,
    apiClient,
    router,
    UIHelpers,
};

// ======================
// Login Page Handler
// ======================
if (window.location.pathname === '/login.html') {
    document.addEventListener('DOMContentLoaded', () => {
        const loginForm = document.querySelector('form');
        if (loginForm) {
            loginForm.addEventListener('submit', async (e) => {
                e.preventDefault();

                const email = document.getElementById('email').value;
                const password = document.getElementById('password').value;
                const submitBtn = loginForm.querySelector('button[type="submit"]');

                try {
                    UIHelpers.showLoading(submitBtn);
                    await authManager.login(email, password);
                    UIHelpers.showNotification('Login successful!', 'success');
                    setTimeout(() => {
                        router.navigate('/dashboard');
                    }, 500);
                } catch (error) {
                    UIHelpers.showNotification(error.message, 'error');
                    submitBtn.disabled = false;
                    submitBtn.textContent = 'Sign In';
                }
            });
        }

        // Password visibility toggle
        const toggleButtons = document.querySelectorAll('[onclick*="togglePassword"]');
        toggleButtons.forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.preventDefault();
                const input = btn.closest('.relative').querySelector('input');
                const icon = btn.querySelector('.material-symbols-outlined');

                if (input.type === 'password') {
                    input.type = 'text';
                    icon.textContent = 'visibility_off';
                } else {
                    input.type = 'password';
                    icon.textContent = 'visibility';
                }
            });
        });
    });
}

// ======================
// Signup Page Handler
// ======================
if (window.location.pathname === '/signup.html') {
    document.addEventListener('DOMContentLoaded', () => {
        const signupForm = document.getElementById('signup-form');
        if (signupForm) {
            const passwordInput = document.getElementById('password');
            const confirmPasswordInput = document.getElementById('confirm-password');
            const passwordMatchError = document.getElementById('password-match-error');

            // Password strength checker
            passwordInput.addEventListener('input', function() {
                const password = this.value;
                let strength = 0;
                const strengthBars = [
                    document.getElementById('strength-bar-1'),
                    document.getElementById('strength-bar-2'),
                    document.getElementById('strength-bar-3'),
                    document.getElementById('strength-bar-4')
                ];
                const strengthText = document.getElementById('strength-text');

                if (password.length >= 8) strength++;
                if (password.match(/[a-z]/) && password.match(/[A-Z]/)) strength++;
                if (password.match(/[0-9]/)) strength++;
                if (password.match(/[^a-zA-Z0-9]/)) strength++;

                // Reset all bars
                strengthBars.forEach(bar => {
                    bar.className = 'password-strength-bar flex-1 rounded bg-gray-700';
                });

                // Update bars based on strength
                const colors = ['bg-red-500', 'bg-orange-500', 'bg-yellow-500', 'bg-green-500'];
                const texts = ['Weak password', 'Fair password', 'Good password', 'Strong password'];

                for (let i = 0; i < strength; i++) {
                    strengthBars[i].classList.remove('bg-gray-700');
                    strengthBars[i].classList.add(colors[strength - 1]);
                }

                strengthText.textContent = password.length > 0 ? texts[strength - 1] || 'Very weak password' : 'Enter a password';
                const textColors = ['text-red-400', 'text-orange-400', 'text-yellow-400', 'text-green-400'];
                strengthText.className = `text-xs ${strength > 0 ? textColors[strength - 1] : 'text-gray-500'}`;
            });

            // Password match validation
            confirmPasswordInput.addEventListener('input', function() {
                if (this.value !== passwordInput.value && this.value.length > 0) {
                    passwordMatchError.classList.remove('hidden');
                    this.classList.add('border-red-500');
                } else {
                    passwordMatchError.classList.add('hidden');
                    this.classList.remove('border-red-500');
                }
            });

            // Form submission
            signupForm.addEventListener('submit', async (e) => {
                e.preventDefault();

                const fullName = document.getElementById('full-name').value;
                const email = document.getElementById('email').value;
                const password = passwordInput.value;
                const confirmPassword = confirmPasswordInput.value;
                const termsAccepted = document.getElementById('terms').checked;
                const submitBtn = signupForm.querySelector('button[type="submit"]');

                if (password !== confirmPassword) {
                    UIHelpers.showNotification('Passwords do not match', 'error');
                    return;
                }

                if (!termsAccepted) {
                    UIHelpers.showNotification('Please accept the Terms & Conditions', 'error');
                    return;
                }

                try {
                    UIHelpers.showLoading(submitBtn);
                    await authManager.register(fullName, email, password);
                    UIHelpers.showNotification('Account created successfully!', 'success');
                    setTimeout(() => {
                        router.navigate('/dashboard');
                    }, 500);
                } catch (error) {
                    UIHelpers.showNotification(error.message, 'error');
                    submitBtn.disabled = false;
                    submitBtn.textContent = 'Create Account';
                }
            });

            // Password visibility toggles
            window.togglePassword = function(inputId) {
                const input = document.getElementById(inputId);
                const icon = document.getElementById(inputId + '-toggle-icon');

                if (input.type === 'password') {
                    input.type = 'text';
                    icon.textContent = 'visibility_off';
                } else {
                    input.type = 'password';
                    icon.textContent = 'visibility';
                }
            };
        }
    });
}

// ======================
// Dashboard Page Handler
// ======================
if (window.location.pathname === '/dashboard.html') {
    document.addEventListener('DOMContentLoaded', async () => {
        try {
            // Load orchestrator metrics
            const metrics = await apiClient.getOrchestratorMetrics();

            // Update metric cards
            if (metrics) {
                document.querySelector('[data-metric="active-agents"]').textContent =
                    UIHelpers.formatNumber(metrics.active_agents || 0);
                document.querySelector('[data-metric="tasks-completed"]').textContent =
                    UIHelpers.formatNumber(metrics.tasks_completed || 0);
                document.querySelector('[data-metric="network-calls"]').textContent =
                    UIHelpers.formatNumber(metrics.network_calls || 0);
                document.querySelector('[data-metric="errors"]').textContent =
                    UIHelpers.formatNumber(metrics.errors || 0);
            }

            // Load recent tasks
            const tasks = await apiClient.getTasks({ limit: 5, sort: 'recent' });
            if (tasks && tasks.tasks) {
                renderRecentTasks(tasks.tasks);
            }
        } catch (error) {
            console.error('Error loading dashboard data:', error);
            UIHelpers.showNotification('Failed to load dashboard data', 'error');
        }

        // Set current date/time
        const dateElement = document.querySelector('[data-current-date]');
        if (dateElement) {
            const now = new Date();
            dateElement.textContent = now.toLocaleDateString('en-US', {
                weekday: 'long',
                year: 'numeric',
                month: 'long',
                day: 'numeric'
            });
        }

        function renderRecentTasks(tasks) {
            const container = document.querySelector('[data-recent-tasks]');
            if (!container) return;

            container.innerHTML = tasks.map(task => `
                <div class="flex items-center gap-4 p-3 bg-white/5 rounded-lg hover:bg-white/10 transition-colors cursor-pointer">
                    <div class="flex-1">
                        <p class="font-medium">${task.query || 'Task'}</p>
                        <p class="text-sm text-gray-400">${UIHelpers.formatDate(task.created_at)}</p>
                    </div>
                    <span class="px-3 py-1 rounded-full text-xs font-medium ${
                        task.status === 'completed' ? 'bg-green-500/20 text-green-400' :
                        task.status === 'running' ? 'bg-blue-500/20 text-blue-400' :
                        task.status === 'failed' ? 'bg-red-500/20 text-red-400' :
                        'bg-gray-500/20 text-gray-400'
                    }">${task.status}</span>
                </div>
            `).join('');
        }
    });
}

// ======================
// Agents Page Handler
// ======================
if (window.location.pathname === '/agents.html') {
    document.addEventListener('DOMContentLoaded', async () => {
        let currentPage = 1;
        const pageSize = 6;

        async function loadAgents(search = '', status = '', sort = 'recent') {
            try {
                const agents = await apiClient.getAgents({
                    page: currentPage,
                    limit: pageSize,
                    search,
                    status,
                    sort
                });

                if (agents && agents.agents) {
                    renderAgents(agents.agents);
                    updatePagination(agents.total, agents.page, agents.total_pages);
                }
            } catch (error) {
                console.error('Error loading agents:', error);
                UIHelpers.showNotification('Failed to load agents', 'error');
            }
        }

        function renderAgents(agents) {
            // Agent rendering logic will be added based on actual API response structure
            console.log('Agents loaded:', agents);
        }

        function updatePagination(total, page, totalPages) {
            // Pagination update logic
            console.log(`Page ${page} of ${totalPages}, Total: ${total}`);
        }

        // Initial load
        await loadAgents();

        // Search functionality
        const searchInput = document.querySelector('input[placeholder*="Search"]');
        if (searchInput) {
            let searchTimeout;
            searchInput.addEventListener('input', (e) => {
                clearTimeout(searchTimeout);
                searchTimeout = setTimeout(() => {
                    loadAgents(e.target.value);
                }, 300);
            });
        }
    });
}

// ======================
// Global Logout Handler
// ======================
window.logout = function() {
    authManager.logout();
};

console.log('ZeroState app initialized');
