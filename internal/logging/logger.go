package logging

import (
	"fmt"
	"io"
	"log"
	"strings"
)

// Level represents the logging verbosity level
type Level int

const (
	// Silent shows only fatal errors and final summary
	Silent Level = iota
	// Verbose shows all log messages
	Verbose
	// Debug shows all log messages with timestamps and file info
	Debug
)

// SyncLogger wraps log.Logger with level-based filtering and statistics
type SyncLogger struct {
	level  Level
	logger *log.Logger
	stats  map[string]int
	errors []string
	writer io.Writer
}

// NewSyncLogger creates a new SyncLogger with the specified level
func NewSyncLogger(level Level, writer io.Writer) *SyncLogger {
	logFlags := 0
	if level == Debug {
		logFlags = log.Ldate | log.Ltime | log.Lshortfile
	} else if level == Verbose {
		logFlags = log.Ldate | log.Ltime
	}

	return &SyncLogger{
		level:  level,
		logger: log.New(writer, "", logFlags),
		stats:  make(map[string]int),
		errors: make([]string, 0),
		writer: writer,
	}
}

// Info logs informational messages (shown in Verbose and Debug modes)
func (l *SyncLogger) Info(format string, args ...interface{}) {
	if l.level >= Verbose {
		l.logger.Printf(format, args...)
	}
}

// Warn logs warning messages (shown in Verbose and Debug modes)
func (l *SyncLogger) Warn(format string, args ...interface{}) {
	if l.level >= Verbose {
		l.logger.Printf("WARNING: "+format, args...)
	}
}

// Error logs error messages (buffered for summary, shown in Verbose/Debug)
func (l *SyncLogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.errors = append(l.errors, msg)
	if l.level >= Verbose {
		l.logger.Printf("ERROR: %s", msg)
	}
}

// AddStat increments a statistic counter
func (l *SyncLogger) AddStat(key string, count int) {
	l.stats[key] += count
}

// PrintSummary prints the final operation summary
func (l *SyncLogger) PrintSummary(operation string) {
	var parts []string

	// Build summary based on operation type
	if operation == "Push" {
		if created := l.stats["created"]; created > 0 {
			parts = append(parts, fmt.Sprintf("%d new", created))
		}
		if updated := l.stats["updated"]; updated > 0 {
			parts = append(parts, fmt.Sprintf("%d updated", updated))
		}
		if unchanged := l.stats["unchanged"]; unchanged > 0 {
			parts = append(parts, fmt.Sprintf("%d unchanged", unchanged))
		}
	} else if operation == "Pull" {
		if updated := l.stats["updated"]; updated > 0 {
			parts = append(parts, fmt.Sprintf("%d updated", updated))
		}
		if skipped := l.stats["skipped"]; skipped > 0 {
			parts = append(parts, fmt.Sprintf("%d skipped", skipped))
		}
	}

	// Add error count if any
	if len(l.errors) > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", len(l.errors)))
	}

	// Print summary
	summary := fmt.Sprintf("%s complete: %s", operation, strings.Join(parts, ", "))
	fmt.Fprintln(l.writer, summary)
}
