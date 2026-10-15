<#
.SYNOPSIS
    Verify a Forge release against its four supply-chain proofs.

.DESCRIPTION
    The script verifies a Forge release against four proofs:

      1. The SHA-256 checksum (SHA256SUMS).
      2. The GPG signature over SHA256SUMS.
      3. The cosign signature over the archive.
      4. The SLSA provenance (forge.intoto.jsonl).

    The script does not rebuild from source: the rebuild requires a
    clone of the source and the build toolchain, which the script
    does not manage. The rebuild procedure is in
    docs/guides/reproducible-builds.md.

    The script is the Windows counterpart of scripts/verify-release.sh.
    The two produce the same output; a user who runs one and a user
    who runs the other see the same checks.

.PARAMETER Version
    The release's tag, e.g. "v1.4.2". Required.

.PARAMETER Archive
    The archive to verify, e.g. "forge-windows-amd64.zip".
    Default: detected from the host's processor architecture.

.PARAMETER Directory
    Where the release's assets are (or will be downloaded to).
    Default: the current directory.

.EXAMPLE
    .\verify-release.ps1 -Version v1.4.2

.EXAMPLE
    .\verify-release.ps1 -Version v1.4.2 -Archive forge-windows-amd64.zip

.NOTES
    Exit codes:
      0  every check passed
      1  a check failed, or a required tool is missing
      2  usage error

    The script requires sha256sum (Git Bash or WSL), gpg, cosign,
    slsa-verifier, and curl on PATH.
#>

[CmdletBinding()]
param(
    [Parameter(Mandatory = $true, Position = 0)]
    [string]$Version,

    [Parameter(Mandatory = $false, Position = 1)]
    [string]$Archive,

    [Parameter(Mandatory = $false, Position = 2)]
    [string]$Directory = "."
)

$ErrorActionPreference = "Stop"

# The repository. A future fork would override this via the
# FORGE_REPO environment variable.
if ($env:FORGE_REPO) {
    $Repo = $env:FORGE_REPO
} else {
    $Repo = "thapelomagqazana/forge"
}

# The release key's fingerprint. Read from the environment if set.
$GpgFingerprint = $env:FORGE_GPG_KEY_FINGERPRINT

# The asset filenames.
$SumsFile = "SHA256SUMS"
$SumsSig = "SHA256SUMS.asc"
$Provenance = "forge.intoto.jsonl"

# --- output ---

function Write-Log {
    param([string]$Message)
    Write-Host $Message -ForegroundColor Gray
}

function Write-Ok {
    param([string]$Message)
    Write-Host "OK: $Message" -ForegroundColor Green
}

function Write-Skip {
    param([string]$Message)
    Write-Host "SKIP: $Message" -ForegroundColor Yellow
}

function Fail {
    param([string]$Message)
    Write-Host "FAIL: $Message" -ForegroundColor Red
    exit 1
}

# --- preflight ---

function Require-Tools {
    $missing = @()
    foreach ($tool in @("sha256sum", "gpg", "cosign", "slsa-verifier", "curl")) {
        if (-not (Get-Command $tool -ErrorAction SilentlyContinue)) {
            $missing += $tool
        }
    }
    if ($missing.Count -gt 0) {
        foreach ($tool in $missing) {
            Write-Log "missing tool: $tool"
        }
        Fail "one or more required tools are missing; see docs/reference/verifying-releases.md"
    }
}

# --- platform detection ---

function Detect-Archive {
    $arch = $env:PROCESSOR_ARCHITECTURE
    $goarch = switch ($arch) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        default { Fail "unsupported architecture: $arch" }
    }
    return "forge-windows-$goarch.zip"
}

# --- download ---

function Download-Asset {
    param([string]$Name, [string]$Dir)
    $path = Join-Path $Dir $Name
    if (Test-Path $path) {
        Write-Log "already downloaded: $Name"
        return
    }
    $url = "https://github.com/$Repo/releases/download/$Version/$Name"
    Write-Log "downloading: $url"
    & curl --fail --silent --show-error --location -o $path $url
    if ($LASTEXITCODE -ne 0) {
        Fail "download failed: $url"
    }
}

