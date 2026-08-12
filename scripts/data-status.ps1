$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "compose.yaml"

Push-Location $root
try {
    docker compose -f $compose exec -T backend gvideo data-status
    if ($LASTEXITCODE -ne 0) {
        throw "Reading database statistics failed."
    }
} finally { Pop-Location }
