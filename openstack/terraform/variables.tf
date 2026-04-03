variable "auth_url" {
  description = "OpenStack Keystone auth URL"
  type        = string
}

variable "project_name" {
  description = "OpenStack project / tenant name"
  type        = string
}

variable "username" {
  description = "OpenStack username"
  type        = string
}

variable "password" {
  description = "OpenStack password"
  type        = string
  sensitive   = true
}

variable "region" {
  description = "OpenStack region"
  type        = string
  default     = "RegionOne"
}

variable "external_network_name" {
  description = "Name of the external (floating IP) network"
  type        = string
  default     = "public"
}

variable "subnet_cidr" {
  description = "CIDR for the internal k8s subnet"
  type        = string
  default     = "10.0.0.0/24"
}

variable "image_name" {
  description = "Name of the Ubuntu 24.04 image in Glance"
  type        = string
  default     = "Ubuntu-24.04"
}

variable "keypair_name" {
  description = "Name of the OpenStack keypair for SSH access"
  type        = string
}

variable "control_plane_flavor" {
  description = "Flavor for the control plane node (2 vCPU, 2-4 GB RAM)"
  type        = string
  default     = "m1.small"
}

variable "worker_flavor" {
  description = "Flavor for the worker node (2-4 vCPU, 4-8 GB RAM)"
  type        = string
  default     = "m1.medium"
}

variable "user_domain_name" {
  description = "OpenStack user domain name"
  type        = string
  default     = "Default"
}

variable "project_domain_name" {
  description = "OpenStack project domain name"
  type        = string
  default     = "Default"
}

variable "ssh_user" {
  description = "SSH username used to connect to instances"
  type        = string
  default     = "ubuntu"
}

variable "ssh_private_key_path" {
  description = "Path to private key matching the OpenStack keypair"
  type        = string
}

variable "pod_cidr" {
  description = "Pod network CIDR for cluster CNI"
  type        = string
  default     = "10.244.0.0/16"
}

variable "service_cidr" {
  description = "Service CIDR for Kubernetes cluster services"
  type        = string
  default     = "10.96.0.0/12"
}