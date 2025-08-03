package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Background workers for audit service

// Event processor - handles real-time event processing
func (s *AuditService) startEventProcessor() {
	log.Println("Starting audit event processor...")
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processRecentEvents()
		}
	}
}

// Security monitor - detects security threats and anomalies
func (s *AuditService) startSecurityMonitor() {
	log.Println("Starting security monitor...")
	
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.detectSecurityThreats()
			s.updateSecurityAlertMetrics()
		}
	}
}

// Compliance monitor - tracks compliance metrics and generates reports
func (s *AuditService) startComplianceMonitor() {
	log.Println("Starting compliance monitor...")
	
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.updateComplianceMetrics()
		}
	}
}

// Process recent events for patterns and anomalies
func (s *AuditService) processRecentEvents() {
	// Get events from the last 5 minutes
	since := time.Now().UTC().Add(-5 * time.Minute)
	
	var events []AuditEvent
	if err := s.db.Where("timestamp >= ?", since).Find(&events).Error; err != nil {
		log.Printf("Error fetching recent events: %v", err)
		return
	}

	// Process events for patterns
	s.detectAnomalies(events)
	s.updateEventPatterns(events)
}

// Detect security threats and create alerts
func (s *AuditService) detectSecurityThreats() {
	// Look for suspicious patterns in the last hour
	since := time.Now().UTC().Add(-1 * time.Hour)
	
	// Detect multiple failed login attempts
	s.detectFailedLoginAttempts(since)
	
	// Detect unusual access patterns
	s.detectUnusualAccess(since)
	
	// Detect privilege escalation attempts
	s.detectPrivilegeEscalation(since)
	
	// Detect data exfiltration patterns
	s.detectDataExfiltration(since)
}

// Detect multiple failed login attempts
func (s *AuditService) detectFailedLoginAttempts(since time.Time) {
	var results []struct {
		UserID    string `json:"user_id"`
		IPAddress string `json:"ip_address"`
		Count     int64  `json:"count"`
	}

	s.db.Raw(`
		SELECT user_id, ip_address, COUNT(*) as count
		FROM audit_events 
		WHERE timestamp >= ? 
		AND event_type = ? 
		AND action = 'login' 
		AND success = false
		GROUP BY user_id, ip_address
		HAVING COUNT(*) >= 5
	`, since, EventTypeAuthentication).Scan(&results)

	for _, result := range results {
		// Check if alert already exists
		var existingAlert SecurityAlert
		if err := s.db.Where("alert_type = ? AND user_id = ? AND ip_address = ? AND status != 'resolved'", 
			"failed_login_attempts", result.UserID, result.IPAddress).First(&existingAlert).Error; err != nil {
			
			// Create new security alert
			alert := &SecurityAlert{
				ID:          uuid.New().String(),
				AlertType:   "failed_login_attempts",
				Severity:    RiskLevelHigh,
				Title:       "Multiple Failed Login Attempts Detected",
				Description: fmt.Sprintf("User %s from IP %s has %d failed login attempts in the last hour", 
					result.UserID, result.IPAddress, result.Count),
				UserID:      result.UserID,
				IPAddress:   result.IPAddress,
				Status:      "open",
				Metadata: map[string]interface{}{
					"failed_attempts": result.Count,
					"time_window":     "1h",
				},
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}

			if err := s.db.Create(alert).Error; err != nil {
				log.Printf("Error creating security alert: %v", err)
			}
		}
	}
}

