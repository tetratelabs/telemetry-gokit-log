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
	"fmt"
	"sort"
	"strings"
	"sync"
)

var levelToString = map[Level]string{
	None:  "none",
	Error: "error",
	Info:  "info",
	Debug: "debug",
}

// Manager manages scoped loggers.
type Manager struct {
	logger *Logger

	mtx      sync.Mutex
	registry map[string]*scope
}

type scope struct {
	name        string
	description string
	logger      *Logger
}

// NewManager returns a new Scope Manager for Logger.
func NewManager(logger *Logger) *Manager {
	return &Manager{
		logger:   logger,
		registry: make(map[string]*scope),
	}
}

// RegisterScope takes a name and description and returns a scoped Logger.
func (s *Manager) RegisterScope(name, description string) *Logger {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	name = strings.ToLower(strings.Trim(name, "\r\n\t "))
	scoped, ok := s.registry[name]
	if ok {
		return scoped.logger
	}
	newLogger := New(s.logger.UnwrapLogger())
	newLogger.SetLevel(s.logger.Level())
	scoped = &scope{
		name:        name,
		description: description,
		logger:      newLogger,
	}
	s.registry[name] = scoped

	return scoped.logger
}

// DeregisterScope will attempt to deregister a scoped Logger identified by the
// provided name. If the logger was found, the function returns true.
func (s *Manager) DeregisterScope(name string) bool {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	name = strings.ToLower(strings.Trim(name, "\r\n\t "))
	if _, has := s.registry[name]; !has {
		return false
	}
	delete(s.registry, name)
	return true
}

func (s *Manager) Scopes() []string {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	keys := make([]string, 0, len(s.registry))
	for _, scope := range s.registry {
		keys = append(keys, scope.name)
	}
	sort.Strings(keys)

	return keys
}

// SetDefaultOutputLevel sets the minimum log output level for all scopes.
func (s *Manager) SetDefaultOutputLevel(lvl Level) {
	// update base logger
	s.logger.SetLevel(lvl)
	// update all scoped loggers
	for _, sg := range s.registry {
		sg.logger.SetLevel(lvl)
	}
}

// SetScopeOutputLevel sets the minimum log output level for a given scope.
func (s *Manager) SetScopeOutputLevel(name string, lvl Level) error {
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
func (s *Manager) GetDefaultOutputLevel() Level {
	return s.logger.Level()
}

// GetOutputLevel returns the minimum log output level for a given scope.
func (s *Manager) GetOutputLevel(name string) (Level, error) {
	s.mtx.Lock()
	name = strings.ToLower(strings.Trim(name, "\r\n\t "))
	sc, has := s.registry[name]
	s.mtx.Unlock()
	if !has {
		return None, fmt.Errorf("scope %q not found", name)
	}
	return sc.logger.Level(), nil
}

// PrintRegisteredScopes logs all the registered scopes and their configured
// output levels.
func (s *Manager) PrintRegisteredScopes() {
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
		levelToString[s.logger.Level()],
		"",
	)
	for _, n := range names {
		sc := s.registry[n]
		fmt.Printf("- %-*s [%-5s]  %s\n",
			pad,
			sc.name,
			levelToString[sc.logger.Level()],
			sc.description,
		)
	}
}
