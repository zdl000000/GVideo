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

function Test-PowerShellScripts {
    $errors = @()
    Get-ChildItem (Join-Path $root "scripts") -Filter "*.ps1" -File | ForEach-Object {
        $parseErrors = $null
        [void][System.Management.Automation.Language.Parser]::ParseFile(
            $_.FullName,
            [ref]$null,
            [ref]$parseErrors
        )
        if ($parseErrors.Count -gt 0) {
            $errors += "$($_.Name): $($parseErrors[0].Message)"
        }
    }
    if ($errors.Count -gt 0) {
        throw "PowerShell syntax check failed:`n$($errors -join [Environment]::NewLine)"
    }
}

function Test-ComposeConfig([string[]]$Arguments, [string]$Message) {
    & docker compose @Arguments config --quiet
    if ($LASTEXITCODE -ne 0) { throw $Message }
}

function Resolve-ProjectPath([string]$Path) {
    if ([IO.Path]::IsPathRooted($Path)) { return [IO.Path]::GetFullPath($Path) }
    return [IO.Path]::GetFullPath((Join-Path $root $Path))
}

function Test-TlsFile([string]$Name, [string]$Path) {
    $resolved = Resolve-ProjectPath $Path
    if (-not (Test-Path -LiteralPath $resolved -PathType Leaf)) {
        throw "$Name does not point to a readable file: $resolved"
    }
}

Test-PowerShellScripts
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker CLI was not found on PATH."
}
Test-ComposeConfig @("-f", (Join-Path $root "compose.yaml")) "Compose HTTP configuration is invalid."

$tlsCert = $env:TLS_CERT_FILE
$tlsKey = $env:TLS_KEY_FILE
if ([string]::IsNullOrWhiteSpace($tlsCert) -or [string]::IsNullOrWhiteSpace($tlsKey)) {
    Write-Host "Skipping HTTPS Compose config: TLS_CERT_FILE and TLS_KEY_FILE are not set."
} else {
    Test-TlsFile "TLS_CERT_FILE" $tlsCert
    Test-TlsFile "TLS_KEY_FILE" $tlsKey
    if ((Resolve-ProjectPath $tlsCert) -eq (Resolve-ProjectPath $tlsKey)) {
        throw "TLS_CERT_FILE and TLS_KEY_FILE must point to different files."
    }
    Test-ComposeConfig @(
        "-f", (Join-Path $root "compose.yaml"),
        "-f", (Join-Path $root "compose.https.yaml")
    ) "Compose HTTPS configuration is invalid."
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
    npm test
    Assert-LastExitCode "Frontend tests failed."
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
