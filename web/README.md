# ZeroState Web UI

Modern web interface for the ZeroState AI orchestration platform with "Aether Gradient" theme.

## Features

- **Dashboard**: Real-time metrics and orchestrator health monitoring
- **Task Management**: Submit, monitor, and manage tasks
- **Agent Registry**: View and manage registered agents
- **Performance Metrics**: Comprehensive orchestrator performance dashboard
- **Responsive Design**: Mobile-first design with Tailwind CSS
- **Client-Side Routing**: SPA-style navigation without page reloads

## Design System

### Aether Gradient Theme
- **Primary Colors**: Cyan (#00FFFF) → Magenta (#FF00FF) → Purple (#8A2BE2)
- **Background**: Dark mode (#0A0E1A, #110f23, #1A1C2A)
- **Typography**: Space Grotesk font family
- **Icons**: Material Symbols Outlined

### Status Colors
- **Success**: #00FF85 (green)
- **Running**: #2472F2 (blue)
- **Pending**: #FFD700 (gold)
- **Failed**: #FF0055 (red)
- **Canceled**: #958ecc (purple-gray)

## Architecture

### Frontend Stack
- **HTML5**: Semantic markup
- **Tailwind CSS**: Utility-first styling (via CDN)
- **Vanilla JavaScript**: No framework dependencies
- **Material Icons**: Icon system

### API Integration
- Base URL: `/api/v1`
- RESTful endpoints
- JSON request/response format
- Real-time status polling

### File Structure
```
web/
├── static/
│   ├── index.html       # Main SPA shell
│   └── js/
│       └── app.js       # Application logic and routing
└── README.md
```

## Pages

### 1. Dashboard (`/`)
- Orchestrator metrics (tasks processed, success rate, active workers)
- Health status
- Recent tasks list

### 2. Task Submission (`/submit-task`)
- Query input
- Priority selector (Low, Normal, High, Critical)
- Budget configuration
- Timeout settings

### 3. Task List (`/tasks`)
- Filterable task table
- Status badges
- Click to view details modal

### 4. Task Details Modal
- Progress bar
- Task metadata (priority, budget, timeout, created date)
- Input/output display
- Agent assignment
- Result visualization
- Cancel task action

### 5. Metrics Dashboard (`/metrics`)
- Tasks processed, succeeded, failed
- Success rate
- Average execution time
- Active workers
- Tasks timed out

## API Endpoints Used

### Tasks
- `POST /api/v1/tasks/submit` - Submit new task
- `GET /api/v1/tasks` - List tasks (with filters)
- `GET /api/v1/tasks/:id` - Get task details
- `GET /api/v1/tasks/:id/status` - Get task status
- `GET /api/v1/tasks/:id/result` - Get task result
- `DELETE /api/v1/tasks/:id` - Cancel task

### Agents
- `GET /api/v1/agents` - List agents
- `POST /api/v1/agents/register` - Register agent

### Orchestrator
- `GET /api/v1/orchestrator/metrics` - Get metrics
- `GET /api/v1/orchestrator/health` - Get health status

## Usage

### Starting the Server

The web UI is served automatically by the API server:

```bash
# From project root
go run cmd/api/main.go
```

Then navigate to: `http://localhost:8080`

### Development

To modify the UI:

1. Edit `web/static/index.html` for layout changes
2. Edit `web/static/js/app.js` for functionality
3. Refresh browser to see changes (no build step required)

### Adding New Pages

1. Add route in `router.routes` object
2. Create component function in `components` object
3. Add navigation link in header
4. Add route mapping in `router.init()`

## Features

### Real-Time Updates
The dashboard automatically refreshes task status. For production use, consider implementing WebSocket support for live updates.

### Error Handling
All API calls include error handling with user-friendly messages via `alert()`. For production, consider implementing a toast notification system.

### Responsive Design
The UI adapts to mobile, tablet, and desktop screen sizes using Tailwind's responsive utilities.

## Browser Support

- Chrome/Edge: Latest 2 versions
- Firefox: Latest 2 versions
- Safari: Latest 2 versions

## Future Enhancements

- [ ] WebSocket support for real-time updates
- [ ] Advanced filtering and search
- [ ] Data visualization charts (Chart.js integration)
- [ ] User authentication UI
- [ ] Agent registration form
- [ ] Task result export (JSON/CSV)
- [ ] Dark/light mode toggle
- [ ] Toast notification system
- [ ] Pagination for task list
- [ ] Task templates

## License

MIT
