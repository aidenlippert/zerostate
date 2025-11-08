# Vercel Frontend Deployment Instructions

## Step 1: Configure Vercel Project Settings

In the Vercel dashboard, configure the following settings:

### Root Directory
```
web/static
```

### Framework Preset
```
Other
```

### Build Settings
- **Build Command**: Leave empty (no build needed)
- **Output Directory**: `.` (current directory)
- **Install Command**: Leave empty (no install needed)

## Step 2: Deploy

Click **"Deploy"** button in Vercel dashboard.

Vercel will automatically:
- Pull code from GitHub
- Detect the `vercel.json` configuration
- Deploy static files from `web/static` directory
- Enable SPA routing (all routes redirect to index.html)

## Step 3: Update Backend CORS (After Deployment)

After your Vercel deployment completes, you'll get a URL like: `https://zerostate-[random].vercel.app`

Update the Fly.io backend to allow requests from your Vercel domain:

```bash
fly secrets set ALLOWED_ORIGINS="https://your-vercel-url.vercel.app,http://localhost:3000"
```

Replace `your-vercel-url.vercel.app` with your actual Vercel deployment URL.

## Step 4: Test Your Deployment

1. Visit your Vercel URL: `https://your-vercel-url.vercel.app`
2. Try signing up with a new account
3. Login with your credentials
4. Navigate to different pages (agents, tasks, dashboard)

## How It Works

### Automatic API Detection

The frontend (`web/static/js/app.js`) automatically detects the environment:

```javascript
const API_BASE_URL = window.location.hostname.includes('vercel.app')
    ? 'https://zerostate-api.fly.dev/api/v1'
    : window.location.origin + '/api/v1';
```

- **On Vercel** (*.vercel.app): Uses production API at `https://zerostate-api.fly.dev/api/v1`
- **Local development**: Uses local API at `http://localhost:8080/api/v1`

### SPA Routing

The `vercel.json` configuration includes rewrites for SPA routing:

```json
{
  "rewrites": [
    {
      "source": "/(.*)",
      "destination": "/index.html"
    }
  ]
}
```

This ensures all routes (e.g., `/agents`, `/tasks`) properly load the single-page application.

## Deployment Stack Summary

| Component | Platform | URL |
|-----------|----------|-----|
| **Backend API** | Fly.io | https://zerostate-api.fly.dev |
| **Database** | Supabase | PostgreSQL (managed) |
| **Frontend** | Vercel | https://your-app.vercel.app |

## Custom Domain (Optional)

To add a custom domain:

1. Go to Vercel dashboard → Your Project → Settings → Domains
2. Add your custom domain (e.g., `app.yourdomain.com`)
3. Update DNS records as instructed by Vercel
4. Update Fly.io CORS settings to include your custom domain:

```bash
fly secrets set ALLOWED_ORIGINS="https://app.yourdomain.com,https://your-vercel-url.vercel.app,http://localhost:3000"
```

## Troubleshooting

### CORS Errors

If you see CORS errors in the browser console:

1. Check that you've updated the backend CORS settings with your Vercel URL
2. Verify the URL doesn't have a trailing slash
3. Check browser DevTools Network tab for the actual error

### Authentication Not Working

1. Check that JWT_SECRET is set on Fly.io: `fly secrets list`
2. Verify DATABASE_URL is correct: `fly secrets list`
3. Check Fly.io logs: `fly logs`

### Frontend Not Loading

1. Check that `web/static` directory is set as Root Directory in Vercel
2. Verify all static files are committed to GitHub
3. Check Vercel build logs for any errors

## Next Steps

- Set up GitHub Actions for automatic deployments
- Add environment-specific analytics
- Configure CDN caching for static assets
- Set up monitoring and error tracking