// Detect unusual access patterns
func (s *AuditService) detectUnusualAccess(since time.Time) {
	// Detect access from unusual locations
	var results []struct {
		UserID    string `json:"user_id"`
		IPAddress string `json:"ip_address"`
		Count     int64  `json:"count"`
	}

	// This is a simplified version - in production, you'd use geolocation data
	s.db.Raw(`
		SELECT user_id, ip_address, COUNT(*) as count
		FROM audit_events 
		WHERE timestamp >= ? 
		AND event_type = ? 
		AND user_id != ''
		GROUP BY user_id, ip_address
		HAVING COUNT(*) >= 10
	`, since, EventTypeUserAction).Scan(&results)

	for _, result := range results {
		// Check if this IP is unusual for this user (simplified logic)
		var historicalCount int64
		s.db.Model(&AuditEvent{}).
			Where("user_id = ? AND ip_address = ? AND timestamp < ?", 
				result.UserID, result.IPAddress, since.Add(-24*time.Hour)).
			Count(&historicalCount)

		if historicalCount == 0 && result.Count >= 10 {
			alert := &SecurityAlert{
				ID:          uuid.New().String(),
				AlertType:   "unusual_access_pattern",
				Severity:    RiskLevelMedium,
				Title:       "Unusual Access Pattern Detected",
				Description: fmt.Sprintf("User %s accessing from new IP address %s with %d actions", 
					result.UserID, result.IPAddress, result.Count),
				UserID:      result.UserID,
				IPAddress:   result.IPAddress,
				Status:      "open",
				Metadata: map[string]interface{}{
					"action_count":    result.Count,
					"new_ip_address":  true,
				},
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}

			if err := s.db.Create(alert).Error; err != nil {
				log.Printf("Error creating security alert: %v", err)
			}
		}
	}
}

