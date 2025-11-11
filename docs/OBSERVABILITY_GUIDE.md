# ZeroState Observability Guide

Comprehensive guide for production observability, monitoring, and tracing.

## Overview

ZeroState uses a three-pillar observability approach:
1. **Metrics**: Prometheus for quantitative system metrics
2. **Logs**: Structured logging with correlation IDs
3. **Traces**: OpenTelemetry with Jaeger for distributed tracing

## Distributed Tracing (P3)

### Current State

OpenTelemetry tracing middleware is implemented in [libs/api/middleware.go](../libs/api/middleware.go:137-167):
```go
func tracingMiddleware(tracer trace.Tracer) gin.HandlerFunc {
    // Traces HTTP requests with OpenTelemetry
    // Adds span context propagation
    // Records HTTP method, path, status, response size
}
```

### Jaeger Setup

#### 1. Local Development with Docker

```bash
# Start Jaeger all-in-one
docker run -d --name jaeger \
  -e COLLECTOR_ZIPKIN_HTTP_PORT=9411 \
  -p 5775:5775/udp \
  -p 6831:6831/udp \
  -p 6832:6832/udp \
  -p 5778:5778 \
  -p 16686:16686 \
  -p 14268:14268 \
  -p 14250:14250 \
  -p 9411:9411 \
  jaegertracing/all-in-one:latest

# Access Jaeger UI at http://localhost:16686
```

#### 2. Production Deployment (Kubernetes)

```yaml
# jaeger-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger
  namespace: observability
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger
  template:
    metadata:
      labels:
        app: jaeger
    spec:
      containers:
      - name: jaeger
        image: jaegertracing/all-in-one:latest
        ports:
          - containerPort: 16686  # UI
          - containerPort: 14268  # HTTP collector
          - containerPort: 14250  # gRPC collector
          - containerPort: 6831   # UDP agent
        env:
          - name: COLLECTOR_ZIPKIN_HTTP_PORT
            value: "9411"
          - name: SPAN_STORAGE_TYPE
            value: "elasticsearch"  # Use persistent storage
          - name: ES_SERVER_URLS
            value: "http://elasticsearch:9200"
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger
  namespace: observability
spec:
  type: ClusterIP
  ports:
    - name: ui
      port: 16686
      targetPort: 16686
    - name: collector-http
      port: 14268
      targetPort: 14268
    - name: collector-grpc
      port: 14250
      targetPort: 14250
    - name: agent-udp
      port: 6831
      protocol: UDP
      targetPort: 6831
  selector:
    app: jaeger
```

#### 3. Enable Tracing in ZeroState

Set environment variables:

```bash
# Enable tracing
export ENABLE_TRACING=true

# Jaeger endpoint (gRPC)
export OTEL_EXPORTER_JAEGER_ENDPOINT=http://localhost:14250

# Or use OTLP exporter
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

# Service name
export OTEL_SERVICE_NAME=zerostate-api

# Sampling rate (0.0 to 1.0)
export OTEL_TRACES_SAMPLER=parentbased_traceidratio
export OTEL_TRACES_SAMPLER_ARG=0.1  # Sample 10% of traces
```

#### 4. Application Integration

Update [cmd/api/main.go](../cmd/api/main.go) to initialize Jaeger:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Initialize Jaeger tracer
func initJaeger(serviceName string) (*trace.TracerProvider, error) {
    // Create Jaeger exporter
    exporter, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint(os.Getenv("OTEL_EXPORTER_JAEGER_ENDPOINT")),
    ))
    if err != nil {
        return nil, err
    }

    // Create tracer provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String("0.1.0"),
        )),
        trace.WithSampler(trace.TraceIDRatioBased(0.1)), // 10% sampling
    )

    // Set global tracer provider
    otel.SetTracerProvider(tp)

    // Set global propagator (for cross-service traces)
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    return tp, nil
}

