#!/bin/sh
# Builds a FreeBSD (.pkg) package for ETL Madness Flow.
# Installs binaries (flow, crypto_tool, db_importer), man pages, GNU Info
# manual, XSD schemas, and license under /opt/flow, plus PATH and MANPATH
# drop-ins under /usr/local/etc/profile.d/go-flow.sh and /usr/local/etc/man.d/go-flow.conf.
# Does NOT install any /usr/local/etc/rc.d service script.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=packaging/scripts/common.sh
. "${SCRIPT_DIR}/common.sh"

if ! command -v pkg >/dev/null 2>&1; then
    log_err "FreeBSD 'pkg' utility is required (run this script on FreeBSD)."
    exit 1
fi

RAW_VERSION=$(resolve_version)
# FreeBSD pkg-version(8) treats '-' as revision separator; normalize pre-releases.
FLOW_VERSION=$(printf '%s' "$RAW_VERSION" | tr '-' '.')
GOARCH_TARGET=$(normalize_goarch "${GOARCH:-${1:-}}")
FREEBSD_MAJOR="${FREEBSD_MAJOR:-14}"

case "$GOARCH_TARGET" in
    amd64) FBSD_ARCH="amd64" ;;
    arm64) FBSD_ARCH="aarch64" ;;
    *)
        log_err "Unsupported FreeBSD architecture: $GOARCH_TARGET"
        exit 1
        ;;
esac

FBSD_ABI="FreeBSD:${FREEBSD_MAJOR}:${FBSD_ARCH}"
WORK_DIR="${BUILD_ROOT}/freebsd-${FBSD_ARCH}"
STAGE_DIR="${WORK_DIR}/stage"
DOC_TMP="${WORK_DIR}/docs"
MANIFEST_FILE="${WORK_DIR}/+MANIFEST"
PLIST_FILE="${WORK_DIR}/pkg-plist"

rm -rf "$WORK_DIR"
mkdir -p \
    "${STAGE_DIR}/opt/flow/bin" \
    "${STAGE_DIR}/opt/flow/share/man/man1" \
    "${STAGE_DIR}/opt/flow/share/man/man5" \
    "${STAGE_DIR}/opt/flow/share/info" \
    "${STAGE_DIR}/opt/flow/share/xsd" \
    "${STAGE_DIR}/opt/flow/share/licenses" \
    "${STAGE_DIR}/usr/local/etc/profile.d" \
    "${STAGE_DIR}/usr/local/etc/man.d" \
    "$DIST_DIR"

compile_flow_binaries "freebsd" "$GOARCH_TARGET" "${STAGE_DIR}/opt/flow/bin"
stage_compressed_docs "$DOC_TMP"

cp "${DOC_TMP}/man1/"*.1.gz "${STAGE_DIR}/opt/flow/share/man/man1/"
cp "${DOC_TMP}/man5/"*.5.gz "${STAGE_DIR}/opt/flow/share/man/man5/"
cp "${DOC_TMP}/info/flow.info.gz" "${STAGE_DIR}/opt/flow/share/info/flow.info.gz"
cp "${DOC_TMP}/info/dir" "${STAGE_DIR}/opt/flow/share/info/dir"
cp "${REPO_ROOT}/xsd/pipeline.xsd" "${STAGE_DIR}/opt/flow/share/xsd/pipeline.xsd"
cp "${REPO_ROOT}/xsd/options.xsd" "${STAGE_DIR}/opt/flow/share/xsd/options.xsd"
cp "${REPO_ROOT}/LICENSE" "${STAGE_DIR}/opt/flow/share/licenses/LICENSE"
cp "${PACKAGING_DIR}/etc/profile.d/go-flow.sh" "${STAGE_DIR}/usr/local/etc/profile.d/go-flow.sh"
printf 'MANPATH /opt/flow/share/man\n' > "${STAGE_DIR}/usr/local/etc/man.d/go-flow.conf"

chmod 0644 \
    "${STAGE_DIR}/opt/flow/share/man/man1/"*.1.gz \
    "${STAGE_DIR}/opt/flow/share/man/man5/"*.5.gz \
    "${STAGE_DIR}/opt/flow/share/info/flow.info.gz" \
    "${STAGE_DIR}/opt/flow/share/info/dir" \
    "${STAGE_DIR}/opt/flow/share/xsd/"*.xsd \
    "${STAGE_DIR}/opt/flow/share/licenses/LICENSE" \
    "${STAGE_DIR}/usr/local/etc/profile.d/go-flow.sh" \
    "${STAGE_DIR}/usr/local/etc/man.d/go-flow.conf"

SIZE_KB=$(du -sk "${STAGE_DIR}/opt/flow" "${STAGE_DIR}/usr/local/etc" | awk '{s+=$1} END {print s}')
FLATSIZE=$((SIZE_KB * 1024))

sed \
    -e "s/__VERSION__/${FLOW_VERSION}/g" \
    -e "s/__ABI__/${FBSD_ABI}/g" \
    -e "s/__FLATSIZE__/${FLATSIZE}/g" \
    "${PACKAGING_DIR}/freebsd/+MANIFEST.tmpl" > "$MANIFEST_FILE"

cat > "$PLIST_FILE" <<'EOF'
/opt/flow/bin/flow
/opt/flow/bin/crypto_tool
/opt/flow/bin/db_importer
/opt/flow/share/man/man1/flow.1.gz
/opt/flow/share/man/man1/crypto_tool.1.gz
/opt/flow/share/man/man1/db_importer.1.gz
/opt/flow/share/man/man5/flow-pipeline.5.gz
/opt/flow/share/info/flow.info.gz
/opt/flow/share/info/dir
/opt/flow/share/xsd/pipeline.xsd
/opt/flow/share/xsd/options.xsd
/opt/flow/share/licenses/LICENSE
@config /usr/local/etc/profile.d/go-flow.sh
@config /usr/local/etc/man.d/go-flow.conf
EOF

PKG_TMP_OUT="${WORK_DIR}/pkg-out"
mkdir -p "$PKG_TMP_OUT"

log_info "Building FreeBSD package (${FBSD_ABI})..."
pkg create \
    -M "$MANIFEST_FILE" \
    -p "$PLIST_FILE" \
    -r "$STAGE_DIR" \
    -o "$PKG_TMP_OUT"

BUILT_PKG=$(find "$PKG_TMP_OUT" -name '*.pkg' -o -name '*.txz' -o -name '*.tzst' | head -n 1)
if [ -z "$BUILT_PKG" ]; then
    log_err "pkg create completed but no package archive was found in ${PKG_TMP_OUT}"
    exit 1
fi

OUT_PKG="${DIST_DIR}/${PKG_NAME}-${FLOW_VERSION}-freebsd-${FBSD_ARCH}.pkg"
cp "$BUILT_PKG" "$OUT_PKG"

log_info "Created FreeBSD package: ${OUT_PKG}"
