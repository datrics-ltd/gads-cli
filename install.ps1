# gads-cli Windows installer
# Usage: irm https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.ps1 | iex
# With GitHub PAT (private repo):
#   $env:GITHUB_TOKEN = "ghp_xxx"
#   irm https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.ps1 | iex

[CmdletBinding()]
param(
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"

$Repo    = "datrics-ltd/gads-cli"
$Binary  = "gads"
$Os      = "windows"
$Arch    = "amd64"
$BinaryFile = "$Binary-$Os-$Arch.exe"

# Determine install directory
if ($InstallDir -eq "") {
    if ($env:GADS_INSTALL_DIR -ne $null -and $env:GADS_INSTALL_DIR -ne "") {
        $InstallDir = $env:GADS_INSTALL_DIR
    } else {
        $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\gads"
    }
}

# Build request headers
$Headers = @{ "User-Agent" = "gads-cli-installer" }
if ($env:GITHUB_TOKEN) {
    $Headers["Authorization"] = "token $($env:GITHUB_TOKEN)"
}

# Fetch latest release
Write-Host "Fetching latest release..."
$ApiUrl = "https://api.github.com/repos/$Repo/releases/latest"
try {
    $Release = Invoke-RestMethod -Uri $ApiUrl -Headers $Headers
} catch {
    Write-Error "Failed to fetch latest release from $ApiUrl`n$_"
    exit 1
}

$Version = $Release.tag_name
if (-not $Version) {
    Write-Error "Could not determine latest release version. Check that the repository has published releases."
    exit 1
}

Write-Host "Latest version: $Version"

$BaseUrl      = "https://github.com/$Repo/releases/download/$Version"
$BinaryUrl    = "$BaseUrl/$BinaryFile"
$ChecksumUrl  = "$BaseUrl/checksums.txt"

# Create temp directory
$TmpDir = Join-Path $env:TEMP "gads-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir | Out-Null

$BinaryPath   = Join-Path $TmpDir $BinaryFile
$ChecksumPath = Join-Path $TmpDir "checksums.txt"

try {
    # Download binary
    Write-Host "Downloading $BinaryFile..."
    try {
        Invoke-WebRequest -Uri $BinaryUrl -OutFile $BinaryPath -Headers $Headers
    } catch {
        Write-Error "Failed to download binary from $BinaryUrl`n$_"
        exit 1
    }

    # Download checksums (optional)
    $HasChecksums = $false
    try {
        Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath -Headers $Headers
        $HasChecksums = $true
    } catch {
        Write-Warning "checksums.txt not found — skipping checksum verification"
    }

    # Verify SHA256 checksum
    if ($HasChecksums) {
        Write-Host "Verifying checksum..."
        $ChecksumContent = Get-Content $ChecksumPath
        $ExpectedLine = $ChecksumContent | Where-Object { $_ -match [regex]::Escape($BinaryFile) } | Select-Object -First 1
        if (-not $ExpectedLine) {
            Write-Warning "No checksum found for $BinaryFile in checksums.txt — skipping verification"
        } else {
            $ExpectedHash = ($ExpectedLine -split '\s+')[0].ToUpper()
            $ActualHash   = (Get-FileHash -Path $BinaryPath -Algorithm SHA256).Hash.ToUpper()
            if ($ActualHash -ne $ExpectedHash) {
                Write-Error "Checksum mismatch!`n  Expected: $ExpectedHash`n  Actual:   $ActualHash`nThe downloaded file may be corrupted or tampered with."
                exit 1
            }
            Write-Host "Checksum verified."
        }
    }

    # Ensure install directory exists
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir | Out-Null
    }

    # Install binary
    $InstallPath = Join-Path $InstallDir "gads.exe"
    Copy-Item -Path $BinaryPath -Destination $InstallPath -Force

    Write-Host ""
    Write-Host "gads $Version installed to $InstallPath"

    # Check if install dir is on PATH
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        Write-Host ""
        Write-Host "NOTE: $InstallDir is not in your PATH."
        Write-Host "Adding it now for the current user..."
        $NewPath = "$InstallDir;$UserPath"
        [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
        $env:PATH = "$InstallDir;$env:PATH"
        Write-Host "Done. Restart your terminal (or open a new PowerShell window) for the change to take effect."
    }

} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}

# Print next steps
Write-Host ""
Write-Host "Next steps:"
Write-Host "  1. Set your developer token:   gads config set developer_token YOUR_TOKEN"
Write-Host "  2. Set your OAuth2 client ID:  gads config set client_id YOUR_CLIENT_ID"
Write-Host "  3. Set your client secret:     gads config set client_secret YOUR_SECRET"
Write-Host "  4. Set your customer ID:       gads config set default_customer_id 123-456-7890"
Write-Host "  5. Authenticate:               gads auth login"
Write-Host ""
Write-Host "Run 'gads --help' to see all available commands."