// In main()
if os.Getenv("ENABLE_TRACING") == "true" {
    tp, err := initJaeger("zerostate-api")
    if err != nil {
        logger.Fatal("failed to initialize tracer", zap.Error(err))
    }
    defer func() {
        if err := tp.Shutdown(context.Background()); err != nil {
            logger.Error("error shutting down tracer", zap.Error(err))
        }
    }()
    tracer = tp.Tracer("zerostate-api")
}
```

### Adding Custom Spans

Add custom instrumentation for key operations:

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

func (h *Handlers) ExecuteTask(ctx context.Context, task *Task) error {
    tracer := otel.Tracer("zerostate-api")
    ctx, span := tracer.Start(ctx, "ExecuteTask",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.type", task.Type),
            attribute.Float64("task.budget", task.Budget),
        ),
    )
    defer span.End()

    // Execute task...
    result, err := h.executeInternal(ctx, task)
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return err
    }

    span.SetAttributes(
        attribute.String("result.status", "success"),
        attribute.Int("result.size", len(result)),
    )
    return nil
}
```

### Trace Context Propagation

Correlation IDs are already propagated via headers. Extend to include trace context:

```go
// In correlationIDMiddleware
func correlationIDMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract trace context from headers
        ctx := otel.GetTextMapPropagator().Extract(
            c.Request.Context(),
            propagation.HeaderCarrier(c.Request.Header),
        )
        c.Request = c.Request.WithContext(ctx)

        // Generate correlation IDs...
        // (existing code)

        // Inject trace context into response
        otel.GetTextMapPropagator().Inject(
            c.Request.Context(),
            propagation.HeaderCarrier(c.Writer.Header()),
        )

        c.Next()
    }
}
```

### Useful Queries in Jaeger

1. **Find slow requests**:
   - Service: `zerostate-api`
   - Operation: `HTTP GET /api/v1/tasks`
   - Min Duration: `>2s`

2. **Error traces**:
   - Service: `zerostate-api`
   - Tags: `error=true`

3. **Database queries**:
   - Service: `zerostate-api`
   - Operation: contains `database`
   - Tags: `db.statement`

4. **Cross-service traces**:
   - Service: `zerostate-api`
   - Operation: `HTTP POST /api/v1/agents`
   - View full trace to see S3, database, P2P calls

## Load Testing (P3)

### k6 Load Testing

#### Installation

```bash
# macOS
brew install k6

# Linux
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
  --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | \
  sudo tee /usr/share/keyrings/k6-archive-keyring.gpg
sudo apt-get update
sudo apt-get install k6
```

#### Basic Load Test

