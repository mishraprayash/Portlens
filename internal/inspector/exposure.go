package inspector

import "github.com/portlens/portlens/internal/model"

// riskRank orders risk levels from least to most severe.
var riskRank = map[model.RiskLevel]int{
	model.RiskLow:       0,
	model.RiskWarning:   1,
	model.RiskDangerous: 2,
}

// assessExposure evaluates how a listener is exposed. It is deliberately
// cautious: it never declares a service "safe", only "low risk" when it is
// bound to loopback and nothing suggests otherwise.
func assessExposure(listeners []model.Listener) *model.Exposure {
	e := &model.Exposure{}
	if len(listeners) == 0 {
		e.Worst = model.RiskLow
		return e
	}

	pids := map[int32]bool{}
	for _, l := range listeners {
		switch {
		case isWildcard(l.Address):
			e.BoundWildcard = true
		case isLoopback(l.Address):
			e.BoundLocalhost = true
		case l.Address != "":
			e.PublicInterface = true
		}
		if l.PID > 0 {
			pids[l.PID] = true
		}
	}

	add := func(level model.RiskLevel, reason string) {
		e.Findings = append(e.Findings, model.Finding{Level: level, Reason: reason})
		if e.Worst == "" || riskRank[level] > riskRank[e.Worst] {
			e.Worst = level
		}
	}

	switch {
	case e.BoundWildcard && e.PublicInterface:
		add(model.RiskDangerous, "Bound to all interfaces (0.0.0.0) on a non-loopback interface; potentially reachable from the network")
	case e.BoundWildcard:
		add(model.RiskWarning, "Bound to all interfaces (0.0.0.0); reachable from other machines on the local network")
	case e.PublicInterface:
		add(model.RiskWarning, "Bound to a non-loopback interface; may be reachable from other machines")
	default:
		add(model.RiskLow, "Bound only to loopback; not reachable from other machines")
	}

	if len(pids) > 1 {
		add(model.RiskWarning, "Multiple processes are associated with this port")
	}
	return e
}

func isWildcard(addr string) bool {
	return addr == "0.0.0.0" || addr == "::" || addr == "*"
}
