$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

function Invoke-Checked([string[]]$Arguments, [string]$Message) {
    & $Arguments[0] $Arguments[1..($Arguments.Count - 1)]
    if ($LASTEXITCODE -ne 0) { throw $Message }
}

function Wait-ForBackend([int]$TimeoutSeconds = 60) {
    $healthUrl = "http://127.0.0.1:8080/healthz"
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    do {
        try {
            $response = Invoke-WebRequest -UseBasicParsing -Uri $healthUrl -TimeoutSec 3
            if ($response.StatusCode -eq 200) { return }
            Start-Sleep -Seconds 2
        } catch {
            Start-Sleep -Seconds 2
        }
    } while ((Get-Date) -lt $deadline)

    & docker compose -f (Join-Path $root "compose.yaml") ps backend
    & docker compose -f (Join-Path $root "compose.yaml") logs --tail 80 backend
    throw "Backend did not become healthy within $TimeoutSeconds seconds."
}

Push-Location $root
try {
    Invoke-Checked @("docker", "info") "Docker Desktop is not running or the active Docker context is unavailable."
    if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
        throw "npm was not found on PATH. Install Node.js before starting the frontend."
    }
    if (-not (Test-Path (Join-Path $root "frontend\node_modules"))) {
        throw "Frontend dependencies are missing. Run 'npm ci' in frontend first."
    }
    Invoke-Checked @("docker", "compose", "-f", (Join-Path $root "compose.yaml"), "up", "-d", "backend") "Failed to start the persistent backend."
    Wait-ForBackend
} finally { Pop-Location }

Write-Host "GVideo frontend starting at http://127.0.0.1:5173"
Write-Host "API uses the same persistent Docker database as http://127.0.0.1:8088"
Write-Host "Press Ctrl+C to stop Vite. The backend remains available for later sessions."
try {
    Push-Location (Join-Path $root "frontend")
    npm run dev -- --host 127.0.0.1 --port 5173 --strictPort
} finally {
    Pop-Location
}
