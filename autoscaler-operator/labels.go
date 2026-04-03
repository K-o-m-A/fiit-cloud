// Package controller implements the label-driven autoscaler reconciliation loop.
package controller

// Label keys that users apply to their Deployments to opt in to autoscaling.
// All keys live under the "autoscaler.yourorg.io" prefix to avoid collisions.
const (
	// LabelEnabled opts the Deployment into the autoscaler. Must be "true".
	//   autoscaler.yourorg.io/enabled: "true"
	LabelEnabled = "autoscaler.yourorg.io/enabled"

	// --- Replica bounds ---

	// AnnotationMinReplicas is the minimum number of replicas allowed. Default: "1".
	AnnotationMinReplicas = "autoscaler.yourorg.io/min-replicas"

	// AnnotationMaxReplicas is the maximum number of replicas allowed. Required.
	AnnotationMaxReplicas = "autoscaler.yourorg.io/max-replicas"

	// --- Scale step sizes ---

	// AnnotationScaleUpStep is how many replicas to add per scale-up event. Default: "1".
	AnnotationScaleUpStep = "autoscaler.yourorg.io/scale-up-step"

	// AnnotationScaleDownStep is how many replicas to remove per scale-down event. Default: "1".
	AnnotationScaleDownStep = "autoscaler.yourorg.io/scale-down-step"

	// --- Cooldown windows (seconds) ---

	// AnnotationScaleUpCooldown is minimum seconds between consecutive scale-ups. Default: "60".
	AnnotationScaleUpCooldown = "autoscaler.yourorg.io/scale-up-cooldown"

	// AnnotationScaleDownCooldown is minimum seconds between consecutive scale-downs. Default: "300".
	AnnotationScaleDownCooldown = "autoscaler.yourorg.io/scale-down-cooldown"

	// --- CPU thresholds (percentage of requests) ---

	// AnnotationCPUScaleUp triggers a scale-up when avg CPU utilization exceeds this %. Default: "80".
	AnnotationCPUScaleUp = "autoscaler.yourorg.io/cpu-scale-up-threshold"

	// AnnotationCPUScaleDown triggers a scale-down when avg CPU utilization falls below this %. Default: "20".
	AnnotationCPUScaleDown = "autoscaler.yourorg.io/cpu-scale-down-threshold"

	// AnnotationCPUEnabled enables/disables CPU-based scaling. Default: "true".
	AnnotationCPUEnabled = "autoscaler.yourorg.io/cpu-enabled"

	// --- Memory thresholds (percentage of requests) ---

	// AnnotationMemScaleUp triggers a scale-up when avg memory utilization exceeds this %. Default: "80".
	AnnotationMemScaleUp = "autoscaler.yourorg.io/mem-scale-up-threshold"

	// AnnotationMemScaleDown triggers a scale-down when avg memory utilization falls below this %. Default: "20".
	AnnotationMemScaleDown = "autoscaler.yourorg.io/mem-scale-down-threshold"

	// AnnotationMemEnabled enables/disables memory-based scaling. Default: "true".
	AnnotationMemEnabled = "autoscaler.yourorg.io/mem-enabled"
)
