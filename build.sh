#!/bin/bash

set -e  # 遇到错误立即退出

# ==================== 配置区域 ====================
PROJECT_NAME="go-layout"
BUILD_DIR="build"
DIST_DIR="${BUILD_DIR}/dist"
KEEP_VERSIONS=3

# 默认平台（单平台构建时使用）
DEFAULT_OS="linux"
DEFAULT_ARCH="amd64"

# 多平台构建列表
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 生成版本号
VERSION=$(date +"%Y%m%d_%H%M%S")

# ==================== 工具函数 ====================

usage() {
    cat << EOF
用法：$0 [选项]

选项:
  -o, --os <os>       目标操作系统 (linux, darwin, windows)
  -a, --arch <arch>   目标架构 (amd64, arm64, 386)
  -p, --platform      指定平台 (格式：os/arch，如 linux/amd64)
  -a, --all           构建所有支持的平台
  -n, --no-compress   不使用 UPX 压缩
  -k, --keep <num>    保留的历史版本数量 (默认：3，0=全部清理)
  -h, --help          显示帮助信息

示例:
  $0                              # 使用默认平台 linux/amd64
  $0 -o linux -a arm64            # 构建 linux/arm64
  $0 -p darwin/amd64              # 构建 darwin/amd64
  $0 --all                        # 构建所有平台
  $0 --all -n                     # 构建所有平台，不压缩
  $0 --all -k 5                   # 构建所有平台，保留 5 个历史版本
  $0 --clean-only -k 0            # 仅清理，删除所有历史版本
EOF
    exit 0
}

# 清理 macOS 资源分叉文件
clean_macos_artifacts() {
    local target_dir="$1"
    if command -v dot_clean &> /dev/null; then
        dot_clean "${target_dir}" 2>/dev/null || true
    fi
    find "${target_dir}" -type f \( -name "._*" -o -name ".DS_Store" \) -delete 2>/dev/null || true
}

# 获取文件大小（字节）
get_file_size() {
    stat -f%z "$1" 2>/dev/null || stat -c%s "$1" 2>/dev/null
}

# 获取人类可读的文件大小
get_human_size() {
    local bytes=$1
    if [ "$bytes" -lt 1024 ]; then
        echo "${bytes}B"
    elif [ "$bytes" -lt 1048576 ]; then
        awk "BEGIN {printf \"%.1fK\", $bytes/1024}"
    else
        awk "BEGIN {printf \"%.1fM\", $bytes/1048576}"
    fi
}

# ==================== 构建流程 ====================

print_header() {
    local os="$1"
    local arch="$2"
    echo -e "${BLUE}=========================================="
    echo "构建：${os}/${arch}"
    echo "版本：${VERSION}"
    echo "==========================================${NC}"
}

# 编译 Go 二进制文件
build_binary() {
    local os="$1"
    local arch="$2"
    local dist_dir="$3"
    local binary_name="$4"

    echo "正在编译..."

    local binary_file="${dist_dir}/${binary_name}"
    # Windows 平台添加.exe 后缀
    if [ "$os" = "windows" ]; then
        binary_file="${dist_dir}/${binary_name}.exe"
    fi

    if ! CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build \
        -ldflags="-w -s" -trimpath -o "${binary_file}" .; then
        echo -e "${RED}编译失败：${os}/${arch}${NC}"
        return 1
    fi

    local size=$(get_file_size "${binary_file}")
    echo "编译完成：$(get_human_size ${size})"
    return 0
}

# 使用 UPX 压缩二进制文件
compress_with_upx() {
    local os="$1"
    local dist_dir="$2"
    local binary_name="$3"
    local use_compress="$4"

    if [ "$use_compress" = "false" ]; then
        echo "跳过 UPX 压缩"
        return 0
    fi

    if ! command -v upx &> /dev/null; then
        echo "提示：安装 UPX 可减小二进制大小 (brew install upx)"
        return 0
    fi

    local binary_file="${dist_dir}/${binary_name}"
    [ "$os" = "windows" ] && binary_file="${dist_dir}/${binary_name}.exe"

    echo "正在使用 UPX 压缩..."
    local original_size=$(get_file_size "${binary_file}")

    if upx --best --lzma "${binary_file}" > /dev/null 2>&1; then
        local compressed_size=$(get_file_size "${binary_file}")
        local ratio=$(awk "BEGIN {printf \"%.1f\", (1 - ${compressed_size}/${original_size}) * 100}")
        echo -e "${GREEN}压缩完成：$(get_human_size ${compressed_size}) (压缩率：${ratio}%)${NC}"
    else
        echo -e "${YELLOW}UPX 压缩失败，使用原始二进制${NC}"
    fi
}

