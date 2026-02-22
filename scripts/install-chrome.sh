#!/bin/bash

# ============================================================
# Chrome/Chromium 安装脚本
# 支持 macOS, Ubuntu/Debian, CentOS/RHEL, Alpine
# ============================================================

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印函数
info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# 检测操作系统
detect_os() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
    elif [ -f /etc/os-release ]; then
        . /etc/os-release
        case "$ID" in
            ubuntu|debian|linuxmint)
                OS="debian"
                ;;
            centos|rhel|fedora|rocky|almalinux)
                OS="rhel"
                ;;
            alpine)
                OS="alpine"
                ;;
            *)
                OS="unknown"
                ;;
        esac
    else
        OS="unknown"
    fi
    echo "$OS"
}

# 检测 Chrome 是否已安装
check_chrome_installed() {
    info "检测 Chrome/Chromium 是否已安装..."
    
    # macOS 检测
    if [[ "$OSTYPE" == "darwin"* ]]; then
        if [ -d "/Applications/Google Chrome.app" ]; then
            CHROME_PATH="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
            success "找到 Google Chrome: $CHROME_PATH"
            return 0
        elif [ -d "/Applications/Chromium.app" ]; then
            CHROME_PATH="/Applications/Chromium.app/Contents/MacOS/Chromium"
            success "找到 Chromium: $CHROME_PATH"
            return 0
        fi
    fi
    
    # Linux 检测
    local chrome_paths=(
        "/usr/bin/google-chrome"
        "/usr/bin/google-chrome-stable"
        "/usr/bin/chromium"
        "/usr/bin/chromium-browser"
        "/snap/bin/chromium"
        "/usr/lib/chromium/chromium"
    )
    
    for path in "${chrome_paths[@]}"; do
        if [ -x "$path" ]; then
            CHROME_PATH="$path"
            success "找到 Chrome/Chromium: $CHROME_PATH"
            return 0
        fi
    done
    
    # 使用 which 命令检测
    if command -v google-chrome &> /dev/null; then
        CHROME_PATH=$(which google-chrome)
        success "找到 Google Chrome: $CHROME_PATH"
        return 0
    elif command -v chromium &> /dev/null; then
        CHROME_PATH=$(which chromium)
        success "找到 Chromium: $CHROME_PATH"
        return 0
    elif command -v chromium-browser &> /dev/null; then
        CHROME_PATH=$(which chromium-browser)
        success "找到 Chromium: $CHROME_PATH"
        return 0
    fi
    
    warn "未检测到 Chrome/Chromium"
    return 1
}

# 获取 Chrome 版本
get_chrome_version() {
    if [ -n "$CHROME_PATH" ]; then
        VERSION=$("$CHROME_PATH" --version 2>/dev/null || echo "Unknown")
        info "Chrome 版本: $VERSION"
    fi
}

# macOS 安装
install_macos() {
    info "在 macOS 上安装 Chrome..."
    
    # 检查是否有 Homebrew
    if command -v brew &> /dev/null; then
        info "使用 Homebrew 安装 Chromium..."
        brew install --cask chromium
        success "Chromium 安装完成"
    else
        warn "未检测到 Homebrew"
        info "请手动下载安装 Google Chrome:"
        echo "  https://www.google.com/chrome/"
        echo ""
        info "或者先安装 Homebrew，然后运行:"
        echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
        echo "  brew install --cask chromium"
        exit 1
    fi
}

# Debian/Ubuntu 安装
install_debian() {
    info "在 Debian/Ubuntu 上安装 Chromium..."
    
    # 更新包列表
    sudo apt-get update
    
    # 安装 Chromium
    sudo apt-get install -y chromium-browser || sudo apt-get install -y chromium
    
    # 安装依赖（用于无头模式）
    sudo apt-get install -y \
        fonts-liberation \
        libasound2 \
        libatk-bridge2.0-0 \
        libatk1.0-0 \
        libatspi2.0-0 \
        libcups2 \
        libdbus-1-3 \
        libdrm2 \
        libgbm1 \
        libgtk-3-0 \
        libnspr4 \
        libnss3 \
        libxcomposite1 \
        libxdamage1 \
        libxfixes3 \
        libxkbcommon0 \
        libxrandr2 \
        xdg-utils \
        2>/dev/null || true
    
    success "Chromium 安装完成"
}

# RHEL/CentOS 安装
install_rhel() {
    info "在 RHEL/CentOS 上安装 Chromium..."
    
    # 检测包管理器
    if command -v dnf &> /dev/null; then
        PKG_MANAGER="dnf"
    else
        PKG_MANAGER="yum"
    fi
    
    # 安装 EPEL 仓库（如果需要）
    sudo $PKG_MANAGER install -y epel-release 2>/dev/null || true
    
    # 安装 Chromium
    sudo $PKG_MANAGER install -y chromium
    
    success "Chromium 安装完成"
}

# Alpine 安装
install_alpine() {
    info "在 Alpine 上安装 Chromium..."
    
    apk update
    apk add --no-cache \
        chromium \
        chromium-chromedriver \
        nss \
        freetype \
        harfbuzz \
        ca-certificates \
        ttf-freefont
    
    success "Chromium 安装完成"
}

# 安装 Google Chrome（可选，适用于需要最新版本的情况）
install_google_chrome_debian() {
    info "安装 Google Chrome (Debian/Ubuntu)..."
    
    # 下载并安装
    wget -q -O /tmp/google-chrome.deb https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
    sudo dpkg -i /tmp/google-chrome.deb || sudo apt-get install -f -y
    rm /tmp/google-chrome.deb
    
    success "Google Chrome 安装完成"
}

# 主函数
main() {
    echo "=============================================="
    echo "   Chrome/Chromium 检测与安装脚本"
    echo "=============================================="
    echo ""
    
    # 检测操作系统
    OS=$(detect_os)
    info "检测到操作系统: $OS"
    echo ""
    
    # 检测是否已安装
    if check_chrome_installed; then
        get_chrome_version
        echo ""
        success "Chrome/Chromium 已安装，无需重复安装"
        echo ""
        echo "Chrome 路径: $CHROME_PATH"
        exit 0
    fi
    
    echo ""
    info "开始安装 Chrome/Chromium..."
    echo ""
    
    # 根据操作系统安装
    case "$OS" in
        macos)
            install_macos
            ;;
        debian)
            install_debian
            ;;
        rhel)
            install_rhel
            ;;
        alpine)
            install_alpine
            ;;
        *)
            error "不支持的操作系统: $OS"
            ;;
    esac
    
    echo ""
    
    # 验证安装
    if check_chrome_installed; then
        get_chrome_version
        echo ""
        success "安装完成！"
        echo ""
        echo "Chrome 路径: $CHROME_PATH"
    else
        error "安装失败，请手动安装 Chrome/Chromium"
    fi
}

# 运行主函数
main "$@"
