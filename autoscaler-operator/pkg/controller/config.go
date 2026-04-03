package controller

import (
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
)

// DeploymentConfig is the parsed scaling configuration derived from a
// Deployment's labels and annotations.
type DeploymentConfig struct {
	MinReplicas int32
	MaxReplicas int32

	ScaleUpStep   int32
	ScaleDownStep int32

	ScaleUpCooldownSec   int64
	ScaleDownCooldownSec int64

	CPUEnabled      bool
	CPUScaleUpPct   int32
	CPUScaleDownPct int32

	MemEnabled      bool
	MemScaleUpPct   int32
	MemScaleDownPct int32
}

// ParseDeploymentConfig reads annotations from a Deployment and returns the
// resolved configuration with safe defaults.
func ParseDeploymentConfig(d *appsv1.Deployment) (*DeploymentConfig, error) {
	ann := d.Annotations
	if ann == nil {
		ann = map[string]string{}
	}

	cfg := &DeploymentConfig{
		MinReplicas:          getInt32(ann, AnnotationMinReplicas, 1),
		MaxReplicas:          getInt32(ann, AnnotationMaxReplicas, 0), // 0 = not set
		ScaleUpStep:          getInt32(ann, AnnotationScaleUpStep, 1),
		ScaleDownStep:        getInt32(ann, AnnotationScaleDownStep, 1),
		ScaleUpCooldownSec:   getInt64(ann, AnnotationScaleUpCooldown, 60),
		ScaleDownCooldownSec: getInt64(ann, AnnotationScaleDownCooldown, 300),

		CPUEnabled:      getBool(ann, AnnotationCPUEnabled, true),
		CPUScaleUpPct:   getInt32(ann, AnnotationCPUScaleUp, 80),
		CPUScaleDownPct: getInt32(ann, AnnotationCPUScaleDown, 20),

		MemEnabled:      getBool(ann, AnnotationMemEnabled, true),
		MemScaleUpPct:   getInt32(ann, AnnotationMemScaleUp, 80),
		MemScaleDownPct: getInt32(ann, AnnotationMemScaleDown, 20),
	}

	if err := cfg.validate(d.Name); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *DeploymentConfig) validate(name string) error {
	if c.MaxReplicas == 0 {
		return fmt.Errorf("deployment %q: annotation %s is required", name, AnnotationMaxReplicas)
	}
	if c.MinReplicas > c.MaxReplicas {
		return fmt.Errorf("deployment %q: minReplicas (%d) > maxReplicas (%d)", name, c.MinReplicas, c.MaxReplicas)
	}
	if c.CPUEnabled && c.CPUScaleDownPct >= c.CPUScaleUpPct {
		return fmt.Errorf("deployment %q: CPU scaleDown threshold (%d) must be < scaleUp (%d)",
			name, c.CPUScaleDownPct, c.CPUScaleUpPct)
	}
	if c.MemEnabled && c.MemScaleDownPct >= c.MemScaleUpPct {
		return fmt.Errorf("deployment %q: memory scaleDown threshold (%d) must be < scaleUp (%d)",
			name, c.MemScaleDownPct, c.MemScaleUpPct)
	}
	return nil
}

// --- helpers ---

func getInt32(ann map[string]string, key string, def int32) int32 {
	v, ok := ann[key]
	if !ok {
		return def
	}
	i, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return def
	}
	return int32(i)
}

func getInt64(ann map[string]string, key string, def int64) int64 {
	v, ok := ann[key]
	if !ok {
		return def
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return i
}

func getBool(ann map[string]string, key string, def bool) bool {
	v, ok := ann[key]
	if !ok {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