# 复制必要的资源文件
copy_resources() {
    local dist_dir="$1"

    echo "复制资源文件..."

    # 配置文件示例
    [ -f "config/config.yaml.example" ] && cp "config/config.yaml.example" "${dist_dir}/"
    [ -f "AI_DEPLOYMENT.md" ] && cp "AI_DEPLOYMENT.md" "${dist_dir}/"

    # 数据库迁移文件
    if [ -d "data/migrations" ]; then
        mkdir -p "${dist_dir}/data/migrations"
        rsync -av --exclude='._*' --exclude='.DS_Store' \
            data/migrations/ "${dist_dir}/data/migrations/" 2>/dev/null \
            || cp -r data/migrations/* "${dist_dir}/data/migrations/" 2>/dev/null || true
    fi

    # 权限相关文件
    [ -f "policy.csv" ] && cp "policy.csv" "${dist_dir}/"
    [ -f "rbac_model.conf" ] && cp "rbac_model.conf" "${dist_dir}/"
}

# 创建压缩包
create_tarball() {
    local os="$1"
    local arch="$2"
    local dist_dir="$3"

    local tarball_name="${PROJECT_NAME}_${os}_${arch}_${VERSION}.tar.gz"

    # 输出到 stderr（不污染返回值）
    echo "创建压缩包..." >&2
    cd "${BUILD_DIR}"

    # Windows 平台使用 zip 格式
    if [ "$os" = "windows" ]; then
        tarball_name="${PROJECT_NAME}_${os}_${arch}_${VERSION}.zip"
        if command -v zip &> /dev/null; then
            zip -rq "${tarball_name}" dist
        else
            echo -e "${YELLOW}警告：zip 命令不可用，使用 tar.gz 格式${NC}" >&2
            tar -czf "${tarball_name%.zip}.tar.gz" -C dist .
            tarball_name="${tarball_name%.zip}.tar.gz"
        fi
    else
        tar -czf "${tarball_name}" -C dist .
    fi

    cd ..
    # 只输出文件名（用于返回值）
    echo "TARBALL:${tarball_name}"
}

# 验证压缩包
verify_tarball() {
    local tarball="$1"

    echo "验证压缩包..."

    # Windows zip 文件验证
    if [[ "$tarball" == *.zip ]]; then
        if command -v unzip &> /dev/null; then
            unzip -l "${tarball}" | grep -q "\._" && {
                echo -e "${RED}错误：压缩包中包含 macOS 资源分叉文件！${NC}"
                return 1
            }
        fi
    else
        if tar -tzf "${tarball}" | grep -q "\._"; then
            echo -e "${RED}错误：压缩包中包含 macOS 资源分叉文件！${NC}"
            return 1
        fi
    fi

    local file_count
    if [[ "$tarball" == *.zip ]]; then
        file_count=$(unzip -l "${tarball}" | tail -1 | awk '{print $2}')
    else
        file_count=$(tar -tzf "${tarball}" | wc -l | tr -d ' ')
    fi
    echo "验证通过：共 ${file_count} 个文件"
    return 0
}

# 构建单个平台
build_platform() {
    local os="$1"
    local arch="$2"
    local use_compress="$3"

    local platform_dist="${DIST_DIR}/${os}_${arch}"
    mkdir -p "${platform_dist}"

    print_header "${os}" "${arch}"

    if ! build_binary "${os}" "${arch}" "${platform_dist}" "${PROJECT_NAME}"; then
        return 1
    fi

    compress_with_upx "${os}" "${platform_dist}" "${PROJECT_NAME}" "${use_compress}"
    copy_resources "${platform_dist}"

    echo "清理 macOS 资源分叉文件..."
    clean_macos_artifacts "${platform_dist}"

    local tarball
    tarball=$(create_tarball "${os}" "${arch}" "${platform_dist}")
    tarball="${tarball#TARBALL:}"

    # 压缩包已在 BUILD_DIR 目录下，无需移动
    # 清理平台临时目录
    rm -rf "${platform_dist}"

    if ! verify_tarball "${BUILD_DIR}/${tarball}"; then
        return 1
    fi

    echo -e "${GREEN}构建完成：${tarball}${NC}"
    echo ""
    return 0
}

# 清理旧版本
cleanup_old_versions() {
    local keep_count="${1:-$KEEP_VERSIONS}"

    if [ "$keep_count" -eq 0 ]; then
        echo "清理所有历史版本..."
        rm -f "${BUILD_DIR}"/${PROJECT_NAME}_*_*.tar.gz "${BUILD_DIR}"/${PROJECT_NAME}_*_*.zip 2>/dev/null || true
        echo "已删除所有历史版本"
        return
    fi

    echo "清理旧版本（保留最新${keep_count}个）..."

    for platform in "${PLATFORMS[@]}"; do
        local os="${platform%/*}"
        local arch="${platform#*/}"
        local pattern="${PROJECT_NAME}_${os}_${arch}_*"

        local all_versions=($(ls -t "${BUILD_DIR}"/${pattern}.tar.gz "${BUILD_DIR}"/${pattern}.zip 2>/dev/null || true))
        local count=${#all_versions[@]}

        if [ "${count}" -le "${keep_count}" ]; then
            continue
        fi

        local delete_count=$((count - keep_count))
        for ((i=keep_count; i<count; i++)); do
            echo "  删除：$(basename "${all_versions[$i]}")"
            rm -f "${all_versions[$i]}"
        done
    done
    echo "旧版本清理完成"
}

