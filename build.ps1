# Mahiru DyBot build script
# Usage:
#   .\build.ps1              # Build Windows + Linux amd64, package deploy archive
#   .\build.ps1 -WebUI       # Build frontend first (requires Node)
param(
    [switch]$WebUI
)

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if ($WebUI) {
    Write-Host "==> Building frontend (webui-src -> webui)" -ForegroundColor Cyan
    Push-Location webui-src
    npm run build
    Pop-Location
}

Write-Host "==> Building Windows amd64" -ForegroundColor Cyan
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w" -o dist/windows-amd64/mahiru-dybot.exe .
if ($LASTEXITCODE -ne 0) { throw "windows build failed" }

Write-Host "==> Building Linux amd64" -ForegroundColor Cyan
$env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w" -o dist/linux-amd64/mahiru-dybot .
if ($LASTEXITCODE -ne 0) { throw "linux build failed" }
Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "==> Packaging" -ForegroundColor Cyan
$stamp = Get-Date -Format "yyyyMMdd-HHmm"
$pkg = "dist/mahiru-dybot-linux-$stamp"
New-Item -ItemType Directory -Force -Path $pkg | Out-Null

Copy-Item dist/linux-amd64/mahiru-dybot "$pkg/"
Copy-Item config.json "$pkg/config.example.json"
Copy-Item API.md "$pkg/" -ErrorAction SilentlyContinue

# README for deployment
@'
# Mahiru DyBot - Linux amd64 deployment

## Requirements
- System Chrome/Chromium: apt install -y google-chrome-stable
- Playwright browsers: npx playwright install chromium
- Port 17836 (configurable in config.json via listen_addr)

## Quick Start
    chmod +x mahiru-dybot
    cp config.example.json config.json
    ./mahiru-dybot

## First Run
1. Open http://<host>:17836/webui/
2. Set admin password (or delete storage/webui_auth.json to reset)
'@ | Set-Content "$pkg/README.txt" -Encoding UTF8

Write-Host "==> Package ready: $pkg" -ForegroundColor Green
