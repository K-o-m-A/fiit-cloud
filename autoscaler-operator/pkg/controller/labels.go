package controller

const (
	LabelEnabled = "autoscaler.fiit-cloud.io/enabled"

	AnnotationMinReplicas = "autoscaler.fiit-cloud.io/min-replicas"

	AnnotationMaxReplicas = "autoscaler.fiit-cloud.io/max-replicas"

	AnnotationScaleUpStep = "autoscaler.fiit-cloud.io/scale-up-step"

	AnnotationScaleDownStep = "autoscaler.fiit-cloud.io/scale-down-step"

	AnnotationScaleUpCooldown = "autoscaler.fiit-cloud.io/scale-up-cooldown"

	AnnotationScaleDownCooldown = "autoscaler.fiit-cloud.io/scale-down-cooldown"

	AnnotationCPUScaleUp = "autoscaler.fiit-cloud.io/cpu-scale-up-threshold"

	AnnotationCPUScaleDown = "autoscaler.fiit-cloud.io/cpu-scale-down-threshold"

	AnnotationCPUEnabled = "autoscaler.fiit-cloud.io/cpu-enabled"

	AnnotationMemScaleUp = "autoscaler.fiit-cloud.io/mem-scale-up-threshold"

	AnnotationMemScaleDown = "autoscaler.fiit-cloud.io/mem-scale-down-threshold"

	AnnotationMemEnabled = "autoscaler.fiit-cloud.io/mem-enabled"

	AnnotationRPSEnabled = "autoscaler.fiit-cloud.io/rps-enabled"

	AnnotationRPSScaleUp = "autoscaler.fiit-cloud.io/rps-scale-up-threshold"

	AnnotationRPSScaleDown = "autoscaler.fiit-cloud.io/rps-scale-down-threshold"
)
