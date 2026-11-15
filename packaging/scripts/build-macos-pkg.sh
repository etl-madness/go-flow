#!/bin/sh
# Builds a macOS Installer (.pkg) package for ETL Madness Flow.
# Installs binaries (flow, crypto_tool, db_importer), man pages, GNU Info
# manual, XSD schemas, and license under /opt/flow, plus path_helper drop-ins
# in /private/etc/paths.d/go-flow and /private/etc/manpaths.d/go-flow.
# Supports GOARCH=arm64, GOARCH=amd64, or GOARCH=universal (via lipo).
# Does NOT install any LaunchDaemon or LaunchAgent service.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=packaging/scripts/common.sh
. "${SCRIPT_DIR}/common.sh"

if ! command -v pkgbuild >/dev/null 2>&1 || ! command -v productbuild >/dev/null 2>&1; then
    log_err "pkgbuild and productbuild are required (run this script on macOS with Xcode Command Line Tools)."
    exit 1
fi

FLOW_VERSION=$(resolve_version)
GOARCH_TARGET=$(normalize_goarch "${GOARCH:-${1:-}}")

case "$GOARCH_TARGET" in
    arm64)
        PKG_ARCH="arm64"
        HOST_ARCHS="arm64"
        ;;
    amd64)
        PKG_ARCH="amd64"
        HOST_ARCHS="x86_64"
        ;;
    universal)
        if ! command -v lipo >/dev/null 2>&1; then
            log_err "lipo is required for GOARCH=universal builds."
            exit 1
        fi
        PKG_ARCH="universal"
        HOST_ARCHS="x86_64,arm64"
        ;;
    *)
        log_err "Unsupported macOS architecture: $GOARCH_TARGET"
        exit 1
        ;;
esac

WORK_DIR="${BUILD_ROOT}/macos-${PKG_ARCH}"
PAYLOAD_DIR="${WORK_DIR}/payload"
DOC_TMP="${WORK_DIR}/docs"
COMPONENT_PKG="${WORK_DIR}/go-flow-component.pkg"
DIST_XML="${WORK_DIR}/Distribution.xml"
OUT_PKG="${DIST_DIR}/${PKG_NAME}-${FLOW_VERSION}-darwin-${PKG_ARCH}.pkg"

rm -rf "$WORK_DIR"
mkdir -p \
    "${PAYLOAD_DIR}/opt/flow/bin" \
    "${PAYLOAD_DIR}/opt/flow/share/man/man1" \
    "${PAYLOAD_DIR}/opt/flow/share/man/man5" \
    "${PAYLOAD_DIR}/opt/flow/share/info" \
    "${PAYLOAD_DIR}/opt/flow/share/xsd" \
    "${PAYLOAD_DIR}/opt/flow/share/licenses" \
    "${PAYLOAD_DIR}/private/etc/paths.d" \
    "${PAYLOAD_DIR}/private/etc/manpaths.d" \
    "$DIST_DIR"

if [ "$GOARCH_TARGET" = "universal" ]; then
    compile_flow_binaries "darwin" "amd64" "${WORK_DIR}/bin-amd64"
    compile_flow_binaries "darwin" "arm64" "${WORK_DIR}/bin-arm64"
    for bin_name in flow crypto_tool db_importer; do
        lipo -create \
            "${WORK_DIR}/bin-amd64/${bin_name}" \
            "${WORK_DIR}/bin-arm64/${bin_name}" \
            -output "${PAYLOAD_DIR}/opt/flow/bin/${bin_name}"
        chmod 0755 "${PAYLOAD_DIR}/opt/flow/bin/${bin_name}"
    done
else
    compile_flow_binaries "darwin" "$GOARCH_TARGET" "${PAYLOAD_DIR}/opt/flow/bin"
fi

stage_compressed_docs "$DOC_TMP"

cp "${DOC_TMP}/man1/"*.1.gz "${PAYLOAD_DIR}/opt/flow/share/man/man1/"
cp "${DOC_TMP}/man5/"*.5.gz "${PAYLOAD_DIR}/opt/flow/share/man/man5/"
cp "${DOC_TMP}/info/flow.info.gz" "${PAYLOAD_DIR}/opt/flow/share/info/flow.info.gz"
cp "${DOC_TMP}/info/dir" "${PAYLOAD_DIR}/opt/flow/share/info/dir"
cp "${REPO_ROOT}/xsd/pipeline.xsd" "${PAYLOAD_DIR}/opt/flow/share/xsd/pipeline.xsd"
cp "${REPO_ROOT}/xsd/options.xsd" "${PAYLOAD_DIR}/opt/flow/share/xsd/options.xsd"
cp "${REPO_ROOT}/LICENSE" "${PAYLOAD_DIR}/opt/flow/share/licenses/LICENSE"

printf '/opt/flow/bin\n' > "${PAYLOAD_DIR}/private/etc/paths.d/go-flow"
printf '/opt/flow/share/man\n' > "${PAYLOAD_DIR}/private/etc/manpaths.d/go-flow"

chmod 0644 \
    "${PAYLOAD_DIR}/opt/flow/share/man/man1/"*.1.gz \
    "${PAYLOAD_DIR}/opt/flow/share/man/man5/"*.5.gz \
    "${PAYLOAD_DIR}/opt/flow/share/info/flow.info.gz" \
    "${PAYLOAD_DIR}/opt/flow/share/info/dir" \
    "${PAYLOAD_DIR}/opt/flow/share/xsd/"*.xsd \
    "${PAYLOAD_DIR}/opt/flow/share/licenses/LICENSE" \
    "${PAYLOAD_DIR}/private/etc/paths.d/go-flow" \
    "${PAYLOAD_DIR}/private/etc/manpaths.d/go-flow"

sed \
    -e "s/__VERSION__/${FLOW_VERSION}/g" \
    -e "s/__HOST_ARCHS__/${HOST_ARCHS}/g" \
    "${PACKAGING_DIR}/macos/Distribution.xml.tmpl" > "$DIST_XML"

log_info "Building macOS component package..."
pkgbuild \
    --root "$PAYLOAD_DIR" \
    --identifier "com.etlmadness.go-flow" \
    --version "$FLOW_VERSION" \
    --install-location "/" \
    "$COMPONENT_PKG"

log_info "Building macOS distribution package..."
if [ -n "${MACOS_SIGN_IDENTITY:-}" ]; then
    productbuild \
        --distribution "$DIST_XML" \
        --package-path "$WORK_DIR" \
        --sign "$MACOS_SIGN_IDENTITY" \
        "$OUT_PKG"
else
    productbuild \
        --distribution "$DIST_XML" \
        --package-path "$WORK_DIR" \
        "$OUT_PKG"
fi

log_info "Created macOS package: ${OUT_PKG}"
