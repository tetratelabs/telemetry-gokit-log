// Copyright (c) Tetrate, Inc 2021.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package logger provides a tetratelabs/telemetry Logger implementation based
// on Go kit log.
package logger

import (
	"context"
	"io"
	"sync/atomic"

	"github.com/go-kit/log"
	"github.com/tetratelabs/telemetry"
	"github.com/tetratelabs/telemetry/level"
)

// compile time check for compatibility with the telemetry.Logger interface.
var (
	_ telemetry.Logger = (*Logger)(nil)
	_ level.Logger     = (*Logger)(nil)
)

// Available log levels.
const (
	None  = level.None
	Error = level.Error
	Info  = level.Info
	Debug = level.Debug
)

// Logger implements the telemetry.Logger interface using Go kit Log.
type Logger struct {
	// ctx holds the Context to extract key-value pairs from to be added to each
	// log line.
	ctx context.Context
	// args holds the key-value pairs to be added to each log line.
	args []interface{}
	// metric holds the Metric to increment each time Info() or Error() is called.
	metric telemetry.Metric
	// lvl holds the configured log level.
	lvl *int32
	// logger holds the Go kit logger to use.
	logger log.Logger
}

// New returns a new telemetry.Logger implementation based on Go kit log.
func New(logger log.Logger) *Logger {
	lvl := int32(Info)
	return &Logger{
		ctx:    context.Background(),
		lvl:    &lvl,
		logger: logger,
	}
}

// NewSyncLogfmt returns a new telemetry.Logger implementation using Go kit's
// sync writer and logfmt output format.
func NewSyncLogfmt(w io.Writer) *Logger {
	return New(log.NewSyncLogger(log.NewLogfmtLogger(w)))
}

// UnwrapLogger returns the wrapped original logger implementation used by this
// Logging bridge.
func (l *Logger) UnwrapLogger() log.Logger {
	return l.logger
}

// Debug logging with key-value pairs. Don't be shy, use it.
func (l *Logger) Debug(msg string, keyValues ...interface{}) {
	if atomic.LoadInt32(l.lvl) < int32(Debug) {
		return
	}
	args := []interface{}{"msg", msg, "level", "debug"}
	args = append(args, telemetry.KeyValuesFromContext(l.ctx)...)
	args = append(args, l.args...)
	args = append(args, keyValues...)
	_ = l.logger.Log(args...)
}

// Info logging with key-value pairs. This is for informational, but not
// directly actionable conditions. It is highly recommended you attach a
// Metric to these types of messages. Where a single informational or
// warning style message might not be reason for action, a change in
// occurrence does warrant action. By attaching a Metric for these logging
// situations, you make this easy through histograms, thresholds, etc.
func (l *Logger) Info(msg string, keyValues ...interface{}) {
	// even if we don't output the log line due to the level configuration,
	// we always emit the Metric if it is set.
	if l.metric != nil {
		l.metric.RecordContext(l.ctx, 1)
	}
	if atomic.LoadInt32(l.lvl) < int32(Info) {
		return
	}
	args := []interface{}{"msg", msg, "level", "info"}
	args = append(args, telemetry.KeyValuesFromContext(l.ctx)...)
	args = append(args, l.args...)
	args = append(args, keyValues...)
	_ = l.logger.Log(args...)
}

// Error logging with key-value pairs. Use this when application state and
// stability are at risk. These types of conditions are actionable and often
// alerted on. It is very strongly encouraged to add a Metric to each of
// these types of messages. Metrics provide the easiest way to coordinate
// processing of these concerns and triggering alerting systems through your
// metrics backend.
func (l *Logger) Error(msg string, err error, keyValues ...interface{}) {
	if l.metric != nil {
		l.metric.RecordContext(l.ctx, 1)
	}
	if atomic.LoadInt32(l.lvl) < int32(Error) {
		return
	}
	args := []interface{}{"msg", msg, "level", "error", "error", err}
	args = append(args, telemetry.KeyValuesFromContext(l.ctx)...)
	args = append(args, l.args...)
	args = append(args, keyValues...)
	_ = l.logger.Log(args...)
}

// With returns Logger with provided key value pairs attached.
func (l *Logger) With(keyValues ...interface{}) telemetry.Logger {
	if len(keyValues) == 0 {
		return l
	}
	if len(keyValues)%2 != 0 {
		keyValues = append(keyValues, "(MISSING)")
	}
	newLogger := &Logger{
		args:   make([]interface{}, len(l.args), len(l.args)+len(keyValues)),
		ctx:    l.ctx,
		metric: l.metric,
		logger: l.logger,
		lvl:    l.lvl,
	}
	copy(newLogger.args, l.args)

	for i := 0; i < len(keyValues); i += 2 {
		if k, ok := keyValues[i].(string); ok {
			newLogger.args = append(newLogger.args, k, keyValues[i+1])
		}
	}
	return newLogger
}

// Context attaches provided Context to the Logger allowing metadata found in
// this context to be used for log lines and metrics labels.
func (l *Logger) Context(ctx context.Context) telemetry.Logger {
	newLogger := &Logger{
		args:   make([]interface{}, len(l.args)),
		ctx:    ctx,
		metric: l.metric,
		logger: l.logger,
		lvl:    l.lvl,
	}
	copy(newLogger.args, l.args)

	return newLogger
}

// Metric attaches provided Metric to the Logger allowing this metric to
// record each invocation of Info and Error log lines. If context is available
// in the logger, it can be used for Metrics labels.
func (l *Logger) Metric(m telemetry.Metric) telemetry.Logger {
	newLogger := &Logger{
		args:   make([]interface{}, len(l.args)),
		ctx:    l.ctx,
		metric: m,
		logger: l.logger,
		lvl:    l.lvl,
	}
	copy(newLogger.args, l.args)

	return newLogger
}

// SetLevel provides the ability to set the desired logging level.
// This function can be used at runtime and is safe for concurrent use.
func (l *Logger) SetLevel(lvl level.Value) {
	if lvl < level.Info {
		lvl = level.Error
	} else if lvl < level.Debug {
		lvl = level.Info
	} else {
		lvl = level.Debug
	}
	atomic.StoreInt32(l.lvl, int32(lvl))
}

// Level returns the currently configured logging level.
func (l *Logger) Level() level.Value {
	return level.Value(atomic.LoadInt32(l.lvl))
}

// New returns a new Logger based on the original implementation but with
// the log level decoupled.
func (l *Logger) New() telemetry.Logger {
	lvl := atomic.LoadInt32(l.lvl)
	newLogger := &Logger{
		args:   make([]interface{}, len(l.args)),
		ctx:    l.ctx,
		metric: l.metric,
		logger: l.logger,
		lvl:    &lvl,
	}
	copy(newLogger.args, l.args)

	return newLogger
}
