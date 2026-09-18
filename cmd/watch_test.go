package cmd

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"
)

func TestDiffWatch(t *testing.T) {
	prev := watchSnap{
		"Port 3000": "up:100:node",
		"Port 4000": "down",
	}
	cur := watchSnap{
		"Port 3000": "up:100:node",
		"Port 4000": "up:200:python",
		"Port 5000": "up:300:go",
	}
	got := diffWatch(prev, cur)
	want := []watchChange{
		{kind: "up", target: "Port 4000", detail: "up:200:python"},
		{kind: "up", target: "Port 5000", detail: "up:300:go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}

func TestWaitWatchTick(t *testing.T) {
	t.Run("tick received", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		tickCh := make(chan time.Time, 1)
		tickCh <- time.Now()

		buf := &bytes.Buffer{}
		got := waitWatchTick(ctx, buf, true, tickCh)
		if !got {
			t.Errorf("waitWatchTick() = false, want true")
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output on tick, got %q", buf.String())
		}
	})

	t.Run("context cancelled interactive", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		tickCh := make(chan time.Time)
		buf := &bytes.Buffer{}
		got := waitWatchTick(ctx, buf, true, tickCh)
		if got {
			t.Errorf("waitWatchTick() = true, want false")
		}
		if buf.String() != "\n" {
			t.Errorf("expected newline output, got %q", buf.String())
		}
	})

	t.Run("context cancelled non-interactive", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		tickCh := make(chan time.Time)
		buf := &bytes.Buffer{}
		got := waitWatchTick(ctx, buf, false, tickCh)
		if got {
			t.Errorf("waitWatchTick() = true, want false")
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output, got %q", buf.String())
		}
	})
}

func TestDiffWatchDown(t *testing.T) {
	prev := watchSnap{"Port 3000": "up:1:node"}
	got := diffWatch(prev, watchSnap{"Port 3000": "down"})
	want := []watchChange{{kind: "down", target: "Port 3000"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}

func TestDiffWatchProcessChange(t *testing.T) {
	prev := watchSnap{"Port 3000": "up:1:node"}
	got := diffWatch(prev, watchSnap{"Port 3000": "up:2:node"})
	want := []watchChange{{kind: "changed", target: "Port 3000", detail: "up:2:node"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("diffWatch = %+v, want %+v", got, want)
	}
}
