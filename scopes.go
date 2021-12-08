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

package logger

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tetratelabs/multierror"
	"github.com/tetratelabs/run"
)

// compile time check for compatibility with the run.Config interface.
var _ run.Config = (*ScopeManager)(nil)

// ScopeManager manages scoped loggers.
type ScopeManager struct {
	logger       *Logger
	outputLevels string

	mtx      sync.Mutex
	registry map[string]*scopedLogger
}

type scopedLogger struct {
	name        string
	description string
	logger      *Logger
}

// NewScopeManager returns a new Scope Manager for Logger.
func NewScopeManager(logger *Logger) *ScopeManager {
	return &ScopeManager{
		logger:       logger,
		outputLevels: levelToString[Level(atomic.LoadInt32(logger.lvl))],
		registry:     make(map[string]*scopedLogger),
	}
}

// Register takes a name and description and returns a scoped Logger.
func (s *ScopeManager) Register(name, description string) *Logger {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	name = strings.ToLower(strings.Trim(name, "\r\n\t "))
	scoped, ok := s.registry[name]
	if ok {
		return scoped.logger
	}
	lvl := atomic.LoadInt32(s.logger.lvl)
	scoped = &scopedLogger{
		name:        name,
		description: description,
		logger: &Logger{
			ctx:    context.Background(),
			lvl:    &lvl,
			logger: s.logger.logger,
		},
	}
	s.registry[name] = scoped

	return scoped.logger
}

// Deregister will attempt to deregister a scoped Logger identified by the
// provided name. If the logger was found, the function returns true.
func (s *ScopeManager) Deregister(name string) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	name = strings.ToLower(strings.Trim(name, "\r\n\t "))
	if _, has := s.registry[name]; !has {
		return false
	}
	delete(s.registry, name)
	return true
}

// Name implements run.Unit.
func (s *ScopeManager) Name() string {
	return "log-manager"
}

// FlagSet implements run.Config.
func (s *ScopeManager) FlagSet() *run.FlagSet {
	keys := make([]string, 0, len(s.registry))
	for k := range s.registry {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fs := run.NewFlagSet("Logging options")
	fs.StringVar(&s.outputLevels, "log-output-level", s.outputLevels, fmt.Sprintf(
		"Comma-separated minimum per-scope logging level of messages to output, "+
			"in the form of [default_level,]<scope>:<level>,<scope>:<level>,... "+
			"where scope can be one of [%s] and default_level or level can be "+
			"one of [%s, %s, %s]",
		strings.Join(keys, ", "),
		"debug", "info", "error",
	))

	return fs
}

// Validate implements run.Config.
func (s *ScopeManager) Validate() error {
	var mErr error

	s.outputLevels = strings.ToLower(s.outputLevels)
	outputLevels := strings.Split(s.outputLevels, ",")
	if len(outputLevels) == 0 {
		return nil
	}
	for _, ol := range outputLevels {
		osl := strings.Split(ol, ":")
		switch len(osl) {
		case 1:
			lvl, ok := stringToLevel[strings.Trim(ol, "\r\n\t ")]
			if !ok {
				mErr = multierror.Append(mErr, fmt.Errorf("%q is not a valid log level", ol))
				continue
			}
			s.SetDefaultOutputLevel(lvl)
		case 2:
			lvl, ok := stringToLevel[strings.Trim(osl[1], "\r\n\t ")]
			if !ok {
				mErr = multierror.Append(mErr, fmt.Errorf("%q is not a valid log level", ol))
				continue
			}
			if err := s.SetScopeOutputLevel(osl[0], lvl); err != nil {
				mErr = multierror.Append(mErr, err)
			}
		default:
			mErr = multierror.Append(mErr, fmt.Errorf("%q is not a valid <scope>:<level> pair", ol))
		}
	}

	return mErr
}

// SetDefaultOutputLevel sets the minimum log output level for all scopes.
func (s *ScopeManager) SetDefaultOutputLevel(lvl Level) {
	// update base logger
	s.logger.SetLevel(lvl)
	// update all scoped loggers
	for _, sg := range s.registry {
		sg.logger.SetLevel(lvl)
	}
}

// SetScopeOutputLevel sets the minimum log output level for a given scope.
func (s *ScopeManager) SetScopeOutputLevel(name string, lvl Level) error {
	s.mtx.Lock()
	name = strings.ToLower(strings.Trim(name, "\r\n\t "))
	sc, has := s.registry[name]
	s.mtx.Unlock()
	if !has {
		return fmt.Errorf("scope %q not found", name)
	}

	sc.logger.SetLevel(lvl)
	return nil
}

// GetDefaultOutputLevel returns the default minimum output level for scopes.
func (s *ScopeManager) GetDefaultOutputLevel() Level {
	return Level(atomic.LoadInt32(s.logger.lvl))
}

// GetOutputLevel returns the minimum log output level for a given scope.
func (s *ScopeManager) GetOutputLevel(name string) (Level, error) {
	s.mtx.Lock()
	name = strings.ToLower(strings.Trim(name, "\r\n\t "))
	sc, has := s.registry[name]
	s.mtx.Unlock()
	if !has {
		return None, fmt.Errorf("scope %q not found", name)
	}
	return Level(atomic.LoadInt32(sc.logger.lvl)), nil
}

// PrintRegisteredScopes logs all the registered scopes and their configured
// output levels.
func (s *ScopeManager) PrintRegisteredScopes() {
	pad := 7

	names := make([]string, 0, len(s.registry))
	for n := range s.registry {
		names = append(names, n)
		if len(n) > pad {
			pad = len(n)
		}
	}
	sort.Strings(names)

	fmt.Println("registered logging scopes:")
	fmt.Printf("- %-*s [%-5s]  %s\n",
		pad,
		"default",
		levelToString[Level(atomic.LoadInt32(s.logger.lvl))],
		"",
	)
	for _, n := range names {
		sc := s.registry[n]
		fmt.Printf("- %-*s [%-5s]  %s\n",
			pad,
			sc.name,
			levelToString[Level(atomic.LoadInt32(sc.logger.lvl))],
			sc.description,
		)
	}
}
