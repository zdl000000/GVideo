$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go) { $go = "C:\Program Files\Go\bin\go.exe" }

Push-Location (Join-Path $root "backend")
try {
    & $go test ./...
    & $go vet ./...
} finally { Pop-Location }

Push-Location (Join-Path $root "frontend")
try { npm run build } finally { Pop-Location }
