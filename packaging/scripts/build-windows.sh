#!/bin/sh
# Cross-builds Windows WinGet-compatible packages (.zip portable archive and
# optional .msi installer via msitools 'wixl' or WiX 'wix') and generates
# WinGet 1.6.0 YAML manifests from Linux, macOS, or FreeBSD hosts.

set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=packaging/scripts/common.sh
. "${SCRIPT_DIR}/common.sh"

FLOW_VERSION=$(resolve_version)
GOARCH_TARGET=$(normalize_goarch "${GOARCH:-${1:-amd64}}")
WIN_FORMAT="${WIN_FORMAT:-all}"
RELEASE_BASE_URL="${RELEASE_BASE_URL:-https://github.com/etl-madness/go-flow/releases/download/v${FLOW_VERSION}}"
RELEASE_BASE_URL="${RELEASE_BASE_URL%/}"
UPGRADE_CODE="{8D4A7E29-6C3F-4B18-9E52-3F71C0A8E412}"

case "$GOARCH_TARGET" in
    amd64)
        WINGET_ARCH="x64"
        WIX_ARCH="x64"
        ;;
    arm64)
        WINGET_ARCH="arm64"
        WIX_ARCH="arm64"
        ;;
    *)
        log_err "Unsupported Windows architecture: $GOARCH_TARGET (expected amd64 or arm64)"
        exit 1
        ;;
esac

MSI_VERSION=$(printf '%s' "$FLOW_VERSION" | sed 's/[-+].*//' | awk -F. '{printf "%d.%d.%d", ($1+0), ($2+0), ($3+0)}')

sha256_upper() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | awk '{print toupper($1)}'
    elif command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "$1" | awk '{print toupper($1)}'
    elif command -v sha256 >/dev/null 2>&1; then
        sha256 -q "$1" | tr '[:lower:]' '[:upper:]'
    else
        log_err "No SHA-256 tool found (sha256sum, shasum, or sha256)"
        exit 1
    fi
}

deterministic_product_code() {
    seed_hex=$(printf 'ETLMadness.GoFlow.MSI:%s:%s' "$FLOW_VERSION" "$GOARCH_TARGET" | {
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum | awk '{print toupper($1)}'
        elif command -v shasum >/dev/null 2>&1; then
            shasum -a 256 | awk '{print toupper($1)}'
        else
            sha256 -q | tr '[:lower:]' '[:upper:]'
        fi
    })
    p1=$(printf '%s' "$seed_hex" | cut -c1-8)
    p2=$(printf '%s' "$seed_hex" | cut -c9-12)
    p3="5$(printf '%s' "$seed_hex" | cut -c14-16)"
    p4="9$(printf '%s' "$seed_hex" | cut -c18-20)"
    p5=$(printf '%s' "$seed_hex" | cut -c21-32)
    printf '{%s-%s-%s-%s-%s}' "$p1" "$p2" "$p3" "$p4" "$p5"
}

resolve_guide_doc() {
    doc_name="$1"
    if [ -f "${REPO_ROOT}/../flow-docs/docs/guides/${doc_name}" ]; then
        printf '%s' "${REPO_ROOT}/../flow-docs/docs/guides/${doc_name}"
    elif [ -f "${REPO_ROOT}/docs/guides/${doc_name}" ]; then
        printf '%s' "${REPO_ROOT}/docs/guides/${doc_name}"
    else
        log_err "Missing documentation guide: ${doc_name}"
        exit 1
    fi
}

WORK_DIR="${BUILD_ROOT}/windows-${GOARCH_TARGET}"
STAGE_DIR="${WORK_DIR}/payload"

rm -rf "$WORK_DIR"
mkdir -p \
    "${STAGE_DIR}/bin" \
    "${STAGE_DIR}/docs" \
    "${STAGE_DIR}/share/xsd" \
    "${STAGE_DIR}/share/man/man1" \
    "${STAGE_DIR}/share/man/man5" \
    "$DIST_DIR"

