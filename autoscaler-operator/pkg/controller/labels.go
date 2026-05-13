// Package controller implements the label-driven autoscaler reconciliation loop.
package controller

// Label keys that users apply to their Deployments to opt in to autoscaling.
// All keys live under the "autoscaler.fiit-cloud.io" prefix to avoid collisions.
const (
	// LabelEnabled opts the Deployment into the autoscaler. Must be "true".
	//   autoscaler.fiit-cloud.io/enabled: "true"
	LabelEnabled = "autoscaler.fiit-cloud.io/enabled"

	// --- Replica bounds ---

	// AnnotationMinReplicas is the minimum number of replicas allowed. Default: "1".
	AnnotationMinReplicas = "autoscaler.fiit-cloud.io/min-replicas"

	// AnnotationMaxReplicas is the maximum number of replicas allowed. Required.
	AnnotationMaxReplicas = "autoscaler.fiit-cloud.io/max-replicas"

	// --- Scale step sizes ---

	// AnnotationScaleUpStep is how many replicas to add per scale-up event. Default: "1".
	AnnotationScaleUpStep = "autoscaler.fiit-cloud.io/scale-up-step"

	// AnnotationScaleDownStep is how many replicas to remove per scale-down event. Default: "1".
	AnnotationScaleDownStep = "autoscaler.fiit-cloud.io/scale-down-step"

	// --- Cooldown windows (seconds) ---

	// AnnotationScaleUpCooldown is minimum seconds between consecutive scale-ups. Default: "60".
	AnnotationScaleUpCooldown = "autoscaler.fiit-cloud.io/scale-up-cooldown"

	// AnnotationScaleDownCooldown is minimum seconds between consecutive scale-downs. Default: "300".
	AnnotationScaleDownCooldown = "autoscaler.fiit-cloud.io/scale-down-cooldown"

	// --- CPU thresholds (percentage of requests) ---

	// AnnotationCPUScaleUp triggers a scale-up when avg CPU utilization exceeds this %. Default: "80".
	AnnotationCPUScaleUp = "autoscaler.fiit-cloud.io/cpu-scale-up-threshold"

	// AnnotationCPUScaleDown triggers a scale-down when avg CPU utilization falls below this %. Default: "20".
	AnnotationCPUScaleDown = "autoscaler.fiit-cloud.io/cpu-scale-down-threshold"

	// AnnotationCPUEnabled enables/disables CPU-based scaling. Default: "true".
	AnnotationCPUEnabled = "autoscaler.fiit-cloud.io/cpu-enabled"

	// --- Memory thresholds (percentage of requests) ---

	// AnnotationMemScaleUp triggers a scale-up when avg memory utilization exceeds this %. Default: "80".
	AnnotationMemScaleUp = "autoscaler.fiit-cloud.io/mem-scale-up-threshold"

	// AnnotationMemScaleDown triggers a scale-down when avg memory utilization falls below this %. Default: "20".
	AnnotationMemScaleDown = "autoscaler.fiit-cloud.io/mem-scale-down-threshold"

	// AnnotationMemEnabled enables/disables memory-based scaling. Default: "true".
	AnnotationMemEnabled = "autoscaler.fiit-cloud.io/mem-enabled"

	// --- Requests-per-second thresholds (queried from Prometheus) ---

	// AnnotationRPSEnabled enables/disables RPS-based scaling. Default: "false" (opt-in).
	AnnotationRPSEnabled = "autoscaler.fiit-cloud.io/rps-enabled"

	// AnnotationRPSScaleUp triggers a scale-up when avg per-pod RPS exceeds this value.
	AnnotationRPSScaleUp = "autoscaler.fiit-cloud.io/rps-scale-up-threshold"

	// AnnotationRPSScaleDown triggers a scale-down when avg per-pod RPS falls below this value.
	AnnotationRPSScaleDown = "autoscaler.fiit-cloud.io/rps-scale-down-threshold"
)
