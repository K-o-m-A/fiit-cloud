// Package scaler contains the pure scaling decision logic.
// It is intentionally free of Kubernetes client dependencies so it can be
// unit-tested without a real or fake cluster.
package scaler

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourorg/autoscaler-operator/pkg/metrics"
)

// Direction is the outcome of an Evaluate call.
type Direction int

const (
	Hold      Direction = iota // No change needed.
	ScaleUp                    // Increase replicas.
	ScaleDown                  // Decrease replicas.
)

func (d Direction) String() string {
	switch d {
	case ScaleUp:
		return "ScaleUp"
	case ScaleDown:
		return "ScaleDown"
	default:
		return "Hold"
	}
}

// Reason captures why a particular direction was chosen.
type Reason struct {
	Metric    string
	Observed  string
	Threshold string
}

func (r Reason) String() string {
	return fmt.Sprintf("%s observed=%s threshold=%s", r.Metric, r.Observed, r.Threshold)
}

// Decision is the fully resolved output of Evaluate.
type Decision struct {
	Direction       Direction
	DesiredReplicas int32
	Reasons         []Reason
}

func (d Decision) String() string {
	if len(d.Reasons) == 0 {
		return fmt.Sprintf("%s → %d replicas (no metric pressure)", d.Direction, d.DesiredReplicas)
	}
	parts := make([]string, len(d.Reasons))
	for i, r := range d.Reasons {
		parts[i] = r.String()
	}
	return fmt.Sprintf("%s → %d replicas [%s]", d.Direction, d.DesiredReplicas, strings.Join(parts, "; "))
}

// Input bundles everything Evaluate needs.
type Input struct {
	// Current state
	CurrentReplicas   int32
	LastScaleUpTime   time.Time
	LastScaleDownTime time.Time

	// Bounds & steps
	MinReplicas   int32
	MaxReplicas   int32
	ScaleUpStep   int32
	ScaleDownStep int32

	// Cooldowns
	ScaleUpCooldownSec   int64
	ScaleDownCooldownSec int64

	// Thresholds
	CPUEnabled      bool
	CPUScaleUpPct   int32
	CPUScaleDownPct int32

	MemEnabled      bool
	MemScaleUpPct   int32
	MemScaleDownPct int32

	// Observed values (use -1 to signal "not available")
	Snapshot *metrics.DeploymentSnapshot

	// Now is injected for testability.
	Now time.Time
}

// Evaluate returns the scaling Decision for the given Input.
// Rules:
//  1. If ANY metric is above its scale-up threshold → ScaleUp (unless at max or in cooldown).
//  2. If ALL active metrics are below their scale-down threshold → ScaleDown
//     (unless at min or in cooldown).
//  3. Otherwise → Hold.
func Evaluate(in Input) Decision {
	if in.Now.IsZero() {
		in.Now = time.Now()
	}
	if in.ScaleUpStep <= 0 {
		in.ScaleUpStep = 1
	}
	if in.ScaleDownStep <= 0 {
		in.ScaleDownStep = 1
	}

	snap := in.Snapshot
	if snap == nil || snap.PodCount == 0 {
		return Decision{Direction: Hold, DesiredReplicas: in.CurrentReplicas}
	}

	var (
		scaleUpReasons []Reason
		belowDownCount int
		activeMetrics  int
	)

	// --- CPU ---
	if in.CPUEnabled && snap.AvgCPUUtilizationPct >= 0 {
		activeMetrics++
		obs := snap.AvgCPUUtilizationPct
		switch {
		case obs >= in.CPUScaleUpPct:
			scaleUpReasons = append(scaleUpReasons, Reason{
				Metric:    "CPU",
				Observed:  fmt.Sprintf("%d%%", obs),
				Threshold: fmt.Sprintf(">=%d%%", in.CPUScaleUpPct),
			})
		case obs < in.CPUScaleDownPct:
			belowDownCount++
		}
	}

	// --- Memory ---
	if in.MemEnabled && snap.AvgMemUtilizationPct >= 0 {
		activeMetrics++
		obs := snap.AvgMemUtilizationPct
		switch {
		case obs >= in.MemScaleUpPct:
			scaleUpReasons = append(scaleUpReasons, Reason{
				Metric:    "Memory",
				Observed:  fmt.Sprintf("%d%%", obs),
				Threshold: fmt.Sprintf(">=%d%%", in.MemScaleUpPct),
			})
		case obs < in.MemScaleDownPct:
			belowDownCount++
		}
	}

	// --- Scale UP decision ---
	if len(scaleUpReasons) > 0 {
		if in.CurrentReplicas >= in.MaxReplicas {
			return Decision{Direction: Hold, DesiredReplicas: in.MaxReplicas,
				Reasons: append(scaleUpReasons, Reason{Metric: "guard", Observed: "at max replicas"})}
		}
		if !in.LastScaleUpTime.IsZero() {
			cooldownEnd := in.LastScaleUpTime.Add(time.Duration(in.ScaleUpCooldownSec) * time.Second)
			if in.Now.Before(cooldownEnd) {
				return Decision{Direction: Hold, DesiredReplicas: in.CurrentReplicas,
					Reasons: append(scaleUpReasons, Reason{
						Metric:    "cooldown",
						Observed:  "scale-up cooldown active",
						Threshold: fmt.Sprintf("ends at %s", cooldownEnd.Format(time.RFC3339)),
					})}
			}
		}
		desired := min32(in.CurrentReplicas+in.ScaleUpStep, in.MaxReplicas)
		return Decision{Direction: ScaleUp, DesiredReplicas: desired, Reasons: scaleUpReasons}
	}

	// --- Scale DOWN decision: ALL active metrics must be below their down threshold ---
	if activeMetrics > 0 && belowDownCount == activeMetrics {
		if in.CurrentReplicas <= in.MinReplicas {
			return Decision{Direction: Hold, DesiredReplicas: in.MinReplicas}
		}
		if !in.LastScaleDownTime.IsZero() {
			cooldownEnd := in.LastScaleDownTime.Add(time.Duration(in.ScaleDownCooldownSec) * time.Second)
			if in.Now.Before(cooldownEnd) {
				return Decision{Direction: Hold, DesiredReplicas: in.CurrentReplicas,
					Reasons: []Reason{{
						Metric:    "cooldown",
						Observed:  "scale-down cooldown active",
						Threshold: fmt.Sprintf("ends at %s", cooldownEnd.Format(time.RFC3339)),
					}}}
			}
		}
		desired := max32(in.CurrentReplicas-in.ScaleDownStep, in.MinReplicas)
		return Decision{
			Direction:       ScaleDown,
			DesiredReplicas: desired,
			Reasons: []Reason{{
				Metric:    "all metrics",
				Observed:  "all below scale-down threshold",
				Threshold: "unanimous down-vote required",
			}},
		}
	}

	return Decision{Direction: Hold, DesiredReplicas: in.CurrentReplicas}
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func max32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
