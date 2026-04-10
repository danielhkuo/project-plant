package engine_test

import (
	"testing"
	"time"

	"github.com/danielkuo/project-plant/pkg/telemetry"
	"github.com/danielkuo/project-plant/services/processor/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseEvent(temp, humidity, soilMoisture float64) telemetry.TelemetryEvent {
	return telemetry.TelemetryEvent{
		DeviceID:     "dev-001",
		Timestamp:    time.Now().UTC(),
		Temperature:  temp,
		Humidity:     humidity,
		SoilMoisture: soilMoisture,
	}
}

func TestRuleEngine_Evaluate(t *testing.T) {
	eng := engine.NewDefaultRuleEngine()

	tests := []struct {
		name           string
		event          telemetry.TelemetryEvent
		wantAlertCount int
		wantRules      []string
		wantSeverities []string
	}{
		{
			name:           "high temperature",
			event:          baseEvent(50, 60, 45),
			wantAlertCount: 1,
			wantRules:      []string{"high_temperature"},
			wantSeverities: []string{"warning"},
		},
		{
			name:           "critical temperature",
			event:          baseEvent(70, 60, 45),
			wantAlertCount: 2, // high_temperature AND critical_temperature
			wantRules:      []string{"high_temperature", "critical_temperature"},
			wantSeverities: []string{"warning", "critical"},
		},
		{
			name:           "low temperature",
			event:          baseEvent(3, 60, 45),
			wantAlertCount: 1,
			wantRules:      []string{"low_temperature"},
			wantSeverities: []string{"warning"},
		},
		{
			name:           "dry soil",
			event:          baseEvent(25, 60, 15),
			wantAlertCount: 1,
			wantRules:      []string{"dry_soil"},
			wantSeverities: []string{"warning"},
		},
		{
			name:           "critical dry soil",
			event:          baseEvent(25, 60, 5),
			wantAlertCount: 2, // dry_soil AND critical_dry_soil
			wantRules:      []string{"dry_soil", "critical_dry_soil"},
			wantSeverities: []string{"warning", "critical"},
		},
		{
			name:           "waterlogged",
			event:          baseEvent(25, 60, 95),
			wantAlertCount: 1,
			wantRules:      []string{"waterlogged"},
			wantSeverities: []string{"warning"},
		},
		{
			name:           "low humidity",
			event:          baseEvent(25, 8, 45),
			wantAlertCount: 1,
			wantRules:      []string{"low_humidity"},
			wantSeverities: []string{"info"},
		},
		{
			name:           "high humidity",
			event:          baseEvent(25, 98, 45),
			wantAlertCount: 1,
			wantRules:      []string{"high_humidity"},
			wantSeverities: []string{"info"},
		},
		{
			name:           "all normal",
			event:          baseEvent(25, 60, 45),
			wantAlertCount: 0,
		},
		{
			name:           "multiple breaches — high temp and dry soil",
			event:          baseEvent(50, 60, 8),
			wantAlertCount: 3, // high_temperature + dry_soil + critical_dry_soil
			wantRules:      []string{"high_temperature", "dry_soil", "critical_dry_soil"},
		},
		{
			name:           "boundary exact — temp at 40.0 (threshold is > 40, not >=)",
			event:          baseEvent(40.0, 60, 45),
			wantAlertCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alerts := eng.Evaluate(tt.event)
			assert.Len(t, alerts, tt.wantAlertCount)

			if tt.wantRules != nil {
				ruleNames := make([]string, len(alerts))
				for i, a := range alerts {
					ruleNames[i] = a.RuleName
				}
				assert.ElementsMatch(t, tt.wantRules, ruleNames)
			}

			if tt.wantSeverities != nil {
				severities := make([]string, len(alerts))
				for i, a := range alerts {
					severities[i] = a.Severity
				}
				assert.ElementsMatch(t, tt.wantSeverities, severities)
			}

			// Verify alert structure for non-empty results
			for _, a := range alerts {
				assert.NotEmpty(t, a.AlertID)
				assert.Equal(t, "dev-001", a.DeviceID)
				assert.False(t, a.TriggeredAt.IsZero())
				assert.Equal(t, tt.event.Temperature, a.Reading.Temperature)
			}
		})
	}
}

func TestRuleEngine_CustomRules(t *testing.T) {
	customRules := []engine.Rule{
		{Name: "custom_high_temp", Field: "temperature", Operator: ">=", Threshold: 30, Severity: "warning"},
	}
	eng := engine.NewRuleEngine(customRules)

	alerts := eng.Evaluate(baseEvent(30, 60, 45))
	require.Len(t, alerts, 1)
	assert.Equal(t, "custom_high_temp", alerts[0].RuleName)
}

func TestAlert_HasValidUUID(t *testing.T) {
	eng := engine.NewDefaultRuleEngine()
	alerts := eng.Evaluate(baseEvent(50, 60, 45))
	require.NotEmpty(t, alerts)

	// UUID v4 format: 8-4-4-4-12 hex chars
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, alerts[0].AlertID)
}
