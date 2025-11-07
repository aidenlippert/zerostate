# ZeroState - Team Collaboration Setup

**Last Updated:** November 7, 2025
**Purpose:** FAANG-level team collaboration guide

---

## Quick Start for New Team Members

### Day 1: Environment Setup

```bash
# 1. Clone repository
git clone https://github.com/YOUR_ORG/zerostate.git
cd zerostate

# 2. Install dependencies
make setup

# 3. Run tests to verify
make test

# 4. Start development environment
docker-compose up -d

# 5. Verify all services
make health-check
```

### Day 2-3: First Contribution

1. **Read CONTRIBUTING.md**
2. **Pick a "good first issue"** from GitHub
3. **Create feature branch**
4. **Make changes**
5. **Submit PR**

---

## GitHub Project Board Setup

### Board Structure

We use **GitHub Projects (Beta)** with automation.

#### Columns

1. **📋 Backlog** - All issues not yet started
2. **🎯 Sprint Backlog** - Issues planned for current sprint
3. **🔄 In Progress** - Actively being worked on
4. **👀 In Review** - PR submitted, awaiting review
5. **✅ Done** - Completed and merged

#### Views

1. **Sprint Board** - Kanban view (default)
2. **By Priority** - Group by P0/P1/P2/P3
3. **By Component** - Group by API/UI/Infrastructure
4. **By Assignee** - Who's working on what
5. **Timeline** - Gantt chart view

### Creating the Project Board

```bash
# Using GitHub CLI
gh project create \
  --title "ZeroState MVP" \
  --body "Track all work for ZeroState MVP launch"

# Add fields
gh project field-create <project-number> \
  --name "Priority" \
  --data-type "SINGLE_SELECT" \
  --single-select-options "P0,P1,P2,P3"

gh project field-create <project-number> \
  --name "Component" \
  --data-type "SINGLE_SELECT" \
  --single-select-options "API,UI,Infrastructure,P2P,Execution,Payments"

gh project field-create <project-number> \
  --name "Sprint" \
  --data-type "SINGLE_SELECT" \
  --single-select-options "Sprint 7,Sprint 8,Sprint 9,Sprint 10"
```

### Issue Workflow Automation

**Automation Rules:**

1. **New Issue** → Backlog
2. **Assigned** → Sprint Backlog
3. **PR Created** → In Review
4. **PR Merged** → Done
5. **Issue Closed** → Done

---

## Sprint Planning Process

### Sprint Cadence

- **Sprint Duration:** 2 weeks
- **Sprint Planning:** Monday 10am
- **Daily Standup:** Every day 9:30am (15 min)
- **Sprint Review:** Friday 2pm (before sprint end)
- **Sprint Retro:** Friday 3pm

### Sprint Planning Meeting

**Agenda (2 hours):**

1. **Review previous sprint** (15 min)
   - Completed work
   - Uncompleted work
   - Blockers

2. **Demo completed features** (30 min)
   - Live demos
   - Metrics review

3. **Plan next sprint** (60 min)
   - Review backlog
   - Estimate stories
   - Commit to sprint goal
   - Assign issues

4. **Identify risks** (15 min)
   - Dependencies
   - Blockers
   - Capacity

### Issue Estimation

**Story Points:**
- **XS (1)**: < 2 hours
- **S (2)**: 2-4 hours
- **M (3)**: 4-8 hours (half day)
- **L (5)**: 1-2 days
- **XL (8)**: 2+ days (break into smaller tasks)

**Planning Poker:**
- Team votes simultaneously
- Discuss outliers
- Re-vote until consensus

---

## Parallel Development Workflow

### Branch Strategy

```
main
  ↑
  merge ← PR ← feature/alice/agent-registration
  ↑
  merge ← PR ← feature/bob/task-submission
  ↑
  merge ← PR ← feature/carol/web-ui
```

### Avoiding Conflicts

**1. Assign Different Components**
- Alice: API layer (`libs/api/`)
- Bob: Orchestration (`libs/orchestration/`)
- Carol: Web UI (`web/`)

**2. Communicate Changes**
- Slack channel for code updates
- Tag teammates in PRs

**3. Rebase Frequently**
```bash
# Daily rebase
git checkout feature/your-feature
git fetch origin
git rebase origin/main
```

**4. Small, Focused PRs**
- < 500 lines changed
- Single responsibility
- Quick review cycle

---

## Code Review Process

### Review Assignment

**Auto-assignment via CODEOWNERS:**

```
# .github/CODEOWNERS
/libs/api/          @alice @bob
/libs/orchestration/ @bob @carol
/web/               @carol @alice
/libs/p2p/          @dave
/libs/execution/    @dave @bob
/docs/              @everyone
```

### Review Checklist

**Reviewers must verify:**

