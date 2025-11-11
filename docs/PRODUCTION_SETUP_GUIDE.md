# Production Setup Guide - ZeroState Beta Launch

## Current Status
- ✅ Backend deployed on Fly.io: https://zerostate-api.fly.dev
- ✅ Frontend configured to connect to backend
- ✅ User registration working
- ✅ JWT authentication working
- ✅ Database operational (SQLite)
- ✅ Redis configured
- ⚠️ Storage (S3) needs configuration
- ⚠️ Vercel needs login/deployment
- ⚠️ Payment system needs implementation

---

## Step 1: Set Up Cloudflare R2 Storage (S3-Compatible)

### Why Cloudflare R2?
- **Cost**: $0.015/GB ($5-10/month for beta)
- **S3-Compatible**: Works with existing AWS SDK code
- **Fast**: Global CDN built-in
- **No egress fees**: Free data transfer out

### Setup Steps:

1. **Create Cloudflare R2 Bucket**:
   ```bash
   # Go to Cloudflare Dashboard → R2
   # Create bucket: "zerostate-agents"
   ```

2. **Get R2 Credentials**:
   - Navigate to: R2 → Manage R2 API Tokens
   - Create new API token with "Object Read & Write" permissions
   - Save these values:
     - `Access Key ID`
     - `Secret Access Key`
     - `Bucket Name`: zerostate-agents
     - `Endpoint`: https://[account-id].r2.cloudflarestorage.com

3. **Set Fly.io Secrets**:
   ```bash
   fly secrets set \
     S3_BUCKET="zerostate-agents" \
     S3_ENDPOINT="https://[account-id].r2.cloudflarestorage.com" \
     S3_REGION="auto" \
     AWS_ACCESS_KEY_ID="your-r2-access-key" \
     AWS_SECRET_ACCESS_KEY="your-r2-secret-key" \
     --app zerostate-api
   ```

4. **Verify Deployment**:
   ```bash
   fly logs --app zerostate-api
   ```

---

## Step 2: Vercel Deployment (Frontend)

### Option A: Deploy via Vercel Dashboard
1. Go to https://vercel.com/dashboard
2. Click "Add New Project"
3. Import your GitHub repo
4. Set root directory to `web/static` (or leave as is if deploying whole repo)
5. Deploy!

### Option B: Deploy via CLI
```bash
# Login to Vercel
vercel login

# Deploy from project root
cd /home/rocz/vegalabs/zerostate
vercel --prod

# Or deploy just the web directory
cd web
vercel --prod
```

### Frontend URL Configuration
The frontend already auto-detects the environment:
```javascript
const API_BASE_URL = window.location.hostname.includes('vercel.app')
    ? 'https://zerostate-api.fly.dev/api/v1'
    : window.location.origin + '/api/v1';
```

No changes needed! ✅

---

## Step 3: Test Agent Registration Flow

Once S3 is configured, test the full flow:

