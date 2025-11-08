# Sprint 7: Web UI & User Experience

**Duration**: 2 weeks
**Start Date**: Week of Nov 8, 2025
**Focus**: Build production-ready web interface and complete user workflows

---

## Sprint Goals

1. ✅ **Build functional web UI** for agent marketplace and task submission
2. ✅ **Implement real-time updates** via WebSocket for task status
3. ✅ **Complete end-to-end user workflows** from signup to task execution
4. ✅ **Add basic analytics dashboard** for users to track usage
5. ✅ **Polish UX** and ensure mobile responsiveness

---

## Current Status (Starting Point)

### ✅ **What's Working** (Production-Ready)
- Backend API deployed on Fly.io ([https://zerostate-api.fly.dev](https://zerostate-api.fly.dev/))
- User authentication (register, login, JWT tokens)
- 15 mock agents in marketplace
- Task submission and queueing via Redis
- WebSocket hub for real-time updates
- Prometheus metrics and health monitoring
- Test suite (7/7 tests passing)

### ⚠️ **What's Missing** (Sprint 7 Focus)
- **No web UI** - Only API endpoints exist
- **No task execution** - Tasks queue but don't run
- **No agent upload workflow** - Binary upload endpoint exists but no UI
- **No payment integration** - Free for now, need to add later
- **No real-time UI updates** - WebSocket works but no UI to display them

---

## Sprint 7 Tasks

### **Task 1: React Frontend Setup** (3 days)
**Objective**: Bootstrap modern React app with routing and authentication

**Deliverables**:
- [ ] Initialize React app with Vite (fast build times)
- [ ] Set up React Router for navigation
- [ ] Implement authentication context with JWT storage
- [ ] Create protected route wrapper component
- [ ] Set up Tailwind CSS for styling
- [ ] Configure Axios/Fetch for API calls with auth headers
- [ ] Add environment variables for API URLs

**Tech Stack**:
- React 18 with TypeScript
- Vite for bundling
- React Router v6
- Tailwind CSS + HeadlessUI
- React Query for server state
- Zustand for client state

**Acceptance Criteria**:
- ✅ User can navigate between pages
- ✅ Login/signup forms functional
- ✅ JWT persists in localStorage
- ✅ Protected routes redirect to login
- ✅ API requests include auth headers
- ✅ Mobile-responsive layout

---

### **Task 2: Agent Marketplace UI** (4 days)
**Objective**: Build beautiful marketplace for browsing and searching agents

**Pages**:
1. **`/agents`** - Agent marketplace listing
2. **`/agents/:id`** - Individual agent details
3. **`/agents/upload`** - Upload new agent (for creators)

**Components**:
- `AgentCard` - Display agent with image, name, price, rating, capabilities
- `AgentGrid` - Responsive grid layout with filtering/sorting
- `AgentSearch` - Search bar with autocomplete
- `AgentFilters` - Filter by capabilities, price range, rating
- `AgentDetails` - Full agent profile with reviews and metrics
- `AgentUploadForm` - Multi-step wizard for uploading WASM binaries

**Features**:
- 🔍 Real-time search across agent names and capabilities
- 🎯 Filter by capabilities (compute, storage, ml_training, etc.)
- 💰 Sort by price, rating, tasks_completed
- ⭐ Display average rating with star visualization
- 📊 Show task completion stats
- 🎨 Beautiful card design with hover effects
- 📱 Mobile-optimized with infinite scroll

**Acceptance Criteria**:
- ✅ User can browse all 15 agents
- ✅ Search returns relevant results in <300ms
- ✅ Filters work correctly
- ✅ Agent details page shows full information
- ✅ Upload form validates WASM files (<50MB)
- ✅ Responsive on mobile, tablet, desktop

---

### **Task 3: Task Submission & Dashboard** (4 days)
**Objective**: Allow users to submit tasks and monitor execution

**Pages**:
1. **`/submit-task`** - Task submission form
2. **`/tasks`** - Task history and status dashboard
3. **`/tasks/:id`** - Individual task detail view

**Components**:
- `TaskSubmissionForm` - Multi-step form for creating tasks
  - Step 1: Describe task and requirements
  - Step 2: Select agent(s) from marketplace
  - Step 3: Set budget and timeout
  - Step 4: Review and submit
- `TaskDashboard` - Overview of all user tasks
- `TaskCard` - Display task with status badge (queued, running, completed, failed)
- `TaskDetailView` - Full task info with logs and results
- `TaskStatusBadge` - Colored badge for task status
- `RealTimeTaskUpdates` - WebSocket integration for live status

**Features**:
- 📝 Natural language task descriptions
- 🤖 Smart agent recommendations based on task type
- 💵 Budget calculator with cost estimates
- ⏱️ Real-time status updates via WebSocket
- 📊 Progress bar for running tasks
- 📈 Task history with pagination
- 🔔 Notifications for task completion
- 📁 Download task results

**Acceptance Criteria**:
- ✅ User can submit task with all required fields
- ✅ Task appears in dashboard immediately with "queued" status
- ✅ WebSocket updates task status in real-time
- ✅ User can view task details and download results
- ✅ Task history shows all past tasks
- ✅ Error handling for failed submissions

---

### **Task 4: WebSocket Real-Time Integration** (2 days)
**Objective**: Connect frontend to WebSocket for live updates

**Implementation**:
- Create `useWebSocket` custom React hook
- Implement reconnection logic with exponential backoff
- Add heartbeat/ping-pong for connection health
- Handle task status updates from backend
- Display toast notifications for events
- Update UI state reactively without page refresh

**Events to Handle**:
- `task_update` - Task status changed (queued → running → completed)
- `agent_update` - New agent added to marketplace
- `system` - System messages and notifications

**Acceptance Criteria**:
- ✅ WebSocket connects on user login
- ✅ Connection auto-reconnects on disconnect
- ✅ Task status updates appear without refresh
- ✅ Toast notifications for important events
- ✅ Graceful degradation if WebSocket unavailable

---

### **Task 5: User Profile & Settings** (2 days)
**Objective**: Let users manage their account and view usage stats

**Pages**:
1. **`/profile`** - User profile and account settings
2. **`/dashboard`** - Personal analytics and usage metrics

**Components**:
- `ProfileSettings` - Edit email, name, password, avatar
- `UsageMetrics` - Charts showing task history, spending, agent usage
- `BillingInfo` - Credit balance and transaction history (placeholder for now)
- `APIKeyManager` - Generate and manage API keys

**Features**:
- 👤 Avatar upload
- 📊 Usage charts (tasks over time, budget spent, agents used)
- 🔑 API key generation for programmatic access
- 📧 Email preferences and notifications
- 🎨 Theme toggle (light/dark mode)

**Acceptance Criteria**:
- ✅ User can update profile information
- ✅ Avatar upload works (max 5MB)
- ✅ Charts display correctly with real data
- ✅ API keys can be generated and revoked
- ✅ Settings persist across sessions

---

### **Task 6: Polish & Mobile Optimization** (2 days)
**Objective**: Ensure professional UX across all devices

**Focus Areas**:
- 📱 Mobile responsiveness (test on iOS and Android)
- ⚡ Performance optimization (lazy loading, code splitting)
- ♿ Accessibility (WCAG 2.1 AA compliance)
- 🎨 Visual polish (animations, transitions, micro-interactions)
- 🐛 Bug fixes and edge cases
- 📝 User onboarding flow (first-time user tutorial)

**Performance Targets**:
- Lighthouse score >90 for all categories
- First Contentful Paint <1.5s
- Time to Interactive <3s
- Bundle size <500KB gzipped

**Acceptance Criteria**:
- ✅ Works on iPhone, Android, tablet, desktop
- ✅ Lighthouse scores >90
- ✅ Keyboard navigation works throughout
- ✅ Screen reader friendly
- ✅ Loading states for all async operations
- ✅ Error boundaries catch React errors
- ✅ Onboarding tutorial guides new users

---

### **Task 7: Deployment & CI/CD** (1 day)
**Objective**: Deploy frontend to Vercel with automated builds

**Setup**:
- Connect GitHub repo to Vercel
- Configure build settings (Vite production build)
- Set environment variables (API_URL, WS_URL)
- Enable preview deployments for PRs
- Set up branch deployments (main → production, develop → staging)

**Acceptance Criteria**:
- ✅ Frontend deployed to Vercel
- ✅ Custom domain configured (optional)
- ✅ HTTPS enabled
- ✅ Environment variables set correctly
- ✅ Preview deployments work for PRs

---

## Success Metrics

### **User Experience**
- [ ] Users can sign up and complete first task in <5 minutes
- [ ] Task submission success rate >95%
- [ ] WebSocket connection uptime >99%
- [ ] Mobile users can complete all workflows

### **Performance**
- [ ] Page load <2s on 3G
- [ ] API response time <200ms p95
- [ ] WebSocket latency <100ms
- [ ] Lighthouse score >90

### **Completeness**
- [ ] All core user workflows functional
- [ ] 0 critical bugs in production
- [ ] Mobile responsive on all pages
- [ ] Accessibility WCAG 2.1 AA compliant

---

## Technical Architecture

### **Frontend Stack**
```
Frontend (Vercel)
├── React 18 + TypeScript
├── React Router v6
├── Tailwind CSS + HeadlessUI
├── React Query (server state)
├── Zustand (client state)
├── Axios (HTTP client)
├── Socket.io-client (WebSocket)
└── Vite (bundler)
```

### **Backend Integration**
```
API Endpoints (Fly.io)
├── POST /api/v1/users/register
├── POST /api/v1/users/login
├── GET  /api/v1/agents
├── GET  /api/v1/agents/:id
├── POST /api/v1/agents/:id/binary
├── POST /api/v1/tasks/submit
├── GET  /api/v1/tasks
├── GET  /api/v1/tasks/:id
└── WS   /api/v1/ws/connect
```

### **Folder Structure**
```
web/
├── src/
│   ├── components/
│   │   ├── agents/
│   │   │   ├── AgentCard.tsx
│   │   │   ├── AgentGrid.tsx
│   │   │   └── AgentDetails.tsx
│   │   ├── tasks/
│   │   │   ├── TaskDashboard.tsx
│   │   │   ├── TaskSubmissionForm.tsx
│   │   │   └── TaskCard.tsx
│   │   ├── layout/
│   │   │   ├── Navbar.tsx
│   │   │   ├── Sidebar.tsx
│   │   │   └── Footer.tsx
│   │   └── common/
│   │       ├── Button.tsx
│   │       ├── Input.tsx
│   │       └── Modal.tsx
│   ├── pages/
│   │   ├── AgentMarketplace.tsx
│   │   ├── TaskDashboard.tsx
│   │   ├── SubmitTask.tsx
│   │   └── Profile.tsx
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   ├── useWebSocket.ts
│   │   └── useAgents.ts
│   ├── api/
│   │   ├── client.ts
│   │   ├── agents.ts
│   │   └── tasks.ts
│   └── store/
│       ├── authStore.ts
│       └── taskStore.ts
└── public/
    └── assets/
```

---

## Dependencies on Previous Sprints

### **Sprints 1-5** (Infrastructure) ✅
- P2P networking, task queue, payment channels, reputation system
- All foundation components complete

### **Sprint 6** (Tier 1 Production) ✅
- S3 cloud storage
- WebSocket hub
- Redis task queue
- User authentication
- Production deployment on Fly.io

### **Sprint 7** (This Sprint)
- Builds on top of working API
- No backend changes required (unless bugs found)
- Pure frontend development sprint

---

## Risks & Mitigations

### **Risk 1: WebSocket Connection Issues**
- **Mitigation**: Implement robust reconnection logic with exponential backoff
- **Fallback**: Poll API every 5s if WebSocket unavailable

### **Risk 2: Mobile Performance**
- **Mitigation**: Lazy load components, optimize bundle size, use CDN
- **Testing**: Test on real devices, not just emulators

### **Risk 3: Browser Compatibility**
- **Mitigation**: Polyfills for older browsers, progressive enhancement
- **Testing**: Test on Chrome, Firefox, Safari, Edge

### **Risk 4**: Scope Creep
- **Mitigation**: Stick to MVP features, defer nice-to-haves to Sprint 8
- **Priority**: Core workflows (signup → browse → submit task) must work perfectly

---

## Post-Sprint 7 Roadmap

### **Sprint 8: Task Execution & Agent Runtime**
- Implement actual task execution (currently just queues)
- WASM runtime integration
- Agent-to-agent communication
- Result persistence

### **Sprint 9: Payments & Billing**
- Stripe integration
- Credit system
- Transaction history
- Payment channels for micro-transactions

### **Sprint 10: Advanced Features**
- Agent reviews and ratings
- Referral program
- Advanced analytics
- Admin dashboard

---

## Sprint Cadence

- **Week 1**: Tasks 1-4 (Setup, Marketplace, Task Submission, WebSocket)
- **Week 2**: Tasks 5-7 (Profile, Polish, Deployment)
- **Daily Standups**: Async via Slack/Discord
- **Sprint Review**: Demo all features end-to-end
- **Sprint Retro**: Document lessons learned

---

## Definition of Done

A task is "done" when:
- ✅ Code is written and tested
- ✅ Component is responsive (mobile, tablet, desktop)
- ✅ Accessibility requirements met
- ✅ Unit tests pass (if applicable)
- ✅ Integration with backend API verified
- ✅ Performance benchmarks met
- ✅ Code reviewed and merged to main
- ✅ Deployed to staging for QA

---

**Next Steps**: Begin Task 1 (React Frontend Setup) on Monday!
