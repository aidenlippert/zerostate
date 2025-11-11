package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/aidenlippert/zerostate/libs/analytics"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Analytics Handlers

// GetEscrowMetrics retrieves comprehensive escrow lifecycle metrics
func (h *Handlers) GetEscrowMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetEscrowMetrics"))

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	metrics, err := metricsSvc.GetEscrowMetrics(c.Request.Context(), startTime, endTime)
	if err != nil {
		logger.Error("failed to get escrow metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get escrow metrics", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":    metrics,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	})
}

// GetAuctionMetrics retrieves auction performance metrics
func (h *Handlers) GetAuctionMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetAuctionMetrics"))

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	metrics, err := metricsSvc.GetAuctionMetrics(c.Request.Context(), startTime, endTime)
	if err != nil {
		logger.Error("failed to get auction metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get auction metrics", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":    metrics,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	})
}

// GetPaymentChannelMetrics retrieves payment channel utilization metrics
func (h *Handlers) GetPaymentChannelMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetPaymentChannelMetrics"))

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	metrics, err := metricsSvc.GetPaymentChannelMetrics(c.Request.Context(), startTime, endTime)
	if err != nil {
		logger.Error("failed to get payment channel metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment channel metrics", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":    metrics,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	})
}

// GetReputationMetrics retrieves reputation score distributions
func (h *Handlers) GetReputationMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetReputationMetrics"))

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	metrics, err := metricsSvc.GetReputationMetrics(c.Request.Context())
	if err != nil {
		logger.Error("failed to get reputation metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get reputation metrics", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics": metrics,
	})
}

// GetDelegationMetrics retrieves meta-orchestrator performance metrics
func (h *Handlers) GetDelegationMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetDelegationMetrics"))

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	metrics, err := metricsSvc.GetDelegationMetrics(c.Request.Context(), startTime, endTime)
	if err != nil {
		logger.Error("failed to get delegation metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get delegation metrics", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":    metrics,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	})
}

// GetDisputeMetrics retrieves dispute resolution statistics
func (h *Handlers) GetDisputeMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetDisputeMetrics"))

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	metrics, err := metricsSvc.GetDisputeMetrics(c.Request.Context(), startTime, endTime)
	if err != nil {
		logger.Error("failed to get dispute metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get dispute metrics", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":    metrics,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	})
}

// GetEconomicHealthMetrics provides overall system health indicators
func (h *Handlers) GetEconomicHealthMetrics(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetEconomicHealthMetrics"))

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	metrics, err := metricsSvc.GetEconomicHealthMetrics(c.Request.Context(), startTime, endTime)
	if err != nil {
		logger.Error("failed to get economic health metrics", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get economic health metrics", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metrics":    metrics,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
	})
}

// GetTimeSeriesData retrieves time-series data for charts
func (h *Handlers) GetTimeSeriesData(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetTimeSeriesData"))

	// Required metric type
	metricType := c.Query("metric")
	if metricType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric parameter is required"})
		return
	}

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	// Parse interval (default: 15 minutes)
	intervalMinutes := 15
	if intervalStr := c.Query("interval"); intervalStr != "" {
		if parsed, err := strconv.Atoi(intervalStr); err == nil && parsed > 0 {
			intervalMinutes = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	dataPoints, err := metricsSvc.GetTimeSeriesData(
		c.Request.Context(),
		metricType,
		startTime,
		endTime,
		intervalMinutes,
	)
	if err != nil {
		logger.Error("failed to get time series data", zap.String("metric", metricType), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get time series data", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"metric":     metricType,
		"start_time": startTime.Format(time.RFC3339),
		"end_time":   endTime.Format(time.RFC3339),
		"interval":   intervalMinutes,
		"data":       dataPoints,
	})
}

// DetectAnomalies identifies unusual patterns in economic transactions
func (h *Handlers) DetectAnomalies(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "DetectAnomalies"))

	// Parse lookback hours (default: 24)
	lookbackHours := 24
	if lookbackStr := c.Query("lookback_hours"); lookbackStr != "" {
		if parsed, err := strconv.Atoi(lookbackStr); err == nil && parsed > 0 {
			lookbackHours = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)
	anomalies, err := metricsSvc.DetectAnomalies(c.Request.Context(), lookbackHours)
	if err != nil {
		logger.Error("failed to detect anomalies", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to detect anomalies", "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"lookback_hours": lookbackHours,
		"anomalies":      anomalies,
		"count":          len(anomalies),
		"detected_at":    time.Now().Format(time.RFC3339),
	})
}

// GetAnalyticsDashboard provides a comprehensive analytics overview
func (h *Handlers) GetAnalyticsDashboard(c *gin.Context) {
	logger := h.logger.With(zap.String("handler", "GetAnalyticsDashboard"))

	// Parse time range (default: last 24 hours)
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startStr := c.Query("start_time"); startStr != "" {
		if parsed, err := time.Parse(time.RFC3339, startStr); err == nil {
			startTime = parsed
		}
	}

	if endStr := c.Query("end_time"); endStr != "" {
		if parsed, err := time.Parse(time.RFC3339, endStr); err == nil {
			endTime = parsed
		}
	}

	metricsSvc := analytics.NewMetricsService(h.db.Conn(), h.logger)

	// Fetch all metrics in parallel
	type result struct {
		escrow        *analytics.EscrowMetrics
		auction       *analytics.AuctionMetrics
		channel       *analytics.PaymentChannelMetrics
		reputation    *analytics.ReputationMetrics
		delegation    *analytics.DelegationMetrics
		dispute       *analytics.DisputeMetrics
		economicHealth *analytics.EconomicHealthMetrics
		err           error
	}

	resChan := make(chan result, 1)

	go func() {
		res := result{}

		// Get all metrics
		res.escrow, res.err = metricsSvc.GetEscrowMetrics(c.Request.Context(), startTime, endTime)
		if res.err != nil {
			resChan <- res
			return
		}

		res.auction, res.err = metricsSvc.GetAuctionMetrics(c.Request.Context(), startTime, endTime)
		if res.err != nil {
			resChan <- res
			return
		}

		res.channel, res.err = metricsSvc.GetPaymentChannelMetrics(c.Request.Context(), startTime, endTime)
		if res.err != nil {
			resChan <- res
			return
		}

		res.reputation, res.err = metricsSvc.GetReputationMetrics(c.Request.Context())
		if res.err != nil {
			resChan <- res
			return
		}

		res.delegation, res.err = metricsSvc.GetDelegationMetrics(c.Request.Context(), startTime, endTime)
		if res.err != nil {
			resChan <- res
			return
		}

		res.dispute, res.err = metricsSvc.GetDisputeMetrics(c.Request.Context(), startTime, endTime)
		if res.err != nil {
			resChan <- res
			return
		}

		res.economicHealth, res.err = metricsSvc.GetEconomicHealthMetrics(c.Request.Context(), startTime, endTime)
		if res.err != nil {
			resChan <- res
			return
		}

		resChan <- res
	}()

	res := <-resChan
	if res.err != nil {
		logger.Error("failed to get dashboard metrics", zap.Error(res.err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get dashboard metrics", "message": res.err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"start_time":      startTime.Format(time.RFC3339),
		"end_time":        endTime.Format(time.RFC3339),
		"escrow":          res.escrow,
		"auction":         res.auction,
		"payment_channel": res.channel,
		"reputation":      res.reputation,
		"delegation":      res.delegation,
		"dispute":         res.dispute,
		"economic_health": res.economicHealth,
	})
}
