# ==============================================================================
# fiit-cloud — Cross-Platform Setup Makefile
# Targets: Windows, Linux, macOS
# Tools installed: minikube, kubectl, helm
# Cluster: fiit-cloud | 4 CPU | 4096MB RAM
# Stack: kube-prometheus-stack (Prometheus + Grafana + Alertmanager)
# make install                  # Install only the tools
# make start-cluster            # Start fiit-cloud cluster
# make install-prometheus-stack # Install Prometheus + Grafana
# make grafana-open             # Open Grafana in browser (minikube tunnel)
# make grafana-creds            # Print admin credentials
# make prometheus-open          # Open Prometheus UI
# make status                   # Cluster + pod health overview
# make clean                    # Stop cluster, uninstall stack
# make nuke                     # Permanently delete cluster + stack



# ==============================================================================

CLUSTER_NAME   := fiit-cloud
MEMORY         := 4096
CPUS           := 4
DRIVER         := docker
NAMESPACE      := monitoring
HELM_RELEASE   := prometheus-stack
HELM_REPO_NAME := prometheus-community
HELM_REPO_URL  := https://prometheus-community.github.io/helm-charts
HELM_CHART     := prometheus-community/kube-prometheus-stack

# ── OS Detection ──────────────────────────────────────────────────────────────
ifeq ($(OS),Windows_NT)
  DETECTED_OS := windows
  # Check if actually running inside WSL
  ifneq ($(shell uname -r 2>/dev/null | grep -i microsoft),)
    DETECTED_OS := linux
  endif
else
  UNAME_S := $(shell uname -s)
  ifeq ($(UNAME_S),Linux)
    DETECTED_OS := linux
  endif
  ifeq ($(UNAME_S),Darwin)
    DETECTED_OS := darwin
  endif
endif

# ── Architecture Detection ─────────────────────────────────────────────────────
UNAME_M := $(shell uname -m 2>/dev/null || echo x86_64)
ifeq ($(UNAME_M),arm64)
  ARCH := arm64
else ifeq ($(UNAME_M),aarch64)
  ARCH := arm64
else
  ARCH := amd64
endif

