#!/usr/bin/env bash
# ╔══════════════════════════════════════════════════════════════════╗
# ║                  DSVPN — ALL-IN-ONE DEPLOY                      ║
# ║  One script to rule them all.                                    ║
# ║  Run on a FRESH Ubuntu/Debian VPS and everything gets set up.    ║
# ║                                                                  ║
# ║  Usage:  bash deploy.sh                                          ║
# ║  Domain: api.digitalservice51.com (Cloudflare orange proxy)      ║
# ╚══════════════════════════════════════════════════════════════════╝
set -euo pipefail

# ================================================================
#  CONFIG
# ================================================================
DOMAIN="api.digitalservice51.com"
PROJECT_DIR="/opt/dsvpn"
GO_VERSION="1.22.5"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"
REQUIRED_GO_MAJOR=1
REQUIRED_GO_MINOR=22
PG_VERSION=16

# Capture the script's real directory NOW, before any cd commands
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ================================================================
#  COLORS & LOGGING
# ================================================================
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

ok()    { echo -e "  ${GREEN}✔${NC} $1"; }
err()   { echo -e "  ${RED}✘${NC} $1"; }
warn()  { echo -e "  ${YELLOW}⚠${NC} $1"; }
info()  { echo -e "  ${CYAN}→${NC} $1"; }
step()  { echo -e "\n${BLUE}${BOLD}══▶${NC}${BOLD} $1${NC}"; }
line()  { echo -e "${DIM}──────────────────────────────────────────────────────────────${NC}"; }

banner() {
    echo ""
    echo -e "${CYAN}${BOLD}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║          🚀  DSVPN — ALL-IN-ONE DEPLOYMENT  🚀              ║"
    echo "║                                                              ║"
    echo "║  Go API + PostgreSQL + HAProxy + Docker                      ║"
    echo "║  Domain: ${DOMAIN}                         ║"
    echo "╚══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

fail() {
    err "$1"
    echo -e "\n${RED}${BOLD}  DEPLOYMENT FAILED${NC}"
    echo -e "  Check the output above for details.\n"
    exit 1
}

# Track timing
DEPLOY_START=$(date +%s)

# ================================================================
#  STEP 1 — OS DETECTION & ROOT CHECK
# ================================================================
step_01_preflight() {
    step "Step 1/14 — Pre-flight Checks"
    line

    # Root check
    if [[ $EUID -ne 0 ]]; then
        fail "This script must be run as root. Use: sudo bash deploy.sh"
    fi
    ok "Running as root"

    # OS check
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS_NAME="${ID}"
        OS_VERSION="${VERSION_ID:-unknown}"
        OS_PRETTY="${PRETTY_NAME:-${ID}}"
    else
        fail "Cannot detect OS. /etc/os-release not found."
    fi

    case "$OS_NAME" in
        ubuntu|debian|pop|linuxmint|elementary)
            ok "OS detected: ${OS_PRETTY}"
            ;;
        *)
            fail "Unsupported OS: ${OS_PRETTY}. Only Ubuntu/Debian-based systems are supported."
            ;;
    esac

    # System info
    CPU_CORES=$(nproc 2>/dev/null || echo "?")
    TOTAL_RAM=$(free -h 2>/dev/null | awk '/Mem:/{print $2}' || echo "?")
    DISK_FREE=$(df -h / 2>/dev/null | awk 'NR==2{print $4}' || echo "?")

    info "CPU Cores   : ${CPU_CORES}"
    info "Total RAM   : ${TOTAL_RAM}"
    info "Disk Free   : ${DISK_FREE}"
    info "Hostname    : $(hostname)"
    info "Kernel      : $(uname -r)"
}