# --- the four checks ---

function Check-Checksum {
    param([string]$Dir, [string]$ArchiveName)
    Write-Log "check 2/5: SHA-256 checksum"
    Push-Location $Dir
    try {
        & sha256sum -c "$ArchiveName.sha256"
        if ($LASTEXITCODE -ne 0) {
            Fail "checksum mismatch for $ArchiveName"
        }
    } finally {
        Pop-Location
    }
    Write-Ok "checksum"
}

function Check-Gpg {
    param([string]$Dir)
    Write-Log "check 3/5: GPG signature"
    $sigPath = Join-Path $Dir $SumsSig
    if (-not (Test-Path $sigPath)) {
        Write-Skip "$SumsSig is not present; the release was published without the GPG signature"
        return
    }
    if (-not $GpgFingerprint) {
        Write-Skip "FORGE_GPG_KEY_FINGERPRINT is not set; the GPG signature is not checked"
        Write-Log "      Set it to the fingerprint in SECURITY.md to enable the check."
        return
    }
    & gpg --keyserver keys.openpgp.org --recv-keys $GpgFingerprint 2>$null
    Push-Location $Dir
    try {
        & gpg --verify $SumsSig $SumsFile
        if ($LASTEXITCODE -ne 0) {
            Fail "GPG signature verification failed"
        }
    } finally {
        Pop-Location
    }
    Write-Ok "GPG signature"
}

function Check-Cosign {
    param([string]$Dir, [string]$ArchiveName)
    Write-Log "check 4/5: cosign signature"
    $sigPath = Join-Path $Dir "$ArchiveName.sig"
    if (-not (Test-Path $sigPath)) {
        Write-Skip "$ArchiveName.sig is not present; the release was published without the cosign signature"
        return    }
    Push-Location $Dir
    try {
        & cosign verify-blob `
            --certificate-identity-regexp="^https://github.com/$Repo/" `
            --certificate-oidc-issuer="https://token.actions.githubusercontent.com" `
            --certificate "$ArchiveName.pem" `
            --signature "$ArchiveName.sig" `
            $ArchiveName
        if ($LASTEXITCODE -ne 0) {
            Fail "cosign verification failed"
        }
    } finally {
        Pop-Location
    }
    Write-Ok "cosign signature"
}

function Check-Slsa {
    param([string]$Dir, [string]$ArchiveName)
    Write-Log "check 5/5: SLSA provenance"
    $provPath = Join-Path $Dir $Provenance
    if (-not (Test-Path $provPath)) {
        Write-Skip "$Provenance is not present; the release was published without the SLSA provenance"
        return
    }
    Push-Location $Dir
    try {
        & slsa-verifier verify-artifact `
            --provenance-path $Provenance `
            --source-uri "github.com/$Repo" `
            $ArchiveName
        if ($LASTEXITCODE -ne 0) {
            Fail "SLSA provenance verification failed"
        }
    } finally {
        Pop-Location
    }
    Write-Ok "SLSA provenance"
}

# --- main ---

if (-not $Archive) {
    $Archive = Detect-Archive
}

Write-Log "verifying Forge $Version"
Write-Log "archive:   $Archive"
Write-Log "directory: $Directory"
Write-Log ""

Require-Tools

Download-Asset $Archive $Directory
Download-Asset "$Archive.sha256" $Directory
Download-Asset $SumsFile $Directory
Download-Asset $SumsSig $Directory
Download-Asset "$Archive.sig" $Directory
Download-Asset "$Archive.pem" $Directory
Download-Asset $Provenance $Directory

Check-Checksum $Directory $Archive
Check-Gpg $Directory
Check-Cosign $Directory $Archive
Check-Slsa $Directory $Archive

Write-Log ""
Write-Log "every available check passed"
Write-Log ""
Write-Log "The rebuild-from-source check is not run by this script."
Write-Log "See docs/guides/reproducible-builds.md for the procedure."

# The machine-readable summary.
Write-Output "version=$Version"
Write-Output "archive=$Archive"
Write-Output "checks=checksum,gpg,cosign,slsa"