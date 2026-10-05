#!/bin/sh
# Generates lzc/gen/Dockerfile from the upstream Dockerfile: everything stays
# as upstream built it, except
#  - the default command changes from `serve` (gateway alone) to `web`
#    (gateway + browser UI), matching the compose example in the upstream
#    README. The services.magpie manifest entry relies on this (setup_script
#    cannot be combined with a command override).
#  - the build stage downloads Go modules through goproxy.cn: the module
#    proxy's storage.googleapis.com is unreachable from a mainland-China
#    network (the Lazycat box and typical dev machines).
set -e
cd "$(dirname "$0")/.."
mkdir -p lzc/gen
sed 's|^CMD \["serve"\]$|CMD ["web", "--addr", "0.0.0.0:3430", "--no-open"]|' Dockerfile \
  | awk '/^WORKDIR \/src$/ && !done { print; print "ENV GOPROXY=https://goproxy.cn,direct"; done=1; next } { print }' \
  > lzc/gen/Dockerfile
# fail loudly if upstream renames its CMD instead of silently shipping `serve`
grep -q '^CMD \["web", "--addr", "0.0.0.0:3430", "--no-open"\]$' lzc/gen/Dockerfile
