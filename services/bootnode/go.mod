module github.com/zerostate/services/bootnode

go 1.21

replace github.com/zerostate/libs/p2p => ../../libs/p2p

require (
	github.com/zerostate/libs/p2p v0.0.0
	go.uber.org/zap v1.26.0
)

require github.com/prometheus/client_golang v1.16.0 // indirect
