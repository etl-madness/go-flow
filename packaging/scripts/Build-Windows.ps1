<#
.SYNOPSIS
    Builds Windows WinGet-compatible packages (.zip portable archive and .msi installer)
    and generates validated WinGet 1.6.0 multi-file YAML manifests for ETLMadness.GoFlow.

.DESCRIPTION
    Compiles flow.exe, crypto_tool.exe, and db_importer.exe (with no Windows service),
    bundles XSD schemas and CLI/XML documentation, builds:
      1. go-flow-<version>-windows-<arch>.zip (WinGet NestedInstallerType: portable)
      2. go-flow-<version>-windows-<arch>.msi (WiX v4/v5/v6 'wix', WiX v3 'candle/light', or 'wixl')
    and generates WinGet manifests in packaging/dist/winget/e/ETLMadness/GoFlow/<version>/.

.EXAMPLE
    .\packaging\scripts\Build-Windows.ps1
    .\packaging\scripts\Build-Windows.ps1 -Format all -Arch both
    .\packaging\scripts\Build-Windows.ps1 -Format msi -Arch amd64 -RequireMsi
#>
[CmdletBinding()]
param(
    [ValidateSet("all", "zip", "msi")]
    [string]$Format = "all",

    [ValidateSet("amd64", "arm64", "both")]
    [string]$Arch = "amd64",

    [string]$Version = "",

    [string]$ReleaseBaseUrl = "",

    [string]$SignCertThumbprint = "",

    [string]$TimestampServer = "http://timestamp.digicert.com",

    [switch]$RequireMsi,

    [switch]$SkipWingetValidate
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$ScriptsDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$PackagingDir = Split-Path -Parent $ScriptsDir
$RepoRoot = Split-Path -Parent $PackagingDir
$BuildRoot = Join-Path $PackagingDir "build"
$DistDir = Join-Path $PackagingDir "dist"
$UpgradeCode = "{8D4A7E29-6C3F-4B18-9E52-3F71C0A8E412}"

if ([string]::IsNullOrWhiteSpace($Version)) {
    try {
        $rawTag = git -C $RepoRoot describe --tags --abbrev=0 2>$null
        if ($LASTEXITCODE -eq 0 -and -not [string]::IsNullOrWhiteSpace($rawTag)) {
            $Version = $rawTag -replace '^[vV]', ''
        } else {
            $Version = "1.2.1"
        }
    } catch {
        $Version = "1.2.1"
    }
}

if ([string]::IsNullOrWhiteSpace($ReleaseBaseUrl)) {
    $ReleaseBaseUrl = "https://github.com/etl-madness/go-flow/releases/download/v$Version"
}
$ReleaseBaseUrl = $ReleaseBaseUrl.TrimEnd('/')

function Get-MsiNumericVersion {
    param([string]$Ver)
    $clean = ($Ver -split '[-+]')[0]
    $parts = $clean -split '\.'
    $nums = @(0, 0, 0)
    for ($i = 0; $i -lt [Math]::Min(3, $parts.Count); $i++) {
        $val = 0
        if ([int]::TryParse($parts[$i], [ref]$val)) {
            $nums[$i] = [Math]::Min(65535, [Math]::Max(0, $val))
        }
    }
    return "$($nums[0]).$($nums[1]).$($nums[2])"
}

function Get-DeterministicProductCode {
    param([string]$Ver, [string]$GoArch)
    $sha = [System.Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [System.Text.Encoding]::UTF8.GetBytes("ETLMadness.GoFlow.MSI:$Ver`:$GoArch")
        $hash = $sha.ComputeHash($bytes)
        $guidBytes = New-Object byte[] 16
        [Array]::Copy($hash, 0, $guidBytes, 0, 16)
        # RFC 4122 version 4/5 variant bits
        $guidBytes[7] = ($guidBytes[7] -band 0x0F) -bor 0x50
        $guidBytes[8] = ($guidBytes[8] -band 0x3F) -bor 0x80
        $g = New-Object System.Guid (,$guidBytes)
        return "{$($g.ToString().ToUpperInvariant())}"
    } finally {
        $sha.Dispose()
    }
}

function Resolve-GuideFile {
    param([string]$FileName)
    $flowDocsCandidate = Join-Path (Split-Path -Parent $RepoRoot) "flow-docs\docs\guides\$FileName"
    if (Test-Path $flowDocsCandidate) {
        return $flowDocsCandidate
    }
    $localCandidate = Join-Path $RepoRoot "docs\guides\$FileName"
    if (Test-Path $localCandidate) {
        return $localCandidate
    }
    throw "Required documentation file not found: $FileName"
}

function Invoke-AuthenticodeSign {
    param([string]$FilePath)
    if ([string]::IsNullOrWhiteSpace($SignCertThumbprint)) {
        return
    }
    $signtool = Get-Command signtool.exe -ErrorAction SilentlyContinue
    if (-not $signtool) {
        throw "SignCertThumbprint was provided, but signtool.exe was not found in PATH."
    }
    Write-Host "[INFO] Signing $FilePath..."
    & $signtool.Source sign /sha1 $SignCertThumbprint /fd SHA256 /tr $TimestampServer /td SHA256 $FilePath
    if ($LASTEXITCODE -ne 0) {
        throw "signtool.exe failed on $FilePath"
    }
}

function Stage-WindowsPayload {
    param([string]$GoArch, [string]$StageDir)
    if (Test-Path $StageDir) {
        Remove-Item -Recurse -Force $StageDir
    }
    $binDir = Join-Path $StageDir "bin"
    $docsDir = Join-Path $StageDir "docs"
    $xsdDir = Join-Path $StageDir "share\xsd"
    $man1Dir = Join-Path $StageDir "share\man\man1"
    $man5Dir = Join-Path $StageDir "share\man\man5"

    New-Item -ItemType Directory -Path $binDir, $docsDir, $xsdDir, $man1Dir, $man5Dir -Force | Out-Null

    Write-Host "[INFO] Compiling flow.exe, crypto_tool.exe, and db_importer.exe for windows/$GoArch..."
    Push-Location $RepoRoot
    try {
        $env:CGO_ENABLED = "0"
        $env:GOOS = "windows"
        $env:GOARCH = $GoArch

        go build -trimpath -ldflags="-s -w" -o (Join-Path $binDir "flow.exe") .
        if ($LASTEXITCODE -ne 0) { throw "go build failed for flow.exe (windows/$GoArch)" }

        go build -trimpath -ldflags="-s -w" -o (Join-Path $binDir "crypto_tool.exe") ./crypto_tool
        if ($LASTEXITCODE -ne 0) { throw "go build failed for crypto_tool.exe (windows/$GoArch)" }

        go build -trimpath -ldflags="-s -w" -o (Join-Path $binDir "db_importer.exe") ./db_importer
        if ($LASTEXITCODE -ne 0) { throw "go build failed for db_importer.exe (windows/$GoArch)" }
    } finally {
        Remove-Item Env:\CGO_ENABLED -ErrorAction SilentlyContinue
        Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue
        Pop-Location
    }

    Invoke-AuthenticodeSign (Join-Path $binDir "flow.exe")
    Invoke-AuthenticodeSign (Join-Path $binDir "crypto_tool.exe")
    Invoke-AuthenticodeSign (Join-Path $binDir "db_importer.exe")

    Copy-Item -Force (Resolve-GuideFile "COMPLETE_CLI_OPTIONS_AND_ARGUMENTS.md") (Join-Path $docsDir "COMPLETE_CLI_OPTIONS_AND_ARGUMENTS.md")
    Copy-Item -Force (Resolve-GuideFile "SUMMARY_OF_AVAILABLE_OPTIONS.md") (Join-Path $docsDir "SUMMARY_OF_AVAILABLE_OPTIONS.md")
    Copy-Item -Force (Join-Path $RepoRoot "xsd\pipeline.xsd") (Join-Path $xsdDir "pipeline.xsd")
    Copy-Item -Force (Join-Path $RepoRoot "xsd\options.xsd") (Join-Path $xsdDir "options.xsd")
    Copy-Item -Force (Join-Path $PackagingDir "man\man1\flow.1") (Join-Path $man1Dir "flow.1")
    Copy-Item -Force (Join-Path $PackagingDir "man\man1\crypto_tool.1") (Join-Path $man1Dir "crypto_tool.1")
    Copy-Item -Force (Join-Path $PackagingDir "man\man1\db_importer.1") (Join-Path $man1Dir "db_importer.1")
    Copy-Item -Force (Join-Path $PackagingDir "man\man5\flow-pipeline.5") (Join-Path $man5Dir "flow-pipeline.5")
    Copy-Item -Force (Join-Path $RepoRoot "LICENSE") (Join-Path $StageDir "LICENSE.txt")
}

function Build-PortableZip {
    param([string]$StageDir, [string]$OutZip)
    if (Test-Path $OutZip) {
        Remove-Item -Force $OutZip
    }
    Write-Host "[INFO] Creating portable archive: $OutZip"
    $zipStream = [System.IO.File]::Open($OutZip, [System.IO.FileMode]::Create)
    try {
        $archive = New-Object System.IO.Compression.ZipArchive($zipStream, [System.IO.Compression.ZipArchiveMode]::Create)
        try {
            $baseLen = $StageDir.TrimEnd('\', '/').Length + 1
            Get-ChildItem -Path $StageDir -Recurse -File | Sort-Object FullName | ForEach-Object {
                $relPath = $_.FullName.Substring($baseLen) -replace '\\', '/'
                [System.IO.Compression.ZipFileExtensions]::CreateEntryFromFile(
                    $archive,
                    $_.FullName,
                    $relPath,
                    [System.IO.Compression.CompressionLevel]::Optimal
                ) | Out-Null
            }
        } finally {
            $archive.Dispose()
        }
    } finally {
        $zipStream.Dispose()
    }
}

function Build-WindowsMsi {
    param([string]$GoArch, [string]$StageDir, [string]$OutMsi, [string]$ProductCode)
    $msiVersion = Get-MsiNumericVersion -Ver $Version
    $wixArch = if ($GoArch -eq "amd64") { "x64" } else { "arm64" }

    # Check WiX v4/v5/v6 CLI ('wix') — including ~/.dotnet/tools/wix.exe if installed via dotnet tool
    $wixCmd = Get-Command wix -ErrorAction SilentlyContinue
    if (-not $wixCmd) {
        $dotnetToolWix = Join-Path $HOME ".dotnet\tools\wix.exe"
        if (Test-Path $dotnetToolWix) {
            $wixCmd = Get-Item $dotnetToolWix
        }
    }

    $candleCmd = Get-Command candle.exe -ErrorAction SilentlyContinue
    $lightCmd = Get-Command light.exe -ErrorAction SilentlyContinue
    $wixlCmd = Get-Command wixl -ErrorAction SilentlyContinue

    if ($wixCmd) {
        $wixExe = if ($wixCmd.PSObject.Properties['Source']) { $wixCmd.Source } else { $wixCmd.FullName }
        Write-Host "[INFO] Building MSI with WiX Toolset ($wixExe) for $wixArch..."
        $wxsPath = Join-Path $PackagingDir "windows\flow.wxs"
        & $wixExe build $wxsPath `
            -arch $wixArch `
            -d "StageDir=$StageDir" `
            -d "FlowMsiVersion=$msiVersion" `
            -d "FlowProductCode=$ProductCode" `
            -o $OutMsi
        if ($LASTEXITCODE -ne 0) {
            throw "wix build failed for $OutMsi"
        }
        Invoke-AuthenticodeSign $OutMsi
        return $true
    } elseif ($candleCmd -and $lightCmd) {
        Write-Host "[INFO] Building MSI with WiX v3 (candle/light) for $wixArch..."
        $wxs3Path = Join-Path $PackagingDir "windows\flow-wix3.wxs"
        $wixObj = Join-Path (Split-Path -Parent $StageDir) "flow.wixobj"
        & $candleCmd.Source -nologo -arch $wixArch `
            "-dStageDir=$StageDir" `
            "-dFlowMsiVersion=$msiVersion" `
            "-dFlowProductCode=$ProductCode" `
            -out $wixObj $wxs3Path
        if ($LASTEXITCODE -ne 0) { throw "candle.exe failed" }

        & $lightCmd.Source -nologo -sval -out $OutMsi $wixObj
        if ($LASTEXITCODE -ne 0) { throw "light.exe failed" }
        Invoke-AuthenticodeSign $OutMsi
        return $true
    } elseif ($wixlCmd) {
        Write-Host "[INFO] Building MSI with wixl (msitools) for $wixArch..."
        $wxs3Path = Join-Path $PackagingDir "windows\flow-wix3.wxs"
        & $wixlCmd.Source --arch $wixArch `
            -D "StageDir=$StageDir" `
            -D "FlowMsiVersion=$msiVersion" `
            -D "FlowProductCode=$ProductCode" `
            -o $OutMsi $wxs3Path
        if ($LASTEXITCODE -ne 0) { throw "wixl failed" }
        return $true
    } else {
        $msg = "WiX Toolset not found. Install WiX v5/v6 via 'dotnet tool install --global wix' (or 'choco install wixtoolset') to build .msi installers."
        if ($Format -eq "msi" -or $RequireMsi) {
            throw $msg
        }
        Write-Warning "$msg Skipping .msi build (portable .zip and WinGet manifests will still be generated)."
        return $false
    }
}

New-Item -ItemType Directory -Path $DistDir -Force | Out-Null

$archList = if ($Arch -eq "both") { @("amd64", "arm64") } else { @($Arch) }
$installerEntries = New-Object System.Collections.Generic.List[string]

foreach ($goArch in $archList) {
    $wingetArch = if ($goArch -eq "amd64") { "x64" } else { "arm64" }
    $workDir = Join-Path $BuildRoot "windows-$goArch"
    $stageDir = Join-Path $workDir "payload"

    Stage-WindowsPayload -GoArch $goArch -StageDir $stageDir

    if ($Format -in @("all", "zip")) {
        $zipName = "go-flow-$Version-windows-$goArch.zip"
        $zipPath = Join-Path $DistDir $zipName
        Build-PortableZip -StageDir $stageDir -OutZip $zipPath
        $zipSha256 = (Get-FileHash -Algorithm SHA256 -Path $zipPath).Hash.ToUpperInvariant()

        $zipBlock = @"
  - Architecture: $wingetArch
    InstallerType: zip
    NestedInstallerType: portable
    NestedInstallerFiles:
      - RelativeFilePath: bin\flow.exe
        PortableCommandAlias: flow
      - RelativeFilePath: bin\crypto_tool.exe
        PortableCommandAlias: crypto_tool
      - RelativeFilePath: bin\db_importer.exe
        PortableCommandAlias: db_importer
    InstallerUrl: $ReleaseBaseUrl/$zipName
    InstallerSha256: $zipSha256
"@
        $installerEntries.Add($zipBlock.TrimEnd())
    }

    if ($Format -in @("all", "msi")) {
        $msiName = "go-flow-$Version-windows-$goArch.msi"
        $msiPath = Join-Path $DistDir $msiName
        $productCode = Get-DeterministicProductCode -Ver $Version -GoArch $goArch
        $msiBuilt = Build-WindowsMsi -GoArch $goArch -StageDir $stageDir -OutMsi $msiPath -ProductCode $productCode
        if ($msiBuilt -and (Test-Path $msiPath)) {
            $msiSha256 = (Get-FileHash -Algorithm SHA256 -Path $msiPath).Hash.ToUpperInvariant()
            $msiBlock = @"
  - Architecture: $wingetArch
    Scope: machine
    InstallerType: wix
    InstallerUrl: $ReleaseBaseUrl/$msiName
    InstallerSha256: $msiSha256
    ProductCode: '$productCode'
    AppsAndFeaturesEntries:
      - DisplayName: ETL Madness Flow
        Publisher: ETL Madness
        DisplayVersion: $(Get-MsiNumericVersion -Ver $Version)
        ProductCode: '$productCode'
        UpgradeCode: '$UpgradeCode'
"@
            $installerEntries.Add($msiBlock.TrimEnd())
        }
    }
}

if ($installerEntries.Count -eq 0) {
    throw "No Windows installer artifacts were produced."
}

$wingetManifestDir = Join-Path $DistDir "winget\e\ETLMadness\GoFlow\$Version"
New-Item -ItemType Directory -Path $wingetManifestDir -Force | Out-Null

$versionYaml = @"
# yaml-language-server: `$schema=https://aka.ms/winget-manifest.version.1.6.0.schema.json

PackageIdentifier: ETLMadness.GoFlow
PackageVersion: $Version
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
"@

$localeYaml = @"
# yaml-language-server: `$schema=https://aka.ms/winget-manifest.defaultLocale.1.6.0.schema.json

PackageIdentifier: ETLMadness.GoFlow
PackageVersion: $Version
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
"@

$joinedInstallers = ($installerEntries -join "`n")
$installerYaml = @"
# yaml-language-server: `$schema=https://aka.ms/winget-manifest.installer.1.6.0.schema.json

PackageIdentifier: ETLMadness.GoFlow
PackageVersion: $Version
Commands:
  - flow
  - crypto_tool
  - db_importer
Installers:
$joinedInstallers
ManifestType: installer
ManifestVersion: 1.6.0
"@

$utf8NoBom = New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $wingetManifestDir "ETLMadness.GoFlow.yaml"), ($versionYaml -replace "`r`n", "`n") + "`n", $utf8NoBom)
[System.IO.File]::WriteAllText((Join-Path $wingetManifestDir "ETLMadness.GoFlow.locale.en-US.yaml"), ($localeYaml -replace "`r`n", "`n") + "`n", $utf8NoBom)
[System.IO.File]::WriteAllText((Join-Path $wingetManifestDir "ETLMadness.GoFlow.installer.yaml"), ($installerYaml -replace "`r`n", "`n") + "`n", $utf8NoBom)

Write-Host "[INFO] Generated WinGet manifests in: $wingetManifestDir"

if (-not $SkipWingetValidate) {
    $wingetCmd = Get-Command winget.exe -ErrorAction SilentlyContinue
    if ($wingetCmd) {
        Write-Host "[INFO] Validating WinGet manifests with winget validate..."
        & $wingetCmd.Source validate --manifest $wingetManifestDir
        if ($LASTEXITCODE -ne 0) {
            throw "winget validate reported manifest errors in $wingetManifestDir"
        }
    }
}
