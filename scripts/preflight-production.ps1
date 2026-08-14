param(
    [ValidateRange(1, 365)]
    [int]$MinimumCertificateDays = 14,
    [switch]$SkipGatewayBuild
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$compose = Join-Path $root "compose.yaml"
$httpsCompose = Join-Path $root "compose.https.yaml"

function Assert-LastExitCode([string]$Message) {
    if ($LASTEXITCODE -ne 0) {
        throw $Message
    }
}

function Get-RequiredEnvironmentValue([string]$Name) {
    $value = [System.Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        throw "$Name must be set for production deployment."
    }
    return $value.Trim()
}

function Resolve-ProjectPath([string]$Path) {
    if ([System.IO.Path]::IsPathRooted($Path)) {
        return [System.IO.Path]::GetFullPath($Path)
    }
    return [System.IO.Path]::GetFullPath((Join-Path $root $Path))
}

function Get-Port([string]$Name, [int]$Default) {
    $value = [System.Environment]::GetEnvironmentVariable($Name)
    if ([string]::IsNullOrWhiteSpace($value)) {
        return $Default
    }
    [int]$port = 0
    if (-not [int]::TryParse($value, [ref]$port) -or $port -lt 1 -or $port -gt 65535) {
        throw "$Name must be an integer between 1 and 65535."
    }
    return $port
}

function Test-CertificateHost([System.Security.Cryptography.X509Certificates.X509Certificate2]$Certificate, [string]$HostName) {
    $candidateNames = New-Object System.Collections.Generic.List[string]
    foreach ($extension in $Certificate.Extensions) {
        if ($extension.Oid.Value -ne "2.5.29.17") { continue }
        foreach ($line in ($extension.Format($true) -split "`r?`n")) {
            if ($line.Trim() -match '^DNS Name=(.+)$') {
                $candidateNames.Add($Matches[1].Trim().TrimEnd("."))
            }
        }
    }
    if ($candidateNames.Count -eq 0) {
        $commonName = $Certificate.GetNameInfo(
            [System.Security.Cryptography.X509Certificates.X509NameType]::DnsName,
            $false
        )
        if (-not [string]::IsNullOrWhiteSpace($commonName)) {
            $candidateNames.Add($commonName.Trim().TrimEnd("."))
        }
    }

    foreach ($candidate in $candidateNames) {
        if ($candidate.Equals($HostName, [System.StringComparison]::OrdinalIgnoreCase)) {
            return $true
        }
        if ($candidate.StartsWith("*.") -and $HostName.EndsWith($candidate.Substring(1), [System.StringComparison]::OrdinalIgnoreCase)) {
            $hostLabels = $HostName.Split(".").Count
            $candidateLabels = $candidate.Split(".").Count
            if ($hostLabels -eq $candidateLabels) { return $true }
        }
    }
    return $false
}

$frontendURL = Get-RequiredEnvironmentValue "FRONTEND_URL"
$gvideoHost = (Get-RequiredEnvironmentValue "GVIDEO_HOST").TrimEnd(".").ToLowerInvariant()
$adminUsername = Get-RequiredEnvironmentValue "ADMIN_USERNAME"
$certificatePath = Resolve-ProjectPath (Get-RequiredEnvironmentValue "TLS_CERT_FILE")
$privateKeyPath = Resolve-ProjectPath (Get-RequiredEnvironmentValue "TLS_KEY_FILE")
$dataVolume = Get-RequiredEnvironmentValue "GVIDEO_DATA_VOLUME"
$mediaVolume = Get-RequiredEnvironmentValue "GVIDEO_MEDIA_VOLUME"
$httpsPort = Get-Port "HTTPS_PORT" 443
$httpPort = Get-Port "HTTPS_HTTP_PORT" 80

if ($adminUsername -eq "admin" -or $adminUsername -eq "administrator") {
    Write-Warning "ADMIN_USERNAME uses a predictable administrative account name."
}
if ([System.Uri]::CheckHostName($gvideoHost) -ne [System.UriHostNameType]::Dns) {
    throw "GVIDEO_HOST must be a DNS hostname without a scheme, path, or port."
}

$parsedFrontendURL = $null
if (-not [System.Uri]::TryCreate($frontendURL, [System.UriKind]::Absolute, [ref]$parsedFrontendURL)) {
    throw "FRONTEND_URL must be an absolute HTTPS origin."
}
if ($parsedFrontendURL.Scheme -ne "https" -or $parsedFrontendURL.UserInfo -or $parsedFrontendURL.AbsolutePath -ne "/" -or $parsedFrontendURL.Query -or $parsedFrontendURL.Fragment) {
    throw "FRONTEND_URL must be an HTTPS origin without credentials, path, query, or fragment."
}
if (-not $parsedFrontendURL.DnsSafeHost.Equals($gvideoHost, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "FRONTEND_URL host must match GVIDEO_HOST."
}
if ($parsedFrontendURL.Port -ne $httpsPort) {
    throw "FRONTEND_URL port $($parsedFrontendURL.Port) does not match HTTPS_PORT $httpsPort."
}
if ($httpPort -eq $httpsPort) {
    throw "HTTPS_HTTP_PORT and HTTPS_PORT must be different."
}

foreach ($volume in @($dataVolume, $mediaVolume)) {
    if ($volume -notmatch '^[A-Za-z0-9][A-Za-z0-9_.-]+$') {
        throw "Docker volume name contains unsupported characters: $volume"
    }
}
if ($dataVolume.Equals($mediaVolume, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "GVIDEO_DATA_VOLUME and GVIDEO_MEDIA_VOLUME must be different."
}

foreach ($file in @($certificatePath, $privateKeyPath)) {
    if (-not (Test-Path -LiteralPath $file -PathType Leaf)) {
        throw "Production TLS file was not found: $file"
    }
}
if ($certificatePath.Equals($privateKeyPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "TLS_CERT_FILE and TLS_KEY_FILE must point to different files."
}

try {
    $certificate = New-Object System.Security.Cryptography.X509Certificates.X509Certificate2($certificatePath)
} catch {
    throw "TLS_CERT_FILE does not contain a readable X.509 certificate: $certificatePath"
}
$minimumExpiry = (Get-Date).ToUniversalTime().AddDays($MinimumCertificateDays)
if ($certificate.NotBefore.ToUniversalTime() -gt (Get-Date).ToUniversalTime()) {
    throw "TLS certificate is not valid yet: $($certificate.NotBefore.ToUniversalTime().ToString('o'))"
}
if ($certificate.NotAfter.ToUniversalTime() -lt $minimumExpiry) {
    throw "TLS certificate expires before the required $MinimumCertificateDays day window: $($certificate.NotAfter.ToUniversalTime().ToString('o'))"
}
if (-not (Test-CertificateHost $certificate $gvideoHost)) {
    throw "TLS certificate does not cover GVIDEO_HOST $gvideoHost."
}

if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Docker CLI was not found on PATH."
}
docker info --format "{{.ServerVersion}}" | Out-Null
Assert-LastExitCode "Docker daemon is not available. Start Docker Desktop and retry."

Push-Location $root
try {
    docker compose -f $compose -f $httpsCompose config --quiet
    Assert-LastExitCode "Production Compose configuration is invalid."

    if (-not $SkipGatewayBuild) {
        docker compose -f $compose -f $httpsCompose build gateway
        Assert-LastExitCode "Building the HTTPS gateway image failed."
    }

    $templatePath = Join-Path $root "deploy\nginx\https.conf.template"
    $templateMount = "type=bind,source=$templatePath,target=/etc/nginx/templates/default.conf.template,readonly"
    $certificateMount = "type=bind,source=$certificatePath,target=/etc/nginx/tls/fullchain.pem,readonly"
    $privateKeyMount = "type=bind,source=$privateKeyPath,target=/etc/nginx/tls/privkey.pem,readonly"
    docker run --rm --pull never `
        -e "HTTPS_PORT=$httpsPort" `
        -e "GVIDEO_HOST=$gvideoHost" `
        --add-host "frontend:127.0.0.1" `
        --mount $templateMount `
        --mount $certificateMount `
        --mount $privateKeyMount `
        gvideo-gateway nginx -t
    Assert-LastExitCode "Nginx rejected the rendered configuration or the TLS certificate/private key pair."

    $existingVolumes = @(docker volume ls --format "{{.Name}}")
    foreach ($volume in @($dataVolume, $mediaVolume)) {
        if ($existingVolumes -contains $volume) {
            Write-Host "Docker volume exists: $volume"
        } else {
            Write-Warning "Docker volume does not exist yet and will be created on first deployment: $volume"
        }
    }
} finally {
    Pop-Location
    if ($certificate) { $certificate.Dispose() }
}

Write-Host "Production preflight passed for https://$gvideoHost$(if ($httpsPort -ne 443) { ":$httpsPort" })."
