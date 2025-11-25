#!/bin/bash

# 构建脚本配置
PROJECT_NAME="go-layout"
TARGET_OS="linux"
TARGET_ARCH="amd64"
BUILD_DIR="build"
DIST_DIR="${BUILD_DIR}/dist"
KEEP_VERSIONS=3

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 生成版本号
VERSION=$(date +"%Y%m%d_%H%M%S")
BINARY_NAME="${PROJECT_NAME}"
TARBALL_NAME="${PROJECT_NAME}_${TARGET_OS}_${TARGET_ARCH}_${VERSION}.tar.gz"

echo "=========================================="
echo "开始构建生产版本"
echo "项目名称: ${PROJECT_NAME}"
echo "目标平台: ${TARGET_OS}"
echo "目标架构: ${TARGET_ARCH}"
echo "版本: ${VERSION}"
echo "=========================================="

# 清理旧的构建文件
echo "清理旧的构建文件..."
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

# 编译
echo "正在编译..."
CGO_ENABLED=0 GOOS=${TARGET_OS} GOARCH=${TARGET_ARCH} go build -ldflags="-w -s" -trimpath -o "${DIST_DIR}/${BINARY_NAME}" .
if [ $? -ne 0 ]; then
    echo -e "${RED}编译失败！${NC}"
    exit 1
fi

# 显示编译后的文件大小
BINARY_SIZE=$(ls -lh "${DIST_DIR}/${BINARY_NAME}" | awk '{print $5}')
echo "编译完成: ${DIST_DIR}/${BINARY_NAME} (大小: ${BINARY_SIZE})"

# 使用 UPX 压缩二进制文件（如果可用）
if command -v upx &> /dev/null; then
    echo "正在使用 UPX 压缩二进制文件..."
    ORIGINAL_SIZE=$(stat -f%z "${DIST_DIR}/${BINARY_NAME}" 2>/dev/null || stat -c%s "${DIST_DIR}/${BINARY_NAME}" 2>/dev/null)
    upx --best --lzma "${DIST_DIR}/${BINARY_NAME}" > /dev/null 2>&1
    if [ $? -eq 0 ]; then
        COMPRESSED_SIZE=$(stat -f%z "${DIST_DIR}/${BINARY_NAME}" 2>/dev/null || stat -c%s "${DIST_DIR}/${BINARY_NAME}" 2>/dev/null)
        COMPRESSION_RATIO=$(awk "BEGIN {printf \"%.1f\", (1 - ${COMPRESSED_SIZE}/${ORIGINAL_SIZE}) * 100}")
        FINAL_SIZE=$(ls -lh "${DIST_DIR}/${BINARY_NAME}" | awk '{print $5}')
        echo -e "${GREEN}压缩完成: ${FINAL_SIZE} (压缩率: ${COMPRESSION_RATIO}%)${NC}"
    else
        echo -e "${YELLOW}UPX 压缩失败，使用原始二进制文件${NC}"
    fi
else
    echo "提示: 安装 UPX 可以进一步减小二进制文件大小 (brew install upx)"
fi

# 复制必要文件到 dist 目录
echo "复制必要文件到 dist 目录..."

# 只复制配置文件示例（直接放在根目录）
if [ -f "config/config.yaml.example" ]; then
    cp "config/config.yaml.example" "${DIST_DIR}/"
fi

# 复制数据库迁移文件
if [ -d "data/migrations" ]; then
    mkdir -p "${DIST_DIR}/data/migrations"
    rsync -av --exclude='._*' --exclude='.DS_Store' \
        data/migrations/ \
        "${DIST_DIR}/data/migrations/" 2>/dev/null || cp -r data/migrations/* "${DIST_DIR}/data/migrations/" 2>/dev/null
fi

# 复制权限相关文件
if [ -f "policy.csv" ]; then
    cp "policy.csv" "${DIST_DIR}/"
fi
if [ -f "rbac_model.conf" ]; then
    cp "rbac_model.conf" "${DIST_DIR}/"
fi

# 彻底清理 dist 目录中的 macOS 资源分叉文件
echo "彻底清理 dist 目录中的 macOS 资源分叉文件..."

# 使用 dot_clean 清理扩展属性（如果可用）
if command -v dot_clean &> /dev/null; then
    echo "使用 dot_clean 清理扩展属性..."
    dot_clean "${DIST_DIR}" 2>/dev/null || true
fi

# 强制清理所有 macOS 相关文件
find "${DIST_DIR}" -type f -name "._*" -delete 2>/dev/null || true
find "${DIST_DIR}" -type f -name ".DS_Store" -delete 2>/dev/null || true

# 验证清理结果
if find "${DIST_DIR}" -name "._*" -o -name ".DS_Store" | grep -q .; then
    echo -e "${YELLOW}警告: 仍发现 macOS 资源分叉文件${NC}"
else
    echo "检查完成: dist 目录中未发现 macOS 资源分叉文件（这很好）"
fi

echo "最终验证通过: dist 目录中已无 macOS 资源分叉文件"

# 从 dist 目录创建压缩包
echo "从 dist 目录创建压缩包..."
cd "${BUILD_DIR}"
tar -czf "${TARBALL_NAME}" -C dist .
cd ..

# 验证压缩包内容
echo "验证压缩包内容..."
if tar -tzf "${BUILD_DIR}/${TARBALL_NAME}" | grep -q "\._"; then
    echo -e "${RED}错误: 压缩包中包含 macOS 资源分叉文件！${NC}"
    exit 1
else
    echo "验证通过: 压缩包中不包含 macOS 资源分叉文件"
fi

FILE_COUNT=$(tar -tzf "${BUILD_DIR}/${TARBALL_NAME}" | wc -l | tr -d ' ')
echo "压缩包总文件数: ${FILE_COUNT}"

# 清理旧版本（保留最新的3个）
echo "清理旧版本（保留最新的${KEEP_VERSIONS}个）..."
# 按修改时间排序，最新的在前
ALL_VERSIONS=($(ls -t "${BUILD_DIR}"/${PROJECT_NAME}_${TARGET_OS}_${TARGET_ARCH}_*.tar.gz 2>/dev/null))
VERSION_COUNT=${#ALL_VERSIONS[@]}

if [ "${VERSION_COUNT}" -le "${KEEP_VERSIONS}" ]; then
    echo "当前有 ${VERSION_COUNT} 个版本，无需清理（保留最新${KEEP_VERSIONS}个）"
else
    DELETE_COUNT=$((VERSION_COUNT - KEEP_VERSIONS))
    echo "找到 ${VERSION_COUNT} 个版本，保留最新的${KEEP_VERSIONS}个，删除 ${DELETE_COUNT} 个旧版本..."
    # 从第 KEEP_VERSIONS+1 个开始删除（索引从0开始，所以是 KEEP_VERSIONS）
    for ((i=${KEEP_VERSIONS}; i<${VERSION_COUNT}; i++)); do
        old_file="${ALL_VERSIONS[$i]}"
        echo "  删除: $(basename "${old_file}")"
        rm -f "${old_file}"
    done
    echo "已删除 ${DELETE_COUNT} 个旧版本"
fi

echo "注意: dist 目录已保留，位于 ${DIST_DIR}，可手动检查"

echo "=========================================="
echo "构建完成！"
echo "压缩包位置: ${BUILD_DIR}/${TARBALL_NAME}"
echo "dist 目录位置: ${DIST_DIR}（已保留，可手动检查）"
echo "=========================================="