Create `tests/load/basic_load_test.js`:

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '30s', target: 20 },  // Ramp up to 20 users
    { duration: '1m',  target: 20 },  // Stay at 20 users
    { duration: '30s', target: 50 },  // Ramp up to 50 users
    { duration: '2m',  target: 50 },  // Stay at 50 users
    { duration: '30s', target: 100 }, // Ramp up to 100 users
    { duration: '2m',  target: 100 }, // Stay at 100 users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<2000'], // 95% < 500ms, 99% < 2s
    http_req_failed: ['rate<0.01'],  // Error rate < 1%
    errors: ['rate<0.05'],           // Custom error rate < 5%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export default function () {
  // Health check
  let healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, {
    'health status 200': (r) => r.status === 200,
    'health response time < 200ms': (r) => r.timings.duration < 200,
  }) || errorRate.add(1);

  sleep(1);

  // Metrics endpoint
  let metricsRes = http.get(`${BASE_URL}/metrics`);
  check(metricsRes, {
    'metrics status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(2);
}
```

#### Advanced Load Test with Authentication

Create `tests/load/api_load_test.js`:

```javascript
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const taskSubmissionTime = new Trend('task_submission_duration');

export const options = {
  scenarios: {
    user_registration: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 10 },
        { duration: '2m', target: 10 },
        { duration: '1m', target: 0 },
      ],
      exec: 'registerUsers',
    },
    task_submission: {
      executor: 'constant-vus',
      vus: 20,
      duration: '5m',
      exec: 'submitTasks',
    },
    agent_operations: {
      executor: 'ramping-arrival-rate',
      startRate: 5,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 100,
      stages: [
        { duration: '2m', target: 10 },
        { duration: '3m', target: 20 },
        { duration: '2m', target: 5 },
      ],
      exec: 'agentOperations',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000', 'p(99)<3000'],
    http_req_failed: ['rate<0.02'],
    errors: ['rate<0.05'],
    task_submission_duration: ['p(95)<2000'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Shared token storage
let userToken = null;

export function setup() {
  // Register a test user for authentication
  const registerRes = http.post(`${BASE_URL}/api/v1/users/register`, JSON.stringify({
    email: `loadtest-${Date.now()}@zerostate.ai`,
    password: 'testpass123',
    full_name: 'Load Test User',
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  if (registerRes.status === 201) {
    const body = JSON.parse(registerRes.body);
    return { token: body.token };
  }
  return { token: null };
}

export function registerUsers() {
  const email = `user-${Date.now()}-${__VU}@example.com`;
  const res = http.post(`${BASE_URL}/api/v1/users/register`, JSON.stringify({
    email: email,
    password: 'password123',
    full_name: `Test User ${__VU}`,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(res, {
    'register status 201': (r) => r.status === 201,
    'register has token': (r) => JSON.parse(r.body).token !== undefined,
  }) || errorRate.add(1);

  sleep(1);
}

export function submitTasks(data) {
  if (!data.token) {
    errorRate.add(1);
    return;
  }

  const payload = JSON.stringify({
    query: `Test task from VU ${__VU} iteration ${__ITER}`,
    budget: 0.10,
    timeout: 30,
    priority: 'normal',
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${data.token}`,
    },
  };

  const startTime = Date.now();
  const res = http.post(`${BASE_URL}/api/v1/tasks/submit`, payload, params);
  const duration = Date.now() - startTime;

  taskSubmissionTime.add(duration);

  check(res, {
    'task submit status 201': (r) => r.status === 201,
    'task submit has id': (r) => JSON.parse(r.body).task_id !== undefined,
  }) || errorRate.add(1);

  sleep(Math.random() * 3 + 1); // Random sleep 1-4s
}

export function agentOperations(data) {
  if (!data.token) {
    errorRate.add(1);
    return;
  }

  const params = {
    headers: {
      'Authorization': `Bearer ${data.token}`,
    },
  };

  // List agents
  const listRes = http.get(`${BASE_URL}/api/v1/agents`, params);
  check(listRes, {
    'agents list status 200': (r) => r.status === 200,
  }) || errorRate.add(1);

  sleep(2);
}

export function teardown(data) {
  console.log('Load test completed');
}
```

#### Run Load Tests

```bash
# Basic load test
k6 run tests/load/basic_load_test.js

# API load test with custom base URL
k6 run --env BASE_URL=https://zerostate-api.fly.dev tests/load/api_load_test.js

# With HTML report
k6 run --out html=report.html tests/load/api_load_test.js

# With InfluxDB + Grafana
k6 run --out influxdb=http://localhost:8086/k6 tests/load/api_load_test.js
```

### Continuous Load Testing

Set up GitHub Actions workflow:

```yaml
# .github/workflows/load-test.yml
name: Load Test

on:
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM
  workflow_dispatch:

jobs:
  load-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Install k6
        run: |
          sudo gpg -k
          sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
            --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
          echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | \
            sudo tee /usr/share/keyrings/k6-archive-keyring.gpg
          sudo apt-get update
          sudo apt-get install k6

      - name: Run load test
        env:
          BASE_URL: https://zerostate-api.fly.dev
        run: |
          k6 run --out json=results.json tests/load/api_load_test.js

      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: load-test-results
          path: results.json
```

## Security Hardening (P3)

### Current Security Features

1. **JWT Authentication** ([libs/auth](../libs/auth))
   - HMAC-SHA256 signed tokens
   - Configurable expiry (default: 24h)
   - Secure token validation

2. **Rate Limiting** ([libs/api/middleware.go:244-264](../libs/api/middleware.go#L244-L264))
   - Per-IP rate limiting
   - Configurable requests per minute
   - 429 response with retry-after header

3. **CORS** ([libs/api/middleware.go:169-199](../libs/api/middleware.go#L169-L199))
   - Configurable allowed origins
   - Credential support
   - Preflight request handling

### Recommended Security Enhancements

#### 1. Rate Limiting Improvements

**Current**: Basic per-IP rate limiting
**Enhance**: Tiered rate limiting based on authentication

```go
type RateLimitTier struct {
    Name           string
    RequestsPerMin int
    BurstSize      int
}

var rateLimitTiers = map[string]RateLimitTier{
    "anonymous": {
        Name:           "anonymous",
        RequestsPerMin: 10,
        BurstSize:      20,
    },
    "authenticated": {
        Name:           "authenticated",
        RequestsPerMin: 100,
        BurstSize:      150,
    },
    "premium": {
        Name:           "premium",
        RequestsPerMin: 1000,
        BurstSize:      1500,
    },
}

func adaptiveRateLimitMiddleware() gin.HandlerFunc {
    limiters := make(map[string]*rate.Limiter)
    mu := sync.RWMutex{}

    return func(c *gin.Context) {
        // Determine tier based on authentication
        tier := "anonymous"
        if userID, exists := c.Get("user_id"); exists && userID != "" {
            // Check user tier from database
            tier = "authenticated" // or "premium"
        }

        key := fmt.Sprintf("%s:%s", c.ClientIP(), tier)
        config := rateLimitTiers[tier]

        mu.RLock()
        limiter, exists := limiters[key]
        mu.RUnlock()

        if !exists {
            mu.Lock()
            limiter = rate.NewLimiter(
                rate.Limit(config.RequestsPerMin)/60.0,
                config.BurstSize,
            )
            limiters[key] = limiter
            mu.Unlock()
        }

        if !limiter.Allow() {
            c.JSON(http.StatusTooManyRequests, gin.H{
                "error":       "rate limit exceeded",
                "tier":        tier,
                "limit":       config.RequestsPerMin,
                "retry_after": 60,
            })
            c.Abort()
            return
        }

        c.Next()
    }
}
```

#### 2. Input Validation & Sanitization

Add comprehensive validation middleware:

```go
import "github.com/go-playground/validator/v10"

var validate = validator.New()

// ValidateJSON validates request body against struct tags
func ValidateJSON[T any](c *gin.Context) (*T, error) {
    var data T
    if err := c.ShouldBindJSON(&data); err != nil {
        return nil, fmt.Errorf("invalid JSON: %w", err)
    }

    if err := validate.Struct(&data); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    return &data, nil
}

// Example usage in handler
type TaskSubmitRequest struct {
    Query    string  `json:"query" validate:"required,min=1,max=10000"`
    Budget   float64 `json:"budget" validate:"required,gt=0,lte=1000"`
    Timeout  int     `json:"timeout" validate:"required,gte=1,lte=3600"`
    Priority string  `json:"priority" validate:"required,oneof=low normal high"`
}

func (h *Handlers) SubmitTask(c *gin.Context) {
    req, err := ValidateJSON[TaskSubmitRequest](c)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "error":   "validation error",
            "message": err.Error(),
        })
        return
    }

    // Process validated request...
}
```

#### 3. Security Headers

Add security headers middleware:

```go
func securityHeadersMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Prevent MIME type sniffing
        c.Header("X-Content-Type-Options", "nosniff")

        // Enable browser XSS protection
        c.Header("X-XSS-Protection", "1; mode=block")

        // Prevent clickjacking
        c.Header("X-Frame-Options", "DENY")

        // Enforce HTTPS
        c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

        // Control referrer information
        c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

        // Content Security Policy
        c.Header("Content-Security-Policy",
            "default-src 'self'; "+
            "script-src 'self' 'unsafe-inline' 'unsafe-eval'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data: https:; "+
            "font-src 'self'; "+
            "connect-src 'self'")

        // Permissions Policy
        c.Header("Permissions-Policy",
            "geolocation=(), microphone=(), camera=()")

        c.Next()
    }
}
```

#### 4. API Key Management

Implement API key rotation and scoping:

```go
type APIKey struct {
    ID          string    `json:"id"`
    Key         string    `json:"-"` // Never expose
    UserID      string    `json:"user_id"`
    Name        string    `json:"name"`
    Scopes      []string  `json:"scopes"`
    ExpiresAt   time.Time `json:"expires_at"`
    LastUsedAt  time.Time `json:"last_used_at"`
    CreatedAt   time.Time `json:"created_at"`
}

func apiKeyAuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        apiKey := c.GetHeader("X-API-Key")
        if apiKey == "" {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "missing API key",
            })
            c.Abort()
            return
        }

        // Validate and load API key from database
        key, err := validateAPIKey(apiKey)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "invalid API key",
            })
            c.Abort()
            return
        }

        // Check expiration
        if time.Now().After(key.ExpiresAt) {
            c.JSON(http.StatusUnauthorized, gin.H{
                "error": "API key expired",
            })
            c.Abort()
            return
        }

        // Store in context
        c.Set("api_key_id", key.ID)
        c.Set("user_id", key.UserID)
        c.Set("api_key_scopes", key.Scopes)

        // Update last used timestamp
        go updateAPIKeyLastUsed(key.ID)

        c.Next()
    }
}
```

#### 5. Audit Logging

Implement comprehensive audit logging:

```go
type AuditEvent struct {
    ID            string    `json:"id"`
    UserID        string    `json:"user_id"`
    Action        string    `json:"action"`
    Resource      string    `json:"resource"`
    ResourceID    string    `json:"resource_id"`
    IPAddress     string    `json:"ip_address"`
    UserAgent     string    `json:"user_agent"`
    CorrelationID string    `json:"correlation_id"`
    Result        string    `json:"result"` // success/failure
    Details       map[string]interface{} `json:"details"`
    Timestamp     time.Time `json:"timestamp"`
}

func auditLogMiddleware(logger *zap.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Skip health/metrics endpoints
        if strings.HasPrefix(c.Request.URL.Path, "/health") ||
           strings.HasPrefix(c.Request.URL.Path, "/metrics") {
            c.Next()
            return
        }

        start := time.Now()
        c.Next()

        // Log after request completes
        userID, _ := c.Get("user_id")
        correlationID, _ := c.Get("correlation_id")

        event := AuditEvent{
            ID:            uuid.New().String(),
            UserID:        toString(userID),
            Action:        fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
            IPAddress:     c.ClientIP(),
            UserAgent:     c.Request.UserAgent(),
            CorrelationID: toString(correlationID),
            Result:        getResult(c.Writer.Status()),
            Timestamp:     time.Now(),
        }

        logger.Info("audit_event",
            zap.String("event_id", event.ID),
            zap.String("user_id", event.UserID),
            zap.String("action", event.Action),
            zap.String("ip", event.IPAddress),
            zap.String("result", event.Result),
            zap.Duration("duration", time.Since(start)),
        )

        // Store in audit log database
        go storeAuditEvent(event)
    }
}

func getResult(statusCode int) string {
    if statusCode >= 200 && statusCode < 400 {
        return "success"
    }
    return "failure"
}
```

### Security Checklist

- [x] JWT authentication with secure signing
- [x] Per-IP rate limiting
- [x] CORS protection
- [x] Structured logging with correlation IDs
- [ ] Tiered rate limiting (authenticated vs anonymous)
- [ ] Comprehensive input validation
- [ ] Security headers (CSP, HSTS, etc.)
- [ ] API key management with scopes
- [ ] Audit logging for sensitive operations
- [ ] Regular security audits
- [ ] Dependency vulnerability scanning
- [ ] TLS/HTTPS enforcement
- [ ] Database connection encryption
- [ ] Secrets management (never commit secrets)
- [ ] OWASP Top 10 compliance

### Security Testing

Run regular security scans:

```bash
# Dependency vulnerability scanning
go get -u golang.org/x/vuln/cmd/govulncheck
govulncheck ./...

# Static analysis
go get -u golang.org/x/tools/cmd/staticcheck
staticcheck ./...

# Security-focused linting
go get github.com/securego/gosec/v2/cmd/gosec
gosec ./...
```

## Monitoring Dashboard

### Grafana Setup

Create comprehensive monitoring dashboard:

1. **System Health Panel**
   - Service uptime
   - Request rate
   - Error rate
   - P50/P95/P99 latency

2. **Resource Usage Panel**
   - CPU usage
   - Memory usage
   - Goroutine count
   - Database connection pool

3. **Business Metrics Panel**
   - Task submission rate
   - Task success/failure rate
   - Agent registration rate
   - User growth

4. **Alert Status Panel**
   - Active alerts
   - Alert history
   - Time to resolution

### Access Links

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000
- **Jaeger UI**: http://localhost:16686
- **API Metrics**: http://localhost:8080/metrics
- **API Health**: http://localhost:8080/health

## Next Steps

1. Complete Jaeger integration in production
2. Set up continuous load testing
3. Implement tiered rate limiting
4. Add comprehensive audit logging
5. Regular security audits and penetration testing

---

For questions or issues, see:
- [Prometheus docs](https://prometheus.io/docs/)
- [Jaeger docs](https://www.jaegertracing.io/docs/)
- [k6 docs](https://k6.io/docs/)
- [OWASP](https://owasp.org/)