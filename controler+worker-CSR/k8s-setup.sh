#!/bin/bash

# Kubernetes Control Plane and Node Setup Script
# Supports: Basic setup, kubeconfig registration, and CSR-based registration
# Author: DevOps Team
# Date: 2025-10-07

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
K8S_VERSION="1.34"
POD_NETWORK_CIDR="10.244.0.0/16"
SERVICE_CIDR="10.96.0.0/12"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_root() {
    if [[ $EUID -ne 0 ]]; then
        log_error "This script must be run as root"
        exit 1
    fi
}

# Function to install dependencies
install_dependencies() {
    log_info "Installing dependencies..."
    
    # Update system
    apt-get update
    apt-get install -y apt-transport-https ca-certificates curl gpg socat conntrack ipset
    
    # Install container runtime (containerd)
    apt-get install -y containerd
    
    # Configure containerd
    mkdir -p /etc/containerd
    containerd config default | tee /etc/containerd/config.toml
    sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
    systemctl restart containerd
    systemctl enable containerd
    
    # Disable swap
    swapoff -a
    sed -i '/ swap / s/^/#/' /etc/fstab
    
    # Load kernel modules
    cat <<EOF | tee /etc/modules-load.d/k8s.conf
overlay
br_netfilter
EOF
    modprobe overlay
    modprobe br_netfilter
    
    # Set sysctl params
    cat <<EOF | tee /etc/sysctl.d/k8s.conf
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF
    sysctl --system
    
    log_info "Dependencies installed successfully"
}

# Function to install kubeadm, kubelet, kubectl
install_kubernetes_tools() {
    log_info "Installing Kubernetes tools version ${K8S_VERSION}..."
    
    # Remove old repository if exists
    rm -f /etc/apt/sources.list.d/kubernetes.list
    
    # Add Kubernetes apt repository for v1.34
    mkdir -p /etc/apt/keyrings
    
    # Download GPG key
    curl -fsSL https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/Release.key | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
    
    # Add repository with proper format
    echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION}/deb/ /" | tee /etc/apt/sources.list.d/kubernetes.list
    
    # Update and install
    apt-get update 2>&1 | grep -v "Malformed entry" || true
    apt-get install -y kubelet kubeadm kubectl
    apt-mark hold kubelet kubeadm kubectl
    
    systemctl enable kubelet
    
    # Verify installation
    log_info "Installed versions:"
    kubeadm version
    
    log_info "Kubernetes tools installed successfully"
}

# LEVEL 1: Initialize Control Plane
init_control_plane() {
    log_info "Initializing Kubernetes Control Plane..."
    
    # Get the primary IP address
    PRIMARY_IP=$(hostname -I | awk '{print $1}')
    
    # Initialize cluster (with ignore preflight errors for low RAM environments)
    kubeadm init \
        --pod-network-cidr=${POD_NETWORK_CIDR} \
        --service-cidr=${SERVICE_CIDR} \
        --apiserver-advertise-address=${PRIMARY_IP} \
        --node-name=$(hostname) \
        --ignore-preflight-errors=Mem,NumCPU
    
    # Setup kubeconfig for root
    mkdir -p $HOME/.kube
    cp -f /etc/kubernetes/admin.conf $HOME/.kube/config
    chown $(id -u):$(id -g) $HOME/.kube/config
    
    # Setup kubeconfig for regular user if exists
    if [ -n "$SUDO_USER" ]; then
        mkdir -p /home/$SUDO_USER/.kube
        cp -f /etc/kubernetes/admin.conf /home/$SUDO_USER/.kube/config
        chown -R $SUDO_USER:$SUDO_USER /home/$SUDO_USER/.kube
    fi
    
    # Install Flannel CNI
    kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
    
    # Save join command
    kubeadm token create --print-join-command > /root/join-command.sh
    chmod +x /root/join-command.sh
    
    log_info "Control Plane initialized successfully!"
    log_info "Join command saved to /root/join-command.sh"
}

# LEVEL 2: Setup kubeconfig for worker node registration
setup_worker_kubeconfig() {
    log_info "Setting up worker node with kubeconfig..."
    
    read -p "Enter the path to kubeconfig file: " KUBECONFIG_PATH
    
    if [ ! -f "$KUBECONFIG_PATH" ]; then
        log_error "Kubeconfig file not found: $KUBECONFIG_PATH"
        exit 1
    fi
    
    # Copy kubeconfig
    mkdir -p $HOME/.kube
    cp -f "$KUBECONFIG_PATH" $HOME/.kube/config
    chown $(id -u):$(id -g) $HOME/.kube/config
    
    # Get join command from control plane
    read -p "Enter control plane IP: " CONTROL_PLANE_IP
    read -p "Enter join token: " JOIN_TOKEN
    read -p "Enter discovery token CA cert hash: " CA_CERT_HASH
    
    # Join cluster
    kubeadm join ${CONTROL_PLANE_IP}:6443 \
        --token ${JOIN_TOKEN} \
        --discovery-token-ca-cert-hash sha256:${CA_CERT_HASH}
    
    log_info "Worker node joined successfully using kubeconfig!"
}

