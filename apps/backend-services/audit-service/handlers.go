package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Request/Response types
type CreateAuditEventRequest struct {
	EventType       string                 `json:"event_type" binding:"required"`
	Action          string                 `json:"action" binding:"required"`
	Resource        string                 `json:"resource" binding:"required"`
	ResourceID      string                 `json:"resource_id"`
	UserID          string                 `json:"user_id"`
	SessionID       string                 `json:"session_id"`
	IPAddress       string                 `json:"ip_address"`
	UserAgent       string                 `json:"user_agent"`
	RiskLevel       string                 `json:"risk_level"`
	ComplianceFlags []string               `json:"compliance_flags"`
	Metadata        map[string]interface{} `json:"metadata"`
	Success         bool                   `json:"success"`
	ErrorMessage    string                 `json:"error_message"`
	Duration        int64                  `json:"duration_ms"`
	ServiceName     string                 `json:"service_name"`
	ServiceVersion  string                 `json:"service_version"`
	TraceID         string                 `json:"trace_id"`
	SpanID          string                 `json:"span_id"`
}

type BatchAuditEventsRequest struct {
	Events []CreateAuditEventRequest `json:"events" binding:"required"`
}

type GenerateComplianceReportRequest struct {
	Standard    string    `json:"standard" binding:"required"`
	ReportType  string    `json:"report_type" binding:"required"`
	StartDate   time.Time `json:"start_date" binding:"required"`
	EndDate     time.Time `json:"end_date" binding:"required"`
	GeneratedBy string    `json:"generated_by" binding:"required"`
}

type UpdateSecurityAlertRequest struct {
	Status     string `json:"status"`
	AssignedTo string `json:"assigned_to"`
	Resolution string `json:"resolution"`
}

// Audit Event Handlers
func (s *AuditService) createAuditEvent(c *gin.Context) {
	var req CreateAuditEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()

	// Create audit event
	event := &AuditEvent{
		ID:              uuid.New().String(),
		Timestamp:       time.Now().UTC(),
		EventType:       req.EventType,
		Action:          req.Action,
		Resource:        req.Resource,
		ResourceID:      req.ResourceID,
		UserID:          req.UserID,
		SessionID:       req.SessionID,
		IPAddress:       req.IPAddress,
		UserAgent:       req.UserAgent,
		RiskLevel:       req.RiskLevel,
		ComplianceFlags: req.ComplianceFlags,
		Metadata:        req.Metadata,
		Success:         req.Success,
		ErrorMessage:    req.ErrorMessage,
		Duration:        req.Duration,
		ServiceName:     req.ServiceName,
		ServiceVersion:  req.ServiceVersion,
		TraceID:         req.TraceID,
		SpanID:          req.SpanID,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	// Set default risk level if not provided
	if event.RiskLevel == "" {
		event.RiskLevel = s.calculateRiskLevel(event)
	}

	// Store in database
	if err := s.db.Create(event).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create audit event"})
		return
	}

	// Update metrics
	auditEventsTotal.WithLabelValues(event.EventType, event.RiskLevel, strconv.FormatBool(event.Success)).Inc()
	auditProcessingDuration.WithLabelValues(event.EventType).Observe(time.Since(start).Seconds())

	// Check for security alerts
	go s.checkSecurityAlerts(event)

	// Cache recent events
	go s.cacheRecentEvent(event)

	c.JSON(http.StatusCreated, gin.H{
		"id":      event.ID,
		"message": "Audit event created successfully",
	})
}