log_info "Cross-compiling flow.exe, crypto_tool.exe, and db_importer.exe for windows/${GOARCH_TARGET}..."
(
    cd "$REPO_ROOT"
    CGO_ENABLED=0 GOOS="windows" GOARCH="$GOARCH_TARGET" \
        go build -trimpath -ldflags="-s -w" -o "${STAGE_DIR}/bin/flow.exe" .
    CGO_ENABLED=0 GOOS="windows" GOARCH="$GOARCH_TARGET" \
        go build -trimpath -ldflags="-s -w" -o "${STAGE_DIR}/bin/crypto_tool.exe" ./crypto_tool
    CGO_ENABLED=0 GOOS="windows" GOARCH="$GOARCH_TARGET" \
        go build -trimpath -ldflags="-s -w" -o "${STAGE_DIR}/bin/db_importer.exe" ./db_importer
)

cp "$(resolve_guide_doc COMPLETE_CLI_OPTIONS_AND_ARGUMENTS.md)" "${STAGE_DIR}/docs/COMPLETE_CLI_OPTIONS_AND_ARGUMENTS.md"
cp "$(resolve_guide_doc SUMMARY_OF_AVAILABLE_OPTIONS.md)" "${STAGE_DIR}/docs/SUMMARY_OF_AVAILABLE_OPTIONS.md"
cp "${REPO_ROOT}/xsd/pipeline.xsd" "${STAGE_DIR}/share/xsd/pipeline.xsd"
cp "${REPO_ROOT}/xsd/options.xsd" "${STAGE_DIR}/share/xsd/options.xsd"
cp "${PACKAGING_DIR}/man/man1/"*.1 "${STAGE_DIR}/share/man/man1/"
cp "${PACKAGING_DIR}/man/man5/flow-pipeline.5" "${STAGE_DIR}/share/man/man5/flow-pipeline.5"
cp "${REPO_ROOT}/LICENSE" "${STAGE_DIR}/LICENSE.txt"

INSTALLERS_TMP="${WORK_DIR}/installers.yaml.part"
: > "$INSTALLERS_TMP"

if [ "$WIN_FORMAT" = "all" ] || [ "$WIN_FORMAT" = "zip" ]; then
    ZIP_NAME="go-flow-${FLOW_VERSION}-windows-${GOARCH_TARGET}.zip"
    OUT_ZIP="${DIST_DIR}/${ZIP_NAME}"
    rm -f "$OUT_ZIP"
    log_info "Creating portable Windows archive: ${OUT_ZIP}"
    if command -v zip >/dev/null 2>&1; then
        (cd "$STAGE_DIR" && zip -q -r "$OUT_ZIP" .)
    elif command -v python3 >/dev/null 2>&1; then
        python3 - "$STAGE_DIR" "$OUT_ZIP" <<'PY'
import os, sys, zipfile
stage, out = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(out, "w", zipfile.ZIP_DEFLATED) as zf:
    for root, _, files in os.walk(stage):
        for f in sorted(files):
            full = os.path.join(root, f)
            rel = os.path.relpath(full, stage).replace(os.sep, "/")
            zf.write(full, rel)
PY
    else
        log_err "Neither 'zip' nor 'python3' is installed to create .zip archives."
        exit 1
    fi
    ZIP_SHA=$(sha256_upper "$OUT_ZIP")
    cat >> "$INSTALLERS_TMP" <<EOF
  - Architecture: ${WINGET_ARCH}
    InstallerType: zip
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: bin\flow.exe
        PortableCommandAlias: flow
      - RelativeFilePath: bin\crypto_tool.exe
        PortableCommandAlias: crypto_tool
      - RelativeFilePath: bin\db_importer.exe
        PortableCommandAlias: db_importer
    InstallerUrl: ${RELEASE_BASE_URL}/${ZIP_NAME}
    InstallerSha256: ${ZIP_SHA}
EOF
fi

