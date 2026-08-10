$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot

Push-Location $root
try {
    docker compose exec -T backend gvideo data-status
} finally { Pop-Location }