- [ ] **Functionality** - Does it work?
- [ ] **Tests** - Are there tests?
- [ ] **Code Quality** - Follows standards?
- [ ] **Performance** - No regressions?
- [ ] **Security** - No vulnerabilities?
- [ ] **Documentation** - Updated?

### Review SLA

| Priority | Response Time | Completion Time |
|----------|---------------|-----------------|
| P0 (Critical) | 2 hours | 4 hours |
| P1 (High) | 4 hours | 1 day |
| P2 (Medium) | 1 day | 2 days |
| P3 (Low) | 2 days | 1 week |

### Review Best Practices

**For Authors:**
- Self-review first
- Add screenshots/demos
- Explain tricky parts
- Respond to feedback quickly

**For Reviewers:**
- Be constructive
- Ask questions
- Suggest alternatives
- Approve when ready

---

## Communication Channels

### Slack/Discord Channels

```
#general              - Announcements, celebrations
#engineering          - Technical discussions
#sprint-planning      - Sprint planning, retrospectives
#code-review          - PR notifications, reviews
#ci-cd                - Build notifications
#production-alerts    - Critical alerts
#random               - Off-topic
```

### GitHub Features

**1. Discussions**
- Architecture decisions
- RFCs
- Q&A

**2. Issues**
- Bugs
- Feature requests
- Tasks

**3. Pull Requests**
- Code reviews
- Technical discussion

**4. Wiki**
- Architecture docs
- Runbooks
- Onboarding guides

---

## Development Environment

### IDE Setup

**VS Code Extensions:**
- Go (official)
- GitLens
- Better Comments
- Error Lens
- Docker
- Kubernetes

**Settings:**
```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.formatTool": "gofmt",
  "go.testFlags": ["-v", "-race"],
  "editor.formatOnSave": true,
  "files.trimTrailingWhitespace": true
}
```

### Local Development

```bash
# Start full stack
make dev

# Run specific service
make run-api

# Watch mode (auto-reload)
make watch

# View logs
make logs

# Stop everything
make stop
```

---

## Metrics & Dashboards

### Team Metrics

**Track in Grafana:**
- PR cycle time (open → merge)
- Code review time
- Deployment frequency
- MTTR (Mean Time To Recovery)
- Sprint velocity
- Bug escape rate

**Dashboard:**
```
http://localhost:3000/d/team-metrics
```

### Individual Metrics

**Per developer:**
- PRs opened
- PRs merged
- Code reviews completed
- Issues closed
- Lines of code (not a quality metric!)

---

## Troubleshooting

### Common Issues

**1. Tests failing on CI but pass locally**
```bash
# Run in CI environment locally
docker run -v $(pwd):/app -w /app golang:1.21 make test
```

**2. Merge conflicts**
```bash
# Rebase on main
git fetch origin
git rebase origin/main

# Resolve conflicts
git mergetool

# Continue rebase
git rebase --continue
```

**3. Docker out of space**
```bash
# Prune everything
docker system prune -a --volumes
```

**4. Port already in use**
```bash
# Find process
lsof -i :8080

# Kill process
kill -9 <PID>
```

---

## Onboarding Checklist

### Week 1

- [ ] GitHub access granted
- [ ] Slack/Discord invited
- [ ] CODEOWNERS updated
- [ ] Environment setup complete
- [ ] First PR merged
- [ ] Team intro meeting

### Week 2

- [ ] Assigned to sprint
- [ ] Completed 2+ issues
- [ ] Reviewed 2+ PRs
- [ ] Attended standup daily
- [ ] Attended sprint planning

### Week 3

- [ ] Leading feature development
- [ ] Mentoring new contributors
- [ ] On-call rotation (if applicable)

---

## FAQ

**Q: How do I get assigned issues?**
A: Attend sprint planning or comment on issues in backlog.

**Q: Can I work on unassigned issues?**
A: Yes! Comment "I'll take this" and assign yourself.

**Q: How long should PRs take to review?**
A: See Review SLA table above.

**Q: What if I'm blocked?**
A: Post in #engineering channel immediately.

**Q: How do I request code review?**
A: PR is auto-assigned via CODEOWNERS. Mention specific reviewers if needed.

**Q: Can I merge my own PR?**
A: No. Requires 2 approvals from other team members.

**Q: What's the PR size limit?**
A: < 500 lines preferred. Break larger changes into multiple PRs.

---

## Git Aliases (Optional)

Add to `~/.gitconfig`:

```ini
[alias]
  co = checkout
  br = branch
  ci = commit
  st = status
  unstage = reset HEAD --
  last = log -1 HEAD
  visual = log --graph --oneline --all
  amend = commit --amend --no-edit
  pushf = push --force-with-lease
  review = !gh pr checks
```

---

**Welcome to the ZeroState team!** 🚀

For questions, ping in #engineering or open a GitHub Discussion.
