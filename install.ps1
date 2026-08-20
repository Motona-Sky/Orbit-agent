param(
    [ValidateSet("install", "uninstall", "help")]
    [string]$Action = "install"
)

$BinaryName = "orbit.exe"
$InstallDir = Join-Path $env:USERPROFILE ".orbit\bin"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$SourceBinary = Join-Path $ScriptDir $BinaryName

function Get-CurrentArch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        "x86"   { return "x86" }
        default { return $null }
    }
}

function Get-BinaryArch {
    param([string]$Path)
    try {
        $bytes = [System.IO.File]::ReadAllBytes($Path)
        if ($bytes.Length -lt 0x40) { return $null }
        $peOffset = [BitConverter]::ToInt32($bytes, 0x3C)
        if ($peOffset -le 0 -or ($peOffset + 6) -gt $bytes.Length) { return $null }
        $machine = [BitConverter]::ToUInt16($bytes, $peOffset + 4)
        switch ($machine) {
            0x8664 { return "amd64" }
            0xAA64 { return "arm64" }
            0x014C { return "x86" }
            0x01C0 { return "arm" }
            default { return $null }
        }
    } catch {
        return $null
    }
}

function Test-Architecture {
    $current = Get-CurrentArch
    $binary = Get-BinaryArch -Path $SourceBinary

    if ([string]::IsNullOrEmpty($binary)) {
        Write-Host "  Warning: could not detect binary architecture, skipping check." -ForegroundColor Yellow
        return
    }
    if ([string]::IsNullOrEmpty($current)) {
        Write-Host "  Warning: could not detect system architecture, skipping check." -ForegroundColor Yellow
        return
    }
    if ($binary -eq $current) {
        Write-Host "  Architecture check passed ($current)." -ForegroundColor Green
    } else {
        Write-Host "Error: binary architecture ($binary) does not match the current system ($current)." -ForegroundColor Red
        Write-Host "Please download the correct architecture package." -ForegroundColor Red
        exit 1
    }
}

function Show-Usage {
    Write-Host "Usage: .\install.ps1 [install|uninstall|help]" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "  install    Install $BinaryName to $InstallDir and configure PATH"
    Write-Host "  uninstall  Uninstall $BinaryName and clean up PATH"
    Write-Host "  help       Show this help message"
    Write-Host ""
    Write-Host "Defaults to install when no argument is provided."
}

function Add-ToUserPath {
    param([string]$Directory)

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $paths = $currentPath -split ";" | Where-Object { $_ -ne "" }

    if ($paths -contains $Directory) {
        Write-Host "  $Directory is already in user PATH." -ForegroundColor Yellow
        return
    }

    $newPath = ($paths + $Directory) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "  Added $Directory to user PATH." -ForegroundColor Green

    if ($env:Path -notlike "*$Directory*") {
        $env:Path = "$Directory;$env:Path"
    }
}

function Remove-FromUserPath {
    param([string]$Directory)

    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $paths = $currentPath -split ";" | Where-Object { $_ -ne "" }

    if ($paths -notcontains $Directory) {
        Write-Host "  $Directory is not in user PATH, skipping." -ForegroundColor Yellow
        return
    }

    $newPaths = $paths | Where-Object { $_ -ne $Directory }
    $newPath = $newPaths -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "  Removed $Directory from user PATH." -ForegroundColor Green
}

function Install-Orbit {
    Write-Host "Installing Orbit Agent ..." -ForegroundColor Cyan

    if (-not (Test-Path $SourceBinary)) {
        Write-Host "Error: $SourceBinary not found." -ForegroundColor Red
        Write-Host "Please make sure $BinaryName is in the same directory as this script." -ForegroundColor Red
        exit 1
    }

    Test-Architecture

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        Write-Host "  Created directory $InstallDir"
    }

    $destPath = Join-Path $InstallDir $BinaryName
    Copy-Item -Path $SourceBinary -Destination $destPath -Force
    Write-Host "  Copied $BinaryName to $InstallDir\"

    Add-ToUserPath -Directory $InstallDir

    Write-Host ""
    Write-Host "Installation complete!" -ForegroundColor Green
    Write-Host "Restart your terminal, then you can use: orbit" -ForegroundColor Cyan
}

function Uninstall-Orbit {
    Write-Host "Uninstalling Orbit Agent ..." -ForegroundColor Cyan

    $destPath = Join-Path $InstallDir $BinaryName

    if (Test-Path $destPath) {
        Remove-Item -Path $destPath -Force
        Write-Host "  Removed $destPath" -ForegroundColor Green
    } else {
        Write-Host "  $destPath does not exist, skipping." -ForegroundColor Yellow
    }

    if ((Test-Path $InstallDir) -and ((Get-ChildItem $InstallDir | Measure-Object).Count -eq 0)) {
        Remove-Item -Path $InstallDir -Force
        Write-Host "  Removed empty directory $InstallDir"
    }

    Remove-FromUserPath -Directory $InstallDir

    Write-Host ""
    Write-Host "Uninstallation complete!" -ForegroundColor Green
}

switch ($Action) {
    "install"   { Install-Orbit }
    "uninstall" { Uninstall-Orbit }
    "help"      { Show-Usage }
}
