# Flow CLI Packaging (`/opt/flow` & Windows WinGet)

This directory contains build scripts, package metadata, `man` pages, GNU `info` documentation, WiX `.msi` manifests, and WinGet YAML manifests for packaging **ETL Madness Flow** across Linux (`.deb`, `.rpm` for RHEL/Fedora/SUSE/openSUSE, and `.pkg.tar.zst` for Arch Linux), macOS (`.pkg`), FreeBSD (`.pkg`), OpenBSD (`.tgz`), and Windows (`.zip` & `.msi` for WinGet).

- **Isolated Prefixes**: Unix packages install under `/opt/flow`, Windows `.msi` installs under `C:\Program Files\ETL Madness\Flow\`, and WinGet portable `.zip` installs inside `%LOCALAPPDATA%\Microsoft\WinGet\Packages\ETLMadness.GoFlow_...` so binaries never overwrite or conflict with existing system packages.
- **Non-Colliding Package Metadata**: OS packages use `go-flow` (`com.etlmadness.go-flow` on macOS, `ETLMadness.GoFlow` on WinGet) so installing this suite will never replace an unrelated `flow` package.
- **No Background Service**: No `systemd`, `launchd`, `rc.d`, or Windows Service unit is installed.

---

## Included Assets & Install Locations

All packages compile `go-flow` with `-o flow` (`flow.exe` on Windows) alongside `crypto_tool` and `db_importer` (`CGO_ENABLED=0`), and install documentation generated from `../flow-docs/docs/guides/COMPLETE_CLI_OPTIONS_AND_ARGUMENTS.md` and `../flow-docs/docs/guides/SUMMARY_OF_AVAILABLE_OPTIONS.md`:

| Asset | Linux (`.deb` / `.rpm` / Arch `.pkg.tar.zst`) | macOS (`.pkg`) | FreeBSD (`.pkg`) & OpenBSD (`.tgz`) | Windows (`.msi` / WinGet `.zip`) |
| :--- | :--- | :--- | :--- | :--- |
| **`flow` binary** | `/opt/flow/bin/flow` | `/opt/flow/bin/flow` | `/opt/flow/bin/flow` | `<Prefix>\bin\flow.exe` |
| **`crypto_tool` binary** | `/opt/flow/bin/crypto_tool` | `/opt/flow/bin/crypto_tool` | `/opt/flow/bin/crypto_tool` | `<Prefix>\bin\crypto_tool.exe` |
| **`db_importer` binary** | `/opt/flow/bin/db_importer` | `/opt/flow/bin/db_importer` | `/opt/flow/bin/db_importer` | `<Prefix>\bin\db_importer.exe` |
| **CLI & XML Documentation** | `/opt/flow/share/man/man{1,5}/*.gz` & `/opt/flow/share/info/{flow.info.gz,dir}` | `/opt/flow/share/man/man{1,5}/*.gz` & `/opt/flow/share/info/{flow.info.gz,dir}` | `/opt/flow/share/man/man{1,5}/*` & `/opt/flow/share/info/{flow.info.gz,dir}` | `<Prefix>\docs\*.md` & `<Prefix>\share\man\man{1,5}\*` |
| **XSD Schemas** | `/opt/flow/share/xsd/{pipeline,options}.xsd` | `/opt/flow/share/xsd/{pipeline,options}.xsd` | `/opt/flow/share/xsd/{pipeline,options}.xsd` | `<Prefix>\share\xsd\{pipeline,options}.xsd` |
| **License** | `/opt/flow/share/licenses/LICENSE` | `/opt/flow/share/licenses/LICENSE` | `/opt/flow/share/licenses/LICENSE` | `<Prefix>\LICENSE.txt` |
| **`PATH` / `MANPATH` / `INFOPATH`** | `/etc/profile.d/go-flow.sh` *(appends `/opt/flow/bin` & `/opt/flow/share/info:`)* | `/private/etc/paths.d/go-flow` & `/private/etc/manpaths.d/go-flow` | FreeBSD: `/usr/local/etc/profile.d/go-flow.sh` & `/usr/local/etc/man.d/go-flow.conf`<br>OpenBSD: `/etc/profile.d/go-flow.sh` | MSI `Part="last"` `PATH` append / WinGet `Links` |

*(On Windows, `<Prefix>` is `C:\Program Files\ETL Madness\Flow` for `.msi` or `%LOCALAPPDATA%\Microsoft\WinGet\Packages\ETLMadness.GoFlow_...` for portable `.zip`.)*

> **Why appending to `PATH` is safe:** The profile and `paths.d` drop-ins append `/opt/flow/bin` to the *end* of `PATH`, so any pre-existing `flow`, `crypto_tool`, or `db_importer` in `/usr/bin` or `/usr/local/bin` keeps priority, while scripts/cron jobs can always invoke `/opt/flow/bin/flow` directly.

---

## Directory Layout

```text
packaging/
├── etc/
│   └── profile.d/
│       └── go-flow.sh           # POSIX sh login profile drop-in (PATH + INFOPATH)
├── man/
│   ├── man1/
│   │   ├── flow.1               # flow(1) CLI manual page
│   │   ├── crypto_tool.1        # crypto_tool(1) CLI manual page
│   │   └── db_importer.1        # db_importer(1) CLI manual page
│   └── man5/
│       └── flow-pipeline.5      # flow-pipeline(5) XML pipeline specification
├── info/
│   ├── flow.texi                # GNU Texinfo source manual
│   ├── flow.info                # Compiled GNU Info manual (with Tag Table)
│   └── dir                      # Self-contained GNU Info directory index
├── arch/
│   └── PKGBUILD.tmpl            # Arch Linux PKGBUILD template
├── rpm/
│   └── flow.spec                # RPM spec file (RHEL, Fedora, SUSE, openSUSE)
├── macos/
│   └── Distribution.xml.tmpl    # macOS productbuild distribution template
├── freebsd/
│   └── +MANIFEST.tmpl           # FreeBSD pkg manifest template
├── openbsd/
│   ├── DESCR                    # OpenBSD package long description (+DESC)
│   └── PLIST.tmpl               # OpenBSD pkg_create packing-list template
├── windows/
│   ├── flow.wxs                 # WiX Toolset v4/v5/v6 MSI manifest
│   └── flow-wix3.wxs            # WiX v3 / Linux msitools (wixl) MSI manifest
├── winget/
│   └── README.md                # WinGet validation, local install & publishing guide
├── nfpm.yaml                    # Optional nFPM fallback config (.deb / .rpm / Arch)
└── scripts/
    ├── common.sh                # Shared POSIX sh helpers & Go cross-compilation
    ├── generate-docs.sh         # Recompiles flow.texi -> flow.info if makeinfo exists
    ├── build-all.sh             # Unified entrypoint for host or specific targets
    ├── build-deb.sh             # Builds Linux .deb (dpkg-deb or nfpm)
    ├── build-rpm.sh             # Builds Linux .rpm (rpmbuild or nfpm)
    ├── build-arch.sh            # Builds Arch Linux .pkg.tar.zst (makepkg or nfpm)
    ├── build-macos-pkg.sh       # Builds macOS .pkg (pkgbuild + productbuild)
    ├── build-freebsd-pkg.sh     # Builds FreeBSD .pkg (pkg create)
    ├── build-openbsd-pkg.sh     # Builds OpenBSD .tgz (pkg_create)
    ├── build-windows.sh         # Cross-builds Windows .zip/.msi & WinGet YAML from Unix
    ├── Build-Windows.ps1        # Native Windows .zip/.msi & WinGet manifest builder
    └── Build-Packages.ps1       # PowerShell cross-compile & staging helper
```

---

## Building Packages

Output artifacts are written to `packaging/dist/`. All shell scripts are portable POSIX `/bin/sh` scripts.

### 1. Build for Current Host OS
```sh
sh ./packaging/scripts/build-all.sh host
```

### 2. Linux Debian / Ubuntu (`.deb`)
Requires `dpkg-deb` (or `nfpm`):
```sh
sh ./packaging/scripts/build-deb.sh amd64
sh ./packaging/scripts/build-deb.sh arm64
```

### 3. Linux RHEL / Fedora / SUSE / openSUSE (`.rpm`)
Requires `rpmbuild` (`rpm-build` package) or `nfpm`:
```sh
sh ./packaging/scripts/build-rpm.sh amd64
sh ./packaging/scripts/build-rpm.sh arm64
```

### 4. Arch Linux (`.pkg.tar.zst`)
Requires `makepkg` (`pacman`, run as a non-root user) or `nfpm`:
```sh
sh ./packaging/scripts/build-arch.sh amd64
sh ./packaging/scripts/build-arch.sh arm64

# Install with pacman:
sudo pacman -U packaging/dist/go-flow-*-x86_64.pkg.tar.zst
```

### 5. macOS Installer (`.pkg`)
Run on macOS with Xcode Command Line Tools (`pkgbuild` and `productbuild`). Supports `arm64`, `amd64`, or `universal` (fat binary via `lipo`):
```sh
sh ./packaging/scripts/build-macos-pkg.sh arm64
sh ./packaging/scripts/build-macos-pkg.sh universal

# Optional Developer ID signing:
MACOS_SIGN_IDENTITY="Developer ID Installer: Your Name (TEAMID)" \
  sh ./packaging/scripts/build-macos-pkg.sh universal
```

### 6. FreeBSD Package (`.pkg`)
Run on FreeBSD (`pkg create`):
```sh
sh ./packaging/scripts/build-freebsd-pkg.sh amd64
FREEBSD_MAJOR=14 sh ./packaging/scripts/build-freebsd-pkg.sh arm64
```

### 7. OpenBSD Package (`.tgz`)
Run on OpenBSD (`/usr/sbin/pkg_create`); outputs `packaging/dist/openbsd-<arch>/go-flow-<version>.tgz` so the archive basename strictly satisfies `packages-specs(7)`:
```sh
sh ./packaging/scripts/build-openbsd-pkg.sh amd64
sh ./packaging/scripts/build-openbsd-pkg.sh arm64

# Optional signify(1) signing:
OPENBSD_SIGNIFY_KEY="/etc/signify/my-pkg.sec" \
  sh ./packaging/scripts/build-openbsd-pkg.sh amd64
```

### 8. Windows (`.zip` & `.msi`) + WinGet (`ETLMadness.GoFlow`)
On Windows PowerShell (builds `.zip` with zero extra tools, builds `.msi` into `C:\Program Files\ETL Madness\Flow\` when WiX Toolset is installed via `dotnet tool install --global wix`, generates WinGet 1.6.0 manifests in `packaging/dist/winget/`, and runs `winget validate`):
```powershell
.\packaging\scripts\Build-Windows.ps1 -Format all -Arch amd64
# Or build both x64 and arm64:
.\packaging\scripts\Build-Windows.ps1 -Format all -Arch both
```

From Linux/macOS/FreeBSD (cross-compiles `.zip`, builds `.msi` if `wixl`/`wix` is installed, and generates WinGet YAML manifests):
```sh
sh ./packaging/scripts/build-windows.sh amd64
```

See [winget/README.md](winget/README.md) for local `winget install --manifest` testing and Authenticode code signing.

---

## Installing & Uninstalling Packages by OS / Format

### 1. Debian / Ubuntu (`.deb`)

```sh
# Install (pass ./ so apt resolves the local file)
sudo apt install ./packaging/dist/go-flow_1.2.2-1_amd64.deb
# Or directly with dpkg:
sudo dpkg -i ./packaging/dist/go-flow_1.2.2-1_amd64.deb

# Activate PATH & INFOPATH in the current shell (new login shells load this automatically)
. /etc/profile.d/go-flow.sh

# Verify
/opt/flow/bin/flow -help
man flow

# Uninstall (use 'purge' to also remove /etc/profile.d/go-flow.sh)
sudo apt purge go-flow
```

### 2. RHEL / Fedora / Rocky / AlmaLinux (`.rpm`)

```sh
# Install or upgrade
sudo dnf install ./packaging/dist/go-flow-1.2.2-1.x86_64.rpm
# Or directly with rpm:
sudo rpm -Uvh ./packaging/dist/go-flow-1.2.2-1.x86_64.rpm

# Activate PATH & INFOPATH in the current shell
. /etc/profile.d/go-flow.sh

# Verify
flow -help
man flow

# Uninstall
sudo dnf remove go-flow
```

### 3. SUSE Linux Enterprise (SLES) & openSUSE (`.rpm`)

```sh
# Install (use --no-gpg-checks or --allow-unsigned-rpm if the RPM is not GPG-signed)
sudo zypper --no-gpg-checks install ./packaging/dist/go-flow-1.2.2-1.x86_64.rpm

# Activate PATH & INFOPATH in the current shell
. /etc/profile.d/go-flow.sh

# Verify
flow -help
man flow

# Uninstall
sudo zypper remove go-flow
```

### 4. Arch Linux / Manjaro (`.pkg.tar.zst`)

```sh
# Install or upgrade local package archive
sudo pacman -U ./packaging/dist/go-flow-1.2.2-1-x86_64.pkg.tar.zst

# Activate PATH & INFOPATH in the current shell
. /etc/profile.d/go-flow.sh

# Verify
flow -help
man flow

# Uninstall
sudo pacman -Rns go-flow
```

### 5. macOS (`.pkg`)

**Graphical Install:** Double-click `go-flow-<version>-darwin-universal.pkg` (or `arm64`/`amd64`) and follow the macOS Installer prompts.

**Command-Line Install:**
```sh
# Install system-wide under /opt/flow
sudo installer -pkg ./packaging/dist/go-flow-1.2.2-darwin-universal.pkg -target /

# Refresh PATH and MANPATH in the current terminal (new terminals load paths.d automatically)
eval "$(/usr/libexec/path_helper -s)"

# Verify
flow -help
man flow

# Uninstall (macOS installer has no built-in uninstall subcommand)
sudo rm -rf /opt/flow \
            /private/etc/paths.d/go-flow \
            /private/etc/manpaths.d/go-flow
sudo pkgutil --forget com.etlmadness.go-flow
```

### 6. FreeBSD (`.pkg`)

```sh
# Install local package (add -f to force reinstall of the same version)
sudo pkg add ./packaging/dist/go-flow-1.2.2-freebsd-amd64.pkg

# Activate PATH & INFOPATH in the current shell
. /usr/local/etc/profile.d/go-flow.sh

# Verify
flow -help
man flow

# Uninstall
sudo pkg delete go-flow
```

### 7. OpenBSD (`.tgz`)

```sh
# Install local package (-D unsigned allows unsigned local archives; omit if signed with signify)
doas pkg_add -D unsigned ./packaging/dist/openbsd-amd64/go-flow-1.2.2.tgz

# Activate PATH, MANPATH, and INFOPATH in the current shell
. /etc/profile.d/go-flow.sh

# Optional: persist for future login shells on OpenBSD ksh
echo '[ -f /etc/profile.d/go-flow.sh ] && . /etc/profile.d/go-flow.sh' >> ~/.profile

# Verify
flow -help
man flow

# Uninstall
doas pkg_delete go-flow
```

### 8. Windows — WinGet (`.zip` Portable & `.msi` Machine Scope)

**From WinGet Repository (once published):**
```powershell
# Default user-scope install (portable .zip; no Administrator prompt required)
winget install --id ETLMadness.GoFlow

# Machine-wide install into C:\Program Files\ETL Madness\Flow\ (WiX .msi; requires Administrator)
winget install --id ETLMadness.GoFlow --scope machine --installer-type wix

# Uninstall
winget uninstall --id ETLMadness.GoFlow
```

**From Local Generated Manifest (`packaging/dist/winget/...`):**
```powershell
# One-time Administrator setting to allow local manifest directories:
winget settings --enable LocalManifestFiles

# Install from the local manifest folder
winget install --manifest .\packaging\dist\winget\e\ETLMadness\GoFlow\1.2.2\
```

### 9. Windows — Standalone `.msi` (`msiexec`)

**Graphical Install:** Double-click `go-flow-<version>-windows-amd64.msi`.

**Command-Line / Silent Install (Administrator PowerShell or `cmd.exe`):**
```powershell
# Silent machine-wide install into C:\Program Files\ETL Madness\Flow\
msiexec /i .\packaging\dist\go-flow-1.2.2-windows-amd64.msi /qn /norestart

# Open a new terminal so the updated PATH takes effect, then verify:
flow -help
crypto_tool -help
db_importer -help

# Silent uninstall
msiexec /x .\packaging\dist\go-flow-1.2.2-windows-amd64.msi /qn /norestart
```

### 10. Windows — Standalone Portable `.zip` (Manual without WinGet)

```powershell
# Extract to a user directory
$dest = "$env:LOCALAPPDATA\Programs\Flow"
Expand-Archive -Path .\packaging\dist\go-flow-1.2.2-windows-amd64.zip -DestinationPath $dest -Force

# Append bin\ to the end of User PATH if not already present
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if (($userPath -split ';') -notcontains "$dest\bin") {
    [Environment]::SetEnvironmentVariable("Path", "$($userPath.TrimEnd(';'));$dest\bin", "User")
}

# Uninstall: remove $dest and delete "$dest\bin" from User PATH
```

---

## Viewing Documentation After Installation

Open a new login shell (so `/opt/flow/bin` and `INFOPATH` are active) or invoke directly:

```sh
# Unix Manual pages
man flow
man crypto_tool
man db_importer
man 5 flow-pipeline

# Unix GNU Info pages
info flow
info crypto_tool
info db_importer
```

On Windows, reference guides are installed to `C:\Program Files\ETL Madness\Flow\docs\` (`.msi`) or `%LOCALAPPDATA%\Microsoft\WinGet\Packages\ETLMadness.GoFlow_...\docs\` (`.zip`).
