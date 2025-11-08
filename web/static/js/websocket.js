// ZeroState WebSocket Client
// Real-time updates for tasks, agents, and system messages

class WebSocketClient {
    constructor(authManager, apiBaseUrl) {
        this.authManager = authManager;
        this.apiBaseUrl = apiBaseUrl;
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000; // Start with 1s
        this.pingInterval = null;
    }

    connect() {
        // Convert HTTP(S) URL to WS(S)
        const wsUrl = this.apiBaseUrl.replace('https://', 'wss://').replace('http://', 'ws://');
        const wsEndpoint = `${wsUrl}/ws/connect`;

        console.log('🔌 Connecting to WebSocket:', wsEndpoint);

        try {
            this.ws = new WebSocket(wsEndpoint);

            this.ws.onopen = () => {
                console.log('✅ WebSocket connected');
                this.reconnectAttempts = 0;
                this.reconnectDelay = 1000;
                this.updateConnectionStatus('connected');
                this.startPing();
            };

            this.ws.onmessage = (event) => {
                try {
                    const message = JSON.parse(event.data);
                    this.handleMessage(message);
                } catch (error) {
                    console.error('Error parsing WebSocket message:', error);
                }
            };

            this.ws.onerror = (error) => {
                console.error('❌ WebSocket error:', error);
                this.updateConnectionStatus('error');
            };

            this.ws.onclose = () => {
                console.log('🔌 WebSocket disconnected');
                this.stopPing();
                this.updateConnectionStatus('disconnected');
                this.reconnect();
            };
        } catch (error) {
            console.error('Failed to create WebSocket:', error);
            this.updateConnectionStatus('error');
        }
    }

    startPing() {
        // Send ping every 25 seconds to keep connection alive
        this.pingInterval = setInterval(() => {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                this.send({ type: 'ping' });
            }
        }, 25000);
    }

    stopPing() {
        if (this.pingInterval) {
            clearInterval(this.pingInterval);
            this.pingInterval = null;
        }
    }

    handleMessage(message) {
        console.log('📨 WebSocket message:', message);

        switch (message.type) {
            case 'task_update':
                this.handleTaskUpdate(message.data);
                break;
            case 'agent_update':
                this.handleAgentUpdate(message.data);
                break;
            case 'system':
                this.handleSystemMessage(message.data);
                break;
            case 'pong':
                // Pong received, connection is alive
                break;
            default:
                console.log('Unknown message type:', message.type);
        }
    }

    handleTaskUpdate(data) {
        console.log('📋 Task update:', data);

        // Update task in UI if element exists
        const taskElement = document.querySelector(`[data-task-id="${data.task_id}"]`);
        if (taskElement) {
            const statusBadge = taskElement.querySelector('[data-status]');
            if (statusBadge) {
                statusBadge.textContent = data.status;
                statusBadge.className = this.getStatusClass(data.status);
            }
        }

        // Show notification for completed/failed tasks
        if (data.status === 'completed') {
            window.app.UIHelpers.showNotification(
                `Task "${data.query || data.task_id}" completed successfully!`,
                'success'
            );
        } else if (data.status === 'failed') {
            window.app.UIHelpers.showNotification(
                `Task "${data.query || data.task_id}" failed`,
                'error'
            );
        } else if (data.status === 'running') {
            window.app.UIHelpers.showNotification(
                `Task "${data.query || data.task_id}" is now running`,
                'info'
            );
        }

        // Reload task list if on tasks page
        if (window.location.pathname === '/tasks.html') {
            this.reloadTaskList();
        }

        // Update dashboard if on dashboard page
        if (window.location.pathname === '/dashboard.html') {
            this.reloadDashboard();
        }
    }

    handleAgentUpdate(data) {
        console.log('🤖 Agent update:', data);

        window.app.UIHelpers.showNotification(
            `Agent "${data.name || data.agent_id}" updated`,
            'info'
        );

        // Reload agent list if on agents page
        if (window.location.pathname === '/agents.html') {
            setTimeout(() => window.location.reload(), 500);
        }
    }

    handleSystemMessage(data) {
        console.log('⚙️ System message:', data);

        const level = data.level || 'info';
        window.app.UIHelpers.showNotification(data.message, level);
    }

    getStatusClass(status) {
        const classes = {
            'queued': 'px-3 py-1 rounded-full text-xs font-medium bg-gray-500/20 text-gray-400',
            'running': 'px-3 py-1 rounded-full text-xs font-medium bg-blue-500/20 text-blue-400',
            'completed': 'px-3 py-1 rounded-full text-xs font-medium bg-green-500/20 text-green-400',
            'failed': 'px-3 py-1 rounded-full text-xs font-medium bg-red-500/20 text-red-400'
        };
        return classes[status] || classes['queued'];
    }

    updateConnectionStatus(status) {
        const indicator = document.getElementById('ws-status-indicator');
        const text = document.getElementById('ws-status-text');
        const container = document.getElementById('ws-status');

        if (!indicator || !text || !container) return;

        if (status === 'connected') {
            indicator.className = 'inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse';
            text.textContent = 'Connected';
            // Hide after 3 seconds
            setTimeout(() => {
                if (container) container.classList.add('opacity-0');
            }, 3000);
        } else if (status === 'disconnected') {
            indicator.className = 'inline-block w-2 h-2 rounded-full bg-red-500';
            text.textContent = 'Disconnected';
            container.classList.remove('opacity-0');
        } else if (status === 'error') {
            indicator.className = 'inline-block w-2 h-2 rounded-full bg-yellow-500';
            text.textContent = 'Connection Error';
            container.classList.remove('opacity-0');
        } else {
            indicator.className = 'inline-block w-2 h-2 rounded-full bg-yellow-500 animate-pulse';
            text.textContent = 'Connecting...';
            container.classList.remove('opacity-0');
        }
    }

    reloadTaskList() {
        // Trigger task list reload if function exists
        if (typeof window.loadTasks === 'function') {
            window.loadTasks();
        }
    }

    reloadDashboard() {
        // Trigger dashboard reload if function exists
        if (typeof window.loadDashboard === 'function') {
            window.loadDashboard();
        }
    }

    reconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            console.error('❌ Max reconnection attempts reached');
            window.app.UIHelpers.showNotification(
                'Lost connection to server. Please refresh the page.',
                'error'
            );
            return;
        }

        this.reconnectAttempts++;
        this.updateConnectionStatus('disconnected');

        console.log(`🔄 Reconnecting... (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);

        setTimeout(() => {
            this.connect();
        }, this.reconnectDelay);

        // Exponential backoff: 1s, 2s, 4s, 8s, 16s
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, 30000);
    }

    send(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        } else {
            console.warn('WebSocket not connected, cannot send message:', message);
        }
    }

    disconnect() {
        this.stopPing();
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
    }
}

// Initialize WebSocket when included in authenticated pages
if (typeof window.app !== 'undefined' && window.app.authManager && window.app.authManager.isAuthenticated()) {
    // Only connect on authenticated pages (not login/signup)
    const currentPath = window.location.pathname;
    if (currentPath !== '/login.html' && currentPath !== '/signup.html' && currentPath !== '/') {
        const wsClient = new WebSocketClient(window.app.authManager, API_BASE_URL);
        wsClient.connect();
        window.app.wsClient = wsClient;
        console.log('🚀 WebSocket client initialized');
    }
}
