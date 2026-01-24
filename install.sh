#!/bin/bash

# jean - Installation Script
# Downloads and installs jean with shell integration setup

set -e

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
print_header() {
    echo -e "${BLUE}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

# Check prerequisites
check_prerequisites() {
    print_header "Checking prerequisites..."

    # Check if running on Windows (not WSL2)
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
        print_error "jean requires WSL2 on Windows"
        echo "Please install WSL2: https://docs.microsoft.com/en-us/windows/wsl/install"
        exit 1
    fi

    # Check for git
    if ! command -v git &> /dev/null; then
        print_error "git is not installed"
        exit 1
    fi
    print_success "git found"

    # Check for tmux
    if ! command -v tmux &> /dev/null; then
        print_error "tmux is not installed"
        echo "Install tmux:"
        if [[ "$OSTYPE" == "darwin"* ]]; then
            echo "  brew install tmux"
        else
            echo "  sudo apt-get install tmux  (Ubuntu/Debian)"
            echo "  sudo yum install tmux      (CentOS/RHEL)"
        fi
        exit 1
    fi
    print_success "tmux found"
}

# Detect OS and architecture
detect_system() {
    print_header "Detecting system..."

    if [[ "$OSTYPE" == "darwin"* ]]; then
        OS="darwin"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        OS="linux"
    else
        print_error "Unsupported operating system: $OSTYPE"
        exit 1
    fi
    print_success "OS: $OS"

    # Detect architecture
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64) ARCH="amd64" ;;
        arm64) ARCH="arm64" ;;
        aarch64) ARCH="arm64" ;;
        *) print_error "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    print_success "Architecture: $ARCH"
}

# Download precompiled binary from GitHub releases
download_binary() {
    print_header "Downloading binary from GitHub releases..."

    REPO="Danielratmiroff/jean-tui"

    # Use specified version or fetch latest
    if [[ -z "$REQUESTED_VERSION" ]]; then
        RELEASE_API="https://api.github.com/repos/$REPO/releases/latest"
    else
        RELEASE_API="https://api.github.com/repos/$REPO/releases/tags/$REQUESTED_VERSION"
    fi

    # Get release info
    if ! RELEASE_INFO=$(curl -s "$RELEASE_API"); then
        print_warning "Could not fetch release info, will try 'go install' instead"
        return 1
    fi

    # Extract version
    VERSION=$(echo "$RELEASE_INFO" | grep -o '"tag_name": "[^"]*' | cut -d'"' -f4)
    if [[ -z "$VERSION" ]]; then
        print_warning "Could not determine version, will try 'go install' instead"
        return 1
    fi

    print_info "Latest version: $VERSION"

    # Construct download URL
    BINARY_NAME="jean_${VERSION#v}_${OS}_${ARCH}"
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$VERSION/${BINARY_NAME}.tar.gz"

    print_info "Downloading from: $DOWNLOAD_URL"

    # Create temporary directory
    TEMP_DIR=$(mktemp -d)
    trap "rm -rf $TEMP_DIR" EXIT

    # Download and extract
    if curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_DIR/jean.tar.gz"; then
        tar -xzf "$TEMP_DIR/jean.tar.gz" -C "$TEMP_DIR"
        if [[ -f "$TEMP_DIR/jean" ]]; then
            JEAN_BINARY="$TEMP_DIR/jean"
            print_success "Binary downloaded successfully"
            return 0
        else
            print_warning "Binary not found in archive, will try 'go install' instead"
            return 1
        fi
    else
        print_warning "Download failed, will try 'go install' instead"
        return 1
    fi
}

# Fallback to go install
go_install() {
    print_header "Installing via go install..."

    if ! command -v go &> /dev/null; then
        print_error "go is not installed and precompiled binary download failed"
        echo "Install Go from https://golang.org/dl/ or use:"
        if [[ "$OSTYPE" == "darwin"* ]]; then
            echo "  brew install go"
        else
            echo "  Visit https://golang.org/dl/ for installation instructions"
        fi
        exit 1
    fi

    # Use specified version or latest
    GO_VERSION="@latest"
    if [[ -n "$REQUESTED_VERSION" ]]; then
        GO_VERSION="@$REQUESTED_VERSION"
    fi

    if go install github.com/coollabsio/jean-tui${GO_VERSION}; then
        # Find the binary installed by go
        GOPATH="${GOPATH:-$HOME/go}"
        JEAN_BINARY="$GOPATH/bin/jean"

        if [[ ! -f "$JEAN_BINARY" ]]; then
            print_error "Binary not found after go install"
            exit 1
        fi

        print_success "Installed via go install"
        return 0
    else
        print_error "go install failed"
        exit 1
    fi
}