```bash
# 1. Register user
curl -X POST https://zerostate-api.fly.dev/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "beta@example.com",
    "password": "testpass123",
    "full_name": "Beta Tester"
  }' | jq .

# Save the token from response
TOKEN="your-jwt-token-here"

# 2. Register agent with WASM upload
curl -X POST https://zerostate-api.fly.dev/api/v1/agents/register \
  -H "Authorization: Bearer $TOKEN" \
  -F "wasm=@examples/agents/echo-agent/dist/echo-agent.wasm" \
  -F 'agent={
    "name":"Echo Agent",
    "description":"Test agent",
    "capabilities":["echo"],
    "pricing":{"model":"fixed","base_price":0.001},
    "resources":{"memory_mb":64,"cpu_shares":100}
  }' | jq .

# 3. Submit task
curl -X POST https://zerostate-api.fly.dev/api/v1/tasks/submit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Echo: Hello ZeroState!",
    "budget": 0.10,
    "timeout": 30,
    "priority": "normal"
  }' | jq .

# 4. Check task result (use task_id from above)
curl -s https://zerostate-api.fly.dev/api/v1/tasks/{task-id}/result \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

## Step 4: Payment System Decision

### Option A: Start WITHOUT Token (Recommended for Beta)
**Pros**:
- Launch faster (1-2 weeks vs 4-6 weeks)
- Lower regulatory complexity
- Easier to iterate based on feedback
- Simpler for early users

**Implementation**: Stripe Credits System
```bash
# Add Stripe secret key to Fly.io
fly secrets set STRIPE_SECRET_KEY="sk_live_..." --app zerostate-api
```

**User Flow**:
1. User signs up → Free $10 credits
2. User buys more credits via Stripe ($20, $50, $100 packages)
3. Task execution deducts credits
4. Agent providers earn credits → Cash out via Stripe Connect

### Option B: Launch WITH Token ($ZERO)
**Pros**:
- Token appreciation potential
- Network effects (holding = governance)
- True decentralization

**Cons**:
- 4-6 weeks additional dev time
- Regulatory considerations
- Exchange listing challenges
- Liquidity management

**Token Economics**:
- Total Supply: 1,000,000,000 $ZERO
- Distribution: 40% community, 25% team (4yr vest), 20% ecosystem, 15% sale
- Use Cases: Task payments, staking, governance, agent rewards

---

## Step 5: Beta Launch Checklist

### Before Launch:
- [ ] Cloudflare R2 configured and tested
- [ ] Frontend deployed to Vercel
- [ ] Agent registration tested end-to-end
- [ ] Task execution tested with real WASM
- [ ] Payment system implemented (Stripe OR token)
- [ ] Redis working for queues/caching
- [ ] Monitoring/logging set up (Fly.io metrics + Sentry)
- [ ] Rate limiting configured
- [ ] CORS properly configured
- [ ] API documentation published
- [ ] Beta signup form created

### Launch Day:
- [ ] Announce on Twitter/Discord/Reddit
- [ ] Send invites to beta testers (10-20 users)
- [ ] Monitor logs/errors closely
- [ ] Have support channel ready (Discord)
- [ ] Collect feedback actively

### Week 1-2:
- [ ] Fix critical bugs
- [ ] Improve UX based on feedback
- [ ] Add missing features (top requests)
- [ ] Optimize performance bottlenecks
- [ ] Expand to 50-100 users

---

## Step 6: Monitoring & Operations

### Health Checks
```bash
# Backend health
curl https://zerostate-api.fly.dev/health

# Check Fly.io status
fly status --app zerostate-api

# View logs
fly logs --app zerostate-api

# Check metrics
fly dashboard --app zerostate-api
```

### Key Metrics to Track
- User signups per day
- Agent registrations per day
- Tasks submitted per day
- Task success/failure rate
- Average task execution time
- API response times (p50, p95, p99)
- Error rates by endpoint
- Storage usage (R2)
- Database size

### Alerts to Set Up
- API down (health check fails)
- Error rate >5%
- Response time >2s
- Storage >80% full
- Database >500MB

---

## Cost Breakdown (Monthly)

| Service | Cost | Notes |
|---------|------|-------|
| Fly.io (Hobby) | $5 | 1 shared CPU, 512MB RAM |
| Cloudflare R2 | $5-10 | 500GB storage + 10TB transfer |
| Redis (Upstash) | $0-10 | 10K commands/day free, then $0.20/100K |
| Vercel (Hobby) | $0 | 100GB bandwidth free |
| Sentry (errors) | $0 | 5K events/month free |
| **Total** | **$10-25** | For 50-100 beta users |

Scale to 1000 users: $50-100/month

---

## Quick Start Commands

```bash
# 1. Set up R2 storage (after getting credentials)
fly secrets set S3_BUCKET="zerostate-agents" \
  S3_ENDPOINT="https://[account].r2.cloudflarestorage.com" \
  S3_REGION="auto" \
  AWS_ACCESS_KEY_ID="your-key" \
  AWS_SECRET_ACCESS_KEY="your-secret" \
  --app zerostate-api

# 2. Deploy frontend to Vercel
vercel login
vercel --prod

# 3. Test the flow
# (Use curl commands from Step 3 above)
```

---

## Need Help?

- **Cloudflare R2 Setup**: https://developers.cloudflare.com/r2/get-started/
- **Fly.io Secrets**: https://fly.io/docs/reference/secrets/
- **Vercel Deployment**: https://vercel.com/docs/deployments/overview
- **Stripe Integration**: https://stripe.com/docs/api

---

## Next Steps

1. **Immediate** (This Week):
   - Set up Cloudflare R2
   - Configure Fly.io secrets
   - Deploy frontend to Vercel
   - Test agent upload flow

2. **Short-term** (Next 2 Weeks):
   - Implement Stripe payment integration
   - Add monitoring/alerts
   - Create beta signup form
   - Test with 5-10 users

3. **Medium-term** (Next Month):
   - Scale to 50-100 users
   - Add advanced features
   - Optimize performance
   - Consider token launch

**Ready to start? Let's configure R2 first!**
