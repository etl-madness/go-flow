#!/bin/sh
# Builds a Debian/Ubuntu .deb package for ETL Madness Flow.
# Installs binaries (flow, crypto_tool, db_importer), man pages, GNU Info
# manual, XSD schemas, and license under /opt/flow, plus /etc/profile.d/go-flow.sh.
# Does NOT overwrite /usr/bin or /usr/local/bin and installs NO service.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=packaging/scripts/common.sh
. "${SCRIPT_DIR}/common.sh"

FLOW_VERSION=$(resolve_version)
GOARCH_TARGET=$(normalize_goarch "${GOARCH:-${1:-}}")

case "$GOARCH_TARGET" in
    amd64) DEB_ARCH="amd64" ;;
    arm64) DEB_ARCH="arm64" ;;
    *)
        log_err "Unsupported .deb architecture: $GOARCH_TARGET"
        exit 1
        ;;
esac

WORK_DIR="${BUILD_ROOT}/deb-${DEB_ARCH}"
STAGE_DIR="${WORK_DIR}/root"
DOC_TMP="${WORK_DIR}/docs"
OUT_DEB="${DIST_DIR}/${PKG_NAME}_${FLOW_VERSION}-${FLOW_RELEASE}_${DEB_ARCH}.deb"

rm -rf "$WORK_DIR"
mkdir -p \
    "${STAGE_DIR}/DEBIAN" \
    "${STAGE_DIR}/opt/flow/bin" \
    "${STAGE_DIR}/opt/flow/share/man/man1" \
    "${STAGE_DIR}/opt/flow/share/man/man5" \
    "${STAGE_DIR}/opt/flow/share/info" \
    "${STAGE_DIR}/opt/flow/share/xsd" \
    "${STAGE_DIR}/opt/flow/share/licenses" \
    "${STAGE_DIR}/usr/share/doc/${PKG_NAME}" \
    "${STAGE_DIR}/etc/profile.d" \
    "$DIST_DIR"

compile_flow_binaries "linux" "$GOARCH_TARGET" "${STAGE_DIR}/opt/flow/bin"
stage_compressed_docs "$DOC_TMP"

cp "${DOC_TMP}/man1/"*.1.gz "${STAGE_DIR}/opt/flow/share/man/man1/"
cp "${DOC_TMP}/man5/"*.5.gz "${STAGE_DIR}/opt/flow/share/man/man5/"
cp "${DOC_TMP}/info/flow.info.gz" "${STAGE_DIR}/opt/flow/share/info/flow.info.gz"
cp "${DOC_TMP}/info/dir" "${STAGE_DIR}/opt/flow/share/info/dir"
cp "${REPO_ROOT}/xsd/pipeline.xsd" "${STAGE_DIR}/opt/flow/share/xsd/pipeline.xsd"
cp "${REPO_ROOT}/xsd/options.xsd" "${STAGE_DIR}/opt/flow/share/xsd/options.xsd"
cp "${REPO_ROOT}/LICENSE" "${STAGE_DIR}/opt/flow/share/licenses/LICENSE"
cp "${REPO_ROOT}/LICENSE" "${STAGE_DIR}/usr/share/doc/${PKG_NAME}/copyright"
cp "${PACKAGING_DIR}/etc/profile.d/go-flow.sh" "${STAGE_DIR}/etc/profile.d/go-flow.sh"

chmod 0644 \
    "${STAGE_DIR}/opt/flow/share/man/man1/"*.1.gz \
    "${STAGE_DIR}/opt/flow/share/man/man5/"*.5.gz \
    "${STAGE_DIR}/opt/flow/share/info/flow.info.gz" \
    "${STAGE_DIR}/opt/flow/share/info/dir" \
    "${STAGE_DIR}/opt/flow/share/xsd/"*.xsd \
    "${STAGE_DIR}/opt/flow/share/licenses/LICENSE" \
    "${STAGE_DIR}/usr/share/doc/${PKG_NAME}/copyright" \
    "${STAGE_DIR}/etc/profile.d/go-flow.sh"

INSTALLED_SIZE=$(du -sk "${STAGE_DIR}/opt" "${STAGE_DIR}/usr" "${STAGE_DIR}/etc" | awk '{s+=$1} END {print s}')

cat > "${STAGE_DIR}/DEBIAN/control" <<EOF
Package: ${PKG_NAME}
Version: ${FLOW_VERSION}-${FLOW_RELEASE}
Section: utils
Priority: optional
Architecture: ${DEB_ARCH}
Installed-Size: ${INSTALLED_SIZE}
Maintainer: ${MAINTAINER}
Homepage: ${HOMEPAGE}
Description: ETL Madness Flow pipeline engine, crypto_tool, and db_importer (/opt/flow)
 XML-driven ETL and data workflow command-line suite installed under /opt/flow:
  - /opt/flow/bin/flow: Core pipeline execution engine, validator, HTMX visual
    builder UI, XSLT/Mermaid diagram generator, and repository manager.
  - /opt/flow/bin/crypto_tool: AES-256-GCM encryption/decryption, digital
    signing (OpenSSL, OpenPGP, PKCS#7/CMS), verification, and keygen tool.
  - /opt/flow/bin/db_importer: SQL Server XML importer and exporter.
EOF

cat > "${STAGE_DIR}/DEBIAN/conffiles" <<'EOF'
/etc/profile.d/go-flow.sh
EOF

chmod 0644 "${STAGE_DIR}/DEBIAN/control" "${STAGE_DIR}/DEBIAN/conffiles"

(
    cd "$STAGE_DIR"
    if command -v md5sum >/dev/null 2>&1; then
        find opt usr etc -type f -exec md5sum {} + > DEBIAN/md5sums
    elif command -v md5 >/dev/null 2>&1; then
        find opt usr etc -type f -exec md5 -r {} + | awk '{sum=$1; $1=""; sub(/^ /, ""); print sum "  " $0}' > DEBIAN/md5sums
    fi
)
[ -f "${STAGE_DIR}/DEBIAN/md5sums" ] && chmod 0644 "${STAGE_DIR}/DEBIAN/md5sums"

if command -v dpkg-deb >/dev/null 2>&1; then
    log_info "Building .deb package with dpkg-deb..."
    if dpkg-deb --help 2>&1 | grep -q -- '--root-owner-group'; then
        dpkg-deb --root-owner-group -Zgzip --build "$STAGE_DIR" "$OUT_DEB"
    elif command -v fakeroot >/dev/null 2>&1; then
        fakeroot dpkg-deb -Zgzip --build "$STAGE_DIR" "$OUT_DEB"
    else
        dpkg-deb -Zgzip --build "$STAGE_DIR" "$OUT_DEB"
    fi
elif command -v nfpm >/dev/null 2>&1; then
    log_info "dpkg-deb not found; falling back to nfpm..."
    REPO_ROOT="$REPO_ROOT" \
    STAGE_BIN_DIR="${STAGE_DIR}/opt/flow/bin" \
    STAGE_DOC_DIR="$DOC_TMP" \
    NFPM_ARCH="$DEB_ARCH" \
    FLOW_VERSION="$FLOW_VERSION" \
    FLOW_RELEASE="$FLOW_RELEASE" \
        nfpm package --config "${PACKAGING_DIR}/nfpm.yaml" --packager deb --target "$OUT_DEB"
else
    log_err "Neither dpkg-deb nor nfpm is installed. Install dpkg-deb (dpkg) or nfpm to build .deb packages."
    exit 1
fi

log_info "Created Debian package: ${OUT_DEB}"
