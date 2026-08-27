# Mahiru DyBot build script
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# ---- Build frontend ----
Write-Host "==> Building frontend" -ForegroundColor Cyan
Push-Location webui-src
npm run build
if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }
Pop-Location

# ---- Locate frontend output ----
$frontendOut = $null
if (Test-Path "webui-src/build") { $frontendOut = "webui-src/build" }
elseif (Test-Path "webui-src/dist") { $frontendOut = "webui-src/dist" }
if (-not $frontendOut) {
    Write-Host "Warning: Frontend build output not found." -ForegroundColor Yellow
}

# ---- Copy to root webui/ for embedding ----
if ($frontendOut) {
    if (Test-Path "webui") { Remove-Item -Recurse -Force "webui" }
    Copy-Item -Recurse $frontendOut "webui"
}

# ---- Platforms ----
$platforms = @(
    @{ GOOS="windows"; GOARCH="amd64"; GOARM=""; OUT="dist/windows-amd64"; BIN="mahiru-dybot.exe" },
    @{ GOOS="linux";   GOARCH="amd64"; GOARM=""; OUT="dist/linux-amd64";   BIN="mahiru-dybot"    },
    @{ GOOS="linux";   GOARCH="arm";   GOARM="7"; OUT="dist/linux-arm";     BIN="mahiru-dybot"    }
)

# ---- Build each platform ----
foreach ($p in $platforms) {
    Write-Host "==> Building $($p.GOOS) $($p.GOARCH)" -ForegroundColor Cyan
    $env:GOOS = $p.GOOS; $env:GOARCH = $p.GOARCH; $env:CGO_ENABLED = "0"
    if ($p.GOARM) { $env:GOARM = $p.GOARM } else { Remove-Item Env:GOARM -ErrorAction SilentlyContinue }
    
    $outDir = $p.OUT
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
    $binPath = Join-Path $outDir $p.BIN
    go build -trimpath -ldflags "-s -w" -o $binPath .
    if ($LASTEXITCODE -ne 0) { throw "$($p.GOOS) $($p.GOARCH) build failed" }

    # ---- Copy frontend to each platform's folder ----
    if ($frontendOut) {
        $destWebui = Join-Path $outDir "webui"
        if (Test-Path $destWebui) { Remove-Item -Recurse -Force $destWebui }
        Copy-Item -Recurse $frontendOut $destWebui
        Write-Host "   -> Copied webui to $destWebui" -ForegroundColor DarkGray
    }
}

Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM, Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "==> Done. Each platform folder contains binary + webui/" -ForegroundColor Green
