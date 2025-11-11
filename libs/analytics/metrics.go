package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MetricsService provides analytics and monitoring for economic transactions
type MetricsService struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewMetricsService creates a new metrics service instance
func NewMetricsService(db *sql.DB, logger *zap.Logger) *MetricsService {
	return &MetricsService{
		db:     db,
		logger: logger,
	}
}

// EscrowMetrics represents escrow lifecycle metrics
type EscrowMetrics struct {
	TotalCreated      int64   `json:"total_created"`
	TotalFunded       int64   `json:"total_funded"`
	TotalReleased     int64   `json:"total_released"`
	TotalRefunded     int64   `json:"total_refunded"`
	TotalDisputed     int64   `json:"total_disputed"`
	TotalCancelled    int64   `json:"total_cancelled"`
	TotalValue        float64 `json:"total_value"`
	AvgEscrowAmount   float64 `json:"avg_escrow_amount"`
	DisputeRate       float64 `json:"dispute_rate"`
	SuccessRate       float64 `json:"success_rate"`
	AvgTimeToRelease  float64 `json:"avg_time_to_release_hours"`
	AvgTimeToDispute  float64 `json:"avg_time_to_dispute_hours"`
}

// AuctionMetrics represents auction performance metrics
type AuctionMetrics struct {
	TotalAuctions     int64   `json:"total_auctions"`
	ActiveAuctions    int64   `json:"active_auctions"`
	CompletedAuctions int64   `json:"completed_auctions"`
	CancelledAuctions int64   `json:"cancelled_auctions"`
	TotalBids         int64   `json:"total_bids"`
	AvgBidsPerAuction float64 `json:"avg_bids_per_auction"`
	AvgWinningBid     float64 `json:"avg_winning_bid"`
	AvgBudget         float64 `json:"avg_budget"`
	CompletionRate    float64 `json:"completion_rate"`
}

// PaymentChannelMetrics represents payment channel utilization
type PaymentChannelMetrics struct {
	TotalChannels       int64   `json:"total_channels"`
	ActiveChannels      int64   `json:"active_channels"`
	SettledChannels     int64   `json:"settled_channels"`
	DisputedChannels    int64   `json:"disputed_channels"`
	TotalVolume         float64 `json:"total_volume"`
	AvgChannelBalance   float64 `json:"avg_channel_balance"`
	AvgSettlementAmount float64 `json:"avg_settlement_amount"`
	DisputeRate         float64 `json:"dispute_rate"`
}

// ReputationMetrics represents reputation score distributions
type ReputationMetrics struct {
	TotalAgents          int64   `json:"total_agents"`
	AvgReputationScore   float64 `json:"avg_reputation_score"`
	MedianReputation     float64 `json:"median_reputation"`
	HighReputationCount  int64   `json:"high_reputation_count"`   // score >= 80
	MediumReputationCount int64  `json:"medium_reputation_count"` // 50 <= score < 80
	LowReputationCount   int64   `json:"low_reputation_count"`    // score < 50
	AvgSuccessRate       float64 `json:"avg_success_rate"`
	AvgCompletionRate    float64 `json:"avg_completion_rate"`
}

// DelegationMetrics represents meta-orchestrator performance
type DelegationMetrics struct {
	TotalDelegations      int64   `json:"total_delegations"`
	PendingDelegations    int64   `json:"pending_delegations"`
	InProgressDelegations int64   `json:"in_progress_delegations"`
	CompletedDelegations  int64   `json:"completed_delegations"`
	FailedDelegations     int64   `json:"failed_delegations"`
	AvgAgentsPerTask      float64 `json:"avg_agents_per_task"`
	AvgBudget             float64 `json:"avg_budget"`
	CompletionRate        float64 `json:"completion_rate"`
	AvgExecutionTime      float64 `json:"avg_execution_time_hours"`
}

// DisputeMetrics represents dispute resolution statistics
type DisputeMetrics struct {
	TotalDisputes          int64   `json:"total_disputes"`
	OpenDisputes           int64   `json:"open_disputes"`
	ReviewingDisputes      int64   `json:"reviewing_disputes"`
	ResolvedDisputes       int64   `json:"resolved_disputes"`
	ClosedDisputes         int64   `json:"closed_disputes"`
	AvgResolutionTime      float64 `json:"avg_resolution_time_hours"`
	AvgEvidencePerDispute  float64 `json:"avg_evidence_per_dispute"`
	EscrowDisputeRate      float64 `json:"escrow_dispute_rate"`
	ChannelDisputeRate     float64 `json:"channel_dispute_rate"`
}