// Detect privilege escalation attempts
func (s *AuditService) detectPrivilegeEscalation(since time.Time) {
	var results []struct {
		UserID string `json:"user_id"`
		Count  int64  `json:"count"`
	}

	s.db.Raw(`
		SELECT user_id, COUNT(*) as count
		FROM audit_events 
		WHERE timestamp >= ? 
		AND (action LIKE '%admin%' OR action LIKE '%privilege%' OR action LIKE '%permission%')
		AND success = false
		GROUP BY user_id
		HAVING COUNT(*) >= 3
	`, since).Scan(&results)

	for _, result := range results {
		alert := &SecurityAlert{
			ID:          uuid.New().String(),
			AlertType:   "privilege_escalation_attempt",
			Severity:    RiskLevelHigh,
			Title:       "Potential Privilege Escalation Detected",
			Description: fmt.Sprintf("User %s has %d failed privilege-related actions", 
				result.UserID, result.Count),
			UserID:      result.UserID,
			Status:      "open",
			Metadata: map[string]interface{}{
				"failed_attempts": result.Count,
				"action_type":     "privilege_escalation",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := s.db.Create(alert).Error; err != nil {
			log.Printf("Error creating security alert: %v", err)
		}
	}
}

// Detect data exfiltration patterns
func (s *AuditService) detectDataExfiltration(since time.Time) {
	var results []struct {
		UserID string `json:"user_id"`
		Count  int64  `json:"count"`
	}

	s.db.Raw(`
		SELECT user_id, COUNT(*) as count
		FROM audit_events 
		WHERE timestamp >= ? 
		AND event_type = ?
		AND (action = 'download' OR action = 'export' OR action = 'copy')
		GROUP BY user_id
		HAVING COUNT(*) >= 20
	`, since, EventTypeDataAccess).Scan(&results)

	for _, result := range results {
		alert := &SecurityAlert{
			ID:          uuid.New().String(),
			AlertType:   "potential_data_exfiltration",
			Severity:    RiskLevelCritical,
			Title:       "Potential Data Exfiltration Detected",
			Description: fmt.Sprintf("User %s has performed %d data access actions in the last hour", 
				result.UserID, result.Count),
			UserID:      result.UserID,
			Status:      "open",
			Metadata: map[string]interface{}{
				"access_count": result.Count,
				"time_window":  "1h",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := s.db.Create(alert).Error; err != nil {
			log.Printf("Error creating security alert: %v", err)
		}
	}
}

// Update security alert metrics
func (s *AuditService) updateSecurityAlertMetrics() {
	severities := []string{RiskLevelLow, RiskLevelMedium, RiskLevelHigh, RiskLevelCritical}
	
	for _, severity := range severities {
		var count int64
		s.db.Model(&SecurityAlert{}).
			Where("severity = ? AND status != 'resolved'", severity).
			Count(&count)
		
		securityAlertsActive.WithLabelValues(severity).Set(float64(count))
	}
}

// Update compliance metrics
func (s *AuditService) updateComplianceMetrics() {
	standards := []string{ComplianceSOX, ComplianceGDPR, ComplianceHIPAA, ComplianceSOC2, CompliancePCIDSS, ComplianceISO27001}
	
	for _, standard := range standards {
		var report ComplianceReport
		if err := s.db.Where("standard = ?", standard).
			Order("generated_at DESC").
			First(&report).Error; err == nil {
			complianceScore.WithLabelValues(standard).Set(report.ComplianceScore)
		}
	}
}

// Detect anomalies in event patterns
func (s *AuditService) detectAnomalies(events []AuditEvent) {
	// Simple anomaly detection based on event frequency
	eventCounts := make(map[string]int)
	
	for _, event := range events {
		key := fmt.Sprintf("%s:%s", event.EventType, event.Action)
		eventCounts[key]++
	}

	// Log unusual spikes (simplified logic)
	for key, count := range eventCounts {
		if count > 100 { // Threshold for anomaly
			log.Printf("Anomaly detected: %s occurred %d times in 5 minutes", key, count)
		}
	}
}

// Update event patterns for trend analysis
func (s *AuditService) updateEventPatterns(events []AuditEvent) {
	ctx := context.Background()
	
	for _, event := range events {
		// Store event pattern in Redis for trend analysis
		key := fmt.Sprintf("pattern:%s:%s", event.EventType, event.Action)
		
		// Increment counter with expiration
		s.redis.Incr(ctx, key)
		s.redis.Expire(ctx, key, 24*time.Hour)
	}
}

// Check for security alerts based on individual events
func (s *AuditService) checkSecurityAlerts(event *AuditEvent) {
	// Check for high-risk events
	if event.RiskLevel == RiskLevelCritical {
		s.createHighRiskAlert(event)
	}

	// Check for failed authentication events
	if event.EventType == EventTypeAuthentication && !event.Success {
		s.trackFailedAuthentication(event)
	}

	// Check for unauthorized access attempts
	if event.EventType == EventTypeAuthorization && !event.Success {
		s.trackUnauthorizedAccess(event)
	}
}

// Create alert for high-risk events
func (s *AuditService) createHighRiskAlert(event *AuditEvent) {
	alert := &SecurityAlert{
		ID:          uuid.New().String(),
		AlertType:   "high_risk_event",
		Severity:    RiskLevelCritical,
		Title:       "High Risk Event Detected",
		Description: fmt.Sprintf("Critical risk event: %s on %s", event.Action, event.Resource),
		EventIDs:    []string{event.ID},
		UserID:      event.UserID,
		IPAddress:   event.IPAddress,
		Status:      "open",
		Metadata: map[string]interface{}{
			"event_type": event.EventType,
			"action":     event.Action,
			"resource":   event.Resource,
		},
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	if err := s.db.Create(alert).Error; err != nil {
		log.Printf("Error creating high-risk alert: %v", err)
	}
}

// Track failed authentication for pattern detection
func (s *AuditService) trackFailedAuthentication(event *AuditEvent) {
	ctx := context.Background()
	key := fmt.Sprintf("failed_auth:%s:%s", event.UserID, event.IPAddress)
	
	// Increment counter
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("Error tracking failed authentication: %v", err)
		return
	}

	// Set expiration on first increment
	if count == 1 {
		s.redis.Expire(ctx, key, 1*time.Hour)
	}

	// Create alert if threshold exceeded
	if count >= 5 {
		alert := &SecurityAlert{
			ID:          uuid.New().String(),
			AlertType:   "repeated_failed_authentication",
			Severity:    RiskLevelHigh,
			Title:       "Repeated Failed Authentication",
			Description: fmt.Sprintf("User %s from IP %s has %d failed authentication attempts", 
				event.UserID, event.IPAddress, count),
			EventIDs:    []string{event.ID},
			UserID:      event.UserID,
			IPAddress:   event.IPAddress,
			Status:      "open",
			Metadata: map[string]interface{}{
				"failed_count": count,
				"time_window": "1h",
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := s.db.Create(alert).Error; err != nil {
			log.Printf("Error creating failed authentication alert: %v", err)
		}

		// Reset counter after creating alert
		s.redis.Del(ctx, key)
	}
}

// Track unauthorized access attempts
func (s *AuditService) trackUnauthorizedAccess(event *AuditEvent) {
	ctx := context.Background()
	key := fmt.Sprintf("unauth_access:%s:%s", event.UserID, event.Resource)
	
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		log.Printf("Error tracking unauthorized access: %v", err)
		return
	}

	if count == 1 {
		s.redis.Expire(ctx, key, 1*time.Hour)
	}

	if count >= 3 {
		alert := &SecurityAlert{
			ID:          uuid.New().String(),
			AlertType:   "repeated_unauthorized_access",
			Severity:    RiskLevelMedium,
			Title:       "Repeated Unauthorized Access Attempts",
			Description: fmt.Sprintf("User %s has %d unauthorized access attempts to %s", 
				event.UserID, count, event.Resource),
			EventIDs:    []string{event.ID},
			UserID:      event.UserID,
			Status:      "open",
			Metadata: map[string]interface{}{
				"resource":     event.Resource,
				"attempt_count": count,
			},
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}

		if err := s.db.Create(alert).Error; err != nil {
			log.Printf("Error creating unauthorized access alert: %v", err)
		}

		s.redis.Del(ctx, key)
	}
}

// Cache recent events for quick access
func (s *AuditService) cacheRecentEvent(event *AuditEvent) {
	ctx := context.Background()
	
	// Cache event data
	eventData, err := json.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling event for cache: %v", err)
		return
	}

	// Store in Redis with expiration
	key := fmt.Sprintf("recent_event:%s", event.ID)
	s.redis.Set(ctx, key, eventData, 1*time.Hour)

	// Add to recent events list
	listKey := "recent_events"
	s.redis.LPush(ctx, listKey, event.ID)
	s.redis.LTrim(ctx, listKey, 0, 999) // Keep last 1000 events
	s.redis.Expire(ctx, listKey, 1*time.Hour)
}

// Calculate risk level for an event
func (s *AuditService) calculateRiskLevel(event *AuditEvent) string {
	score := 0

	// Base score by event type
	switch event.EventType {
	case EventTypeSecurityEvent:
		score += 3
	case EventTypeAuthentication, EventTypeAuthorization:
		score += 2
	case EventTypeConfigChange:
		score += 2
	case EventTypeDataAccess:
		score += 1
	default:
		score += 1
	}

	// Increase score for failed events
	if !event.Success {
		score += 2
	}

	// Increase score for sensitive actions
	sensitiveActions := []string{"delete", "admin", "privilege", "export", "download"}
	for _, action := range sensitiveActions {
		if strings.Contains(strings.ToLower(event.Action), action) {
			score += 1
			break
		}
	}

	// Increase score for sensitive resources
	sensitiveResources := []string{"user", "admin", "config", "secret", "key"}
	for _, resource := range sensitiveResources {
		if strings.Contains(strings.ToLower(event.Resource), resource) {
			score += 1
			break
		}
	}

	// Convert score to risk level
	switch {
	case score >= 6:
		return RiskLevelCritical
	case score >= 4:
		return RiskLevelHigh
	case score >= 2:
		return RiskLevelMedium
	default:
		return RiskLevelLow
	}
}