# ── Dynamic kubectl version ────────────────────────────────────────────────────
# Fetch latest stable version; falls back to v1.32.0 if curl unavailable
KUBECTL_VERSION := $(shell curl -fsSL https://dl.k8s.io/release/stable.txt 2>/dev/null || echo v1.32.0)

# ── Download URLs ──────────────────────────────────────────────────────────────
KUBECTL_URL_LINUX   := https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/linux/$(ARCH)/kubectl
KUBECTL_URL_DARWIN  := https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/darwin/$(ARCH)/kubectl
KUBECTL_URL_WINDOWS := https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/windows/$(ARCH)/kubectl.exe

MINIKUBE_URL_LINUX   := https://github.com/kubernetes/minikube/releases/latest/download/minikube-linux-$(ARCH)
MINIKUBE_URL_DARWIN  := https://github.com/kubernetes/minikube/releases/latest/download/minikube-darwin-$(ARCH)
MINIKUBE_URL_WINDOWS := https://github.com/kubernetes/minikube/releases/latest/download/minikube-windows-amd64.exe

# ==============================================================================
# DEFAULT — show help
# ==============================================================================

.DEFAULT_GOAL := help

# ==============================================================================
# FULL SETUP PIPELINE
# ==============================================================================

.PHONY: all
all: install start-cluster install-prometheus-stack ## Full setup: install tools → start cluster → install Prometheus stack
	@echo ""
	@echo "=========================================="
	@echo " ✅  fiit-cloud is ready!"
	@echo "=========================================="
	@echo " make status         — cluster status"
	@echo " make grafana-open   — open Grafana UI"
	@echo " make grafana-creds  — print credentials"
	@echo "=========================================="

# ==============================================================================
# INSTALL TOOLS
# ==============================================================================

.PHONY: install
install: install-minikube install-kubectl install-helm ## Install minikube, kubectl, and helm

# ── minikube ──────────────────────────────────────────────────────────────────
.PHONY: install-minikube
install-minikube: ## Install minikube for the current OS/arch
	@echo ">>> Installing minikube [$(DETECTED_OS)/$(ARCH)]..."
ifeq ($(DETECTED_OS),linux)
	@if command -v minikube >/dev/null 2>&1; then \
	  echo "  minikube already installed: $$(minikube version --short)"; \
	else \
	  curl -fsSLo /tmp/minikube $(MINIKUBE_URL_LINUX) && \
	  sudo install /tmp/minikube /usr/local/bin/minikube && \
	  rm -f /tmp/minikube; \
	fi
endif
ifeq ($(DETECTED_OS),darwin)
	@if command -v minikube >/dev/null 2>&1; then \
	  echo "  minikube already installed: $$(minikube version --short)"; \
	elif command -v brew >/dev/null 2>&1; then \
	  brew install minikube; \
	else \
	  curl -fsSLo /tmp/minikube $(MINIKUBE_URL_DARWIN) && \
	  chmod +x /tmp/minikube && \
	  sudo mv /tmp/minikube /usr/local/bin/minikube; \
	fi
endif
ifeq ($(DETECTED_OS),windows)
	@where minikube >nul 2>&1 || ( \
	  powershell -NoProfile -ExecutionPolicy Bypass -Command \
	  "New-Item -ItemType Directory -Force -Path 'C:\Program Files\Minikube' | Out-Null; \
	   Invoke-WebRequest -Uri '$(MINIKUBE_URL_WINDOWS)' \
	     -OutFile 'C:\Program Files\Minikube\minikube.exe'; \
	   [Environment]::SetEnvironmentVariable('Path', \
	     [Environment]::GetEnvironmentVariable('Path','Machine') + \
	     ';C:\Program Files\Minikube','Machine')" \
	)
	@echo   NOTE: Re-open your terminal so the PATH change takes effect.
endif
	@echo "  ✔ minikube ready"

# ── kubectl ───────────────────────────────────────────────────────────────────
.PHONY: install-kubectl
install-kubectl: ## Install kubectl for the current OS/arch
	@echo ">>> Installing kubectl $(KUBECTL_VERSION) [$(DETECTED_OS)/$(ARCH)]..."
ifeq ($(DETECTED_OS),linux)
	@if command -v kubectl >/dev/null 2>&1; then \
	  echo "  kubectl already installed: $$(kubectl version --client --short 2>/dev/null)"; \
	else \
	  curl -fsSLo /tmp/kubectl $(KUBECTL_URL_LINUX) && \
	  sudo install -o root -g root -m 0755 /tmp/kubectl /usr/local/bin/kubectl && \
	  rm -f /tmp/kubectl; \
	fi
endif
ifeq ($(DETECTED_OS),darwin)
	@if command -v kubectl >/dev/null 2>&1; then \
	  echo "  kubectl already installed: $$(kubectl version --client --short 2>/dev/null)"; \
	elif command -v brew >/dev/null 2>&1; then \
	  brew install kubectl; \
	else \
	  curl -fsSLo /tmp/kubectl $(KUBECTL_URL_DARWIN) && \
	  chmod +x /tmp/kubectl && \
	  sudo mv /tmp/kubectl /usr/local/bin/kubectl; \
	fi
endif
ifeq ($(DETECTED_OS),windows)
	@where kubectl >nul 2>&1 || ( \
	  powershell -NoProfile -ExecutionPolicy Bypass -Command \
	  "New-Item -ItemType Directory -Force -Path 'C:\Program Files\kubectl' | Out-Null; \
	   Invoke-WebRequest -Uri '$(KUBECTL_URL_WINDOWS)' \
	     -OutFile 'C:\Program Files\kubectl\kubectl.exe'; \
	   [Environment]::SetEnvironmentVariable('Path', \
	     [Environment]::GetEnvironmentVariable('Path','Machine') + \
	     ';C:\Program Files\kubectl','Machine')" \
	)
	@echo   NOTE: Re-open your terminal so the PATH change takes effect.
endif
	@echo "  ✔ kubectl ready"

# ── helm ──────────────────────────────────────────────────────────────────────
.PHONY: install-helm
install-helm: ## Install Helm v3 for the current OS
	@echo ">>> Installing helm [$(DETECTED_OS)]..."
ifeq ($(DETECTED_OS),linux)
	@if command -v helm >/dev/null 2>&1; then \
	  echo "  helm already installed: $$(helm version --short)"; \
	else \
	  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash; \
	fi
endif
ifeq ($(DETECTED_OS),darwin)
	@if command -v helm >/dev/null 2>&1; then \
	  echo "  helm already installed: $$(helm version --short)"; \
	elif command -v brew >/dev/null 2>&1; then \
	  brew install helm; \
	else \
	  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash; \
	fi
endif
ifeq ($(DETECTED_OS),windows)
	@where helm >nul 2>&1 || ( \
	  powershell -NoProfile -ExecutionPolicy Bypass -Command \
	  "$$progressPreference = 'silentlyContinue'; \
	   $$latest = (Invoke-RestMethod 'https://api.github.com/repos/helm/helm/releases/latest').tag_name; \
	   $$url = 'https://get.helm.sh/helm-' + $$latest + '-windows-amd64.zip'; \
	   Invoke-WebRequest -Uri $$url -OutFile C:\helm-setup.zip; \
	   Expand-Archive -Path C:\helm-setup.zip -DestinationPath C:\helm-tmp -Force; \
	   New-Item -ItemType Directory -Force -Path 'C:\Program Files\Helm' | Out-Null; \
	   Copy-Item C:\helm-tmp\windows-amd64\helm.exe 'C:\Program Files\Helm\helm.exe'; \
	   Remove-Item -Recurse -Force C:\helm-tmp, C:\helm-setup.zip; \
	   [Environment]::SetEnvironmentVariable('Path', \
	     [Environment]::GetEnvironmentVariable('Path','Machine') + \
	     ';C:\Program Files\Helm','Machine')" \
	)
	@echo   NOTE: Re-open your terminal so the PATH change takes effect.
endif
	@echo "  ✔ helm ready"

# ==============================================================================
# CLUSTER LIFECYCLE
# ==============================================================================

.PHONY: start-cluster
start-cluster: ## Start the fiit-cloud minikube cluster (4 CPU / 4096MB RAM)
	@echo ">>> Starting minikube cluster '$(CLUSTER_NAME)'..."
	@minikube start \
		--profile=$(CLUSTER_NAME) \
		--driver=$(DRIVER) \
		--memory=$(MEMORY) \
		--cpus=$(CPUS) \
		--wait=all
	@kubectl config use-context $(CLUSTER_NAME)
	@echo "  ✔ Cluster '$(CLUSTER_NAME)' started"

.PHONY: stop-cluster
stop-cluster: ## Stop the cluster (state preserved)
	@echo ">>> Stopping cluster '$(CLUSTER_NAME)'..."
	@minikube stop --profile=$(CLUSTER_NAME)

.PHONY: delete-cluster
delete-cluster: ## Delete the cluster entirely
	@echo ">>> Deleting cluster '$(CLUSTER_NAME)'..."
	@minikube delete --profile=$(CLUSTER_NAME)

.PHONY: status
status: ## Show cluster, node, and monitoring pod status
	@echo "=== Minikube Status ==="
	@minikube status --profile=$(CLUSTER_NAME) || true
	@echo ""
	@echo "=== Nodes ==="
	@kubectl get nodes -o wide || true
	@echo ""
	@echo "=== Pods in namespace '$(NAMESPACE)' ==="
	@kubectl get pods -n $(NAMESPACE) 2>/dev/null || echo "  (namespace '$(NAMESPACE)' not found yet)"

# ==============================================================================
# PROMETHEUS STACK
# ==============================================================================

.PHONY: install-prometheus-stack
install-prometheus-stack: ## Install kube-prometheus-stack 
	@echo ">>> Adding Helm repo '$(HELM_REPO_NAME)'..."
	@helm repo add $(HELM_REPO_NAME) $(HELM_REPO_URL) 2>/dev/null || true
	@helm repo update
	@echo ">>> Ensuring namespace '$(NAMESPACE)' exists..."
	@kubectl get namespace $(NAMESPACE) >/dev/null 2>&1 || \
		kubectl create namespace $(NAMESPACE)
	@echo ">>> Installing $(HELM_CHART) as release '$(HELM_RELEASE)'..."
	@helm upgrade --install $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(NAMESPACE) \
		--set grafana.enabled=true \
		--set grafana.service.type=NodePort \
		--set grafana.adminPassword=fiit-admin \
		--set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
		--set prometheus.service.type=NodePort \
		--set alertmanager.service.type=NodePort \
		--wait \
		--timeout=10m
	@echo "  ✔ kube-prometheus-stack installed in namespace '$(NAMESPACE)'"

.PHONY: upgrade-prometheus-stack
upgrade-prometheus-stack:
	@helm repo update
	@helm upgrade $(HELM_RELEASE) $(HELM_CHART) \
		--namespace $(NAMESPACE) \
		--reuse-values \
		--wait \
		--timeout=10m

.PHONY: uninstall-prometheus-stack
uninstall-prometheus-stack: ## Uninstall prometheus stack and remove CRDs
	@helm uninstall $(HELM_RELEASE) --namespace $(NAMESPACE) 2>/dev/null || true
	@kubectl delete crd \
		alertmanagerconfigs.monitoring.coreos.com \
		alertmanagers.monitoring.coreos.com \
		podmonitors.monitoring.coreos.com \
		probes.monitoring.coreos.com \
		prometheusagents.monitoring.coreos.com \
		prometheuses.monitoring.coreos.com \
		prometheusrules.monitoring.coreos.com \
		scrapeconfigs.monitoring.coreos.com \
		servicemonitors.monitoring.coreos.com \
		thanosrulers.monitoring.coreos.com \
		2>/dev/null || true
	@kubectl delete namespace $(NAMESPACE) --ignore-not-found

# ==============================================================================
# GRAFANA / PROMETHEUS HELPERS
# ==============================================================================

.PHONY: grafana-open
grafana-open: ## Open Grafana in your browser (via minikube service tunnel)
	@minikube service $(HELM_RELEASE)-grafana \
		--namespace=$(NAMESPACE) \
		--profile=$(CLUSTER_NAME)

.PHONY: grafana-creds
grafana-creds: ## Print Grafana admin username and password
	@echo "=== Grafana Credentials ==="
	@echo "  URL:      run 'make grafana-open'"
	@echo "  Username: admin"
	@echo -n "  Password: "
	@kubectl get secret --namespace $(NAMESPACE) $(HELM_RELEASE)-grafana \
		-o jsonpath="{.data.admin-password}" | base64 --decode
	@echo ""

.PHONY: prometheus-open
prometheus-open: ## Open Prometheus UI in your browser
	@minikube service $(HELM_RELEASE)-kube-prometheus-stack-prometheus \
		--namespace=$(NAMESPACE) \
		--profile=$(CLUSTER_NAME)

.PHONY: alertmanager-open
alertmanager-open: ## Open Alertmanager UI in your browser
	@minikube service $(HELM_RELEASE)-kube-prometheus-stack-alertmanager \
		--namespace=$(NAMESPACE) \
		--profile=$(CLUSTER_NAME)

# ==============================================================================
# TEARDOWN
# ==============================================================================

.PHONY: clean
clean: uninstall-prometheus-stack stop-cluster ## Uninstall stack and stop cluster (data preserved)

.PHONY: nuke
nuke: uninstall-prometheus-stack delete-cluster ## Uninstall stack and permanently DELETE cluster

# ==============================================================================
# HELP
# ==============================================================================

.PHONY: help
help: ## Show all available targets
	@echo ""
	@echo "  fiit-cloud Makefile"
	@echo "  Detected OS   : $(DETECTED_OS)"
	@echo "  Detected Arch : $(ARCH)"
	@echo ""
	@echo "  Usage: make <target>"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; \
		{printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}'
	@echo ""