# Sprint 7 - Actual Work (Web UI Integration & Deployment)

**Discovery**: Frontend already exists! The actual Sprint 7 work is integration and deployment.

---

## What We Already Have ✅

### Complete Frontend (Vanilla JS + Tailwind)
- ✅ Login/Signup pages with password strength validation
- ✅ Dashboard with metrics and recent tasks
- ✅ Agent marketplace with search/filtering
- ✅ Task management UI
- ✅ Analytics dashboard
- ✅ Settings page
- ✅ API keys management
- ✅ Beautiful "Aether Gradient" theme (Cyan → Magenta → Purple)
- ✅ Responsive design (mobile-first)
- ✅ Client-side routing (SPA)
- ✅ Authentication with JWT
- ✅ API client with Bearer token auth

### API Integration
- ✅ Configured to use production API: `https://zerostate-api.fly.dev/api/v1`
- ✅ Correct endpoints (`/users/login`, `/users/register`, `/agents`, `/tasks/submit`)
- ✅ Auth token storage in localStorage
- ✅ Auto-redirect on 401 (session expired)

---

## What's Missing (Actual Sprint 7 Tasks)

### 1. WebSocket Real-Time Integration ⏳

**Current State**: Frontend polls for updates, no WebSocket yet
**Goal**: Add WebSocket client for real-time task status updates

**Implementation Plan**:
```javascript
// Add to app.js
class WebSocketClient {
    constructor(authManager) {
        this.authManager = authManager;
        this.ws = null;
        this.reconnectAttempts = 0;
        this.maxReconnectAttempts = 5;
        this.reconnectDelay = 1000; // Start with 1s
    }

    connect() {
        const wsUrl = API_BASE_URL.replace('https://', 'wss://').replace('http://', 'ws://');
        this.ws = new WebSocket(`${wsUrl}/ws/connect`);

        this.ws.onopen = () => {
            console.log('WebSocket connected');
            this.reconnectAttempts = 0;
            this.reconnectDelay = 1000;
            UIHelpers.showNotification('Connected to real-time updates', 'success');
        };

        this.ws.onmessage = (event) => {
            const message = JSON.parse(event.data);
            this.handleMessage(message);
        };

        this.ws.onerror = (error) => {
            console.error('WebSocket error:', error);
        };

        this.ws.onclose = () => {
            console.log('WebSocket disconnected');
            this.reconnect();
        };
    }

    handleMessage(message) {
        switch(message.type) {
            case 'task_update':
                this.handleTaskUpdate(message.data);
                break;
            case 'agent_update':
                this.handleAgentUpdate(message.data);
                break;
            case 'system':
                UIHelpers.showNotification(message.data.message, message.data.level || 'info');
                break;
        }
    }

    handleTaskUpdate(data) {
        // Update task status in UI
        const taskElement = document.querySelector(`[data-task-id="${data.task_id}"]`);
        if (taskElement) {
            // Update status badge
            const statusBadge = taskElement.querySelector('[data-status]');
            if (statusBadge) {
                statusBadge.textContent = data.status;
                statusBadge.className = getStatusClass(data.status);
            }
        }

        // Show notification for completed tasks
        if (data.status === 'completed') {
            UIHelpers.showNotification(`Task "${data.query}" completed!`, 'success');
        } else if (data.status === 'failed') {
            UIHelpers.showNotification(`Task "${data.query}" failed`, 'error');
        }
    }

    handleAgentUpdate(data) {
        // Refresh agent list if on agents page
        if (window.location.pathname === '/agents.html') {
            window.location.reload(); // Simple approach
        }
    }

    reconnect() {
        if (this.reconnectAttempts >= this.maxReconnectAttempts) {
            UIHelpers.showNotification('Lost connection to server', 'error');
            return;
        }

        this.reconnectAttempts++;
        setTimeout(() => {
            console.log(`Reconnecting... (attempt ${this.reconnectAttempts})`);
            this.connect();
        }, this.reconnectDelay);

        // Exponential backoff
        this.reconnectDelay *= 2;
    }

    send(message) {
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(JSON.stringify(message));
        }
    }

    disconnect() {
        if (this.ws) {
            this.ws.close();
        }
    }
}

function getStatusClass(status) {
    const classes = {
        'queued': 'px-3 py-1 rounded-full text-xs font-medium bg-gray-500/20 text-gray-400',
        'running': 'px-3 py-1 rounded-full text-xs font-medium bg-blue-500/20 text-blue-400',
        'completed': 'px-3 py-1 rounded-full text-xs font-medium bg-green-500/20 text-green-400',
        'failed': 'px-3 py-1 rounded-full text-xs font-medium bg-red-500/20 text-red-400'
    };
    return classes[status] || classes['queued'];
}

// Initialize WebSocket on authenticated pages
if (authManager.isAuthenticated() && window.location.pathname !== '/login.html' && window.location.pathname !== '/signup.html') {
    const wsClient = new WebSocketClient(authManager);
    wsClient.connect();
    window.app.wsClient = wsClient;
}
```

### 2. Connection Status Indicator ⏳

Add visual indicator showing WebSocket connection status:

