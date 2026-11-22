#!/bin/sh
# Builds an OpenBSD (.tgz) package for ETL Madness Flow using pkg_create(1).
# Installs binaries (flow, crypto_tool, db_importer), man pages, GNU Info
# manual, XSD schemas, and license under /opt/flow, plus @sample config
# /etc/profile.d/go-flow.sh.
# Does NOT install any /etc/rc.d daemon/service script.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=packaging/scripts/common.sh
. "${SCRIPT_DIR}/common.sh"

PKG_CREATE_BIN=$(command -v pkg_create 2>/dev/null || true)
if [ -z "$PKG_CREATE_BIN" ] && [ -x /usr/sbin/pkg_create ]; then
    PKG_CREATE_BIN="/usr/sbin/pkg_create"
fi
if [ -z "$PKG_CREATE_BIN" ]; then
    log_err "OpenBSD 'pkg_create' utility is required (run this script on OpenBSD)."
    exit 1
fi

RAW_VERSION=$(resolve_version)
# OpenBSD packages-specs(7) forbids hyphens inside the version component
# (e.g. 1.2.2-rc1 -> 1.2.2rc1) because hyphens separate package flavors.
OBSD_BASE_VER=$(printf '%s' "$RAW_VERSION" | tr -d '-')
if [ "${FLOW_RELEASE}" -gt 1 ] 2>/dev/null; then
    OBSD_REV=$((FLOW_RELEASE - 1))
    OBSD_PKGVER="${OBSD_BASE_VER}p${OBSD_REV}"
else
    OBSD_PKGVER="${OBSD_BASE_VER}"
fi

GOARCH_TARGET=$(normalize_goarch "${GOARCH:-${1:-}}")

case "$GOARCH_TARGET" in
    amd64) OBSD_ARCH="amd64" ;;
    arm64) OBSD_ARCH="aarch64" ;;
    *)
        log_err "Unsupported OpenBSD architecture: $GOARCH_TARGET (expected amd64 or arm64)"
        exit 1
        ;;
esac

FULLPKGNAME="${PKG_NAME}-${OBSD_PKGVER}"
WORK_DIR="${BUILD_ROOT}/openbsd-${OBSD_ARCH}"
STAGE_DIR="${WORK_DIR}/stage"
DOC_TMP="${WORK_DIR}/docs"
PLIST_FILE="${WORK_DIR}/PLIST"
# OpenBSD pkg_add(1) validates that the .tgz filename strictly matches
# <stem>-<version>.tgz, so architecture is separated by directory.
OUT_DIR="${DIST_DIR}/openbsd-${OBSD_ARCH}"
OUT_PKG="${OUT_DIR}/${FULLPKGNAME}.tgz"

rm -rf "$WORK_DIR"
mkdir -p \
    "${STAGE_DIR}/opt/flow/bin" \
    "${STAGE_DIR}/opt/flow/share/examples" \
    "${STAGE_DIR}/opt/flow/share/man/man1" \
    "${STAGE_DIR}/opt/flow/share/man/man5" \
    "${STAGE_DIR}/opt/flow/share/info" \
    "${STAGE_DIR}/opt/flow/share/xsd" \
    "${STAGE_DIR}/opt/flow/share/licenses" \
    "$OUT_DIR"

compile_flow_binaries "openbsd" "$GOARCH_TARGET" "${STAGE_DIR}/opt/flow/bin"
stage_compressed_docs "$DOC_TMP"

# OpenBSD @man expects uncompressed roff man pages and manages mandoc.db automatically.
cp "${PACKAGING_DIR}/man/man1/flow.1" "${STAGE_DIR}/opt/flow/share/man/man1/flow.1"
cp "${PACKAGING_DIR}/man/man1/crypto_tool.1" "${STAGE_DIR}/opt/flow/share/man/man1/crypto_tool.1"
cp "${PACKAGING_DIR}/man/man1/db_importer.1" "${STAGE_DIR}/opt/flow/share/man/man1/db_importer.1"
cp "${PACKAGING_DIR}/man/man5/flow-pipeline.5" "${STAGE_DIR}/opt/flow/share/man/man5/flow-pipeline.5"
cp "${DOC_TMP}/info/flow.info.gz" "${STAGE_DIR}/opt/flow/share/info/flow.info.gz"
cp "${DOC_TMP}/info/dir" "${STAGE_DIR}/opt/flow/share/info/dir"
cp "${REPO_ROOT}/xsd/pipeline.xsd" "${STAGE_DIR}/opt/flow/share/xsd/pipeline.xsd"
cp "${REPO_ROOT}/xsd/options.xsd" "${STAGE_DIR}/opt/flow/share/xsd/options.xsd"
cp "${REPO_ROOT}/LICENSE" "${STAGE_DIR}/opt/flow/share/licenses/LICENSE"
cp "${PACKAGING_DIR}/etc/profile.d/go-flow.sh" "${STAGE_DIR}/opt/flow/share/examples/go-flow.sh"

chmod 0644 \
    "${STAGE_DIR}/opt/flow/share/man/man1/"*.1 \
    "${STAGE_DIR}/opt/flow/share/man/man5/"*.5 \
    "${STAGE_DIR}/opt/flow/share/info/flow.info.gz" \
    "${STAGE_DIR}/opt/flow/share/info/dir" \
    "${STAGE_DIR}/opt/flow/share/xsd/"*.xsd \
    "${STAGE_DIR}/opt/flow/share/licenses/LICENSE" \
    "${STAGE_DIR}/opt/flow/share/examples/go-flow.sh"

cp "${PACKAGING_DIR}/openbsd/PLIST.tmpl" "$PLIST_FILE"

log_info "Building OpenBSD package (${OBSD_ARCH})..."
"$PKG_CREATE_BIN" \
    -A "$OBSD_ARCH" \
    -B "$STAGE_DIR" \
    -d "${PACKAGING_DIR}/openbsd/DESCR" \
    -D COMMENT="ETL Madness Flow pipeline engine and utilities (/opt/flow)" \
    -D FULLPKGPATH="misc/${PKG_NAME}" \
    -D MAINTAINER="$MAINTAINER" \
    -D HOMEPAGE="$HOMEPAGE" \
    -D FTP="yes" \
    -f "$PLIST_FILE" \
    -p "/opt/flow" \
    "$OUT_PKG"

if [ -n "${OPENBSD_SIGNIFY_KEY:-}" ]; then
    PKG_SIGN_BIN=$(command -v pkg_sign 2>/dev/null || true)
    [ -z "$PKG_SIGN_BIN" ] && [ -x /usr/sbin/pkg_sign ] && PKG_SIGN_BIN="/usr/sbin/pkg_sign"
    if [ -z "$PKG_SIGN_BIN" ]; then
        log_err "OPENBSD_SIGNIFY_KEY was set, but pkg_sign was not found."
        exit 1
    fi
    SIGNED_DIR="${WORK_DIR}/signed"
    mkdir -p "$SIGNED_DIR"
    log_info "Signing OpenBSD package with signify2 key ${OPENBSD_SIGNIFY_KEY}..."
    "$PKG_SIGN_BIN" -s signify2 -s "$OPENBSD_SIGNIFY_KEY" -o "$SIGNED_DIR" "$OUT_PKG"
    cp "${SIGNED_DIR}/${FULLPKGNAME}.tgz" "$OUT_PKG"
fi

log_info "Created OpenBSD package: ${OUT_PKG}"
