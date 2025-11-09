module github.com/aidenlippert/zerostate/libs/execution

go 1.24.10

require (
	github.com/aidenlippert/zerostate/libs/metrics v0.0.0
	github.com/aidenlippert/zerostate/libs/telemetry v0.0.0
	github.com/libp2p/go-libp2p/core v0.43.0-rc2
	github.com/prometheus/client_golang v1.23.2
	github.com/tetratelabs/wazero v1.9.0
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/trace v1.38.0
	go.uber.org/zap v1.27.0
)

replace github.com/aidenlippert/zerostate/libs/metrics => ../metrics
replace github.com/aidenlippert/zerostate/libs/telemetry => ../telemetry
