# ZeroState Production Deployment Guide

Complete guide for deploying ZeroState to production using Supabase, Fly.io/Render, and Vercel.

## Architecture

**Database**: Supabase (PostgreSQL)
**Backend API**: Fly.io or Render
**Frontend**: Vercel

## Step 1: Set Up Supabase Database

1. Go to [supabase.com](https://supabase.com) and create a free account
2. Create a new project
3. Go to **Settings** → **Database** and copy your connection string
4. It will look like: `postgresql://postgres:[YOUR-PASSWORD]@[PROJECT-REF].supabase.co:5432/postgres`

The database tables will be created automatically when your app first connects!

## Step 2: Deploy Backend (Choose One)

### Option A: Deploy to Fly.io (Recommended - Free Tier Available)

1. Install Fly CLI:
   ```bash
   curl -L https://fly.io/install.sh | sh
   ```

2. Login to Fly:
   ```bash
   fly auth login
   ```

3. Launch your app (from project root):
   ```bash
   fly launch
   # Say 'Yes' to copy configuration from fly.toml
   # Say 'No' to deploying now
   ```

4. Set your secrets:
   ```bash
   fly secrets set DATABASE_URL="your-supabase-connection-string"
   fly secrets set JWT_SECRET="your-super-secret-key-min-32-chars"
   ```

5. Deploy:
   ```bash
   fly deploy
   ```

6. Your API will be available at: `https://your-app-name.fly.dev`

### Option B: Deploy to Render

1. Go to [render.com](https://render.com) and create an account
2. Click **New +** → **Web Service**
3. Connect your GitHub repository
4. Render will auto-detect the `render.yaml` file
5. Set environment variables in Render dashboard:
   - `DATABASE_URL`: Your Supabase connection string
   - `JWT_SECRET`: A secure random string (32+ characters)
6. Click **Create Web Service**

Your API will be available at: `https://your-app-name.onrender.com`

## Step 3: Deploy Frontend to Vercel

1. Go to [vercel.com](https://vercel.com) and create an account
2. Click **Add New** → **Project**
3. Import your GitHub repository
4. Vercel will auto-detect the configuration
5. Set the **Root Directory** to `web/static`
6. Add environment variable:
   - `API_BASE_URL`: Your backend URL from Step 2 (e.g., `https://your-app-name.fly.dev/api/v1`)
7. Click **Deploy**

Your frontend will be available at: `https://your-project.vercel.app`

## Step 4: Update CORS Settings

Update the backend's allowed origins to include your Vercel domain:

**Fly.io**:
```bash
fly secrets set ALLOWED_ORIGINS="https://your-project.vercel.app,https://your-domain.com"
```

**Render**:
Add `ALLOWED_ORIGINS` environment variable in the dashboard.

## Step 5: Test Your Deployment

1. Visit your Vercel frontend URL
2. Create an account (sign up)
3. Login with your credentials
4. Browse the agents page

## Environment Variables Reference

### Backend (Fly.io/Render)

Required:
- `DATABASE_URL`: PostgreSQL connection string from Supabase
- `JWT_SECRET`: Secret key for JWT tokens (32+ characters)

Optional:
- `PORT`: Server port (default: 8080)
- `ORCHESTRATOR_WORKERS`: Number of workers (default: 5)
- `ALLOWED_ORIGINS`: CORS origins (comma-separated)
- `RATE_LIMIT_PER_MINUTE`: API rate limit (default: 100)

### Frontend (Vercel)

- `API_BASE_URL`: Your backend API URL (e.g., `https://your-app.fly.dev/api/v1`)

## Local Development with PostgreSQL

To test with PostgreSQL locally:

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Update `DATABASE_URL` in `.env`:
   ```
   DATABASE_URL=postgresql://user:pass@localhost:5432/zerostate
   ```

3. Run the server:
   ```bash
   go run cmd/api/main.go
   ```

The app automatically detects PostgreSQL vs SQLite from the connection string!

## Monitoring & Logs

**Fly.io**:
```bash
fly logs
fly status
```

**Render**:
View logs in the Render dashboard under your service.

**Vercel**:
View deployment logs and analytics in the Vercel dashboard.

## Scaling

### Fly.io
```bash
# Scale to 2 instances
fly scale count 2

# Increase memory
fly scale memory 1024
```

### Render
Upgrade your plan in the Render dashboard to enable auto-scaling.

## Troubleshooting

### Backend won't connect to database
- Check `DATABASE_URL` is correct
- Ensure Supabase project is not paused (free tier auto-pauses after inactivity)
- Verify connection string includes `?sslmode=require`

### Frontend can't reach backend
- Verify `API_BASE_URL` is set correctly in Vercel
- Check CORS settings include your Vercel domain
- Ensure backend health check passes: `https://your-api.fly.dev/health`

### Authentication not working
- Verify `JWT_SECRET` is set and is the same across all backend instances
- Check browser console for CORS errors
- Ensure cookies/localStorage are not blocked

## Cost Estimates

**Supabase**: Free tier (500MB database, 50,000 requests/month)
**Fly.io**: Free tier (3 shared-cpu VMs, 160GB bandwidth)
**Render**: Free tier (750 hours/month)
**Vercel**: Free tier (unlimited deployments, 100GB bandwidth)

**Total Monthly Cost**: $0 for getting started!

## Next Steps

- Set up custom domain on Vercel
- Configure SSL certificates (automatic on all platforms)
- Set up monitoring and alerts
- Enable database backups in Supabase
- Implement CI/CD with GitHub Actions

Happy deploying! 🚀
