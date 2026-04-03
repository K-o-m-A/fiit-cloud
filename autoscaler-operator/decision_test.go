package scaler_test

import (
	"testing"
	"time"

	"github.com/yourorg/autoscaler-operator/pkg/metrics"
	"github.com/yourorg/autoscaler-operator/pkg/scaler"
)

var baseNow = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

func baseInput() scaler.Input {
	return scaler.Input{
		CurrentReplicas:      2,
		MinReplicas:          1,
		MaxReplicas:          10,
		ScaleUpStep:          1,
		ScaleDownStep:        1,
		ScaleUpCooldownSec:   60,
		ScaleDownCooldownSec: 300,
		CPUEnabled:           true,
		CPUScaleUpPct:        80,
		CPUScaleDownPct:      20,
		MemEnabled:           true,
		MemScaleUpPct:        80,
		MemScaleDownPct:      20,
		Now:                  baseNow,
		Snapshot: &metrics.DeploymentSnapshot{
			PodCount:             2,
			AvgCPUUtilizationPct: 40,
			AvgMemUtilizationPct: 40,
		},
	}
}

func TestHold_NormalLoad(t *testing.T) {
	in := baseInput()
	d := scaler.Evaluate(in)
	if d.Direction != scaler.Hold {
		t.Errorf("expected Hold, got %s: %s", d.Direction, d)
	}
	if d.DesiredReplicas != 2 {
		t.Errorf("expected 2 replicas, got %d", d.DesiredReplicas)
	}
}

func TestScaleUp_CPUHigh(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgCPUUtilizationPct = 90
	d := scaler.Evaluate(in)
	if d.Direction != scaler.ScaleUp {
		t.Errorf("expected ScaleUp, got %s: %s", d.Direction, d)
	}
	if d.DesiredReplicas != 3 {
		t.Errorf("expected 3 replicas, got %d", d.DesiredReplicas)
	}
}

func TestScaleUp_MemHigh(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgMemUtilizationPct = 85
	d := scaler.Evaluate(in)
	if d.Direction != scaler.ScaleUp {
		t.Errorf("expected ScaleUp, got %s: %s", d.Direction, d)
	}
}

func TestScaleUp_RespectsMax(t *testing.T) {
	in := baseInput()
	in.CurrentReplicas = 10
	in.Snapshot.AvgCPUUtilizationPct = 90
	d := scaler.Evaluate(in)
	if d.Direction != scaler.Hold {
		t.Errorf("expected Hold at max, got %s", d.Direction)
	}
	if d.DesiredReplicas != 10 {
		t.Errorf("expected 10, got %d", d.DesiredReplicas)
	}
}

func TestScaleUp_StepSize(t *testing.T) {
	in := baseInput()
	in.ScaleUpStep = 3
	in.Snapshot.AvgCPUUtilizationPct = 90
	d := scaler.Evaluate(in)
	if d.DesiredReplicas != 5 { // 2 + 3
		t.Errorf("expected 5 replicas with step=3, got %d", d.DesiredReplicas)
	}
}

func TestScaleUp_StepClampedToMax(t *testing.T) {
	in := baseInput()
	in.CurrentReplicas = 9
	in.ScaleUpStep = 3
	in.Snapshot.AvgCPUUtilizationPct = 90
	d := scaler.Evaluate(in)
	if d.DesiredReplicas != 10 { // capped at max
		t.Errorf("expected 10 (capped), got %d", d.DesiredReplicas)
	}
}

func TestScaleUp_CooldownBlocks(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgCPUUtilizationPct = 90
	in.LastScaleUpTime = baseNow.Add(-30 * time.Second) // 30s ago, cooldown is 60s
	d := scaler.Evaluate(in)
	if d.Direction != scaler.Hold {
		t.Errorf("expected Hold (cooldown), got %s: %s", d.Direction, d)
	}
}

func TestScaleUp_CooldownExpired(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgCPUUtilizationPct = 90
	in.LastScaleUpTime = baseNow.Add(-90 * time.Second) // 90s ago, cooldown is 60s
	d := scaler.Evaluate(in)
	if d.Direction != scaler.ScaleUp {
		t.Errorf("expected ScaleUp after cooldown expiry, got %s", d.Direction)
	}
}

func TestScaleDown_AllMetricsLow(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgCPUUtilizationPct = 10
	in.Snapshot.AvgMemUtilizationPct = 5
	d := scaler.Evaluate(in)
	if d.Direction != scaler.ScaleDown {
		t.Errorf("expected ScaleDown, got %s: %s", d.Direction, d)
	}
	if d.DesiredReplicas != 1 {
		t.Errorf("expected 1 replica, got %d", d.DesiredReplicas)
	}
}

func TestScaleDown_OnlyOneLow_NoAction(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgCPUUtilizationPct = 10 // low
	in.Snapshot.AvgMemUtilizationPct = 50 // normal
	d := scaler.Evaluate(in)
	if d.Direction != scaler.Hold {
		t.Errorf("expected Hold (mixed metrics), got %s", d.Direction)
	}
}

func TestScaleDown_RespectsMin(t *testing.T) {
	in := baseInput()
	in.CurrentReplicas = 1
	in.Snapshot.AvgCPUUtilizationPct = 5
	in.Snapshot.AvgMemUtilizationPct = 5
	d := scaler.Evaluate(in)
	if d.Direction != scaler.Hold {
		t.Errorf("expected Hold at min, got %s", d.Direction)
	}
}

func TestScaleDown_CooldownBlocks(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgCPUUtilizationPct = 5
	in.Snapshot.AvgMemUtilizationPct = 5
	in.LastScaleDownTime = baseNow.Add(-100 * time.Second) // 100s ago, cooldown=300s
	d := scaler.Evaluate(in)
	if d.Direction != scaler.Hold {
		t.Errorf("expected Hold (down cooldown), got %s", d.Direction)
	}
}

func TestNoSnapshot_Hold(t *testing.T) {
	in := baseInput()
	in.Snapshot = nil
	d := scaler.Evaluate(in)
	if d.Direction != scaler.Hold {
		t.Errorf("expected Hold with nil snapshot, got %s", d.Direction)
	}
}

func TestMixedMetrics_CPUHighMemLow_ScalesUp(t *testing.T) {
	in := baseInput()
	in.Snapshot.AvgCPUUtilizationPct = 90 // triggers scale-up
	in.Snapshot.AvgMemUtilizationPct = 5  // would trigger scale-down alone
	d := scaler.Evaluate(in)
	// Scale-up takes precedence over scale-down
	if d.Direction != scaler.ScaleUp {
		t.Errorf("expected ScaleUp (CPU high wins), got %s: %s", d.Direction, d)
	}
}
