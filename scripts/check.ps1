$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go) { $go = "C:\Program Files\Go\bin\go.exe" }
$env:GOCACHE = Join-Path $root "tmp\go-build"

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

Push-Location (Join-Path $root "backend")
try {
    $goroot = (& $go env GOROOT).Trim()
    Assert-LastExitCode "Resolving Go root failed."
    $gofmt = Join-Path $goroot "bin\gofmt.exe"
    $unformatted = @(& $gofmt -l .)
    Assert-LastExitCode "Go formatting check failed."
    if ($unformatted.Count -gt 0) {
        throw "Go files are not formatted:`n$($unformatted -join [Environment]::NewLine)"
    }
    & $go test ./... -count=1
    Assert-LastExitCode "Go tests failed."
    & $go vet ./...
    Assert-LastExitCode "Go vet failed."
} finally { Pop-Location }

Push-Location (Join-Path $root "frontend")
try {
    npm run typecheck
    Assert-LastExitCode "Frontend typecheck failed."
    npm run build
    Assert-LastExitCode "Frontend build failed."
} finally { Pop-Location }

Push-Location $root
try {
    git diff --check
    Assert-LastExitCode "Git whitespace check failed."
} finally { Pop-Location }