```html
<!-- Add to all authenticated pages -->
<div id="connection-status" class="fixed bottom-4 right-4 z-50 hidden">
    <div class="flex items-center gap-2 px-4 py-2 rounded-lg bg-white/10 backdrop-blur-sm">
        <span class="inline-block w-2 h-2 rounded-full" id="connection-indicator"></span>
        <span class="text-sm" id="connection-text">Connecting...</span>
    </div>
</div>

<script>
function updateConnectionStatus(status) {
    const indicator = document.getElementById('connection-indicator');
    const text = document.getElementById('connection-text');
    const container = document.getElementById('connection-status');

    if (status === 'connected') {
        indicator.className = 'inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse';
        text.textContent = 'Connected';
        setTimeout(() => container.classList.add('hidden'), 3000);
    } else if (status === 'disconnected') {
        indicator.className = 'inline-block w-2 h-2 rounded-full bg-red-500';
        text.textContent = 'Disconnected';
        container.classList.remove('hidden');
    } else {
        indicator.className = 'inline-block w-2 h-2 rounded-full bg-yellow-500 animate-pulse';
        text.textContent = 'Connecting...';
        container.classList.remove('hidden');
    }
}
</script>
```

### 3. Vercel Deployment ⏳

**Files Needed**:

**vercel.json** (already exists):
```json
{
  "rewrites": [
    {
      "source": "/(.*)",
      "destination": "/web/static/$1"
    }
  ]
}
```

**Deployment Steps**:
```bash
# 1. Install Vercel CLI
npm install -g vercel

# 2. Login to Vercel
vercel login

# 3. Deploy
cd /home/rocz/vegalabs/zerostate
vercel --prod

# 4. Set environment variables (if needed)
vercel env add VITE_API_URL production
# Value: https://zerostate-api.fly.dev/api/v1
```

### 4. Update Task Submission Format ⏳

**Current Issue**: Frontend may be sending old format
**Fix**: Update task submission to match new API format

```javascript
// In app.js, update submitTask method
async submitTask(taskData) {
    // Ensure correct format
    const formattedData = {
        query: taskData.query || taskData.description,
        budget: parseFloat(taskData.budget),
        timeout: parseInt(taskData.timeout) || 60,
        priority: taskData.priority || 'medium', // String, not number
        constraints: taskData.constraints || {
            memory_mb: 512,
            cpu_cores: 1
        }
    };

    return this.request('/tasks/submit', {
        method: 'POST',
        body: JSON.stringify(formattedData),
    });
}
```

### 5. Agent Upload with S3 ⏳

**Current State**: Frontend has upload form, needs S3 integration testing
**Goal**: Verify agent upload works with production S3

**Test Plan**:
1. Navigate to agent upload page
2. Select WASM file (<50MB)
3. Fill in agent details (name, description, capabilities)
4. Submit form
5. Verify file uploads to S3
6. Verify agent appears in marketplace

---

## Sprint 7 Timeline (Revised)

### Day 1: WebSocket Integration
- [ ] Add WebSocketClient class to app.js
- [ ] Implement connection/reconnection logic
- [ ] Add message handlers (task_update, agent_update, system)
- [ ] Test real-time updates

### Day 2: UI Enhancements
- [ ] Add connection status indicator
- [ ] Update task submission format
- [ ] Test agent upload with S3
- [ ] Fix any API integration issues

### Day 3: Testing & Polish
- [ ] Test all pages end-to-end
- [ ] Verify real-time updates work
- [ ] Test on mobile/tablet/desktop
- [ ] Performance testing (Lighthouse)

### Day 4: Vercel Deployment
- [ ] Deploy to Vercel
- [ ] Configure custom domain (optional)
- [ ] Set environment variables
- [ ] Test production deployment
- [ ] Update docs with deployment URLs

---

## Success Metrics

### Functionality
- [ ] Users can signup/login successfully
- [ ] Dashboard shows real-time metrics
- [ ] WebSocket connection stable (>99% uptime)
- [ ] Task status updates in real-time
- [ ] Agent upload works with S3
- [ ] All pages responsive on mobile

### Performance
- [ ] Page load <2s on 3G
- [ ] WebSocket latency <100ms
- [ ] Lighthouse score >90
- [ ] No console errors

### Deployment
- [ ] Frontend deployed to Vercel
- [ ] HTTPS enabled
- [ ] Custom domain configured
- [ ] Preview deployments work

---

## Production URLs (After Deployment)

**Backend (Already Live)**:
- API: https://zerostate-api.fly.dev
- Health: https://zerostate-api.fly.dev/health
- WebSocket: wss://zerostate-api.fly.dev/api/v1/ws/connect

**Frontend (To Deploy)**:
- Production: https://zerostate.vercel.app (example)
- Preview: https://zerostate-{branch}.vercel.app

---

## Next Steps

1. **Add WebSocket client** to `web/static/js/app.js`
2. **Add connection indicator** to all authenticated pages
3. **Test everything locally** before deploying
4. **Deploy to Vercel** with `vercel --prod`
5. **Celebrate** - Full-stack production deployment complete! 🎉
