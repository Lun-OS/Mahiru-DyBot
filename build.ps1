# Mahiru DyBot build script
# Builds Windows amd64, Linux amd64, Linux arm (armv7)
# Always builds frontend first
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

Write-Host "==> Building frontend (webui-src -> webui)" -ForegroundColor Cyan
Push-Location webui-src
npm run build
if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }
Pop-Location

Write-Host "==> Building Windows amd64" -ForegroundColor Cyan
$env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w" -o dist/windows-amd64/mahiru-dybot.exe .
if ($LASTEXITCODE -ne 0) { throw "windows build failed" }

Write-Host "==> Building Linux amd64" -ForegroundColor Cyan
$env:GOOS = "linux"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w" -o dist/linux-amd64/mahiru-dybot .
if ($LASTEXITCODE -ne 0) { throw "linux amd64 build failed" }

Write-Host "==> Building Linux arm (armv7)" -ForegroundColor Cyan
$env:GOOS = "linux"; $env:GOARCH = "arm"; $env:GOARM = "7"; $env:CGO_ENABLED = "0"
go build -trimpath -ldflags "-s -w" -o dist/linux-arm/mahiru-dybot .
if ($LASTEXITCODE -ne 0) { throw "linux arm build failed" }

Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM, Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "==> Build complete. Binaries in dist/:" -ForegroundColor Green
Get-ChildItem -Path dist -Recurse -File | ForEach-Object { Write-Host $_.FullName }
