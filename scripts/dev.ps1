$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

Push-Location $root
try {
    docker info *> $null
    if ($LASTEXITCODE -ne 0) { throw "Docker Desktop is not running." }
    docker compose up -d backend
    if ($LASTEXITCODE -ne 0) { throw "Failed to start the persistent backend." }
} finally { Pop-Location }

Write-Host "GVideo frontend starting at http://127.0.0.1:5173"
Write-Host "API uses the same persistent Docker database as http://127.0.0.1:8088"
Write-Host "Press Ctrl+C to stop Vite. The backend remains available for later sessions."
try {
    Push-Location (Join-Path $root "frontend")
    npm run dev
} finally {
    Pop-Location
}
