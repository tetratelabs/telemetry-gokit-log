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

package logger_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/tetratelabs/telemetry"
	log "github.com/tetratelabs/telemetry-gokit-log"
)

func TestSyncLogfmt(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := log.NewSyncLogfmt(buf)
	logger.Info("hello")
	if want, have := "msg=hello level=info\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}
	buf.Reset()
	logger.Debug("hello")
	if want, have := "", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	logger.SetLevel(telemetry.LevelDebug)
	if logger.Level() != telemetry.LevelDebug {
		t.Errorf("want %v, have %v", logger.Level(), telemetry.LevelDebug)
	}
	buf.Reset()
	logger.Debug("hello")
	if want, have := "msg=hello level=debug\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}
	buf.Reset()
	logger.Debug("ok", "a", 1, "err", errors.New("error"))
	if want, have := "msg=ok level=debug a=1 err=error\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	cloned := logger.Clone()
	l := cloned.With()
	buf.Reset()
	l.Error("error", errors.New("error"))
	if want, have := "msg=error level=error error=error\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	l = cloned.With("key")
	buf.Reset()
	l.Error("error", errors.New("error"))
	if want, have := "msg=error level=error error=error key=(MISSING)\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	l = cloned.With("key", "val")
	buf.Reset()
	l.Error("error", errors.New("error"))
	if want, have := "msg=error level=error error=error key=val\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	ctx := telemetry.KeyValuesToContext(context.Background(), "ctx", "val")
	buf.Reset()
	withCtx := l.Context(ctx)
	withCtx.Error("error", errors.New("error"))
	if want, have := "msg=error level=error error=error ctx=val key=val\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	unwrapped := logger.UnwrapLogger()
	buf.Reset()
	unwrapped.Log("unwrapped", "logger")
	if want, have := "unwrapped=logger\n", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}

	o := logger.New()
	o.SetLevel(telemetry.LevelError)
	buf.Reset()
	o.Debug("silence")
	if want, have := "", buf.String(); want != have {
		t.Errorf("want %#v, have %#v", want, have)
	}
}
