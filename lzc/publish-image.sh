#!/bin/sh
# Push the locally built magpie-lzc:{amd64,arm64} images as one multi-arch
# manifest, ready for `lzc-cli appstore copy-image`.
#
# 前置：
#   docker login <registry>（公网可访问的 registry，如 ghcr.io，需要 PAT）
#   本地已构建 magpie-lzc:amd64 与 magpie-lzc:arm64
#     （amd64: docker buildx build --platform linux/amd64 -t magpie-lzc:amd64 \
#               -f lzc/gen/Dockerfile --load .
#       arm64: DOCKER_BUILDKIT=0 docker build --platform linux/arm64 \
#               -f lzc/gen/Dockerfile -t magpie-lzc:arm64 .   # 本机为 arm64 时；
#               amd64 机器上两者互换构建器即可）
#
# 用法：REG=ghcr.io/wuxiy VERSION=0.1.1137 sh lzc/publish-image.sh
set -e
REG=${REG:?需要 REG，如 ghcr.io/wuxiy}
V=${VERSION:?需要 VERSION，如 0.1.1137}

docker tag magpie-lzc:amd64 "$REG/magpie-lzc:$V-amd64"
docker tag magpie-lzc:arm64 "$REG/magpie-lzc:$V-arm64"
docker push "$REG/magpie-lzc:$V-amd64"
docker push "$REG/magpie-lzc:$V-arm64"

docker manifest rm "$REG/magpie-lzc:$V" 2>/dev/null || true
docker manifest create "$REG/magpie-lzc:$V" \
  "$REG/magpie-lzc:$V-amd64" "$REG/magpie-lzc:$V-arm64"
docker manifest push "$REG/magpie-lzc:$V"

echo
echo "多架构镜像: $REG/magpie-lzc:$V"
echo "下一步：lzc-cli appstore copy-image $REG/magpie-lzc:$V"
echo "再把返回的 registry.lazycat.cloud/... 地址填进 lzc-manifest.yml 的"
echo "services.magpie.image（替换 embed:magpie），并从 lzc-build.yml 删掉 images.magpie。"
