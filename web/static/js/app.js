// ZeroState Platform - Frontend Application
// API Configuration
const API_BASE_URL = window.location.origin + '/api/v1';

// State Management
const state = {
    tasks: [],
    agents: [],
    metrics: null,
    currentPage: 'dashboard'
};

// API Client
const api = {
    // Task endpoints
    async submitTask(data) {
        const response = await fetch(`${API_BASE_URL}/tasks/submit`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    async getTasks(filters = {}) {
        const params = new URLSearchParams(filters);
        const response = await fetch(`${API_BASE_URL}/tasks?${params}`);
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    async getTask(taskId) {
        const response = await fetch(`${API_BASE_URL}/tasks/${taskId}`);
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    async getTaskStatus(taskId) {
        const response = await fetch(`${API_BASE_URL}/tasks/${taskId}/status`);
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    async getTaskResult(taskId) {
        const response = await fetch(`${API_BASE_URL}/tasks/${taskId}/result`);
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    async cancelTask(taskId) {
        const response = await fetch(`${API_BASE_URL}/tasks/${taskId}`, {
            method: 'DELETE'
        });
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    // Agent endpoints
    async getAgents(filters = {}) {
        const params = new URLSearchParams(filters);
        const response = await fetch(`${API_BASE_URL}/agents?${params}`);
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    async registerAgent(data) {
        const response = await fetch(`${API_BASE_URL}/agents/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    // Orchestrator endpoints
    async getOrchestratorMetrics() {
        const response = await fetch(`${API_BASE_URL}/orchestrator/metrics`);
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    },

    async getOrchestratorHealth() {
        const response = await fetch(`${API_BASE_URL}/orchestrator/health`);
        if (!response.ok) throw new Error(await response.text());
        return response.json();
    }
};

// UI Components
const components = {
    // Dashboard Page
    dashboard: async () => {
        try {
            const [tasksData, metricsData, healthData] = await Promise.all([
                api.getTasks({ limit: 5 }),
                api.getOrchestratorMetrics(),
                api.getOrchestratorHealth()
            ]);

            return `
                <div class="flex flex-col gap-8">
                    <!-- Stats -->
                    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a] hover:border-primary transition-colors duration-300">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Tasks Processed</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.tasks_processed || 0}</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a] hover:border-primary transition-colors duration-300">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Active Workers</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.active_workers || 0}</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a] hover:border-primary transition-colors duration-300">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Success Rate</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.success_rate ? metricsData.success_rate.toFixed(2) : 0}%</p>
                        </div>
                    </div>

                    <!-- Orchestrator Health -->
                    <div class="rounded-xl bg-[#1A1C2A] border border-[#25214a] p-4">
                        <div class="grid grid-cols-1 sm:grid-cols-3 divide-y sm:divide-y-0 sm:divide-x divide-[#25214a]">
                            <div class="flex justify-between sm:flex-col sm:justify-start gap-x-6 gap-y-1 py-2 sm:px-4">
                                <p class="text-[#A0A0B0] text-sm font-normal leading-normal">Orchestrator Health</p>
                                <p class="text-white text-sm font-normal leading-normal text-right sm:text-left flex items-center gap-2">
                                    <span class="text-[#00FF85]">●</span> ${healthData.status || 'Unknown'}
                                </p>
                            </div>
                            <div class="flex justify-between sm:flex-col sm:justify-start gap-x-6 gap-y-1 py-2 sm:px-4">
                                <p class="text-[#A0A0B0] text-sm font-normal leading-normal">Active Workers</p>
                                <p class="text-white text-sm font-normal leading-normal text-right sm:text-left">${healthData.active_workers || 0}</p>
                            </div>
                            <div class="flex justify-between sm:flex-col sm:justify-start gap-x-6 gap-y-1 py-2 sm:px-4">
                                <p class="text-[#A0A0B0] text-sm font-normal leading-normal">Avg Execution Time</p>
                                <p class="text-white text-sm font-normal leading-normal text-right sm:text-left">${metricsData.avg_execution_ms || 0}ms</p>
                            </div>
                        </div>
                    </div>

                    <!-- Recent Tasks -->
                    <div class="flex flex-col rounded-xl bg-[#1A1C2A] border border-[#25214a]">
                        <div class="flex items-center justify-between px-6 pb-3 pt-5 border-b border-[#25214a]">
                            <h2 class="text-white text-[22px] font-bold leading-tight tracking-[-0.015em]">Recent Tasks</h2>
                            <a href="/tasks" class="text-primary text-sm font-medium hover:underline">View All</a>
                        </div>
                        <div class="flex flex-col">
                            ${tasksData.tasks && tasksData.tasks.length > 0 ? tasksData.tasks.map(task => `
                                <div class="flex items-center gap-4 px-6 min-h-[72px] py-2 justify-between border-b border-[#25214a] last:border-b-0 hover:bg-white/5 transition-colors cursor-pointer" onclick="router.navigate('/task/${task.id}')">
                                    <div class="flex items-center gap-4">
                                        <div class="${getStatusColor(task.status)} flex items-center justify-center rounded-lg bg-opacity-20 shrink-0 size-10">
                                            <span class="material-symbols-outlined">${getStatusIcon(task.status)}</span>
                                        </div>
                                        <div class="flex flex-col justify-center">
                                            <p class="text-white text-base font-medium leading-normal line-clamp-1">${task.type || 'Task'}</p>
                                            <p class="text-[#A0A0B0] text-sm font-normal leading-normal line-clamp-2">${formatDate(task.created_at)}</p>
                                        </div>
                                    </div>
                                    <div class="shrink-0">
                                        <span class="px-3 py-1 rounded-full text-xs font-medium ${getStatusBadgeClass(task.status)}">${task.status}</span>
                                    </div>
                                </div>
                            `).join('') : `
                                <div class="flex items-center justify-center py-12">
                                    <p class="text-[#A0A0B0] text-sm">No tasks yet. <a href="/submit-task" class="text-primary hover:underline">Submit your first task!</a></p>
                                </div>
                            `}
                        </div>
                    </div>
                </div>
            `;
        } catch (error) {
            console.error('Dashboard error:', error);
            return `<div class="text-red-500">Error loading dashboard: ${error.message}</div>`;
        }
    },

    // Task Submission Form
    submitTask: () => {
        return `
            <div class="flex flex-col gap-8">
                <div class="flex flex-wrap justify-between gap-3">
                    <div class="flex min-w-72 flex-col gap-3">
                        <p class="text-white text-4xl font-black leading-tight tracking-[-0.033em]">Submit New Task</p>
                        <p class="text-[#958ecc] text-base font-normal leading-normal">Configure and dispatch a new task to the AI orchestrator.</p>
                    </div>
                </div>

                <form id="task-form" class="flex flex-col gap-8">
                    <!-- Query Input -->
                    <div class="flex max-w-full flex-wrap items-end gap-4">
                        <label class="flex flex-col min-w-40 flex-1">
                            <p class="text-white text-base font-medium leading-normal pb-2">Query</p>
                            <input
                                id="query"
                                name="query"
                                required
                                class="form-input flex w-full min-w-0 flex-1 resize-none overflow-hidden rounded-xl text-white focus:outline-0 border border-[#352f6a] bg-[#1b1835] focus:border-primary focus:ring-2 focus:ring-primary/50 h-14 placeholder:text-[#958ecc] p-[15px] text-base font-normal leading-normal"
                                placeholder="Enter your task query or instructions here"
                            />
                        </label>
                    </div>

                    <!-- Priority Selector -->
                    <div>
                        <h2 class="text-white text-lg font-bold leading-tight tracking-[-0.015em] pb-2">Priority</h2>
                        <div class="flex h-10 w-full items-center justify-center rounded-xl bg-[#25214a] p-1">
                            <label class="flex cursor-pointer h-full grow items-center justify-center overflow-hidden rounded-lg px-2 has-[:checked]:bg-[#121023] text-[#958ecc] has-[:checked]:text-white text-sm font-medium leading-normal transition-colors">
                                <span class="truncate">Low</span>
                                <input class="invisible w-0" name="priority" type="radio" value="low"/>
                            </label>
                            <label class="flex cursor-pointer h-full grow items-center justify-center overflow-hidden rounded-lg px-2 has-[:checked]:bg-[#121023] text-[#958ecc] has-[:checked]:text-white text-sm font-medium leading-normal transition-colors">
                                <span class="truncate">Normal</span>
                                <input checked class="invisible w-0" name="priority" type="radio" value="normal"/>
                            </label>
                            <label class="flex cursor-pointer h-full grow items-center justify-center overflow-hidden rounded-lg px-2 has-[:checked]:bg-[#121023] text-[#958ecc] has-[:checked]:text-white text-sm font-medium leading-normal transition-colors">
                                <span class="truncate">High</span>
                                <input class="invisible w-0" name="priority" type="radio" value="high"/>
                            </label>
                            <label class="flex cursor-pointer h-full grow items-center justify-center overflow-hidden rounded-lg px-2 has-[:checked]:bg-[#121023] text-[#958ecc] has-[:checked]:text-white text-sm font-medium leading-normal transition-colors">
                                <span class="truncate">Critical</span>
                                <input class="invisible w-0" name="priority" type="radio" value="critical"/>
                            </label>
                        </div>
                    </div>

                    <!-- Budget and Timeout -->
                    <div class="flex flex-col md:flex-row gap-4">
                        <label class="flex flex-col min-w-40 flex-1">
                            <p class="text-white text-base font-medium leading-normal pb-2">Budget</p>
                            <div class="relative">
                                <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-[#958ecc]">paid</span>
                                <input
                                    id="budget"
                                    name="budget"
                                    type="number"
                                    step="0.01"
                                    min="0.01"
                                    required
                                    class="form-input pl-10 flex w-full min-w-0 flex-1 resize-none overflow-hidden rounded-xl text-white focus:outline-0 border border-[#352f6a] bg-[#1b1835] focus:border-primary focus:ring-2 focus:ring-primary/50 h-14 placeholder:text-[#958ecc] p-[15px] text-base font-normal leading-normal"
                                    placeholder="e.g., 1.00"
                                />
                            </div>
                        </label>
                        <label class="flex flex-col min-w-40 flex-1">
                            <p class="text-white text-base font-medium leading-normal pb-2">Timeout (seconds)</p>
                            <div class="relative">
                                <span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-[#958ecc]">timer</span>
                                <input
                                    id="timeout"
                                    name="timeout"
                                    type="number"
                                    min="1"
                                    max="300"
                                    class="form-input pl-10 flex w-full min-w-0 flex-1 resize-none overflow-hidden rounded-xl text-white focus:outline-0 border border-[#352f6a] bg-[#1b1835] focus:border-primary focus:ring-2 focus:ring-primary/50 h-14 placeholder:text-[#958ecc] p-[15px] text-base font-normal leading-normal"
                                    placeholder="e.g., 60 (default: 30)"
                                />
                            </div>
                        </label>
                    </div>

                    <!-- Submit Buttons -->
                    <div class="flex flex-col sm:flex-row items-center justify-end gap-4 mt-6 border-t border-[#25214a] pt-6">
                        <button
                            type="button"
                            onclick="router.navigate('/')"
                            class="w-full sm:w-auto flex min-w-[84px] max-w-[480px] cursor-pointer items-center justify-center overflow-hidden rounded-xl h-12 px-6 bg-transparent text-white text-base font-bold leading-normal tracking-[0.015em] hover:bg-white/10 transition-colors"
                        >
                            <span class="truncate">Cancel</span>
                        </button>
                        <button
                            type="submit"
                            class="w-full sm:w-auto flex min-w-[84px] max-w-[480px] cursor-pointer items-center justify-center overflow-hidden rounded-xl h-12 px-6 bg-gradient-to-r from-[#00BFFF] to-[#FF00FF] text-white text-base font-bold leading-normal tracking-[0.015em] hover:opacity-90 transition-opacity"
                        >
                            <span class="truncate">Submit Task</span>
                        </button>
                    </div>
                </form>
            </div>
        `;
    },

    // Task List Page
    tasks: async () => {
        try {
            const tasksData = await api.getTasks({ limit: 50 });

            return `
                <div class="flex flex-col gap-8">
                    <!-- Page Heading -->
                    <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                        <div class="flex flex-col gap-2">
                            <p class="text-white text-4xl font-black leading-tight tracking-[-0.033em]">Task Management</p>
                            <p class="text-white/60 text-base font-normal leading-normal">Monitor and manage all tasks across the platform.</p>
                        </div>
                        <button
                            onclick="router.navigate('/submit-task')"
                            class="flex min-w-[84px] cursor-pointer items-center justify-center overflow-hidden rounded-lg h-10 px-4 bg-gradient-to-r from-[#00BFFF] to-[#FF00FF] text-white text-sm font-bold leading-normal tracking-[0.015em] hover:opacity-90 transition-opacity"
                        >
                            <span class="material-symbols-outlined mr-2 text-base">add</span>
                            <span class="truncate">New Task</span>
                        </button>
                    </div>

                    <!-- Table -->
                    <div class="w-full overflow-hidden rounded-xl border border-white/10 bg-black/20">
                        <div class="overflow-x-auto">
                            <table class="min-w-full font-display">
                                <thead>
                                    <tr class="bg-white/5">
                                        <th class="px-6 py-4 text-left text-xs font-medium text-white/50 uppercase tracking-wider">ID</th>
                                        <th class="px-6 py-4 text-left text-xs font-medium text-white/50 uppercase tracking-wider">Type</th>
                                        <th class="px-6 py-4 text-left text-xs font-medium text-white/50 uppercase tracking-wider">Status</th>
                                        <th class="px-6 py-4 text-left text-xs font-medium text-white/50 uppercase tracking-wider">Priority</th>
                                        <th class="px-6 py-4 text-left text-xs font-medium text-white/50 uppercase tracking-wider">Created</th>
                                    </tr>
                                </thead>
                                <tbody class="divide-y divide-white/10">
                                    ${tasksData.tasks && tasksData.tasks.length > 0 ? tasksData.tasks.map(task => `
                                        <tr class="hover:bg-white/[.02] transition-colors cursor-pointer" onclick="showTaskDetails('${task.id}')">
                                            <td class="px-6 py-4 whitespace-nowrap text-sm font-mono text-white/70">${task.id.substring(0, 12)}...</td>
                                            <td class="px-6 py-4 whitespace-nowrap text-sm text-white">${task.type || 'General'}</td>
                                            <td class="px-6 py-4 whitespace-nowrap text-sm">
                                                <span class="px-3 py-1 rounded-full text-xs font-medium ${getStatusBadgeClass(task.status)}">${task.status}</span>
                                            </td>
                                            <td class="px-6 py-4 whitespace-nowrap text-sm text-white">${task.priority || 'normal'}</td>
                                            <td class="px-6 py-4 whitespace-nowrap text-sm text-white/70">${formatDate(task.created_at)}</td>
                                        </tr>
                                    `).join('') : `
                                        <tr>
                                            <td colspan="5" class="px-6 py-12 text-center text-[#A0A0B0]">
                                                No tasks found. <a href="/submit-task" class="text-primary hover:underline">Submit a task</a>
                                            </td>
                                        </tr>
                                    `}
                                </tbody>
                            </table>
                        </div>
                    </div>

                    <!-- Pagination Info -->
                    <div class="flex items-center justify-center mt-6">
                        <span class="text-sm text-white/60">Showing ${tasksData.count || 0} tasks</span>
                    </div>
                </div>
            `;
        } catch (error) {
            console.error('Tasks page error:', error);
            return `<div class="text-red-500">Error loading tasks: ${error.message}</div>`;
        }
    },

    // Metrics Dashboard
    metrics: async () => {
        try {
            const metricsData = await api.getOrchestratorMetrics();

            return `
                <div class="flex flex-col gap-8">
                    <div class="flex flex-col gap-2">
                        <p class="text-white text-4xl font-black leading-tight tracking-[-0.033em]">Performance Metrics</p>
                        <p class="text-white/60 text-base font-normal leading-normal">Real-time orchestrator performance metrics and statistics.</p>
                    </div>

                    <!-- Metrics Grid -->
                    <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Tasks Processed</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.tasks_processed || 0}</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Tasks Succeeded</p>
                            <p class="text-[#00FF85] tracking-light text-3xl font-bold leading-tight">${metricsData.tasks_succeeded || 0}</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Tasks Failed</p>
                            <p class="text-[#FF0055] tracking-light text-3xl font-bold leading-tight">${metricsData.tasks_failed || 0}</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Success Rate</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.success_rate ? metricsData.success_rate.toFixed(2) : 0}%</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Avg Execution Time</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.avg_execution_ms || 0}ms</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Active Workers</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.active_workers || 0}</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Tasks Timed Out</p>
                            <p class="text-[#FFD700] tracking-light text-3xl font-bold leading-tight">${metricsData.tasks_timed_out || 0}</p>
                        </div>
                        <div class="flex flex-col gap-2 rounded-xl p-6 bg-[#1A1C2A] border border-[#25214a]">
                            <p class="text-[#A0A0B0] text-base font-medium leading-normal">Total Tasks</p>
                            <p class="text-white tracking-light text-3xl font-bold leading-tight">${metricsData.tasks_processed || 0}</p>
                        </div>
                    </div>
                </div>
            `;
        } catch (error) {
            console.error('Metrics page error:', error);
            return `<div class="text-red-500">Error loading metrics: ${error.message}</div>`;
        }
    }
};

// Utility Functions
function getStatusColor(status) {
    const colors = {
        'completed': 'text-[#00FF85]',
        'running': 'text-[#2472F2]',
        'pending': 'text-[#FFD700]',
        'queued': 'text-[#FFD700]',
        'failed': 'text-[#FF4444]',
        'canceled': 'text-[#958ecc]'
    };
    return colors[status] || 'text-gray-400';
}

function getStatusIcon(status) {
    const icons = {
        'completed': 'check_circle',
        'running': 'sync',
        'pending': 'schedule',
        'queued': 'schedule',
        'failed': 'error',
        'canceled': 'cancel'
    };
    return icons[status] || 'help';
}

function getStatusBadgeClass(status) {
    const classes = {
        'completed': 'bg-[#00FF85]/20 text-[#00FF85]',
        'running': 'bg-[#2472F2]/20 text-[#2472F2]',
        'pending': 'bg-[#FFD700]/20 text-[#FFD700]',
        'queued': 'bg-[#FFD700]/20 text-[#FFD700]',
        'failed': 'bg-[#FF4444]/20 text-[#FF4444]',
        'canceled': 'bg-[#958ecc]/20 text-[#958ecc]'
    };
    return classes[status] || 'bg-gray-400/20 text-gray-400';
}

function formatDate(dateString) {
    if (!dateString) return 'N/A';
    const date = new Date(dateString);
    const now = new Date();
    const diff = now - date;
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (seconds < 60) return `${seconds}s ago`;
    if (minutes < 60) return `${minutes}m ago`;
    if (hours < 24) return `${hours}h ago`;
    if (days < 7) return `${days}d ago`;
    return date.toLocaleDateString();
}

// Task Details Modal
async function showTaskDetails(taskId) {
    try {
        const task = await api.getTask(taskId);
        const status = await api.getTaskStatus(taskId);

        let resultData = null;
        if (task.status === 'completed' || task.status === 'failed') {
            try {
                resultData = await api.getTaskResult(taskId);
            } catch (e) {
                console.warn('Could not fetch task result:', e);
            }
        }

        const modal = document.createElement('div');
        modal.className = 'fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm';
        modal.onclick = (e) => {
            if (e.target === modal) modal.remove();
        };

        modal.innerHTML = `
            <div class="flex w-full max-w-4xl flex-col rounded-xl border border-white/10 bg-[#121023] shadow-2xl shadow-primary/20 max-h-[90vh] overflow-y-auto">
                <!-- Modal Header -->
                <header class="flex items-center justify-between gap-4 border-b border-white/10 p-4 sm:p-6 sticky top-0 bg-[#121023] z-10">
                    <p class="text-xl font-bold text-white sm:text-2xl">Task Details: ${task.id.substring(0, 8)}...</p>
                    <button onclick="this.closest('.fixed').remove()" class="flex h-8 w-8 cursor-pointer items-center justify-center rounded-full bg-white/5 text-[#958ecc] transition-colors hover:bg-white/10 hover:text-white">
                        <span class="material-symbols-outlined text-xl">close</span>
                    </button>
                </header>

                <!-- Modal Body -->
                <div class="flex flex-col gap-6 p-4 sm:p-6">
                    <!-- Progress -->
                    <div class="flex flex-col gap-4">
                        <div class="flex flex-col gap-3">
                            <div class="flex items-center justify-between gap-6">
                                <p class="text-base font-medium text-white">Status</p>
                                <p class="text-sm font-normal text-white">${status.progress}%</p>
                            </div>
                            <div class="h-2 w-full rounded-full bg-[#352f6a]">
                                <div class="h-2 rounded-full bg-gradient-to-r from-[#8A2BE2] to-[#00BFFF]" style="width: ${status.progress}%;"></div>
                            </div>
                            <p class="text-sm font-normal">
                                <span class="px-3 py-1 rounded-full text-xs font-medium ${getStatusBadgeClass(task.status)}">${task.status}</span>
                            </p>
                        </div>

                        <!-- Task Details Grid -->
                        <div class="grid grid-cols-2 gap-x-4 gap-y-1 border-t border-solid border-t-[#352f6a] pt-4 md:grid-cols-4">
                            <div class="flex flex-col gap-1 py-2">
                                <p class="text-sm font-normal text-[#958ecc]">Priority</p>
                                <p class="text-sm font-normal text-white capitalize">${task.priority || 'normal'}</p>
                            </div>
                            <div class="flex flex-col gap-1 py-2">
                                <p class="text-sm font-normal text-[#958ecc]">Budget</p>
                                <p class="text-sm font-normal text-white">$${task.budget || 0}</p>
                            </div>
                            <div class="flex flex-col gap-1 py-2">
                                <p class="text-sm font-normal text-[#958ecc]">Created</p>
                                <p class="text-sm font-normal text-white">${new Date(task.created_at).toLocaleString()}</p>
                            </div>
                            <div class="flex flex-col gap-1 py-2">
                                <p class="text-sm font-normal text-[#958ecc]">Timeout</p>
                                <p class="text-sm font-normal text-white">${task.timeout ? (parseInt(task.timeout) / 1000000000) + 's' : 'N/A'}</p>
                            </div>
                        </div>
                    </div>

                    <!-- Query/Input -->
                    <div class="rounded-lg bg-[#25214a]/50 p-4">
                        <div class="flex flex-col gap-2">
                            <p class="text-lg font-bold tracking-[-0.015em] text-white">Task Input</p>
                            <pre class="text-base font-normal leading-normal text-[#958ecc] whitespace-pre-wrap">${JSON.stringify(task.input, null, 2)}</pre>
                        </div>
                    </div>

                    ${task.assigned_to ? `
                        <!-- Agent Info -->
                        <div class="grid grid-cols-2 gap-x-4 border-t border-solid border-t-[#352f6a] pt-4">
                            <div class="flex flex-col gap-1 py-2">
                                <p class="text-sm font-normal text-[#958ecc]">Assigned Agent</p>
                                <p class="text-sm font-normal text-white font-mono">${task.assigned_to}</p>
                            </div>
                        </div>
                    ` : ''}

                    ${resultData && resultData.result ? `
                        <!-- Result -->
                        <div class="rounded-lg bg-[#25214a]/50 p-4">
                            <div class="flex flex-col gap-2">
                                <p class="text-lg font-bold tracking-[-0.015em] text-white">Result</p>
                                <div class="rounded-md bg-[#121023] p-3">
                                    <pre class="text-sm text-[#E0E0E0] font-mono overflow-x-auto">${JSON.stringify(resultData.result, null, 2)}</pre>
                                </div>
                            </div>
                        </div>
                    ` : ''}

                    ${task.error ? `
                        <!-- Error -->
                        <div class="rounded-lg bg-red-500/10 border border-red-500/20 p-4">
                            <div class="flex flex-col gap-2">
                                <p class="text-lg font-bold tracking-[-0.015em] text-red-400">Error</p>
                                <p class="text-sm text-red-300">${task.error}</p>
                            </div>
                        </div>
                    ` : ''}
                </div>

                <!-- Modal Footer -->
                <footer class="flex flex-col items-center justify-between gap-4 border-t border-white/10 p-4 sm:flex-row sm:p-6">
                    <div class="grid w-full grid-cols-2 gap-4 text-sm sm:w-auto sm:flex-row sm:gap-8">
                        ${resultData ? `
                            <div class="flex items-baseline gap-2">
                                <p class="text-[#958ecc]">Cost:</p>
                                <p class="font-medium text-white">$${resultData.cost || 0}</p>
                            </div>
                            <div class="flex items-baseline gap-2">
                                <p class="text-[#958ecc]">Duration:</p>
                                <p class="font-medium text-white">${resultData.duration || 0}ms</p>
                            </div>
                        ` : ''}
                    </div>
                    <div class="flex w-full items-center justify-end gap-3 sm:w-auto">
                        ${task.status !== 'completed' && task.status !== 'failed' && task.status !== 'canceled' ? `
                            <button onclick="cancelTask('${task.id}')" class="h-10 cursor-pointer items-center justify-center overflow-hidden rounded-lg bg-[#ff3b30]/20 px-4 text-sm font-medium text-[#ff7b75] transition-colors hover:bg-[#ff3b30]/30">
                                <span class="truncate">Cancel Task</span>
                            </button>
                        ` : ''}
                        <button onclick="this.closest('.fixed').remove()" class="flex h-10 min-w-[84px] max-w-[480px] cursor-pointer items-center justify-center overflow-hidden rounded-lg bg-primary px-4 text-sm font-medium text-white transition-colors hover:bg-primary/90">
                            <span class="truncate">Close</span>
                        </button>
                    </div>
                </footer>
            </div>
        `;

        document.body.appendChild(modal);
    } catch (error) {
        console.error('Error showing task details:', error);
        alert('Error loading task details: ' + error.message);
    }
}

async function cancelTask(taskId) {
    if (!confirm('Are you sure you want to cancel this task?')) return;

    try {
        await api.cancelTask(taskId);
        alert('Task canceled successfully');
        document.querySelector('.fixed').remove();
        router.navigate('/tasks');
    } catch (error) {
        console.error('Error canceling task:', error);
        alert('Error canceling task: ' + error.message);
    }
}

// Router
const router = {
    routes: {
        '/': components.dashboard,
        '/submit-task': components.submitTask,
        '/tasks': components.tasks,
        '/metrics': components.metrics,
    },

    async navigate(path) {
        const route = this.routes[path];
        if (!route) {
            console.error('Route not found:', path);
            return;
        }

        const content = await route();
        document.getElementById('app-content').innerHTML = content;

        // Update URL without page reload
        window.history.pushState({}, '', path);

        // Set up form handlers if on submit page
        if (path === '/submit-task') {
            this.setupTaskFormHandler();
        }

        // Update active nav link
        this.updateActiveNav(path);
    },

    setupTaskFormHandler() {
        const form = document.getElementById('task-form');
        if (!form) return;

        form.addEventListener('submit', async (e) => {
            e.preventDefault();

            const formData = new FormData(form);
            const data = {
                query: formData.get('query'),
                priority: formData.get('priority'),
                budget: parseFloat(formData.get('budget')),
                timeout: formData.get('timeout') ? parseInt(formData.get('timeout')) : 30
            };

            try {
                const result = await api.submitTask(data);
                alert(`Task submitted successfully! Task ID: ${result.task_id}`);
                router.navigate('/tasks');
            } catch (error) {
                console.error('Error submitting task:', error);
                alert('Error submitting task: ' + error.message);
            }
        });
    },

    updateActiveNav(currentPath) {
        // Remove all active states
        document.querySelectorAll('nav a').forEach(link => {
            link.classList.remove('font-bold');
            link.classList.add('text-white/80');
        });

        // Add active state to current nav item
        const navMap = {
            '/': 'nav-dashboard',
            '/tasks': 'nav-tasks',
            '/submit-task': 'nav-tasks',
            '/agents': 'nav-agents',
            '/metrics': 'nav-metrics'
        };

        const activeId = navMap[currentPath];
        if (activeId) {
            const activeLink = document.getElementById(activeId);
            if (activeLink) {
                activeLink.classList.add('font-bold');
                activeLink.classList.remove('text-white/80');
            }
        }
    },

    init() {
        // Handle browser back/forward
        window.addEventListener('popstate', () => {
            this.navigate(window.location.pathname);
        });

        // Handle navigation links
        document.addEventListener('click', (e) => {
            const link = e.target.closest('a[href^="/"]');
            if (link && !link.hasAttribute('target')) {
                e.preventDefault();
                this.navigate(link.getAttribute('href'));
            }
        });

        // Navigate to current path
        this.navigate(window.location.pathname);
    }
};

// Initialize app when DOM is ready
document.addEventListener('DOMContentLoaded', () => {
    router.init();
});
