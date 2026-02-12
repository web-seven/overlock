package tui

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Level   zapcore.Level
	Message string
	Time    string
}

// LogSink is a custom zap core that captures logs for TUI display
type LogSink struct {
	zapcore.LevelEnabler
	encoder zapcore.Encoder
	output  *LogBuffer
}

// LogBuffer stores log entries and sends them to the TUI
type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	program *tea.Program
}

// NewLogBuffer creates a new log buffer
func NewLogBuffer() *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0),
	}
}

// SetProgram sets the Bubble Tea program for sending messages
func (b *LogBuffer) SetProgram(p *tea.Program) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.program = p
}

// Add appends a log entry and sends it to the TUI
func (b *LogBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, entry)

	// Send message to TUI if program is set
	if b.program != nil {
		b.program.Send(LogMsg{Entry: entry})
	}
}

// GetEntries returns all log entries
func (b *LogBuffer) GetEntries() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Return a copy to avoid race conditions
	entries := make([]LogEntry, len(b.entries))
	copy(entries, b.entries)
	return entries
}

// Clear removes all log entries
func (b *LogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = make([]LogEntry, 0)
}

// LogMsg is a Bubble Tea message for log updates
type LogMsg struct {
	Entry LogEntry
}

// NewLogSink creates a new log sink
func NewLogSink(level zapcore.Level, output *LogBuffer) *LogSink {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05")
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	return &LogSink{
		LevelEnabler: level,
		encoder:      zapcore.NewConsoleEncoder(encoderConfig),
		output:       output,
	}
}

// With adds structured context to the logger
func (s *LogSink) With(fields []zapcore.Field) zapcore.Core {
	return s
}

// Check determines whether the supplied Entry should be logged
func (s *LogSink) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(entry.Level) {
		return checked.AddCore(entry, s)
	}
	return checked
}

// Write serializes the Entry and any Fields supplied
func (s *LogSink) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	// Create log entry
	logEntry := LogEntry{
		Level:   entry.Level,
		Message: entry.Message,
		Time:    entry.Time.Format("15:04:05"),
	}

	// Add to buffer
	s.output.Add(logEntry)

	return nil
}

// Sync flushes buffered logs
func (s *LogSink) Sync() error {
	return nil
}

// CreateTUILogger creates a logger that outputs to the TUI log buffer
func CreateTUILogger(buffer *LogBuffer) *zap.SugaredLogger {
	core := NewLogSink(zapcore.DebugLevel, buffer)
	logger := zap.New(core)
	return logger.Sugar()
}