# Install binary to system
install_binary() {
    print_header "Installing binary to system..."

    INSTALL_DIR="/usr/local/bin"
    INSTALL_PATH="$INSTALL_DIR/jean"

    # Check if we need sudo
    if [[ ! -w "$INSTALL_DIR" ]]; then
        print_info "Need sudo to write to $INSTALL_DIR"
        if sudo -l &> /dev/null; then
            sudo cp "$JEAN_BINARY" "$INSTALL_PATH"
            sudo chmod +x "$INSTALL_PATH"
        else
            print_error "Cannot write to $INSTALL_DIR without sudo access"
            echo "You can manually copy the binary:"
            echo "  sudo cp $JEAN_BINARY $INSTALL_PATH"
            exit 1
        fi
    else
        cp "$JEAN_BINARY" "$INSTALL_PATH"
        chmod +x "$INSTALL_PATH"
    fi

    print_success "Binary installed to $INSTALL_PATH"
}

# Setup shell integration
setup_shell_integration() {
    print_header "Setting up shell integration..."

    # Use jean init command to set up shell integration
    if /usr/local/bin/jean init 2>/dev/null; then
        print_success "Shell integration configured"
        return 0
    else
        print_warning "Could not set up shell integration automatically"
        print_info "You can manually run: jean init"
        return 1
    fi
}

# Auto-source shell RC file to activate jean in current session
activate_shell_integration() {
    print_header "Activating shell integration..."

    CURRENT_SHELL=$(basename "$SHELL")

    case "$CURRENT_SHELL" in
        bash)
            RC_FILE="$HOME/.bashrc"
            ;;
        zsh)
            RC_FILE="$HOME/.zshrc"
            ;;
        fish)
            RC_FILE="$HOME/.config/fish/config.fish"
            ;;
        *)
            print_warning "Could not auto-activate shell integration for $CURRENT_SHELL"
            return 1
            ;;
    esac

    if [[ -f "$RC_FILE" ]]; then
        # Source the RC file in the current shell
        # shellcheck disable=SC1090
        source "$RC_FILE" 2>/dev/null && print_success "Shell integration activated"
    fi
}

# Verify installation
verify_installation() {
    print_header "Verifying installation..."

    if ! command -v jean &> /dev/null; then
        # Try with full path if not in PATH yet
        if [[ -x "/usr/local/bin/jean" ]]; then
            VERSION=$(/usr/local/bin/jean --version 2>/dev/null || echo "unknown")
        else
            print_error "jean not found in PATH"
            return 1
        fi
    else
        VERSION=$(jean --version 2>/dev/null || echo "unknown")
    fi

    print_success "jean installed successfully!"
    print_info "Version: $VERSION"

    if [[ -n "$LOCAL_BIN" && ":$PATH:" != *":$LOCAL_BIN:"* ]]; then
        print_warning "Note: $LOCAL_BIN is not in your PATH"
        print_info "Add it to your PATH in your shell RC file if you want shell integration"
    fi
}

# Print usage instructions
print_usage() {
    echo "Usage: $0 [VERSION]"
    echo ""
    echo "Install jean - Terminal UI for managing Git worktrees"
    echo ""
    echo "Arguments:"
    echo "  VERSION    Specific version to install (e.g., v0.1.1)"
    echo "             If not specified, installs the latest version"
    echo ""
    echo "Examples:"
    echo "  $0                    # Install latest version"
    echo "  $0 v0.1.1             # Install specific version"
    echo ""
}

# Print usage instructions
print_next_steps() {
    echo ""
    print_header "Installation complete!"
    echo ""
    echo "You're all set! Run: jean"
    echo ""
    echo "For more information:"
    echo "  - Help: jean --help"
    echo "  - Documentation: https://github.com/coollabsio/jean-tui"
    echo ""
}

# Main installation flow
main() {
    # Parse arguments
    REQUESTED_VERSION=""
    if [[ $# -gt 0 ]]; then
        if [[ "$1" == "-h" || "$1" == "--help" ]]; then
            print_usage
            exit 0
        fi
        REQUESTED_VERSION="$1"
    fi

    echo ""
    print_header "jean Installation Script"
    if [[ -n "$REQUESTED_VERSION" ]]; then
        print_info "Installing version: $REQUESTED_VERSION"
    fi
    echo ""

    check_prerequisites
    detect_system

    # Try to download binary first
    if ! download_binary; then
        # Fallback to go install
        go_install
    fi

    install_binary
    setup_shell_integration
    activate_shell_integration
    verify_installation
    print_next_steps
}

# Run installation
main "$@"
