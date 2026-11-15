# WinGet Packaging (`ETLMadness.GoFlow`)

Running [`../scripts/Build-Windows.ps1`](../scripts/Build-Windows.ps1) (on Windows) or [`../scripts/build-windows.sh`](../scripts/build-windows.sh) (on Linux/macOS/FreeBSD) automatically builds Windows artifacts and generates a complete multi-file **WinGet 1.6.0** manifest directory under:

```text
packaging/dist/winget/e/ETLMadness/GoFlow/<version>/
├── ETLMadness.GoFlow.yaml
├── ETLMadness.GoFlow.installer.yaml
└── ETLMadness.GoFlow.locale.en-US.yaml
```

*(Notice the path `manifests/e/ETLMadness/GoFlow/<version>/` matches the exact directory hierarchy required by [`microsoft/winget-pkgs`](https://github.com/microsoft/winget-pkgs).)*

---

## Supported Installers in the Manifest

1. **Portable `.zip` (`InstallerType: zip` + `NestedInstallerType: portable`)**
   - Always built with zero external dependencies (supports both user and machine WinGet scopes automatically).
   - Extracts into `%LOCALAPPDATA%\Microsoft\WinGet\Packages\ETLMadness.GoFlow_...` (or `%ProgramFiles%\WinGet\Packages\...` with `--scope machine`) and exposes `flow`, `crypto_tool`, and `db_importer` via WinGet `Links` (at the end of `PATH`).

2. **Windows Installer `.msi` (`Scope: machine`, `InstallerType: wix`)**
   - Built automatically whenever WiX Toolset (`dotnet tool install --global wix`, Chocolatey `wixtoolset`, or Linux `msitools` `wixl`) is installed.
   - Installs isolated into `C:\Program Files\ETL Madness\Flow\` (`bin\`, `docs\`, `share\xsd\`, `share\man\`) and appends `C:\Program Files\ETL Madness\Flow\bin` to the **end** of the machine `PATH` (`Part="last"`) so existing binaries are never replaced or shadowed.

---

## Validating & Testing Locally on Windows

```powershell
# 1. Build Windows .zip, .msi (if WiX installed), and WinGet manifests
.\packaging\scripts\Build-Windows.ps1 -Format all -Arch amd64

# 2. Validate WinGet manifest schema & rules
winget validate --manifest .\packaging\dist\winget\e\ETLMadness\GoFlow\1.2.1\

# 3. Enable local manifest installs (one-time admin setting) & test install
winget settings --enable LocalManifestFiles
winget install --manifest .\packaging\dist\winget\e\ETLMadness\GoFlow\1.2.1\
```

---

## Prerequisites to Build `.msi`

If WiX is not yet installed on Windows, install WiX v5/v6 via the .NET SDK in one command:

```powershell
dotnet tool install --global wix
```

Or via Chocolatey:

```powershell
choco install wixtoolset -y
```

On Linux build runners, install `msitools` (`apt install msitools` or `dnf install msitools`) to cross-build the `.msi` via `wixl`.
