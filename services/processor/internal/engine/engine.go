package engine

import (
	"time"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/google/uuid"
)

// Alert represents a triggered alerting rule.
type Alert struct {
	AlertID     string                   `json:"alert_id"`
	DeviceID    string                   `json:"device_id"`
	RuleName    string                   `json:"rule_name"`
	Severity    string                   `json:"severity"`
	TriggeredAt time.Time                `json:"triggered_at"`
	Reading     telemetry.TelemetryEvent `json:"reading"`
}

// Rule defines a threshold-based alerting rule.
type Rule struct {
	Name      string  `json:"name"`
	Field     string  `json:"field"`
	Operator  string  `json:"operator"` // ">" or "<"
	Threshold float64 `json:"threshold"`
	Severity  string  `json:"severity"` // "info", "warning", "critical"
}

// RuleEngine evaluates telemetry events against a set of rules.
type RuleEngine struct {
	rules []Rule
}

// NewRuleEngine creates a rule engine with custom rules.
func NewRuleEngine(rules []Rule) *RuleEngine {
	return &RuleEngine{rules: rules}
}

// NewDefaultRuleEngine creates a rule engine with the default threshold rules.
func NewDefaultRuleEngine() *RuleEngine {
	return NewRuleEngine([]Rule{
		{Name: "high_temperature", Field: "temperature", Operator: ">", Threshold: 40, Severity: "warning"},
		{Name: "critical_temperature", Field: "temperature", Operator: ">", Threshold: 60, Severity: "critical"},
		{Name: "low_temperature", Field: "temperature", Operator: "<", Threshold: 5, Severity: "warning"},
		{Name: "dry_soil", Field: "soil_moisture", Operator: "<", Threshold: 20, Severity: "warning"},
		{Name: "critical_dry_soil", Field: "soil_moisture", Operator: "<", Threshold: 10, Severity: "critical"},
		{Name: "waterlogged", Field: "soil_moisture", Operator: ">", Threshold: 90, Severity: "warning"},
		{Name: "low_humidity", Field: "humidity", Operator: "<", Threshold: 10, Severity: "info"},
		{Name: "high_humidity", Field: "humidity", Operator: ">", Threshold: 95, Severity: "info"},
	})
}

// Evaluate checks a telemetry event against all rules and returns triggered alerts.
func (e *RuleEngine) Evaluate(event telemetry.TelemetryEvent) []Alert {
	var alerts []Alert

	for _, rule := range e.rules {
		value := fieldValue(event, rule.Field)
		if triggered(value, rule.Operator, rule.Threshold) {
			alerts = append(alerts, Alert{
				AlertID:     uuid.New().String(),
				DeviceID:    event.DeviceID,
				RuleName:    rule.Name,
				Severity:    rule.Severity,
				TriggeredAt: time.Now().UTC(),
				Reading:     event,
			})
		}
	}

	return alerts
}

func fieldValue(event telemetry.TelemetryEvent, field string) float64 {
	switch field {
	case "temperature":
		return event.Temperature
	case "humidity":
		return event.Humidity
	case "soil_moisture":
		return event.SoilMoisture
	default:
		return 0
	}
}

func triggered(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	default:
		return false
	}
}
