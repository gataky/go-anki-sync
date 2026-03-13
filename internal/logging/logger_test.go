package logging

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSyncLogger_Silent(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSyncLogger(Silent, &buf)

	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	assert.Empty(t, buf.String(), "Silent mode should not output Info/Warn/Error")
}

func TestSyncLogger_Verbose(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSyncLogger(Verbose, &buf)

	logger.Info("info message")
	logger.Warn("warn message")

	output := buf.String()
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
}

func TestSyncLogger_AddStat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSyncLogger(Silent, &buf)

	logger.AddStat("created", 3)
	logger.AddStat("updated", 2)
	logger.AddStat("created", 1) // increment existing

	assert.Equal(t, 4, logger.stats["created"])
	assert.Equal(t, 2, logger.stats["updated"])
}

func TestSyncLogger_PrintSummary_Push(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSyncLogger(Silent, &buf)

	logger.AddStat("created", 3)
	logger.AddStat("updated", 2)
	logger.AddStat("unchanged", 5)
	logger.Error("failed to create card X")

	logger.PrintSummary("Push")

	output := buf.String()
	assert.Contains(t, output, "Push complete")
	assert.Contains(t, output, "3 new")
	assert.Contains(t, output, "2 updated")
	assert.Contains(t, output, "5 unchanged")
	assert.Contains(t, output, "1 failed")
}

func TestSyncLogger_PrintSummary_NoErrors(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSyncLogger(Silent, &buf)

	logger.AddStat("created", 3)
	logger.AddStat("updated", 2)

	logger.PrintSummary("Push")

	output := buf.String()
	assert.Contains(t, output, "Push complete")
	assert.NotContains(t, output, "failed")
}
