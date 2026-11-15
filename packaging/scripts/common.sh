#!/bin/sh
# Shared helper functions for ETL Madness Flow Unix package build scripts.
# Portable POSIX sh (compatible with Linux dash/bash, macOS sh, and FreeBSD sh).

set -eu

PACKAGING_SCRIPTS_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PACKAGING_DIR=$(CDPATH= cd -- "${PACKAGING_SCRIPTS_DIR}/.." && pwd)
REPO_ROOT=$(CDPATH= cd -- "${PACKAGING_DIR}/.." && pwd)

BUILD_ROOT="${BUILD_ROOT:-${PACKAGING_DIR}/build}"
DIST_DIR="${DIST_DIR:-${PACKAGING_DIR}/dist}"
PKG_NAME="${PKG_NAME:-go-flow}"
INSTALL_PREFIX="/opt/flow"
FLOW_RELEASE="${FLOW_RELEASE:-1}"
MAINTAINER="${MAINTAINER:-ETL Madness <maintainers@etl-madness.invalid>}"
HOMEPAGE="${HOMEPAGE:-https://github.com/etl-madness/go-flow}"

log_info() {
    printf '[INFO] %s\n' "$*"
}

log_warn() {
    printf '[WARN] %s\n' "$*" >&2
}

log_err() {
    printf '[ERROR] %s\n' "$*" >&2
}

resolve_version() {
    if [ -n "${VERSION:-}" ]; then
        raw_ver="$VERSION"
    elif command -v git >/dev/null 2>&1 && git -C "$REPO_ROOT" describe --tags --abbrev=0 >/dev/null 2>&1; then
        raw_ver=$(git -C "$REPO_ROOT" describe --tags --abbrev=0)
    else
        raw_ver="1.2.1"
    fi
    # Strip leading 'v' or 'V'
    printf '%s' "$raw_ver" | sed 's/^[vV]//'
}

normalize_goarch() {
    arch_in="${1:-}"
    if [ -z "$arch_in" ]; then
        arch_in=$(uname -m)
    fi
    case "$arch_in" in
        x86_64|amd64) printf 'amd64' ;;
        aarch64|arm64) printf 'arm64' ;;
        universal) printf 'universal' ;;
        *)
            log_err "Unsupported architecture: $arch_in (expected amd64 or arm64)"
            exit 1
            ;;
    esac
}

compile_flow_binaries() {
    target_os="$1"
    target_arch="$2"
    out_bin_dir="$3"

    mkdir -p "$out_bin_dir"
    log_info "Compiling flow, crypto_tool, and db_importer for ${target_os}/${target_arch}..."

    (
        cd "$REPO_ROOT"
        CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
            go build -trimpath -ldflags="-s -w" -o "${out_bin_dir}/flow" .
        CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
            go build -trimpath -ldflags="-s -w" -o "${out_bin_dir}/crypto_tool" ./crypto_tool
        CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" \
            go build -trimpath -ldflags="-s -w" -o "${out_bin_dir}/db_importer" ./db_importer
    )

    chmod 0755 \
        "${out_bin_dir}/flow" \
        "${out_bin_dir}/crypto_tool" \
        "${out_bin_dir}/db_importer"
}

stage_compressed_docs() {
    out_doc_dir="$1"

    sh "${PACKAGING_SCRIPTS_DIR}/generate-docs.sh"

    mkdir -p "${out_doc_dir}/man1" "${out_doc_dir}/man5" "${out_doc_dir}/info"

    gzip -9nc "${PACKAGING_DIR}/man/man1/flow.1" > "${out_doc_dir}/man1/flow.1.gz"
    gzip -9nc "${PACKAGING_DIR}/man/man1/crypto_tool.1" > "${out_doc_dir}/man1/crypto_tool.1.gz"
    gzip -9nc "${PACKAGING_DIR}/man/man1/db_importer.1" > "${out_doc_dir}/man1/db_importer.1.gz"
    gzip -9nc "${PACKAGING_DIR}/man/man5/flow-pipeline.5" > "${out_doc_dir}/man5/flow-pipeline.5.gz"
    gzip -9nc "${PACKAGING_DIR}/info/flow.info" > "${out_doc_dir}/info/flow.info.gz"
    cp "${PACKAGING_DIR}/info/dir" "${out_doc_dir}/info/dir"

    chmod 0644 \
        "${out_doc_dir}/man1/flow.1.gz" \
        "${out_doc_dir}/man1/crypto_tool.1.gz" \
        "${out_doc_dir}/man1/db_importer.1.gz" \
        "${out_doc_dir}/man5/flow-pipeline.5.gz" \
        "${out_doc_dir}/info/flow.info.gz" \
        "${out_doc_dir}/info/dir"
}