// EconomicHealthMetrics provides overall system health indicators
type EconomicHealthMetrics struct {
	TotalTransactionVolume float64 `json:"total_transaction_volume"`
	ActiveUsers            int64   `json:"active_users"`
	ActiveAgents           int64   `json:"active_agents"`
	TotalTransactions      int64   `json:"total_transactions"`
	SuccessRate            float64 `json:"success_rate"`
	DisputeRate            float64 `json:"dispute_rate"`
	AvgTransactionValue    float64 `json:"avg_transaction_value"`
	SystemUtilization      float64 `json:"system_utilization"`
}

// TimeSeriesDataPoint represents a single time-series metric point
type TimeSeriesDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// GetEscrowMetrics retrieves comprehensive escrow metrics
func (s *MetricsService) GetEscrowMetrics(ctx context.Context, startTime, endTime time.Time) (*EscrowMetrics, error) {
	metrics := &EscrowMetrics{}

	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'created') as total_created,
			COUNT(*) FILTER (WHERE status = 'funded') as total_funded,
			COUNT(*) FILTER (WHERE status = 'released') as total_released,
			COUNT(*) FILTER (WHERE status = 'refunded') as total_refunded,
			COUNT(*) FILTER (WHERE status = 'disputed') as total_disputed,
			COUNT(*) FILTER (WHERE status = 'cancelled') as total_cancelled,
			COALESCE(SUM(amount), 0) as total_value,
			COALESCE(AVG(amount), 0) as avg_escrow_amount,
			COALESCE(
				COUNT(*) FILTER (WHERE status = 'disputed')::float /
				NULLIF(COUNT(*) FILTER (WHERE status IN ('funded', 'released', 'disputed')), 0),
				0
			) as dispute_rate,
			COALESCE(
				COUNT(*) FILTER (WHERE status = 'released')::float /
				NULLIF(COUNT(*) FILTER (WHERE status IN ('released', 'refunded', 'cancelled')), 0),
				0
			) as success_rate,
			COALESCE(
				EXTRACT(EPOCH FROM AVG(released_at - funded_at)) / 3600,
				0
			) as avg_time_to_release,
			COALESCE(
				EXTRACT(EPOCH FROM AVG(updated_at - funded_at)) / 3600,
				0
			) as avg_time_to_dispute
		FROM escrows
		WHERE created_at BETWEEN $1 AND $2
	`

	err := s.db.QueryRowContext(ctx, query, startTime, endTime).Scan(
		&metrics.TotalCreated,
		&metrics.TotalFunded,
		&metrics.TotalReleased,
		&metrics.TotalRefunded,
		&metrics.TotalDisputed,
		&metrics.TotalCancelled,
		&metrics.TotalValue,
		&metrics.AvgEscrowAmount,
		&metrics.DisputeRate,
		&metrics.SuccessRate,
		&metrics.AvgTimeToRelease,
		&metrics.AvgTimeToDispute,
	)

	if err != nil {
		s.logger.Error("failed to get escrow metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get escrow metrics: %w", err)
	}

	return metrics, nil
}

// GetAuctionMetrics retrieves auction performance metrics
func (s *MetricsService) GetAuctionMetrics(ctx context.Context, startTime, endTime time.Time) (*AuctionMetrics, error) {
	metrics := &AuctionMetrics{}

	query := `
		SELECT
			COUNT(*) as total_auctions,
			COUNT(*) FILTER (WHERE status = 'active') as active_auctions,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_auctions,
			COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled_auctions,
			COALESCE(SUM(bid_count), 0) as total_bids,
			COALESCE(AVG(bid_count), 0) as avg_bids_per_auction,
			COALESCE(AVG(winning_bid), 0) as avg_winning_bid,
			COALESCE(AVG(budget), 0) as avg_budget,
			COALESCE(
				COUNT(*) FILTER (WHERE status = 'completed')::float /
				NULLIF(COUNT(*) FILTER (WHERE status IN ('completed', 'cancelled')), 0),
				0
			) as completion_rate
		FROM auctions
		WHERE created_at BETWEEN $1 AND $2
	`

	err := s.db.QueryRowContext(ctx, query, startTime, endTime).Scan(
		&metrics.TotalAuctions,
		&metrics.ActiveAuctions,
		&metrics.CompletedAuctions,
		&metrics.CancelledAuctions,
		&metrics.TotalBids,
		&metrics.AvgBidsPerAuction,
		&metrics.AvgWinningBid,
		&metrics.AvgBudget,
		&metrics.CompletionRate,
	)

	if err != nil {
		s.logger.Error("failed to get auction metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get auction metrics: %w", err)
	}

	return metrics, nil
}

// GetPaymentChannelMetrics retrieves payment channel utilization metrics
func (s *MetricsService) GetPaymentChannelMetrics(ctx context.Context, startTime, endTime time.Time) (*PaymentChannelMetrics, error) {
	metrics := &PaymentChannelMetrics{}

	query := `
		SELECT
			COUNT(*) as total_channels,
			COUNT(*) FILTER (WHERE status = 'open') as active_channels,
			COUNT(*) FILTER (WHERE status = 'settled') as settled_channels,
			COUNT(*) FILTER (WHERE status = 'disputed') as disputed_channels,
			COALESCE(SUM(balance), 0) as total_volume,
			COALESCE(AVG(balance), 0) as avg_channel_balance,
			COALESCE(AVG(settlement_amount), 0) as avg_settlement_amount,
			COALESCE(
				COUNT(*) FILTER (WHERE status = 'disputed')::float /
				NULLIF(COUNT(*), 0),
				0
			) as dispute_rate
		FROM payment_channels
		WHERE created_at BETWEEN $1 AND $2
	`

	err := s.db.QueryRowContext(ctx, query, startTime, endTime).Scan(
		&metrics.TotalChannels,
		&metrics.ActiveChannels,
		&metrics.SettledChannels,
		&metrics.DisputedChannels,
		&metrics.TotalVolume,
		&metrics.AvgChannelBalance,
		&metrics.AvgSettlementAmount,
		&metrics.DisputeRate,
	)

	if err != nil {
		s.logger.Error("failed to get payment channel metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get payment channel metrics: %w", err)
	}

	return metrics, nil
}

// GetReputationMetrics retrieves reputation score distributions
func (s *MetricsService) GetReputationMetrics(ctx context.Context) (*ReputationMetrics, error) {
	metrics := &ReputationMetrics{}

	query := `
		SELECT
			COUNT(*) as total_agents,
			COALESCE(AVG(score), 0) as avg_reputation_score,
			COALESCE(PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY score), 0) as median_reputation,
			COUNT(*) FILTER (WHERE score >= 80) as high_reputation_count,
			COUNT(*) FILTER (WHERE score >= 50 AND score < 80) as medium_reputation_count,
			COUNT(*) FILTER (WHERE score < 50) as low_reputation_count,
			COALESCE(AVG(success_rate), 0) as avg_success_rate,
			COALESCE(AVG(completion_rate), 0) as avg_completion_rate
		FROM agent_reputation
	`

	err := s.db.QueryRowContext(ctx, query).Scan(
		&metrics.TotalAgents,
		&metrics.AvgReputationScore,
		&metrics.MedianReputation,
		&metrics.HighReputationCount,
		&metrics.MediumReputationCount,
		&metrics.LowReputationCount,
		&metrics.AvgSuccessRate,
		&metrics.AvgCompletionRate,
	)

	if err != nil {
		s.logger.Error("failed to get reputation metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get reputation metrics: %w", err)
	}

	return metrics, nil
}

// GetDelegationMetrics retrieves meta-orchestrator performance metrics
func (s *MetricsService) GetDelegationMetrics(ctx context.Context, startTime, endTime time.Time) (*DelegationMetrics, error) {
	metrics := &DelegationMetrics{}

	query := `
		SELECT
			COUNT(*) as total_delegations,
			COUNT(*) FILTER (WHERE status = 'pending') as pending_delegations,
			COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress_delegations,
			COUNT(*) FILTER (WHERE status = 'completed') as completed_delegations,
			COUNT(*) FILTER (WHERE status = 'failed') as failed_delegations,
			COALESCE(AVG(agents_count), 0) as avg_agents_per_task,
			COALESCE(AVG(budget), 0) as avg_budget,
			COALESCE(
				COUNT(*) FILTER (WHERE status = 'completed')::float /
				NULLIF(COUNT(*) FILTER (WHERE status IN ('completed', 'failed')), 0),
				0
			) as completion_rate,
			COALESCE(
				EXTRACT(EPOCH FROM AVG(actual_completion - created_at)) / 3600,
				0
			) as avg_execution_time
		FROM delegations
		WHERE created_at BETWEEN $1 AND $2
	`

	err := s.db.QueryRowContext(ctx, query, startTime, endTime).Scan(
		&metrics.TotalDelegations,
		&metrics.PendingDelegations,
		&metrics.InProgressDelegations,
		&metrics.CompletedDelegations,
		&metrics.FailedDelegations,
		&metrics.AvgAgentsPerTask,
		&metrics.AvgBudget,
		&metrics.CompletionRate,
		&metrics.AvgExecutionTime,
	)

	if err != nil {
		s.logger.Error("failed to get delegation metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get delegation metrics: %w", err)
	}

	return metrics, nil
}

// GetDisputeMetrics retrieves dispute resolution statistics
func (s *MetricsService) GetDisputeMetrics(ctx context.Context, startTime, endTime time.Time) (*DisputeMetrics, error) {
	metrics := &DisputeMetrics{}

	// First get dispute counts and resolution time
	disputeQuery := `
		SELECT
			COUNT(*) as total_disputes,
			COUNT(*) FILTER (WHERE status = 'open') as open_disputes,
			COUNT(*) FILTER (WHERE status = 'reviewing') as reviewing_disputes,
			COUNT(*) FILTER (WHERE status = 'resolved') as resolved_disputes,
			COUNT(*) FILTER (WHERE status = 'closed') as closed_disputes,
			COALESCE(
				EXTRACT(EPOCH FROM AVG(resolved_at - created_at)) / 3600,
				0
			) as avg_resolution_time
		FROM disputes
		WHERE created_at BETWEEN $1 AND $2
	`

	err := s.db.QueryRowContext(ctx, disputeQuery, startTime, endTime).Scan(
		&metrics.TotalDisputes,
		&metrics.OpenDisputes,
		&metrics.ReviewingDisputes,
		&metrics.ResolvedDisputes,
		&metrics.ClosedDisputes,
		&metrics.AvgResolutionTime,
	)

	if err != nil {
		s.logger.Error("failed to get dispute metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get dispute metrics: %w", err)
	}

	// Get average evidence per dispute
	evidenceQuery := `
		SELECT COALESCE(AVG(evidence_count), 0)
		FROM (
			SELECT COUNT(*) as evidence_count
			FROM dispute_evidence
			WHERE dispute_id IN (
				SELECT id FROM disputes
				WHERE created_at BETWEEN $1 AND $2
			)
			GROUP BY dispute_id
		) evidence_counts
	`

	err = s.db.QueryRowContext(ctx, evidenceQuery, startTime, endTime).Scan(&metrics.AvgEvidencePerDispute)
	if err != nil {
		s.logger.Error("failed to get evidence metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get evidence metrics: %w", err)
	}

	// Get escrow dispute rate
	escrowDisputeQuery := `
		SELECT COALESCE(
			COUNT(*) FILTER (WHERE status = 'disputed')::float /
			NULLIF(COUNT(*), 0),
			0
		)
		FROM escrows
		WHERE created_at BETWEEN $1 AND $2
	`

	err = s.db.QueryRowContext(ctx, escrowDisputeQuery, startTime, endTime).Scan(&metrics.EscrowDisputeRate)
	if err != nil {
		s.logger.Error("failed to get escrow dispute rate", zap.Error(err))
		return nil, fmt.Errorf("failed to get escrow dispute rate: %w", err)
	}

	// Get payment channel dispute rate
	channelDisputeQuery := `
		SELECT COALESCE(
			COUNT(*) FILTER (WHERE status = 'disputed')::float /
			NULLIF(COUNT(*), 0),
			0
		)
		FROM payment_channels
		WHERE created_at BETWEEN $1 AND $2
	`

	err = s.db.QueryRowContext(ctx, channelDisputeQuery, startTime, endTime).Scan(&metrics.ChannelDisputeRate)
	if err != nil {
		s.logger.Error("failed to get channel dispute rate", zap.Error(err))
		return nil, fmt.Errorf("failed to get channel dispute rate: %w", err)
	}

	return metrics, nil
}

// GetEconomicHealthMetrics provides overall system health indicators
func (s *MetricsService) GetEconomicHealthMetrics(ctx context.Context, startTime, endTime time.Time) (*EconomicHealthMetrics, error) {
	metrics := &EconomicHealthMetrics{}

	// Get transaction volume from multiple sources
	volumeQuery := `
		SELECT COALESCE(
			(SELECT SUM(amount) FROM escrows WHERE created_at BETWEEN $1 AND $2) +
			(SELECT SUM(balance) FROM payment_channels WHERE created_at BETWEEN $1 AND $2) +
			(SELECT SUM(budget) FROM auctions WHERE created_at BETWEEN $1 AND $2),
			0
		) as total_volume
	`

	err := s.db.QueryRowContext(ctx, volumeQuery, startTime, endTime).Scan(&metrics.TotalTransactionVolume)
	if err != nil {
		s.logger.Error("failed to get transaction volume", zap.Error(err))
		return nil, fmt.Errorf("failed to get transaction volume: %w", err)
	}

	// Get active users and agents
	activityQuery := `
		SELECT
			(SELECT COUNT(DISTINCT payer_id) FROM escrows WHERE created_at BETWEEN $1 AND $2) +
			(SELECT COUNT(DISTINCT user_id) FROM delegations WHERE created_at BETWEEN $1 AND $2) as active_users,
			(SELECT COUNT(*) FROM agent_reputation WHERE updated_at BETWEEN $1 AND $2) as active_agents
	`

	err = s.db.QueryRowContext(ctx, activityQuery, startTime, endTime).Scan(&metrics.ActiveUsers, &metrics.ActiveAgents)
	if err != nil {
		s.logger.Error("failed to get activity metrics", zap.Error(err))
		return nil, fmt.Errorf("failed to get activity metrics: %w", err)
	}

	// Get transaction counts
	countQuery := `
		SELECT
			(SELECT COUNT(*) FROM escrows WHERE created_at BETWEEN $1 AND $2) +
			(SELECT COUNT(*) FROM payment_channels WHERE created_at BETWEEN $1 AND $2) +
			(SELECT COUNT(*) FROM auctions WHERE created_at BETWEEN $1 AND $2) as total_transactions
	`

	err = s.db.QueryRowContext(ctx, countQuery, startTime, endTime).Scan(&metrics.TotalTransactions)
	if err != nil {
		s.logger.Error("failed to get transaction counts", zap.Error(err))
		return nil, fmt.Errorf("failed to get transaction counts: %w", err)
	}

	// Calculate derived metrics
	if metrics.TotalTransactions > 0 {
		metrics.AvgTransactionValue = metrics.TotalTransactionVolume / float64(metrics.TotalTransactions)
	}

	// Get success rate (completed escrows + settled channels / total)
	successQuery := `
		SELECT COALESCE(
			(
				(SELECT COUNT(*) FROM escrows WHERE status = 'released' AND created_at BETWEEN $1 AND $2) +
				(SELECT COUNT(*) FROM payment_channels WHERE status = 'settled' AND created_at BETWEEN $1 AND $2)
			)::float /
			NULLIF(
				(SELECT COUNT(*) FROM escrows WHERE created_at BETWEEN $1 AND $2) +
				(SELECT COUNT(*) FROM payment_channels WHERE created_at BETWEEN $1 AND $2),
				0
			),
			0
		) as success_rate
	`

	err = s.db.QueryRowContext(ctx, successQuery, startTime, endTime).Scan(&metrics.SuccessRate)
	if err != nil {
		s.logger.Error("failed to get success rate", zap.Error(err))
		return nil, fmt.Errorf("failed to get success rate: %w", err)
	}

	// Get overall dispute rate
	disputeQuery := `
		SELECT COALESCE(
			(SELECT COUNT(*) FROM disputes WHERE created_at BETWEEN $1 AND $2)::float /
			NULLIF(
				(SELECT COUNT(*) FROM escrows WHERE created_at BETWEEN $1 AND $2) +
				(SELECT COUNT(*) FROM payment_channels WHERE created_at BETWEEN $1 AND $2),
				0
			),
			0
		) as dispute_rate
	`

	err = s.db.QueryRowContext(ctx, disputeQuery, startTime, endTime).Scan(&metrics.DisputeRate)
	if err != nil {
		s.logger.Error("failed to get dispute rate", zap.Error(err))
		return nil, fmt.Errorf("failed to get dispute rate: %w", err)
	}

	// Calculate system utilization (active agents / total agents)
	utilizationQuery := `
		SELECT COALESCE(
			(SELECT COUNT(*) FROM agent_reputation WHERE updated_at BETWEEN $1 AND $2)::float /
			NULLIF((SELECT COUNT(*) FROM agent_reputation), 0),
			0
		) as system_utilization
	`

	err = s.db.QueryRowContext(ctx, utilizationQuery, startTime, endTime).Scan(&metrics.SystemUtilization)
	if err != nil {
		s.logger.Error("failed to get system utilization", zap.Error(err))
		return nil, fmt.Errorf("failed to get system utilization: %w", err)
	}

	return metrics, nil
}

// GetTimeSeriesData retrieves time-series data for a specific metric
func (s *MetricsService) GetTimeSeriesData(
	ctx context.Context,
	metricType string,
	startTime, endTime time.Time,
	intervalMinutes int,
) ([]TimeSeriesDataPoint, error) {
	var query string
	var dataPoints []TimeSeriesDataPoint

	switch metricType {
	case "escrow_volume":
		query = `
			SELECT
				date_trunc('hour', created_at) +
				(EXTRACT(minute FROM created_at)::int / $3) * interval '1 minute' * $3 as bucket,
				SUM(amount) as value
			FROM escrows
			WHERE created_at BETWEEN $1 AND $2
			GROUP BY bucket
			ORDER BY bucket
		`
	case "auction_count":
		query = `
			SELECT
				date_trunc('hour', created_at) +
				(EXTRACT(minute FROM created_at)::int / $3) * interval '1 minute' * $3 as bucket,
				COUNT(*) as value
			FROM auctions
			WHERE created_at BETWEEN $1 AND $2
			GROUP BY bucket
			ORDER BY bucket
		`
	case "dispute_rate":
		query = `
			SELECT
				date_trunc('hour', d.created_at) +
				(EXTRACT(minute FROM d.created_at)::int / $3) * interval '1 minute' * $3 as bucket,
				COUNT(*)::float / NULLIF(
					(SELECT COUNT(*) FROM escrows e WHERE e.created_at BETWEEN $1 AND $2),
					0
				) as value
			FROM disputes d
			WHERE d.created_at BETWEEN $1 AND $2
			GROUP BY bucket
			ORDER BY bucket
		`
	case "transaction_volume":
		query = `
			SELECT
				bucket,
				SUM(value) as value
			FROM (
				SELECT
					date_trunc('hour', created_at) +
					(EXTRACT(minute FROM created_at)::int / $3) * interval '1 minute' * $3 as bucket,
					amount as value
				FROM escrows
				WHERE created_at BETWEEN $1 AND $2
				UNION ALL
				SELECT
					date_trunc('hour', created_at) +
					(EXTRACT(minute FROM created_at)::int / $3) * interval '1 minute' * $3 as bucket,
					balance as value
				FROM payment_channels
				WHERE created_at BETWEEN $1 AND $2
			) combined
			GROUP BY bucket
			ORDER BY bucket
		`
	default:
		return nil, fmt.Errorf("unknown metric type: %s", metricType)
	}

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime, intervalMinutes)
	if err != nil {
		s.logger.Error("failed to get time series data", zap.String("metric", metricType), zap.Error(err))
		return nil, fmt.Errorf("failed to get time series data: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var dp TimeSeriesDataPoint
		if err := rows.Scan(&dp.Timestamp, &dp.Value); err != nil {
			s.logger.Error("failed to scan time series data point", zap.Error(err))
			return nil, fmt.Errorf("failed to scan time series data point: %w", err)
		}
		dataPoints = append(dataPoints, dp)
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("error iterating time series data", zap.Error(err))
		return nil, fmt.Errorf("error iterating time series data: %w", err)
	}

	return dataPoints, nil
}

// Anomaly represents a detected anomaly in the system
type Anomaly struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	DetectedAt  time.Time `json:"detected_at"`
}

// DetectAnomalies identifies unusual patterns in economic transactions
func (s *MetricsService) DetectAnomalies(ctx context.Context, lookbackHours int) ([]Anomaly, error) {
	var anomalies []Anomaly
	now := time.Now()
	startTime := now.Add(-time.Duration(lookbackHours) * time.Hour)

	// Check for unusually high dispute rate
	var disputeRate float64
	disputeQuery := `
		SELECT COALESCE(
			COUNT(*) FILTER (WHERE status = 'disputed')::float /
			NULLIF(COUNT(*), 0),
			0
		)
		FROM escrows
		WHERE created_at >= $1
	`
	err := s.db.QueryRowContext(ctx, disputeQuery, startTime).Scan(&disputeRate)
	if err == nil && disputeRate > 0.15 { // Threshold: 15%
		anomalies = append(anomalies, Anomaly{
			ID:          uuid.New(),
			Type:        "high_dispute_rate",
			Severity:    "high",
			Description: fmt.Sprintf("Dispute rate is %.1f%%, exceeding threshold of 15%%", disputeRate*100),
			Value:       disputeRate,
			Threshold:   0.15,
			DetectedAt:  now,
		})
	}

	// Check for unusually high escrow failure rate
	var failureRate float64
	failureQuery := `
		SELECT COALESCE(
			COUNT(*) FILTER (WHERE status IN ('refunded', 'cancelled'))::float /
			NULLIF(COUNT(*), 0),
			0
		)
		FROM escrows
		WHERE created_at >= $1
	`
	err = s.db.QueryRowContext(ctx, failureQuery, startTime).Scan(&failureRate)
	if err == nil && failureRate > 0.25 { // Threshold: 25%
		anomalies = append(anomalies, Anomaly{
			ID:          uuid.New(),
			Type:        "high_failure_rate",
			Severity:    "medium",
			Description: fmt.Sprintf("Escrow failure rate is %.1f%%, exceeding threshold of 25%%", failureRate*100),
			Value:       failureRate,
			Threshold:   0.25,
			DetectedAt:  now,
		})
	}

	// Check for low auction completion rate
	var completionRate float64
	auctionQuery := `
		SELECT COALESCE(
			COUNT(*) FILTER (WHERE status = 'completed')::float /
			NULLIF(COUNT(*) FILTER (WHERE status IN ('completed', 'cancelled')), 0),
			0
		)
		FROM auctions
		WHERE created_at >= $1
	`
	err = s.db.QueryRowContext(ctx, auctionQuery, startTime).Scan(&completionRate)
	if err == nil && completionRate < 0.60 { // Threshold: 60%
		anomalies = append(anomalies, Anomaly{
			ID:          uuid.New(),
			Type:        "low_auction_completion",
			Severity:    "medium",
			Description: fmt.Sprintf("Auction completion rate is %.1f%%, below threshold of 60%%", completionRate*100),
			Value:       completionRate,
			Threshold:   0.60,
			DetectedAt:  now,
		})
	}

	// Check for unusually long dispute resolution times
	var avgResolutionHours float64
	resolutionQuery := `
		SELECT COALESCE(
			EXTRACT(EPOCH FROM AVG(resolved_at - created_at)) / 3600,
			0
		)
		FROM disputes
		WHERE created_at >= $1 AND resolved_at IS NOT NULL
	`
	err = s.db.QueryRowContext(ctx, resolutionQuery, startTime).Scan(&avgResolutionHours)
	if err == nil && avgResolutionHours > 48 { // Threshold: 48 hours
		anomalies = append(anomalies, Anomaly{
			ID:          uuid.New(),
			Type:        "slow_dispute_resolution",
			Severity:    "high",
			Description: fmt.Sprintf("Average dispute resolution time is %.1f hours, exceeding threshold of 48 hours", avgResolutionHours),
			Value:       avgResolutionHours,
			Threshold:   48.0,
			DetectedAt:  now,
		})
	}

	return anomalies, nil
}