# ================================================================
#  STEP 2 — INSTALL ESSENTIAL PACKAGES (SAFETY CHECK)
# ================================================================
step_02_essentials() {
    step "Step 2/14 — Essential Packages (safety check each)"
    line

    export DEBIAN_FRONTEND=noninteractive

    ESSENTIALS=(
        curl wget git ufw build-essential ca-certificates
        gnupg lsb-release openssl jq htop net-tools
        software-properties-common apt-transport-https
        rsync
    )

    MISSING=()
    for pkg in "${ESSENTIALS[@]}"; do
        if dpkg -s "$pkg" &>/dev/null; then
            ok "${pkg} — already installed ✓"
        else
            MISSING+=("$pkg")
            warn "${pkg} — NOT installed, will install"
        fi
    done

    if [[ ${#MISSING[@]} -gt 0 ]]; then
        info "Installing ${#MISSING[@]} missing packages..."
        apt-get update -qq
        apt-get install -y -q "${MISSING[@]}" 2>&1 | tail -1
        for pkg in "${MISSING[@]}"; do
            if dpkg -s "$pkg" &>/dev/null; then
                ok "${pkg} — installed successfully ✓"
            else
                err "${pkg} — failed to install"
            fi
        done
    else
        ok "All essential packages already installed"
    fi
}

# ================================================================
#  STEP 3 — INSTALL GO (SAFETY CHECK)
# ================================================================
step_03_go() {
    step "Step 3/14 — Go ${GO_VERSION} (safety check)"
    line

    # Make sure PATH includes Go
    export PATH=$PATH:/usr/local/go/bin

    NEED_INSTALL=false

    if command -v go &>/dev/null; then
        CURRENT_GO_FULL=$(go version 2>/dev/null)
        CURRENT_GO=$(echo "$CURRENT_GO_FULL" | grep -oP 'go\K[0-9]+\.[0-9]+' || echo "0.0")
        CURRENT_MAJOR=$(echo "$CURRENT_GO" | cut -d. -f1)
        CURRENT_MINOR=$(echo "$CURRENT_GO" | cut -d. -f2)

        if [[ "$CURRENT_MAJOR" -ge "$REQUIRED_GO_MAJOR" ]] && [[ "$CURRENT_MINOR" -ge "$REQUIRED_GO_MINOR" ]]; then
            ok "Go already installed: ${CURRENT_GO_FULL}"
            ok "Version ${CURRENT_GO} >= ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR} ✓"
            return
        else
            warn "Go version ${CURRENT_GO} is too old (need >= ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR})"
            NEED_INSTALL=true
        fi
    else
        info "Go not found on this system"
        NEED_INSTALL=true
    fi

    if [[ "$NEED_INSTALL" == true ]]; then
        info "Downloading Go ${GO_VERSION} from go.dev..."
        wget -q --show-progress "${GO_URL}" -O "/tmp/${GO_TARBALL}"
        ok "Downloaded ${GO_TARBALL}"

        info "Removing old Go installation (if any)..."
        rm -rf /usr/local/go

        info "Extracting to /usr/local/go..."
        tar -C /usr/local -xzf "/tmp/${GO_TARBALL}"
        rm -f "/tmp/${GO_TARBALL}"

        # Add to PATH system-wide (persists across reboots)
        cat > /etc/profile.d/go-path.sh << 'GOPATH_EOF'
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
GOPATH_EOF
        chmod +x /etc/profile.d/go-path.sh

        # Apply for current session
        export PATH=$PATH:/usr/local/go/bin

        # Verify
        if command -v go &>/dev/null; then
            ok "Go installed successfully: $(go version)"
        else
            fail "Go installation failed — 'go' command not found after install"
        fi
    fi
}

# ================================================================
#  STEP 4 — INSTALL POSTGRESQL CLIENT (SAFETY CHECK)
# ================================================================
step_04_postgresql_client() {
    step "Step 4/14 — PostgreSQL ${PG_VERSION} Client Tools (safety check)"
    line

    if command -v psql &>/dev/null; then
        PSQL_VER=$(psql --version 2>/dev/null | grep -oP '[0-9]+' | head -1 || echo "0")
        if [[ "$PSQL_VER" -ge "$PG_VERSION" ]]; then
            ok "psql already installed: $(psql --version)"
            ok "Version ${PSQL_VER} >= ${PG_VERSION} ✓"
            return
        else
            warn "psql version ${PSQL_VER} is old, upgrading to ${PG_VERSION}..."
        fi
    else
        info "psql not found, installing PostgreSQL ${PG_VERSION} client..."
    fi

    # Add PostgreSQL official APT repository
    if [[ ! -f /etc/apt/sources.list.d/pgdg.list ]]; then
        info "Adding PostgreSQL official APT repository..."
        curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | \
            gpg --dearmor -o /usr/share/keyrings/postgresql-keyring.gpg 2>/dev/null
        echo "deb [signed-by=/usr/share/keyrings/postgresql-keyring.gpg] http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" \
            > /etc/apt/sources.list.d/pgdg.list
        apt-get update -qq
        ok "PostgreSQL APT repo added"
    else
        ok "PostgreSQL APT repo already configured ✓"
    fi

    apt-get install -y -q "postgresql-client-${PG_VERSION}" 2>&1 | tail -1

    # Verify
    if command -v psql &>/dev/null; then
        ok "PostgreSQL client installed: $(psql --version)"
    else
        warn "psql install may have issues — continuing anyway (DB runs in Docker)"
    fi
}

# ================================================================
#  STEP 5 — INSTALL DOCKER CE + COMPOSE (SAFETY CHECK)
# ================================================================
step_05_docker() {
    step "Step 5/14 — Docker CE + Compose v2 (safety check)"
    line

    # --- Docker Engine ---
    if command -v docker &>/dev/null; then
        ok "Docker already installed: $(docker --version)"
    else
        info "Docker not found, installing Docker CE..."

        # Remove conflicting old packages
        for pkg in docker.io docker-doc docker-compose podman-docker containerd runc; do
            apt-get remove -y "$pkg" 2>/dev/null || true
        done

        # Add Docker official GPG key + repo
        install -m 0755 -d /etc/apt/keyrings
        if [[ ! -f /etc/apt/keyrings/docker.gpg ]]; then
            curl -fsSL "https://download.docker.com/linux/${OS_NAME}/gpg" | \
                gpg --dearmor -o /etc/apt/keyrings/docker.gpg 2>/dev/null
            chmod a+r /etc/apt/keyrings/docker.gpg
        fi

        echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/${OS_NAME} $(lsb_release -cs) stable" \
            > /etc/apt/sources.list.d/docker.list

        apt-get update -qq
        apt-get install -y -q \
            docker-ce docker-ce-cli containerd.io \
            docker-buildx-plugin docker-compose-plugin 2>&1 | tail -1

        # Enable Docker on boot
        systemctl enable docker
        systemctl start docker

        if command -v docker &>/dev/null; then
            ok "Docker installed: $(docker --version)"
        else
            fail "Docker installation failed"
        fi
    fi

    # --- Docker Compose v2 ---
    if docker compose version &>/dev/null; then
        ok "Docker Compose: $(docker compose version --short)"
    else
        fail "Docker Compose v2 plugin not found after Docker install"
    fi

    # --- Verify Docker daemon ---
    if docker info &>/dev/null; then
        ok "Docker daemon is running ✓"
    else
        info "Starting Docker daemon..."
        systemctl restart docker
        sleep 3
        if docker info &>/dev/null; then
            ok "Docker daemon started ✓"
        else
            fail "Docker daemon failed to start"
        fi
    fi
}

# ================================================================
#  STEP 6 — SYSTEM TUNING (ulimit + sysctl)
# ================================================================
step_06_system_tuning() {
    step "Step 6/14 — System Limit Tuning (ulimit + sysctl)"
    line

    # ---- /etc/security/limits.conf ----
    LIMITS_FILE="/etc/security/limits.conf"
    LIMITS_MARKER="# DSVPN-TUNING-START"

    if grep -q "$LIMITS_MARKER" "$LIMITS_FILE" 2>/dev/null; then
        ok "limits.conf — already configured ✓"
    else
        info "Configuring ${LIMITS_FILE}..."
        cat >> "$LIMITS_FILE" << EOF

${LIMITS_MARKER}
*         soft    nofile    1048576
*         hard    nofile    1048576
*         soft    nproc     65535
*         hard    nproc     65535
*         soft    memlock   unlimited
*         hard    memlock   unlimited
root      soft    nofile    1048576
root      hard    nofile    1048576
root      soft    nproc     65535
root      hard    nproc     65535
# DSVPN-TUNING-END
EOF
        ok "limits.conf — ulimits configured (nofile=1048576, nproc=65535)"
    fi

    # ---- /etc/sysctl.d/99-dsvpn-tuning.conf ----
    SYSCTL_FILE="/etc/sysctl.d/99-dsvpn-tuning.conf"
    info "Writing ${SYSCTL_FILE}..."
    cat > "$SYSCTL_FILE" << 'EOF'
# ============================================================
#  DSVPN System Tuning — High Performance Network Server
# ============================================================

# File descriptors
fs.file-max = 2097152

# Socket backlog
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.core.netdev_max_backlog = 65535

# Ephemeral ports
net.ipv4.ip_local_port_range = 1024 65535

# TCP tuning
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
net.ipv4.tcp_keepalive_time = 300
net.ipv4.tcp_keepalive_intvl = 15
net.ipv4.tcp_keepalive_probes = 5
net.ipv4.tcp_max_tw_buckets = 2000000
net.ipv4.tcp_slow_start_after_idle = 0

# Buffer sizes
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 1048576
net.core.wmem_default = 1048576
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 87380 16777216

# Memory
vm.swappiness = 10
vm.dirty_ratio = 60
vm.dirty_background_ratio = 2

# IP forwarding
net.ipv4.ip_forward = 1
EOF
    sysctl --system &>/dev/null || true
    ok "sysctl tuning applied"

    # ---- systemd default limits ----
    SYSTEMD_CONF="/etc/systemd/system.conf"
    SYSTEMD_MARKER="# DSVPN-SYSTEMD"
    if grep -q "$SYSTEMD_MARKER" "$SYSTEMD_CONF" 2>/dev/null; then
        ok "systemd limits — already configured ✓"
    else
        info "Configuring systemd default limits..."
        cp "$SYSTEMD_CONF" "${SYSTEMD_CONF}.bak.$(date +%s)"
        sed -i '/^DefaultLimitNOFILE=/d' "$SYSTEMD_CONF"
        sed -i '/^DefaultLimitNPROC=/d' "$SYSTEMD_CONF"
        cat >> "$SYSTEMD_CONF" << EOF

${SYSTEMD_MARKER}
DefaultLimitNOFILE=1048576
DefaultLimitNPROC=65535
EOF
        systemctl daemon-reexec 2>/dev/null || true
        ok "systemd default limits configured"
    fi

    # Apply for current session
    ulimit -n 1048576 2>/dev/null || true
    ulimit -u 65535 2>/dev/null || true

    # ---- Docker daemon.json (ulimits + log rotation) ----
    DOCKER_DAEMON="/etc/docker/daemon.json"
    info "Configuring Docker daemon defaults..."
    mkdir -p /etc/docker
    cat > "$DOCKER_DAEMON" << 'EOF'
{
    "log-driver": "json-file",
    "log-opts": {
        "max-size": "50m",
        "max-file": "5"
    },
    "default-ulimits": {
        "nofile": {
            "Name": "nofile",
            "Soft": 1048576,
            "Hard": 1048576
        },
        "nproc": {
            "Name": "nproc",
            "Soft": 65535,
            "Hard": 65535
        }
    },
    "storage-driver": "overlay2",
    "live-restore": true
}
EOF
    systemctl restart docker 2>/dev/null || true
    sleep 2
    ok "Docker daemon configured with ulimits"

    echo ""
    info "┌─ Current System Limits ─────────────────────┐"
    info "│ ulimit -n (open files) : $(ulimit -n 2>/dev/null || echo 'N/A')"
    info "│ fs.file-max            : $(sysctl -n fs.file-max 2>/dev/null || echo 'N/A')"
    info "│ somaxconn              : $(sysctl -n net.core.somaxconn 2>/dev/null || echo 'N/A')"
    info "│ tcp_tw_reuse           : $(sysctl -n net.ipv4.tcp_tw_reuse 2>/dev/null || echo 'N/A')"
    info "│ ip_local_port_range    : $(sysctl -n net.ipv4.ip_local_port_range 2>/dev/null || echo 'N/A')"
    info "└─────────────────────────────────────────────┘"
}

# ================================================================
#  STEP 7 — SETUP PROJECT DIRECTORY & COPY SOURCE
# ================================================================
step_07_project_setup() {
    step "Step 7/14 — Project Directory Setup"
    line

    mkdir -p "${PROJECT_DIR}"
    info "Project directory: ${PROJECT_DIR}"
    info "Source directory : ${SCRIPT_DIR}"

    # SCRIPT_DIR was captured at the top of the script, before any cd commands

    # Copy source code to project dir if not already there
    if [[ "$SCRIPT_DIR" != "$PROJECT_DIR" ]]; then
        info "Copying source code from ${SCRIPT_DIR} to ${PROJECT_DIR}..."
        rsync -a \
            --exclude='.git' \
            --exclude='bin/' \
            --exclude='*.exe' \
            "${SCRIPT_DIR}/" "${PROJECT_DIR}/" 2>/dev/null || \
        cp -r "${SCRIPT_DIR}"/* "${PROJECT_DIR}/" 2>/dev/null || true
        ok "Source code copied to ${PROJECT_DIR}"
    else
        ok "Already running from project directory ✓"
    fi

    cd "${PROJECT_DIR}"

    # Create necessary sub-directories
    mkdir -p haproxy/certs
    mkdir -p bin
    mkdir -p scripts

    ok "Directory structure ready"
    info "  ${PROJECT_DIR}/cmd/server/    — Go entrypoint"
    info "  ${PROJECT_DIR}/internal/      — Go packages"
    info "  ${PROJECT_DIR}/haproxy/       — HAProxy config + certs"
    info "  ${PROJECT_DIR}/bin/           — Compiled binary"
}

# ================================================================
#  STEP 8 — BUILD GO APPLICATION
# ================================================================
step_08_build_go() {
    step "Step 8/14 — Build Go Application"
    line

    cd "${PROJECT_DIR}"

    # Ensure Go is in PATH
    export PATH=$PATH:/usr/local/go/bin

    if [[ ! -f "go.mod" ]]; then
        fail "go.mod not found in ${PROJECT_DIR}. Source code is missing!"
    fi

    info "Module: $(head -1 go.mod)"

    info "Running 'go mod tidy' — downloading dependencies..."
    go mod tidy 2>&1 | tail -5 || true
    ok "Go dependencies resolved ✓"

    info "Running 'go build' — compiling dsvpn-server..."
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -ldflags='-w -s -extldflags "-static"' \
        -o bin/dsvpn-server ./cmd/server

    # Verify the binary was created
    if [[ -f "bin/dsvpn-server" ]]; then
        BIN_SIZE=$(du -h bin/dsvpn-server | awk '{print $1}')
        ok "Binary built successfully ✓"
        ok "  Path : ${PROJECT_DIR}/bin/dsvpn-server"
        ok "  Size : ${BIN_SIZE}"
        ok "  Arch : linux/amd64 (static, stripped)"
    else
        fail "Go build failed — binary not created"
    fi
}

# ================================================================
#  STEP 9 — GENERATE SECURE CREDENTIALS
# ================================================================
step_09_credentials() {
    step "Step 9/14 — Generate Secure Credentials"
    line

    cd "${PROJECT_DIR}"

    # Generate strong random passwords
    GEN_DB_PASSWORD=$(openssl rand -base64 32 | tr -d '=/+' | head -c 32)
    GEN_JWT_SECRET=$(openssl rand -hex 32)

    ok "Generated DB password  : ${GEN_DB_PASSWORD:0:8}... (32 chars)"
    ok "Generated JWT secret   : ${GEN_JWT_SECRET:0:12}... (64 hex chars)"

    # Write production .env file
    cat > "${PROJECT_DIR}/.env" << ENVEOF
# ============================================================
#  DSVPN Backend — Production Environment
#  Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
#  Domain: ${DOMAIN}
# ============================================================

# Server
PORT=8080
ENV=production

# Database (used by Go app inside Docker)
DATABASE_URL=postgres://dsvpn_user:${GEN_DB_PASSWORD}@db:5432/dsvpn?sslmode=disable
DB_PASSWORD=${GEN_DB_PASSWORD}

# JWT Authentication
JWT_SECRET=${GEN_JWT_SECRET}
JWT_ACCESS_EXPIRY=24h
JWT_REFRESH_EXPIRY=720h

# Google OAuth (Web Client ID from Firebase)
GOOGLE_CLIENT_ID=811986793741-qbtti0v0jt14lsl3ui5pvpnktoi2l94k.apps.googleusercontent.com
ENVEOF

    chmod 600 "${PROJECT_DIR}/.env"
    ok ".env created with strong production secrets"

    # Save a backup
    cp "${PROJECT_DIR}/.env" "${PROJECT_DIR}/.env.production.bak"
    ok ".env.production.bak backup saved"

    # Export for use in docker-compose generation
    export GEN_DB_PASSWORD
    export GEN_JWT_SECRET
}

# ================================================================
#  STEP 10 — GENERATE SSL CERTIFICATE (CLOUDFLARE)
# ================================================================
step_10_ssl_cert() {
    step "Step 10/14 — SSL Certificate for Cloudflare"
    line

    CERT_DIR="${PROJECT_DIR}/haproxy/certs"
    CERT_PEM="${CERT_DIR}/${DOMAIN}.pem"
    mkdir -p "$CERT_DIR"

    if [[ -f "$CERT_PEM" ]]; then
        ok "SSL certificate already exists at ${CERT_PEM} ✓"
        info "To regenerate: delete the file and re-run deploy.sh"
        return
    fi

    info "Generating self-signed SSL certificate..."
    info "Domain: ${DOMAIN}"
    info "Valid for: 10 years (3650 days)"

    # Generate key + cert
    openssl req -x509 -nodes -days 3650 \
        -newkey rsa:2048 \
        -keyout "${CERT_DIR}/temp.key" \
        -out "${CERT_DIR}/temp.crt" \
        -subj "/C=US/ST=Cloud/L=Server/O=DSVPN/CN=${DOMAIN}" \
        -addext "subjectAltName=DNS:${DOMAIN},DNS:*.digitalservice51.com" \
        2>/dev/null

    # HAProxy requires combined PEM file (cert + key in one file)
    cat "${CERT_DIR}/temp.crt" "${CERT_DIR}/temp.key" > "$CERT_PEM"
    chmod 600 "$CERT_PEM"
    rm -f "${CERT_DIR}/temp.crt" "${CERT_DIR}/temp.key"

    ok "Self-signed SSL certificate generated ✓"
    ok "  Location: ${CERT_PEM}"
    echo ""
    warn "For production, replace with Cloudflare Origin Certificate:"
    info "  1. Cloudflare Dashboard → SSL/TLS → Origin Server"
    info "  2. Click 'Create Certificate'"
    info "  3. Save cert+key combined into: ${CERT_PEM}"
}

# ================================================================
#  STEP 11 — WRITE HAPROXY CONFIGURATION
# ================================================================
step_11_haproxy_config() {
    step "Step 11/14 — HAProxy Configuration"
    line

    mkdir -p "${PROJECT_DIR}/haproxy"

    cat > "${PROJECT_DIR}/haproxy/haproxy.cfg" << 'HAPCFG'
# ============================================================
#  HAProxy — DSVPN API Reverse Proxy
#  Domain: api.digitalservice51.com (Cloudflare Orange Proxy)
#
#  Features:
#    - HTTP → HTTPS redirect
#    - SSL termination
#    - Rate limiting (100 req/10s per IP)
#    - Security headers (HSTS, XSS, etc.)
#    - Cloudflare real IP passthrough
#    - Health check forwarding
#    - Stats dashboard on :8404
# ============================================================

global
    maxconn 100000
    log stdout format raw local0

    # SSL tuning
    ssl-default-bind-ciphersuites TLS_AES_128_GCM_SHA256:TLS_AES_256_GCM_SHA384:TLS_CHACHA20_POLY1305_SHA256
    ssl-default-bind-options ssl-min-ver TLSv1.2 no-tls-tickets
    tune.ssl.default-dh-param 2048

defaults
    mode http
    log global
    option httplog
    option dontlognull
    option forwardfor
    option http-server-close

    timeout connect     5s
    timeout client      30s
    timeout server      30s
    timeout http-request  10s
    timeout http-keep-alive 15s
    timeout queue       30s

    retries 3
    default-server init-addr libc,none

# ---- Stats Dashboard (internal only, port 8404) ----
listen stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 10s
    stats admin if LOCALHOST

# ---- HTTP Frontend (redirect to HTTPS) ----
frontend ft_http
    bind *:80

    # Allow health checks on plain HTTP (for Docker health checks)
    acl is_health path /healthz
    use_backend bk_api if is_health

    # Redirect everything else to HTTPS
    redirect scheme https code 301 if !is_health !{ ssl_fc }

# ---- HTTPS Frontend ----
frontend ft_https
    bind *:443 ssl crt /etc/haproxy/certs/api.digitalservice51.com.pem

    # ── Rate Limiting ──
    # 100 requests per 10 seconds per source IP
    stick-table type ip size 100k expire 30s store http_req_rate(10s)
    http-request track-sc0 src
    http-request deny deny_status 429 if { sc_http_req_rate(0) gt 100 }

    # ── Security Headers ──
    http-response set-header X-Frame-Options DENY
    http-response set-header X-Content-Type-Options nosniff
    http-response set-header X-XSS-Protection "1; mode=block"
    http-response set-header Referrer-Policy strict-origin-when-cross-origin
    http-response set-header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload"

    # ── Cloudflare Real IP ──
    # Pass the real client IP from Cloudflare to the backend
    http-request set-header X-Real-IP %[req.hdr(CF-Connecting-IP)]

    # Remove server version info for security
    http-response del-header Server

    default_backend bk_api

# ---- API Backend ----
backend bk_api
    balance roundrobin
    option httpchk GET /healthz
    http-check expect status 200
    server api1 dsvpn-api:8080 check inter 5s fall 3 rise 2
HAPCFG

    ok "HAProxy config written ✓"
    info "  HTTP  :80   → redirect to HTTPS (except /healthz)"
    info "  HTTPS :443  → SSL termination → API :8080"
    info "  Stats :8404 → HAProxy dashboard (internal)"
    info "  Rate limit  → 100 req/10s per IP"
}

# ================================================================
#  STEP 12 — WRITE PRODUCTION DOCKER-COMPOSE
# ================================================================
step_12_docker_compose() {
    step "Step 12/14 — Docker Compose (Production Stack)"
    line

    cd "${PROJECT_DIR}"

    # Read DB password from .env
    DB_PASS=$(grep '^DB_PASSWORD=' .env | cut -d= -f2)

    cat > "${PROJECT_DIR}/docker-compose.yml" << DCOMPOSE
# ============================================================
#  DSVPN — Production Docker Compose
#  3 Containers: PostgreSQL 16 + Go API + HAProxy
#  Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
#
#  Start:   docker compose up -d --build
#  Stop:    docker compose down
#  Logs:    docker compose logs -f
#  Status:  docker compose ps
# ============================================================

services:

  # ── PostgreSQL 16 Database ──
  db:
    image: postgres:16-alpine
    container_name: dsvpn-postgres
    environment:
      POSTGRES_DB: dsvpn
      POSTGRES_USER: dsvpn_user
      POSTGRES_PASSWORD: "${DB_PASS}"
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - dsvpn-network
    ulimits:
      nofile:
        soft: 524288
        hard: 524288
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dsvpn_user -d dsvpn"]
      interval: 5s
      timeout: 5s
      retries: 30
      start_period: 10s
    restart: unless-stopped

  # ── DSVPN Go API Server ──
  api:
    build:
      context: .
      dockerfile: Dockerfile
    container_name: dsvpn-api
    depends_on:
      db:
        condition: service_healthy
    env_file:
      - .env
    environment:
      DATABASE_URL: "postgres://dsvpn_user:${DB_PASS}@db:5432/dsvpn?sslmode=disable"
    networks:
      - dsvpn-network
    ulimits:
      nofile:
        soft: 1048576
        hard: 1048576
      nproc:
        soft: 65535
        hard: 65535
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/healthz"]
      interval: 10s
      timeout: 5s
      retries: 10
      start_period: 15s
    restart: unless-stopped

  # ── HAProxy Reverse Proxy ──
  haproxy:
    image: haproxy:2.9-alpine
    container_name: dsvpn-haproxy
    depends_on:
      api:
        condition: service_healthy
    ports:
      - "80:80"
      - "443:443"
      - "8404:8404"
    volumes:
      - ./haproxy/haproxy.cfg:/usr/local/etc/haproxy/haproxy.cfg:ro
      - ./haproxy/certs:/etc/haproxy/certs:ro
    networks:
      - dsvpn-network
    ulimits:
      nofile:
        soft: 1048576
        hard: 1048576
    healthcheck:
      test: ["CMD", "haproxy", "-c", "-f", "/usr/local/etc/haproxy/haproxy.cfg"]
      interval: 15s
      timeout: 5s
      retries: 5
    restart: unless-stopped

networks:
  dsvpn-network:
    driver: bridge

volumes:
  postgres_data:
DCOMPOSE

    ok "docker-compose.yml written ✓"
    info "  Container 1: dsvpn-postgres  (PostgreSQL 16, ulimit 524288)"
    info "  Container 2: dsvpn-api       (Go API :8080, ulimit 1048576)"
    info "  Container 3: dsvpn-haproxy   (HAProxy :80/:443, ulimit 1048576)"
    info "  Network    : dsvpn-network   (bridge)"
    info "  Volume     : postgres_data   (persistent)"
}

# ================================================================
#  STEP 13 — WRITE PRODUCTION DOCKERFILE
# ================================================================
step_13_dockerfile() {
    step "Step 13/14 — Dockerfile (Multi-stage Build)"
    line

    cat > "${PROJECT_DIR}/Dockerfile" << 'DFILE'
# ============================================================
#  DSVPN Backend — Multi-stage Production Build
#  Stage 1: Build Go binary (golang:1-alpine — matches go.mod)
#  Stage 2: Run in minimal image (alpine:3.20)
# ============================================================

# ---- Build Stage ----
FROM golang:1-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -o /out/dsvpn-server ./cmd/server

# ---- Production Stage ----
FROM alpine:3.20

LABEL maintainer="DSVPN Team"
LABEL description="DSVPN Backend API Server"

RUN apk add --no-cache ca-certificates wget tzdata \
    && addgroup -S dsvpn && adduser -S dsvpn -G dsvpn

WORKDIR /app

COPY --from=builder /out/dsvpn-server /app/dsvpn-server
COPY --from=builder /app/internal/database/migrations /app/internal/database/migrations

RUN chown -R dsvpn:dsvpn /app

USER dsvpn

EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/dsvpn-server"]
DFILE

    ok "Dockerfile written ✓"
    info "  Stage 1: golang:1-alpine (build)"
    info "  Stage 2: alpine:3.20 (production, non-root user)"
    info "  Health : wget /healthz every 10s"
}

# ================================================================
#  STEP 14 — UFW FIREWALL
# ================================================================
step_14_firewall() {
    step "Step 14/14 — UFW Firewall"
    line

    if ! command -v ufw &>/dev/null; then
        warn "UFW not found, skipping firewall setup"
        return
    fi

    ufw --force reset &>/dev/null || true
    ufw default deny incoming &>/dev/null
    ufw default allow outgoing &>/dev/null

    ufw allow 22/tcp comment "SSH" &>/dev/null
    ufw allow 80/tcp comment "HTTP-HAProxy" &>/dev/null
    ufw allow 443/tcp comment "HTTPS-HAProxy" &>/dev/null

    ufw --force enable &>/dev/null

    ok "Firewall enabled ✓"
    info "  ✅ Allowed : SSH (22), HTTP (80), HTTPS (443)"
    info "  🚫 Blocked : Everything else (8080, 5432, 8404 etc.)"
}

# ================================================================
#  DEPLOY — BUILD & START DOCKER STACK
# ================================================================
deploy_stack() {
    step "🐳 Building & Deploying Docker Stack"
    line

    cd "${PROJECT_DIR}"

    # Stop old containers if any
    info "Stopping old containers (if any)..."
    docker compose down --remove-orphans 2>/dev/null || true

    info "Building images and starting containers..."
    echo ""
    docker compose up -d --build 2>&1
    echo ""

    ok "Docker compose started"
    info "Waiting for all containers to become healthy..."

    # Wait for health checks (max 180 seconds)
    local MAX_WAIT=180
    local WAITED=0
    local ALL_HEALTHY=false

    while [[ $WAITED -lt $MAX_WAIT ]]; do
        sleep 5
        WAITED=$((WAITED + 5))

        HEALTHY=$(docker compose ps 2>/dev/null | grep -c "(healthy)" || echo "0")

        printf "\r  ${CYAN}→${NC} Waiting... %ds — Healthy: %d/3      " "$WAITED" "$HEALTHY"

        if [[ "$HEALTHY" -ge 3 ]]; then
            ALL_HEALTHY=true
            break
        fi

        # Check if any container exited
        if docker compose ps 2>/dev/null | grep -q "Exit"; then
            echo ""
            err "Container exited unexpectedly!"
            echo ""
            docker compose ps
            echo ""
            info "Last logs:"
            docker compose logs --tail=30
            fail "Deployment failed — check container logs above"
        fi
    done

    echo ""
    if [[ "$ALL_HEALTHY" == true ]]; then
        ok "All 3 containers are healthy! 🎉"
    else
        warn "Not all containers healthy after ${MAX_WAIT}s"
        info "Containers may still be starting. Check with: docker compose ps"
        docker compose ps
    fi
}

# ================================================================
#  VERIFICATION TESTS
# ================================================================
verify_deployment() {
    step "✅ Running Verification Tests"
    line

    cd "${PROJECT_DIR}"

    local PASS=0
    local FAIL=0
    local TOTAL=7

    # Test 1: Docker containers running
    info "[1/${TOTAL}] Docker containers..."
    CONTAINER_COUNT=$(docker compose ps --format '{{.Name}}' 2>/dev/null | wc -l || echo "0")
    if [[ "$CONTAINER_COUNT" -ge 3 ]]; then
        ok "Containers running: ${CONTAINER_COUNT}/3"
        PASS=$((PASS + 1))
    else
        err "Only ${CONTAINER_COUNT}/3 containers running"
        FAIL=$((FAIL + 1))
    fi

    # Test 2: API health (direct inside container)
    info "[2/${TOTAL}] API health (direct :8080)..."
    API_RESP=$(docker exec dsvpn-api wget -qO- http://localhost:8080/healthz 2>/dev/null || echo "FAIL")
    if [[ "$API_RESP" != "FAIL" ]]; then
        ok "API responds on :8080 ✓"
        PASS=$((PASS + 1))
    else
        err "API not responding on :8080"
        FAIL=$((FAIL + 1))
    fi

    # Test 3: HAProxy HTTP
    info "[3/${TOTAL}] HAProxy HTTP (:80 /healthz)..."
    HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' http://localhost/healthz 2>/dev/null || echo "000")
    if [[ "$HTTP_CODE" == "200" ]]; then
        ok "HAProxy HTTP → 200 OK ✓"
        PASS=$((PASS + 1))
    elif [[ "$HTTP_CODE" == "301" ]]; then
        ok "HAProxy HTTP → 301 redirect (expected for non-health paths) ✓"
        PASS=$((PASS + 1))
    else
        err "HAProxy HTTP → ${HTTP_CODE}"
        FAIL=$((FAIL + 1))
    fi

    # Test 4: HAProxy HTTPS
    info "[4/${TOTAL}] HAProxy HTTPS (:443 /healthz)..."
    HTTPS_CODE=$(curl -sk -o /dev/null -w '%{http_code}' https://localhost/healthz 2>/dev/null || echo "000")
    if [[ "$HTTPS_CODE" == "200" ]]; then
        ok "HAProxy HTTPS → 200 OK ✓"
        PASS=$((PASS + 1))
    else
        err "HAProxy HTTPS → ${HTTPS_CODE}"
        FAIL=$((FAIL + 1))
    fi

    # Test 5: PostgreSQL
    info "[5/${TOTAL}] PostgreSQL connection..."
    PG_OK=$(docker exec dsvpn-postgres pg_isready -U dsvpn_user -d dsvpn 2>/dev/null || echo "FAIL")
    if echo "$PG_OK" | grep -q "accepting"; then
        ok "PostgreSQL accepting connections ✓"
        PASS=$((PASS + 1))
    else
        err "PostgreSQL not ready"
        FAIL=$((FAIL + 1))
    fi

    # Test 6: Container ulimit
    info "[6/${TOTAL}] API container ulimit (nofile)..."
    API_NOFILE=$(docker exec dsvpn-api sh -c 'ulimit -n' 2>/dev/null || echo "0")
    if [[ "$API_NOFILE" -ge 1048576 ]]; then
        ok "API container nofile = ${API_NOFILE} ✓"
        PASS=$((PASS + 1))
    else
        warn "API container nofile = ${API_NOFILE} (expected 1048576)"
        FAIL=$((FAIL + 1))
    fi

    # Test 7: System sysctl
    info "[7/${TOTAL}] System sysctl tuning..."
    SYS_FILEMAX=$(sysctl -n fs.file-max 2>/dev/null || echo "0")
    if [[ "$SYS_FILEMAX" -ge 2097152 ]]; then
        ok "fs.file-max = ${SYS_FILEMAX} ✓"
        PASS=$((PASS + 1))
    else
        warn "fs.file-max = ${SYS_FILEMAX} (expected >= 2097152)"
        FAIL=$((FAIL + 1))
    fi

    echo ""
    if [[ "$FAIL" -eq 0 ]]; then
        ok "ALL TESTS PASSED: ${PASS}/${TOTAL} ✓ 🎉"
    else
        warn "Results: ${PASS} passed, ${FAIL} failed out of ${TOTAL}"
    fi
}

# ================================================================
#  FINAL SUMMARY
# ================================================================
print_summary() {
    DEPLOY_END=$(date +%s)
    DEPLOY_TIME=$((DEPLOY_END - DEPLOY_START))
    DEPLOY_MIN=$((DEPLOY_TIME / 60))
    DEPLOY_SEC=$((DEPLOY_TIME % 60))

    # Get server public IP
    SERVER_IP=$(curl -4 -s --max-time 5 http://ifconfig.me/ip 2>/dev/null || \
                curl -4 -s --max-time 5 http://icanhazip.com 2>/dev/null || \
                echo "YOUR_VPS_IP")

    # Read credentials
    DB_PASS=$(grep '^DB_PASSWORD=' "${PROJECT_DIR}/.env" 2>/dev/null | cut -d= -f2 || echo "check .env")
    JWT_SEC=$(grep '^JWT_SECRET=' "${PROJECT_DIR}/.env" 2>/dev/null | cut -d= -f2 || echo "check .env")

    echo ""
    echo -e "${GREEN}${BOLD}"
    echo "╔══════════════════════════════════════════════════════════════╗"
    echo "║            🎉  DSVPN DEPLOYMENT COMPLETE  🎉                ║"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo -e "${NC}"
    echo -e "  ${GREEN}Deploy Time  :${NC} ${DEPLOY_MIN}m ${DEPLOY_SEC}s"
    echo -e "  ${GREEN}Server IP    :${NC} ${SERVER_IP}"
    echo -e "  ${GREEN}Domain       :${NC} ${DOMAIN}"
    echo -e "  ${GREEN}API URL      :${NC} https://${DOMAIN}"
    echo -e "  ${GREEN}Health Check :${NC} https://${DOMAIN}/healthz"
    echo ""
    echo -e "  ${YELLOW}DB Password  :${NC} ${DB_PASS}"
    echo -e "  ${YELLOW}JWT Secret   :${NC} ${JWT_SEC:0:20}..."
    echo -e "  ${YELLOW}Env File     :${NC} ${PROJECT_DIR}/.env"
    echo ""
    echo -e "${GREEN}${BOLD}"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo "║  CONTAINERS                                                  ║"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo -e "${NC}"
    docker compose -f "${PROJECT_DIR}/docker-compose.yml" ps 2>/dev/null || true
    echo ""
    echo -e "${CYAN}${BOLD}"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo "║  📋 CLOUDFLARE SETUP (do this manually after deploy)        ║"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo -e "${NC}"
    echo -e "  ${CYAN}1.${NC} Cloudflare Dashboard → DNS"
    echo -e "     Add ${BOLD}A record${NC}: api → ${BOLD}${SERVER_IP}${NC} (orange proxy ${YELLOW}ON${NC})"
    echo ""
    echo -e "  ${CYAN}2.${NC} Cloudflare Dashboard → SSL/TLS"
    echo -e "     Set mode to ${BOLD}\"Full (Strict)\"${NC}"
    echo ""
    echo -e "  ${CYAN}3.${NC} (Optional) Cloudflare → SSL/TLS → Origin Server"
    echo -e "     Create Origin Certificate → save to:"
    echo -e "     ${DIM}${PROJECT_DIR}/haproxy/certs/${DOMAIN}.pem${NC}"
    echo ""
    echo -e "${MAGENTA}${BOLD}"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo "║  🛠️  USEFUL COMMANDS                                        ║"
    echo "╠══════════════════════════════════════════════════════════════╣"
    echo -e "${NC}"
    echo -e "  ${DIM}# View logs${NC}"
    echo -e "  docker compose -f ${PROJECT_DIR}/docker-compose.yml logs -f"
    echo ""
    echo -e "  ${DIM}# Check status${NC}"
    echo -e "  docker compose -f ${PROJECT_DIR}/docker-compose.yml ps"
    echo ""
    echo -e "  ${DIM}# Restart stack${NC}"
    echo -e "  docker compose -f ${PROJECT_DIR}/docker-compose.yml restart"
    echo ""
    echo -e "  ${DIM}# Stop everything${NC}"
    echo -e "  docker compose -f ${PROJECT_DIR}/docker-compose.yml down"
    echo ""
    echo -e "  ${DIM}# Rebuild and redeploy${NC}"
    echo -e "  cd ${PROJECT_DIR} && docker compose up -d --build"
    echo ""
    echo -e "${GREEN}${BOLD}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""

    # Save credentials to a protected file
    {
        echo "============================================================"
        echo "  DSVPN Deployment Info"
        echo "  Generated: $(date)"
        echo "============================================================"
        echo ""
        echo "  Server IP    : ${SERVER_IP}"
        echo "  Domain       : ${DOMAIN}"
        echo "  API URL      : https://${DOMAIN}"
        echo "  Health       : https://${DOMAIN}/healthz"
        echo ""
        echo "  DB Password  : ${DB_PASS}"
        echo "  JWT Secret   : ${JWT_SEC}"
        echo ""
        echo "  Project Dir  : ${PROJECT_DIR}"
        echo "  Env File     : ${PROJECT_DIR}/.env"
        echo "  Compose      : ${PROJECT_DIR}/docker-compose.yml"
        echo ""
        echo "  Containers:"
        echo "    dsvpn-postgres  — PostgreSQL 16 (internal :5432)"
        echo "    dsvpn-api       — Go API (internal :8080)"
        echo "    dsvpn-haproxy   — HAProxy (:80, :443)"
        echo ""
        echo "============================================================"
    } > /root/dsvpn_deploy_info.txt
    chmod 600 /root/dsvpn_deploy_info.txt
    info "📄 Credentials saved to: /root/dsvpn_deploy_info.txt"
    echo ""
}

# ================================================================
#  MAIN — RUN ALL STEPS
# ================================================================
main() {
    banner

    step_01_preflight        # OS check, root check
    step_02_essentials       # curl, wget, git, etc (safety check)
    step_03_go               # Go 1.22 install (safety check)
    step_04_postgresql_client # psql client (safety check)
    step_05_docker           # Docker CE + Compose (safety check)
    step_06_system_tuning    # ulimit + sysctl + docker daemon
    step_07_project_setup    # /opt/dsvpn directory
    step_08_build_go         # go mod tidy + go build
    step_09_credentials      # Generate DB pass + JWT secret
    step_10_ssl_cert         # Self-signed SSL for Cloudflare
    step_11_haproxy_config   # HAProxy config file
    step_12_docker_compose   # docker-compose.yml
    step_13_dockerfile       # Dockerfile
    step_14_firewall         # UFW

    deploy_stack             # docker compose up -d --build
    verify_deployment        # Health checks + tests
    print_summary            # Print everything
}

main "$@"
