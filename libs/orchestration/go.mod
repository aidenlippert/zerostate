module github.com/aidenlippert/zerostate/libs/orchestration

go 1.21

require (
	github.com/google/uuid v1.6.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/stretchr/testify v1.9.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
)

replace github.com/aidenlippert/zerostate/libs/search => ../search
