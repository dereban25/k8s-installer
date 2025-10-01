.PHONY: build install clean test run help

# Variables
BINARY_NAME=k8s-installer
BUILD_DIR=build
GO=go
GOFLAGS=-v

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the installer binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/installer

install: build ## Install the binary to /usr/local/bin
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)

run: build ## Build and run the installer
	@echo "Running $(BINARY_NAME)..."
	sudo $(BUILD_DIR)/$(BINARY_NAME)

run-verbose: build ## Build and run with verbose output
	@echo "Running $(BINARY_NAME) with verbose output..."
	sudo $(BUILD_DIR)/$(BINARY_NAME) -verbose

test: ## Run tests
	@echo "Running tests..."
	$(GO) test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests and show coverage
	@echo "Generating coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt: ## Format code
	@echo "Formatting code..."
	$(GO) fmt ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GO) vet ./...

lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running linter..."
	golangci-lint run

clean: ## Clean build artifacts and Kubernetes installation
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Cleaning Kubernetes installation..."
	sudo rm -rf ./kubebuilder ./etcd
	sudo rm -rf /var/lib/kubelet
	sudo rm -rf /etc/kubernetes
	sudo rm -rf /var/log/kubernetes
	sudo rm -rf /etc/containerd/config.toml
	sudo rm -rf /opt/cni
	sudo rm -rf /tmp/ca.* /tmp/sa.* /tmp/token.csv

clean-logs: ## Clean only log files
	@echo "Cleaning log files..."
	sudo rm -rf /var/log/kubernetes/*

logs: ## View all Kubernetes logs
	@echo "Viewing Kubernetes logs..."
	@echo "=== ETCD ==="
	@sudo tail -20 /var/log/kubernetes/etcd.log 2>/dev/null || echo "No etcd logs"
	@echo ""
	@echo "=== API SERVER ==="
	@sudo tail -20 /var/log/kubernetes/apiserver.log 2>/dev/null || echo "No apiserver logs"
	@echo ""
	@echo "=== SCHEDULER ==="
	@sudo tail -20 /var/log/kubernetes/scheduler.log 2>/dev/null || echo "No scheduler logs"

logs-apiserver: ## View API server logs
	@sudo tail -f /var/log/kubernetes/apiserver.log

logs-etcd: ## View etcd logs
	@sudo tail -f /var/log/kubernetes/etcd.log

logs-all: ## View all logs in real-time
	@sudo tail -f /var/log/kubernetes/*.log

diagnose: ## Run diagnostics
	@chmod +x scripts/diagnose.sh
	@sudo ./scripts/diagnose.sh

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod verify

verify: ## Verify the Kubernetes cluster
	@echo "Verifying Kubernetes cluster..."
	./kubebuilder/bin/kubectl get nodes
	./kubebuilder/bin/kubectl get pods -A

create-deployment: ## Create a test nginx deployment
	@echo "Creating test deployment..."
	./kubebuilder/bin/kubectl create deployment nginx --image=nginx:latest
	sleep 10
	./kubebuilder/bin/kubectl get deployments
	./kubebuilder/bin/kubectl get pods

all: clean deps build test ## Clean, download deps, build, and test

.DEFAULT_GOAL := help