func (s *AuditService) createBatchAuditEvents(c *gin.Context) {
	var req BatchAuditEventsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()
	events := make([]*AuditEvent, 0, len(req.Events))
	eventIDs := make([]string, 0, len(req.Events))

	// Process each event
	for _, eventReq := range req.Events {
		event := &AuditEvent{
			ID:              uuid.New().String(),
			Timestamp:       time.Now().UTC(),
			EventType:       eventReq.EventType,
			Action:          eventReq.Action,
			Resource:        eventReq.Resource,
			ResourceID:      eventReq.ResourceID,
			UserID:          eventReq.UserID,
			SessionID:       eventReq.SessionID,
			IPAddress:       eventReq.IPAddress,
			UserAgent:       eventReq.UserAgent,
			RiskLevel:       eventReq.RiskLevel,
			ComplianceFlags: eventReq.ComplianceFlags,
			Metadata:        eventReq.Metadata,
			Success:         eventReq.Success,
			ErrorMessage:    eventReq.ErrorMessage,
			Duration:        eventReq.Duration,
			ServiceName:     eventReq.ServiceName,
			ServiceVersion:  eventReq.ServiceVersion,
			TraceID:         eventReq.TraceID,
			SpanID:          eventReq.SpanID,
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		}

		if event.RiskLevel == "" {
			event.RiskLevel = s.calculateRiskLevel(event)
		}

		events = append(events, event)
		eventIDs = append(eventIDs, event.ID)
	}

	// Batch insert
	if err := s.db.CreateInBatches(events, 100).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create batch audit events"})
		return
	}

	// Update metrics
	for _, event := range events {
		auditEventsTotal.WithLabelValues(event.EventType, event.RiskLevel, strconv.FormatBool(event.Success)).Inc()
		go s.checkSecurityAlerts(event)
		go s.cacheRecentEvent(event)
	}

	auditProcessingDuration.WithLabelValues("batch").Observe(time.Since(start).Seconds())

	c.JSON(http.StatusCreated, gin.H{
		"event_ids": eventIDs,
		"count":     len(eventIDs),
		"message":   "Batch audit events created successfully",
	})
}

func (s *AuditService) getAuditEvents(c *gin.Context) {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	eventType := c.Query("event_type")
	userID := c.Query("user_id")
	resource := c.Query("resource")
	riskLevel := c.Query("risk_level")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	// Build query
	query := s.db.Model(&AuditEvent{})

	if eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}
	if userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}
	if riskLevel != "" {
		query = query.Where("risk_level = ?", riskLevel)
	}
	if startDate != "" {
		if start, err := time.Parse(time.RFC3339, startDate); err == nil {
			query = query.Where("timestamp >= ?", start)
		}
	}
	if endDate != "" {
		if end, err := time.Parse(time.RFC3339, endDate); err == nil {
			query = query.Where("timestamp <= ?", end)
		}
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Get events
	var events []AuditEvent
	if err := query.Order("timestamp DESC").Limit(limit).Offset(offset).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve audit events"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"events": events,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *AuditService) getAuditEvent(c *gin.Context) {
	id := c.Param("id")

	var event AuditEvent
	if err := s.db.First(&event, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Audit event not found"})
		return
	}

	c.JSON(http.StatusOK, event)
}

// Compliance Report Handlers
func (s *AuditService) generateComplianceReport(c *gin.Context) {
	var req GenerateComplianceReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	start := time.Now()

	// Generate report
	report, err := s.generateReport(req.Standard, req.ReportType, req.StartDate, req.EndDate, req.GeneratedBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate compliance report"})
		return
	}

	// Store report
	if err := s.db.Create(report).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store compliance report"})
		return
	}

	// Update compliance score metric
	complianceScore.WithLabelValues(report.Standard).Set(report.ComplianceScore)

	c.JSON(http.StatusCreated, gin.H{
		"report_id":       report.ID,
		"compliance_score": report.ComplianceScore,
		"processing_time": time.Since(start).Milliseconds(),
		"message":        "Compliance report generated successfully",
	})
}

