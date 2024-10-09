// Copyright (c) 2024 Md. Tolha Bin Ashraf
// All rights reserved.
// This software is licensed under the MIT License. See the LICENSE file for details.

package troLogger

import (
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	PanicLevel
	TraceLevel
)

var logLevelStrings = [...]string{
	"DEBU",
	"INFO",
	"WARN",
	"ERRO",
	"PANI",
	"TRAC",
	"UNKNOWN",
}

var logIDCounter int32 // Using atomic for thread-safe incrementing

// Logger is a structured logger with configurable options
type Logger struct {
	level   LogLevel
	output  io.Writer
	file    *os.File
	colored bool
	mu      sync.Mutex
	fields  map[string]string
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return &[]byte{} 
	},
}

// NewLogger initializes a new logger instance
func NewLogger(level LogLevel, output io.Writer, colored bool, logFilePath string) (*Logger, error) {
	var logFile *os.File
	var err error
	if logFilePath != "" {
		logFile, err = os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return nil, err
		}
	}

	return &Logger{
		level:   level,
		output:  output,
		file:    logFile, // This can be nil if no file is specified
		colored: colored,
		fields:  make(map[string]string),
	}, nil
}

// Close closes the log file if it's being used
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log handles core logging logic and minimizes allocations
func (l *Logger) log(level LogLevel, message string, extraFields map[string]string) {

	logID := atomic.AddInt32(&logIDCounter, 1)

	buf := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(buf)

	*buf = (*buf)[:0]
	timestamp := time.Now().Format(time.RFC3339)

	// Prepare the log message for terminal output (with colors and level)
	if level == DebugLevel || level == InfoLevel {
		*buf = append(*buf, getColor(level)...)
		*buf = append(*buf, logLevelStrings[level]...)
		*buf = append(*buf, "\033[0m"...)
	} else {
		*buf = append(*buf, getColor(level)...)
		*buf = append(*buf, logLevelStrings[level]...)
	}
	*buf = append(*buf, ' ')
	*buf = append(*buf, timestamp...)
	*buf = append(*buf, ' ')
	*buf = append(*buf, "ID:"...)
	*buf = append(*buf, strconv.Itoa(int(logID))...)
	*buf = append(*buf, ' ')
	*buf = append(*buf, message...)

	// Append fields directly from the logger and extra fields
	l.mu.Lock()
	if len(l.fields) > 0 || len(extraFields) > 0 {
		*buf = append(*buf, ',')
	}

	for key, value := range l.fields {
		*buf = append(*buf, ' ')
		*buf = append(*buf, key...)
		*buf = append(*buf, ':')
		*buf = append(*buf, ' ')
		*buf = append(*buf, '"')
		*buf = append(*buf, value...)
		*buf = append(*buf, '"')
	}

	for key, value := range extraFields {
		*buf = append(*buf, ' ')
		*buf = append(*buf, key...)
		*buf = append(*buf, ':')
		*buf = append(*buf, ' ')
		*buf = append(*buf, '"')
		*buf = append(*buf, value...)
		*buf = append(*buf, '"')
	}
	l.mu.Unlock()

	*buf = append(*buf, '\n')

	// Always write to the file, if it's not nil
	if l.file != nil {
		logMessage := buildLogMessage(level, timestamp, message, l.fields, extraFields, false, logID)
		_, _ = l.file.Write(logMessage)
	}

	// Write to the terminal (with colors and filtering by log level)
	if level >= l.level {
		_, _ = l.output.Write(*buf)

		// Reset color after writing the full log line for WARN and ERRO
		if (level == WarnLevel || level == ErrorLevel) && l.colored {
			_, _ = l.output.Write([]byte("\033[0m"))
		}
	}
}

