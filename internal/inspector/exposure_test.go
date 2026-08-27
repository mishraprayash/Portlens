package inspector

import (
	"testing"

	"github.com/portlens/portlens/internal/model"
)

func TestAssessExposureLoopback(t *testing.T) {
	e := assessExposure([]model.Listener{{Address: "127.0.0.1", Port: 3000, PID: 1}})
	if e.Worst != model.RiskLow {
		t.Errorf("worst = %q, want LOW RISK", e.Worst)
	}
	if !e.BoundLocalhost || e.BoundWildcard || e.PublicInterface {
		t.Errorf("exposure flags = %+v", e)
	}
}

func TestAssessExposureWildcard(t *testing.T) {
	e := assessExposure([]model.Listener{{Address: "0.0.0.0", Port: 3000, PID: 1}})
	if e.Worst != model.RiskWarning {
		t.Errorf("worst = %q, want WARNING", e.Worst)
	}
	if !e.BoundWildcard {
		t.Errorf("expected wildcard flag")
	}
}

func TestAssessExposurePublicInterface(t *testing.T) {
	e := assessExposure([]model.Listener{{Address: "192.168.1.10", Port: 3000, PID: 1}})
	if !e.PublicInterface {
		t.Errorf("expected public interface flag")
	}
}

func TestAssessExposureMultipleProcesses(t *testing.T) {
	e := assessExposure([]model.Listener{
		{Address: "127.0.0.1", Port: 3000, PID: 10},
		{Address: "127.0.0.1", Port: 3000, PID: 11},
	})
	found := false
	for _, f := range e.Findings {
		if f.Level == model.RiskWarning {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a warning about multiple processes, got %+v", e.Findings)
	}
}