func (s *AuditService) getComplianceReports(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	standard := c.Query("standard")

	query := s.db.Model(&ComplianceReport{})
	if standard != "" {
		query = query.Where("standard = ?", standard)
	}

	var total int64
	query.Count(&total)

	var reports []ComplianceReport
	if err := query.Order("generated_at DESC").Limit(limit).Offset(offset).Find(&reports).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve compliance reports"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (s *AuditService) getComplianceReport(c *gin.Context) {
	id := c.Param("id")

	var report ComplianceReport
	if err := s.db.First(&report, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Compliance report not found"})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (s *AuditService) getComplianceScore(c *gin.Context) {
	standard := c.Param("standard")

	// Get latest compliance score
	var report ComplianceReport
	if err := s.db.Where("standard = ?", standard).Order("generated_at DESC").First(&report).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No compliance report found for standard"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"standard":         standard,
		"compliance_score": report.ComplianceScore,
		"last_updated":     report.GeneratedAt,
		"violations":       report.Violations,
		"total_events":     report.TotalEvents,
	})
}

// Security Alert Handlers
func (s *AuditService) getSecurityAlerts(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	severity := c.Query("severity")
	status := c.Query("status")

	query := s.db.Model(&SecurityAlert{})
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	var alerts []SecurityAlert
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&alerts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve security alerts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *AuditService) getSecurityAlert(c *gin.Context) {
	id := c.Param("id")

	var alert SecurityAlert
	if err := s.db.First(&alert, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security alert not found"})
		return
	}

	c.JSON(http.StatusOK, alert)
}

func (s *AuditService) updateSecurityAlert(c *gin.Context) {
	id := c.Param("id")
	var req UpdateSecurityAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var alert SecurityAlert
	if err := s.db.First(&alert, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security alert not found"})
		return
	}

	// Update fields
	if req.Status != "" {
		alert.Status = req.Status
	}
	if req.AssignedTo != "" {
		alert.AssignedTo = req.AssignedTo
	}
	if req.Resolution != "" {
		alert.Resolution = req.Resolution
	}
	alert.UpdatedAt = time.Now().UTC()

	if err := s.db.Save(&alert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update security alert"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Security alert updated successfully",
		"alert":   alert,
	})
}