// buildLogMessage constructs a log message for writing to file
func buildLogMessage(level LogLevel, timestamp, message string, fields, extraFields map[string]string, colored bool, logID int32) []byte {
	var logBuf []byte
	if colored {
		logBuf = append(logBuf, getColor(level)...)
		logBuf = append(logBuf, logLevelStrings[level]...)
		logBuf = append(logBuf, "\033[0m"...)
	} else {
		logBuf = append(logBuf, logLevelStrings[level]...)
	}
	logBuf = append(logBuf, ' ')
	logBuf = append(logBuf, timestamp...)
	logBuf = append(logBuf, ' ')
	logBuf = append(logBuf, "ID:"...)
	logBuf = append(logBuf, strconv.Itoa(int(logID))...)
	logBuf = append(logBuf, ' ')
	logBuf = append(logBuf, message...)

	if len(fields) > 0 || len(extraFields) > 0 {
		logBuf = append(logBuf, ',')
	}

	for key, value := range fields {
		logBuf = append(logBuf, ' ')
		logBuf = append(logBuf, key...)
		logBuf = append(logBuf, ':')
		logBuf = append(logBuf, ' ')
		logBuf = append(logBuf, '"')
		logBuf = append(logBuf, value...)
		logBuf = append(logBuf, '"')
	}

	for key, value := range extraFields {
		logBuf = append(logBuf, ' ')
		logBuf = append(logBuf, key...)
		logBuf = append(logBuf, ':')
		logBuf = append(logBuf, ' ')
		logBuf = append(logBuf, '"')
		logBuf = append(logBuf, value...)
		logBuf = append(logBuf, '"')
	}

	logBuf = append(logBuf, '\n')
	return logBuf
}

// getColor returns the ANSI color code for a given log level
func getColor(level LogLevel) string {
	switch level {
	case DebugLevel:
		return "\033[36m" // Cyan
	case InfoLevel:
		return "\033[32m" // Green
	case WarnLevel:
		return "\033[33m" // Yellow
	case ErrorLevel:
		return "\033[31m" // Red
	default:
		return "\033[0m"  // Default
	}
}

// AddField adds a field to the logger and returns a new logger instance
func (l *Logger) AddField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		level:   l.level,
		output:  l.output,
		file:    l.file,
		colored: l.colored,
		fields:  make(map[string]string),
	}

	l.mu.Lock()
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	l.mu.Unlock()

	newLogger.fields[key] = valueToString(value)

	return newLogger
}

// valueToString converts various types to a string representation
func valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return string(intToString(v))
	case float64:
		return floatToString(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return "unknown"
	}
}

// floatToString converts a float64 to a string
func floatToString(f float64) string {
	if f == 0 {
		return "0.00"
	}

	var result []byte
	integerPart := int(f)
	decimalPart := int((f - float64(integerPart)) * 100) 

	result = append(result, intToString(integerPart)...)
	result = append(result, '.')
	if decimalPart < 10 {
		result = append(result, '0')
	}
	result = append(result, intToString(decimalPart)...)

	return string(result)
}

// Log methods for different levels
func (l *Logger) Info(message string) {
	l.log(InfoLevel, message, nil)
}
func (l *Logger) Warn(message string) {
	l.log(WarnLevel, message, nil)
}
func (l *Logger) Error(message string) {
	l.log(ErrorLevel, message, nil)
}
func (l *Logger) Debug(message string) {
	l.log(DebugLevel, message, nil)
}
func (l *Logger) Panic(message string) {
	l.log(PanicLevel, message, nil)
}
func (l *Logger) Trace(message string) {
	l.log(TraceLevel, message, nil)
}

// Log methods for various levels that format messages using a format string and arguments
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, formatMessage(format, args...), nil)
}
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, formatMessage(format, args...), nil)
}
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, formatMessage(format, args...), nil)
}
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, formatMessage(format, args...), nil)
}
func (l *Logger) Panicf(format string, args ...interface{}) {
	l.log(PanicLevel, formatMessage(format, args...), nil)
}
func (l *Logger) Tracef(format string, args ...interface{}) {
	l.log(TraceLevel, formatMessage(format, args...), nil)
}

// formatMessage formats a string using placeholders ('%s' for strings and '%d' for integers)
// by replacing them with corresponding values from args, returning the final formatted string.
func formatMessage(format string, args ...interface{}) string {
	result := make([]byte, 0, 64)
	argIndex := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			nextChar := format[i+1]
			switch nextChar {
			case 's':
				result = append(result, []byte(args[argIndex].(string))...)
				argIndex++
			case 'd':
				result = append(result, intToString(args[argIndex].(int))...)
				argIndex++
			}
			i++
		} else {
			result = append(result, format[i])
		}
	}
	return string(result)
}

// intToString converts an integer to a string.
func intToString(i int) []byte {
	if i == 0 {
		return []byte{'0'}
	}
	var buf [10]byte
	n := 0
	negative := false
	if i < 0 {
		negative = true
		i = -i
	}
	for i > 0 {
		buf[n] = byte('0' + i%10)
		i /= 10
		n++
	}
	if negative {
		buf[n] = '-'
		n++
	}
	for j := 0; j < n/2; j++ {
		buf[j], buf[n-j-1] = buf[n-j-1], buf[j]
	}
	return buf[:n]
}
