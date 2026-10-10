package detect

import (
	"context"
	"testing"
	"time"

	"github.com/mishraprayash/Portlens/internal/model"
)

type countingDetector struct {
	calls int
}

func (c *countingDetector) Detect(_ context.Context, cwd string) *model.ProjectInfo {
	c.calls++
	if cwd == "/src/app" {
		return &model.ProjectInfo{Directory: cwd, Name: "app"}
	}
	return nil
}

func TestMemoProjectDetectorCachesPerCWD(t *testing.T) {
	inner := &countingDetector{}
	base := time.Now()
	clock := base
	m := Memoize(inner).(*memoProjectDetector)
	m.now = func() time.Time { return clock }

	ctx := context.Background()
	if got := m.Detect(ctx, "/src/app"); got == nil || got.Name != "app" {
		t.Fatalf("Detect = %+v, want app", got)
	}
	if got := m.Detect(ctx, "/src/app/"); got == nil { // cleaned to the same key
		t.Fatal("Detect(same cwd) = nil, want cached hit")
	}
	if inner.calls != 1 {
		t.Errorf("inner calls for one cwd = %d, want 1", inner.calls)
	}

	if got := m.Detect(ctx, "/other"); got != nil {
		t.Errorf("Detect(other) = %+v, want nil", got)
	}
	if inner.calls != 2 {
		t.Errorf("inner calls for second cwd = %d, want 2", inner.calls)
	}
	if got := m.Detect(ctx, "/other"); got != nil {
		t.Errorf("Detect(negative cached) = %+v, want nil", got)
	}
	if inner.calls != 2 {
		t.Errorf("inner calls after negative hit = %d, want 2 (nil cached too)", inner.calls)
	}

	clock = base.Add(projectMemoTTL + time.Millisecond)
	m.Detect(ctx, "/src/app")
	if inner.calls != 3 {
		t.Errorf("inner calls after TTL = %d, want 3", inner.calls)
	}
}