# LEVEL 3: Advanced CSR-based registration
setup_worker_csr() {
    log_info "Setting up worker node with CSR-based authentication..."
    
    NODE_NAME=$(hostname)
    read -p "Enter control plane API server URL (e.g., https://10.0.1.93:6443): " API_SERVER
    
    # Generate private key
    openssl genrsa -out /etc/kubernetes/pki/worker-${NODE_NAME}.key 2048
    
    # Generate CSR
    cat <<EOF > /tmp/csr.conf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${NODE_NAME}
IP.1 = $(hostname -I | awk '{print $1}')
EOF
    
    openssl req -new \
        -key /etc/kubernetes/pki/worker-${NODE_NAME}.key \
        -out /tmp/worker-${NODE_NAME}.csr \
        -subj "/CN=system:node:${NODE_NAME}/O=system:nodes" \
        -config /tmp/csr.conf
    
    # Create Kubernetes CSR object
    cat <<EOF > /tmp/k8s-csr.yaml
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: node-csr-${NODE_NAME}
spec:
  request: $(cat /tmp/worker-${NODE_NAME}.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF
    
    log_info "CSR created. Please apply this on control plane:"
    echo "----------------------------------------"
    cat /tmp/k8s-csr.yaml
    echo "----------------------------------------"
    log_info "On control plane, run:"
    echo "kubectl apply -f csr.yaml"
    echo "kubectl certificate approve node-csr-${NODE_NAME}"
    echo "kubectl get csr node-csr-${NODE_NAME} -o jsonpath='{.status.certificate}' | base64 -d > worker-${NODE_NAME}.crt"
    
    read -p "Press Enter after certificate is approved and downloaded..."
    
    read -p "Enter path to approved certificate: " CERT_PATH
    
    if [ ! -f "$CERT_PATH" ]; then
        log_error "Certificate file not found"
        exit 1
    fi
    
    # Copy certificate
    cp "$CERT_PATH" /etc/kubernetes/pki/worker-${NODE_NAME}.crt
    
    # Create kubeconfig with certificate
    kubectl config set-cluster kubernetes \
        --server=${API_SERVER} \
        --certificate-authority=/etc/kubernetes/pki/ca.crt \
        --embed-certs=true \
        --kubeconfig=/etc/kubernetes/kubelet.conf
    
    kubectl config set-credentials system:node:${NODE_NAME} \
        --client-certificate=/etc/kubernetes/pki/worker-${NODE_NAME}.crt \
        --client-key=/etc/kubernetes/pki/worker-${NODE_NAME}.key \
        --embed-certs=true \
        --kubeconfig=/etc/kubernetes/kubelet.conf
    
    kubectl config set-context default \
        --cluster=kubernetes \
        --user=system:node:${NODE_NAME} \
        --kubeconfig=/etc/kubernetes/kubelet.conf
    
    kubectl config use-context default --kubeconfig=/etc/kubernetes/kubelet.conf
    
    # Configure kubelet
    cat <<EOF > /var/lib/kubelet/config.yaml
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: false
  webhook:
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
clusterDomain: cluster.local
clusterDNS:
  - 10.96.0.10
EOF
    
    systemctl restart kubelet
    
    log_info "Worker node configured with CSR-based authentication!"
}

# Main menu
show_menu() {
    echo ""
    echo "========================================="
    echo "Kubernetes Cluster Setup Script"
    echo "========================================="
    echo "1. Install dependencies (required first)"
    echo "2. Install Kubernetes tools"
    echo "3. Initialize Control Plane (Level 1)"
    echo "4. Join as Worker with kubeconfig (Level 2)"
    echo "5. Join as Worker with CSR (Level 3 - Advanced)"
    echo "6. Full Control Plane setup (1+2+3)"
    echo "7. Full Worker setup with kubeconfig (1+2+4)"
    echo "8. Exit"
    echo "========================================="
}

main() {
    check_root
    
    while true; do
        show_menu
        read -p "Select option: " choice
        
        case $choice in
            1)
                install_dependencies
                ;;
            2)
                install_kubernetes_tools
                ;;
            3)
                init_control_plane
                ;;
            4)
                setup_worker_kubeconfig
                ;;
            5)
                setup_worker_csr
                ;;
            6)
                install_dependencies
                install_kubernetes_tools
                init_control_plane
                ;;
            7)
                install_dependencies
                install_kubernetes_tools
                setup_worker_kubeconfig
                ;;
            8)
                log_info "Exiting..."
                exit 0
                ;;
            *)
                log_error "Invalid option"
                ;;
        esac
    done
}

# Run main function
main