if [ "$WIN_FORMAT" = "all" ] || [ "$WIN_FORMAT" = "msi" ]; then
    MSI_NAME="go-flow-${FLOW_VERSION}-windows-${GOARCH_TARGET}.msi"
    OUT_MSI="${DIST_DIR}/${MSI_NAME}"
    PRODUCT_CODE=$(deterministic_product_code)
    MSI_BUILT=0

    if command -v wix >/dev/null 2>&1; then
        log_info "Building Windows MSI with wix (${WIX_ARCH})..."
        wix build "${PACKAGING_DIR}/windows/flow.wxs" \
            -arch "$WIX_ARCH" \
            -d "StageDir=${STAGE_DIR}" \
            -d "FlowMsiVersion=${MSI_VERSION}" \
            -d "FlowProductCode=${PRODUCT_CODE}" \
            -o "$OUT_MSI"
        MSI_BUILT=1
    elif command -v wixl >/dev/null 2>&1; then
        log_info "Building Windows MSI with msitools wixl (${WIX_ARCH})..."
        wixl --arch "$WIX_ARCH" \
            -D "StageDir=${STAGE_DIR}" \
            -D "FlowMsiVersion=${MSI_VERSION}" \
            -D "FlowProductCode=${PRODUCT_CODE}" \
            -o "$OUT_MSI" "${PACKAGING_DIR}/windows/flow-wix3.wxs"
        MSI_BUILT=1
    elif [ "$WIN_FORMAT" = "msi" ]; then
        log_err "Neither 'wix' nor 'wixl' (msitools) is installed to build .msi packages."
        exit 1
    else
        log_warn "Neither 'wix' nor 'wixl' (msitools) found; skipping .msi build (.zip and WinGet manifests generated)."
    fi

    if [ "$MSI_BUILT" -eq 1 ] && [ -f "$OUT_MSI" ]; then
        MSI_SHA=$(sha256_upper "$OUT_MSI")
        cat >> "$INSTALLERS_TMP" <<EOF
  - Architecture: ${WINGET_ARCH}
    Scope: machine
    InstallerType: wix
    InstallerUrl: ${RELEASE_BASE_URL}/${MSI_NAME}
    InstallerSha256: ${MSI_SHA}
    ProductCode: '${PRODUCT_CODE}'
    AppsAndFeaturesEntries:
      - DisplayName: ETL Madness Flow
        Publisher: ETL Madness
        DisplayVersion: ${MSI_VERSION}
        ProductCode: '${PRODUCT_CODE}'
        UpgradeCode: '${UPGRADE_CODE}'
EOF
    fi
fi

WINGET_DIR="${DIST_DIR}/winget/e/ETLMadness/GoFlow/${FLOW_VERSION}"
mkdir -p "$WINGET_DIR"

cat > "${WINGET_DIR}/ETLMadness.GoFlow.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.1.6.0.schema.json

PackageIdentifier: ETLMadness.GoFlow
PackageVersion: ${FLOW_VERSION}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
EOF

cat > "${WINGET_DIR}/ETLMadness.GoFlow.locale.en-US.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.1.6.0.schema.json

PackageIdentifier: ETLMadness.GoFlow
PackageVersion: ${FLOW_VERSION}
PackageLocale: en-US
Publisher: ETL Madness
PublisherUrl: https://github.com/etl-madness
PublisherSupportUrl: https://github.com/etl-madness/go-flow/issues
PackageName: ETL Madness Flow
PackageUrl: https://github.com/etl-madness/go-flow
License: MIT
LicenseUrl: https://github.com/etl-madness/go-flow/blob/main/LICENSE
ShortDescription: Lightweight XML-driven ETL pipeline engine, crypto_tool, and db_importer.
Description: >-
  ETL Madness Flow is a zero-dependency Go ETL execution engine and workflow orchestrator
  comprising flow (core pipeline runner, validator, and HTMX visual builder),
  crypto_tool (AES-256-GCM encryption and OpenSSL/OpenPGP/PKCS#7 digital signatures),
  and db_importer (SQL Server XML repository importer and exporter).
Moniker: go-flow
Tags:
  - cli
  - database
  - etl
  - golang
  - sql
  - workflow
Documentations:
  - DocumentLabel: Online Documentation
    DocumentUrl: https://etl-madness.github.io/flow-docs/
ManifestType: defaultLocale
ManifestVersion: 1.6.0
EOF

{
    cat <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.1.6.0.schema.json

PackageIdentifier: ETLMadness.GoFlow
PackageVersion: ${FLOW_VERSION}
Commands:
  - flow
  - crypto_tool
  - db_importer
Installers:
EOF
    cat "$INSTALLERS_TMP"
    cat <<'EOF'
ManifestType: installer
ManifestVersion: 1.6.0
EOF
} > "${WINGET_DIR}/ETLMadness.GoFlow.installer.yaml"

log_info "Generated WinGet manifests in: ${WINGET_DIR}"
