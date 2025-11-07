module github.com/aidenlippert/zerostate/libs/api

go 1.21

require (
	github.com/aidenlippert/zerostate/libs/execution v0.0.0
	github.com/aidenlippert/zerostate/libs/identity v0.0.0
	github.com/aidenlippert/zerostate/libs/p2p v0.0.0
	github.com/aidenlippert/zerostate/libs/search v0.0.0
	github.com/aidenlippert/zerostate/libs/telemetry v0.0.0
	github.com/gin-gonic/gin v1.10.0
	github.com/libp2p/go-libp2p v0.33.0
	github.com/prometheus/client_golang v1.19.0
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/trace v1.24.0
	go.uber.org/zap v1.27.0
	golang.org/x/time v0.5.0
)

replace (
	github.com/aidenlippert/zerostate/libs/execution => ../execution
	github.com/aidenlippert/zerostate/libs/identity => ../identity
	github.com/aidenlippert/zerostate/libs/p2p => ../p2p
	github.com/aidenlippert/zerostate/libs/search => ../search
	github.com/aidenlippert/zerostate/libs/telemetry => ../telemetry
)
