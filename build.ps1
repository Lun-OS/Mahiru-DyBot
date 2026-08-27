# Mahiru DyBot build script
$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# ---- Build frontend ----
Write-Host "==> Building frontend" -ForegroundColor Cyan
Push-Location webui-src
npm run build
if ($LASTEXITCODE -ne 0) { throw "Frontend build failed" }
Pop-Location

# ---- 前端产物已经输出到项目根目录的 webui/ ----
$frontendSource = Join-Path $PSScriptRoot "webui"
if (-not (Test-Path $frontendSource)) {
    Write-Host "Warning: webui/ not found after build, skipping copy" -ForegroundColor Yellow
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

    # ---- Copy webui/ from root to this platform folder ----
    if (Test-Path $frontendSource) {
        $destWebui = Join-Path $outDir "webui"
        if (Test-Path $destWebui) { Remove-Item -Recurse -Force $destWebui }
        Copy-Item -Recurse $frontendSource $destWebui
        Write-Host "   -> Copied webui to $destWebui" -ForegroundColor DarkGray
    }
}

Remove-Item Env:GOOS, Env:GOARCH, Env:GOARM, Env:CGO_ENABLED -ErrorAction SilentlyContinue

Write-Host "==> Done. Each platform folder contains binary + webui/" -ForegroundColor Green