# ==================== 参数解析 ====================

BUILD_ALL=false
USE_COMPRESS=true
TARGET_OS=""
TARGET_ARCH=""
PLATFORM=""
CLEAN_ONLY=false
KEEP_COUNT=3

while [[ $# -gt 0 ]]; do
    case $1 in
        -o|--os)
            TARGET_OS="$2"
            shift 2
            ;;
        -a|--arch)
            TARGET_ARCH="$2"
            shift 2
            ;;
        -p|--platform)
            PLATFORM="$2"
            shift 2
            ;;
        --all|-all)
            BUILD_ALL=true
            shift
            ;;
        -n|--no-compress)
            USE_COMPRESS=false
            shift
            ;;
        -k|--keep)
            KEEP_COUNT="$2"
            shift 2
            ;;
        --clean-only)
            CLEAN_ONLY=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo -e "${RED}未知选项：$1${NC}"
            usage
            ;;
    esac
done

# ==================== 主流程 ====================

main() {
    # 仅清理模式
    if [ "$CLEAN_ONLY" = true ]; then
        echo "=========================================="
        echo "清理历史版本"
        echo "=========================================="
        cleanup_old_versions "${KEEP_COUNT}"
        if [ "$KEEP_COUNT" -eq 0 ]; then
            echo "所有历史版本已清理完成"
        else
            echo "旧版本清理完成（保留最新${KEEP_COUNT}个）"
        fi
        ls -lh "${BUILD_DIR}"/*.tar.gz "${BUILD_DIR}"/*.zip 2>/dev/null || echo "无构建文件"
        return
    fi

    echo "=========================================="
    echo "Go 项目构建脚本"
    echo "版本：${VERSION}"
    echo "=========================================="

    # 准备构建目录
    echo "清理旧的构建文件..."
    rm -rf "${DIST_DIR}"
    mkdir -p "${DIST_DIR}"

    if [ "$BUILD_ALL" = true ]; then
        # 多平台构建
        echo -e "${BLUE}开始多平台构建...${NC}"

        for platform in "${PLATFORMS[@]}"; do
            local os="${platform%/*}"
            local arch="${platform#*/}"

            if ! build_platform "${os}" "${arch}" "${USE_COMPRESS}"; then
                echo -e "${YELLOW}跳过失败的平台：${platform}${NC}"
            fi
        done

        cleanup_old_versions "${KEEP_COUNT}"

        echo "=========================================="
        echo -e "${GREEN}所有平台构建完成！${NC}"
        echo "输出目录：${BUILD_DIR}/"
        ls -lh "${BUILD_DIR}"/*.tar.gz "${BUILD_DIR}"/*.zip 2>/dev/null || true
        echo "=========================================="

    elif [ -n "$PLATFORM" ]; then
        # 指定平台构建
        if [[ ! "$PLATFORM" =~ ^[a-z0-9]+/[a-z0-9]+$ ]]; then
            echo -e "${RED}错误：平台格式不正确，应为 os/arch 格式${NC}"
            exit 1
        fi

        local os="${PLATFORM%/*}"
        local arch="${PLATFORM#*/}"
        build_platform "${os}" "${arch}" "${USE_COMPRESS}"

        echo "=========================================="
        echo -e "${GREEN}构建完成！${NC}"
        ls -lh "${BUILD_DIR}"/*.tar.gz "${BUILD_DIR}"/*.zip 2>/dev/null || true
        echo "=========================================="

    else
        # 单平台构建（使用默认或命令行指定）
        local os="${TARGET_OS:-$DEFAULT_OS}"
        local arch="${TARGET_ARCH:-$DEFAULT_ARCH}"
        build_platform "${os}" "${arch}" "${USE_COMPRESS}"

        echo "=========================================="
        echo -e "${GREEN}构建完成！${NC}"
        ls -lh "${BUILD_DIR}"/*.tar.gz "${BUILD_DIR}"/*.zip 2>/dev/null || true
        echo "=========================================="
    fi
}

main "$@"