func (s *AuditService) resolveSecurityAlert(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Resolution string `json:"resolution" binding:"required"`
		ResolvedBy string `json:"resolved_by" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var alert SecurityAlert
	if err := s.db.First(&alert, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Security alert not found"})
		return
	}

	// Resolve alert
	now := time.Now().UTC()
	alert.Status = "resolved"
	alert.Resolution = req.Resolution
	alert.AssignedTo = req.ResolvedBy
	alert.ResolvedAt = &now
	alert.UpdatedAt = now

	if err := s.db.Save(&alert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resolve security alert"})
		return
	}

	// Update metrics
	s.updateSecurityAlertMetrics()

	c.JSON(http.StatusOK, gin.H{
		"message": "Security alert resolved successfully",
		"alert":   alert,
	})
}

// Analytics Handlers
func (s *AuditService) getAnalyticsDashboard(c *gin.Context) {
	// Get dashboard data for the last 24 hours
	since := time.Now().UTC().Add(-24 * time.Hour)

	dashboard := gin.H{
		"period": "24h",
		"timestamp": time.Now().UTC(),
	}

	// Total events
	var totalEvents int64
	s.db.Model(&AuditEvent{}).Where("timestamp >= ?", since).Count(&totalEvents)
	dashboard["total_events"] = totalEvents

	// Events by type
	var eventsByType []struct {
		EventType string `json:"event_type"`
		Count     int64  `json:"count"`
	}
	s.db.Model(&AuditEvent{}).
		Select("event_type, COUNT(*) as count").
		Where("timestamp >= ?", since).
		Group("event_type").
		Find(&eventsByType)
	dashboard["events_by_type"] = eventsByType

	// Risk level distribution
	var riskDistribution []struct {
		RiskLevel string `json:"risk_level"`
		Count     int64  `json:"count"`
	}
	s.db.Model(&AuditEvent{}).
		Select("risk_level, COUNT(*) as count").
		Where("timestamp >= ?", since).
		Group("risk_level").
		Find(&riskDistribution)
	dashboard["risk_distribution"] = riskDistribution

	// Active security alerts
	var activeAlerts int64
	s.db.Model(&SecurityAlert{}).Where("status != 'resolved'").Count(&activeAlerts)
	dashboard["active_security_alerts"] = activeAlerts

	// Top users by activity
	var topUsers []struct {
		UserID string `json:"user_id"`
		Count  int64  `json:"count"`
	}
	s.db.Model(&AuditEvent{}).
		Select("user_id, COUNT(*) as count").
		Where("timestamp >= ? AND user_id != ''", since).
		Group("user_id").
		Order("count DESC").
		Limit(10).
		Find(&topUsers)
	dashboard["top_users"] = topUsers

	c.JSON(http.StatusOK, dashboard)
}

func (s *AuditService) getAuditTrends(c *gin.Context) {
	period := c.DefaultQuery("period", "7d")
	
	var since time.Time
	switch period {
	case "1h":
		since = time.Now().UTC().Add(-1 * time.Hour)
	case "24h":
		since = time.Now().UTC().Add(-24 * time.Hour)
	case "7d":
		since = time.Now().UTC().Add(-7 * 24 * time.Hour)
	case "30d":
		since = time.Now().UTC().Add(-30 * 24 * time.Hour)
	default:
		since = time.Now().UTC().Add(-7 * 24 * time.Hour)
	}

	// Get hourly trends
	var trends []struct {
		Hour  string `json:"hour"`
		Count int64  `json:"count"`
	}
	
	s.db.Raw(`
		SELECT 
			DATE_TRUNC('hour', timestamp) as hour,
			COUNT(*) as count
		FROM audit_events 
		WHERE timestamp >= ?
		GROUP BY DATE_TRUNC('hour', timestamp)
		ORDER BY hour
	`, since).Scan(&trends)

	c.JSON(http.StatusOK, gin.H{
		"period": period,
		"trends": trends,
	})
}

func (s *AuditService) getRiskAssessment(c *gin.Context) {
	// Calculate risk assessment for the last 7 days
	since := time.Now().UTC().Add(-7 * 24 * time.Hour)

	assessment := gin.H{
		"period": "7d",
		"timestamp": time.Now().UTC(),
	}

	// Risk level counts
	var riskCounts []struct {
		RiskLevel string `json:"risk_level"`
		Count     int64  `json:"count"`
	}
	s.db.Model(&AuditEvent{}).
		Select("risk_level, COUNT(*) as count").
		Where("timestamp >= ?", since).
		Group("risk_level").
		Find(&riskCounts)

	// Calculate overall risk score
	var totalEvents int64
	var riskScore float64
	for _, risk := range riskCounts {
		totalEvents += risk.Count
		switch risk.RiskLevel {
		case RiskLevelCritical:
			riskScore += float64(risk.Count) * 4.0
		case RiskLevelHigh:
			riskScore += float64(risk.Count) * 3.0
		case RiskLevelMedium:
			riskScore += float64(risk.Count) * 2.0
		case RiskLevelLow:
			riskScore += float64(risk.Count) * 1.0
		}
	}

	if totalEvents > 0 {
		riskScore = riskScore / float64(totalEvents)
	}

	assessment["risk_distribution"] = riskCounts
	assessment["overall_risk_score"] = riskScore
	assessment["total_events"] = totalEvents

	// Top risky resources
	var riskyResources []struct {
		Resource  string `json:"resource"`
		RiskScore float64 `json:"risk_score"`
		Count     int64  `json:"count"`
	}
	s.db.Raw(`
		SELECT 
			resource,
			AVG(CASE 
				WHEN risk_level = 'critical' THEN 4.0
				WHEN risk_level = 'high' THEN 3.0
				WHEN risk_level = 'medium' THEN 2.0
				ELSE 1.0
			END) as risk_score,
			COUNT(*) as count
		FROM audit_events 
		WHERE timestamp >= ? AND resource != ''
		GROUP BY resource
		HAVING COUNT(*) >= 5
		ORDER BY risk_score DESC, count DESC
		LIMIT 10
	`, since).Scan(&riskyResources)

	assessment["risky_resources"] = riskyResources

	c.JSON(http.StatusOK, assessment)
}
