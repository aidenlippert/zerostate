module github.com/aidenlippert/zerostate/examples/agents/echo-agent

go 1.24

require (
	github.com/aidenlippert/zerostate/libs/agentsdk v0.0.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
)

replace github.com/aidenlippert/zerostate/libs/agentsdk => ../../../libs/agentsdk
