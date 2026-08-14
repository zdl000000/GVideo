[CmdletBinding()]
param(
    [string]$BaseURL = "http://127.0.0.1:8088",
    [string]$SampleVideo = "",
    [ValidateRange(10, 1800)]
    [int]$ProcessingTimeoutSeconds = 180,
    [ValidateRange(1, 30)]
    [int]$PollIntervalSeconds = 2
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$script:StepNumber = 0

Add-Type -AssemblyName System.Net.Http

function Write-Step([string]$Message) {
    $script:StepNumber++
    Write-Host ("[{0:00}] {1}" -f $script:StepNumber, $Message) -ForegroundColor Cyan
}

function Assert-Condition([bool]$Condition, [string]$Message) {
    if (-not $Condition) {
        throw "Assertion failed: $Message"
    }
}

function New-ApiSession([string]$Name) {
    $handler = New-Object System.Net.Http.HttpClientHandler
    $handler.UseCookies = $true
    $handler.CookieContainer = New-Object System.Net.CookieContainer
    $handler.AutomaticDecompression = [System.Net.DecompressionMethods]::GZip -bor [System.Net.DecompressionMethods]::Deflate
    $client = New-Object System.Net.Http.HttpClient($handler)
    $client.Timeout = [TimeSpan]::FromSeconds(60)
    $client.DefaultRequestHeaders.Accept.ParseAdd("application/json")
    return [PSCustomObject]@{
        Name = $Name
        Handler = $handler
        Client = $client
        CSRFToken = ""
        User = $null
    }
}

function Get-AbsoluteURL([string]$Path) {
    if ($Path -match '^https?://') { return $Path }
    return $script:NormalizedBaseURL + "/" + $Path.TrimStart('/')
}

function Convert-HeadersToHashtable($Response) {
    $headers = @{}
    foreach ($header in $Response.Headers) {
        $headers[$header.Key] = ($header.Value -join ", ")
    }
    foreach ($header in $Response.Content.Headers) {
        $headers[$header.Key] = ($header.Value -join ", ")
    }
    return $headers
}

function Invoke-ApiRequest {
    param(
        [Parameter(Mandatory = $true)]$Session,
        [Parameter(Mandatory = $true)][string]$Method,
        [Parameter(Mandatory = $true)][string]$Path,
        $Body = $null,
        [System.Net.Http.HttpContent]$Content = $null,
        [switch]$UseCSRF,
        [int[]]$ExpectedStatus = @(200)
    )

    $request = New-Object System.Net.Http.HttpRequestMessage(
        (New-Object System.Net.Http.HttpMethod($Method)),
        (Get-AbsoluteURL $Path)
    )
    try {
        $request.Headers.TryAddWithoutValidation("X-Request-ID", ("acceptance-" + [Guid]::NewGuid().ToString("N"))) | Out-Null
        if ($UseCSRF) {
            if ([string]::IsNullOrWhiteSpace($Session.CSRFToken)) {
                throw "Session '$($Session.Name)' has no CSRF token."
            }
            $request.Headers.TryAddWithoutValidation("X-CSRF-Token", $Session.CSRFToken) | Out-Null
        }
        if ($null -ne $Content) {
            $request.Content = $Content
        } elseif ($null -ne $Body) {
            $json = $Body | ConvertTo-Json -Compress -Depth 12
            $request.Content = New-Object System.Net.Http.StringContent($json, [Text.Encoding]::UTF8, "application/json")
        }

        $response = $Session.Client.SendAsync($request).GetAwaiter().GetResult()
        try {
            $raw = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
            $status = [int]$response.StatusCode
            $payload = $null
            if (-not [string]::IsNullOrWhiteSpace($raw)) {
                try { $payload = $raw | ConvertFrom-Json } catch { $payload = $null }
            }
            $result = [PSCustomObject]@{
                StatusCode = $status
                Data = if ($null -ne $payload) { $payload.data } else { $null }
                Error = if ($null -ne $payload) { [string]$payload.error } else { "" }
                RequestID = if ($null -ne $payload) { [string]$payload.request_id } else { "" }
                Raw = $raw
                Headers = Convert-HeadersToHashtable $response
            }
            if ($ExpectedStatus -notcontains $status) {
                $detail = if ($result.Error) { $result.Error } elseif ($raw) { $raw } else { "empty response" }
                throw "$Method $Path returned HTTP $status (expected $($ExpectedStatus -join '/')): $detail; request_id=$($result.RequestID)"
            }
            return $result
        } finally {
            $response.Dispose()
        }
    } finally {
        $request.Dispose()
    }
}

function New-MultipartContent([hashtable]$Fields, [object[]]$Files) {
    $form = New-Object System.Net.Http.MultipartFormDataContent
    foreach ($name in $Fields.Keys) {
        $field = New-Object System.Net.Http.StringContent([string]$Fields[$name], [Text.Encoding]::UTF8)
        $form.Add($field, $name) | Out-Null
    }
    foreach ($file in $Files) {
        $stream = [IO.File]::OpenRead($file.Path)
        $part = New-Object System.Net.Http.StreamContent($stream)
        $part.Headers.ContentType = New-Object System.Net.Http.Headers.MediaTypeHeaderValue($file.ContentType)
        $form.Add($part, $file.Field, [IO.Path]::GetFileName($file.Path)) | Out-Null
    }
    return ,$form
}

function Invoke-MultipartRequest {
    param(
        [Parameter(Mandatory = $true)]$Session,
        [Parameter(Mandatory = $true)][string]$Method,
        [Parameter(Mandatory = $true)][string]$Path,
        [hashtable]$Fields = @{},
        [object[]]$Files = @(),
        [switch]$UseCSRF,
        [int[]]$ExpectedStatus = @(200)
    )
    $content = New-MultipartContent $Fields $Files
    try {
        return Invoke-ApiRequest -Session $Session -Method $Method -Path $Path -Content $content -UseCSRF:$UseCSRF -ExpectedStatus $ExpectedStatus
    } finally {
        $content.Dispose()
    }
}

function Invoke-RawRequest {
    param(
        [Parameter(Mandatory = $true)]$Session,
        [Parameter(Mandatory = $true)][string]$Path,
        [string]$Range = "",
        [int[]]$ExpectedStatus = @(200)
    )
    $request = New-Object System.Net.Http.HttpRequestMessage([System.Net.Http.HttpMethod]::Get, (Get-AbsoluteURL $Path))
    try {
        if ($Range) { $request.Headers.TryAddWithoutValidation("Range", $Range) | Out-Null }
        $response = $Session.Client.SendAsync($request).GetAwaiter().GetResult()
        try {
            $status = [int]$response.StatusCode
            $bytes = $response.Content.ReadAsByteArrayAsync().GetAwaiter().GetResult()
            if ($ExpectedStatus -notcontains $status) {
                throw "GET $Path returned HTTP $status (expected $($ExpectedStatus -join '/'))."
            }
            return [PSCustomObject]@{
                StatusCode = $status
                Bytes = $bytes
                Text = [Text.Encoding]::UTF8.GetString($bytes)
                Headers = Convert-HeadersToHashtable $response
            }
        } finally {
            $response.Dispose()
        }
    } finally {
        $request.Dispose()
    }
}

function Find-Video($Page, [long]$VideoID) {
    return @($Page.items) | Where-Object { [long]$_.id -eq $VideoID } | Select-Object -First 1
}

function Wait-ForVideoReady($Session, [long]$VideoID) {
    $deadline = [DateTime]::UtcNow.AddSeconds($ProcessingTimeoutSeconds)
    $lastState = ""
    while ([DateTime]::UtcNow -lt $deadline) {
        $page = (Invoke-ApiRequest -Session $Session -Method GET -Path "/api/v1/me/videos?page=1&page_size=100").Data
        $video = Find-Video $page $VideoID
        Assert-Condition ($null -ne $video) "uploaded video $VideoID is missing from the submission-management API"
        $state = "{0}/{1}%/{2}" -f $video.processing_status, $video.processing_progress, $video.processing_stage
        if ($state -ne $lastState) {
            Write-Host "     media processing: $state"
            $lastState = $state
        }
        if ($video.processing_status -eq "ready") { return $video }
        if ($video.processing_status -eq "failed") {
            throw "Video $VideoID processing failed: $($video.processing_error)"
        }
        Start-Sleep -Seconds $PollIntervalSeconds
    }
    throw "Video $VideoID did not become ready within $ProcessingTimeoutSeconds seconds (last state: $lastState)."
}

function Test-PageContainsVideo($Page, [long]$VideoID, [string]$Name) {
    Assert-Condition ($null -ne (Find-Video $Page $VideoID)) "$Name does not contain video $VideoID"
}

function Remove-TestVideo($Session, $AnonymousSession, [long]$VideoID) {
    $deadline = [DateTime]::UtcNow.AddSeconds($ProcessingTimeoutSeconds)
    while ([DateTime]::UtcNow -lt $deadline) {
        $currentResponse = Invoke-ApiRequest -Session $Session -Method GET -Path "/api/v1/videos/${VideoID}?count_view=false" -ExpectedStatus @(200, 404)
        if ($currentResponse.StatusCode -eq 404) { return }
        if ($currentResponse.Data.processing_status -eq "processing") {
            Start-Sleep -Seconds $PollIntervalSeconds
            continue
        }
        $deleteResponse = Invoke-ApiRequest -Session $Session -Method DELETE -Path "/api/v1/videos/$VideoID" -UseCSRF -ExpectedStatus @(200, 409)
        if ($deleteResponse.StatusCode -eq 200) {
            Invoke-ApiRequest -Session $AnonymousSession -Method GET -Path "/api/v1/videos/${VideoID}?count_view=false" -ExpectedStatus 404 | Out-Null
            return
        }
        Start-Sleep -Seconds $PollIntervalSeconds
    }
    throw "video remained non-deletable for $ProcessingTimeoutSeconds seconds"
}

function Invoke-BestEffort([string]$Description, [scriptblock]$Action) {
    try {
        & $Action
        Write-Host "     cleanup: $Description" -ForegroundColor DarkGray
    } catch {
        Write-Warning "Cleanup failed ($Description): $($_.Exception.Message)"
    }
}

function Initialize-DefaultSampleVideo([string]$Path) {
    if ((Test-Path -LiteralPath $Path -PathType Leaf) -and (Get-Item -LiteralPath $Path).Length -gt 0) {
        return
    }

    $ffmpeg = Get-Command ffmpeg -ErrorAction SilentlyContinue
    if (-not $ffmpeg) {
        throw "Default sample video is missing and ffmpeg was not found on PATH. Install FFmpeg or pass -SampleVideo."
    }

    $directory = Split-Path -Parent $Path
    New-Item -ItemType Directory -Path $directory -Force | Out-Null
    Write-Host "Generating a temporary acceptance video: $Path" -ForegroundColor DarkGray
    & $ffmpeg.Source -hide_banner -loglevel error -y `
        -f lavfi -i "color=c=black:s=320x180:r=24:d=1" `
        -c:v libx264 -preset ultrafast -pix_fmt yuv420p -movflags +faststart -an `
        $Path
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $Path -PathType Leaf) -or (Get-Item -LiteralPath $Path).Length -eq 0) {
        throw "Failed to generate the default sample video with ffmpeg: $Path"
    }
}

$root = Split-Path -Parent $PSScriptRoot
$usingDefaultSampleVideo = [string]::IsNullOrWhiteSpace($SampleVideo)
if ($usingDefaultSampleVideo) {
    $SampleVideo = Join-Path $root "tmp\sample.mp4"
}
$SampleVideo = [IO.Path]::GetFullPath($SampleVideo)
if ($usingDefaultSampleVideo) {
    Initialize-DefaultSampleVideo $SampleVideo
}
Assert-Condition (Test-Path -LiteralPath $SampleVideo -PathType Leaf) "sample video not found: $SampleVideo"
Assert-Condition ((Get-Item -LiteralPath $SampleVideo).Length -gt 0) "sample video is empty: $SampleVideo"

$baseUri = $null
if (-not [Uri]::TryCreate($BaseURL, [UriKind]::Absolute, [ref]$baseUri) -or $baseUri.Scheme -notin @("http", "https")) {
    throw "BaseURL must be an absolute HTTP or HTTPS URL."
}
if ($baseUri.Query -or $baseUri.Fragment) { throw "BaseURL must not contain a query string or fragment." }
$script:NormalizedBaseURL = $BaseURL.TrimEnd('/')

$runID = [Guid]::NewGuid().ToString("N").Substring(0, 10)
$authorName = "api_author_$runID"
$viewerName = "api_viewer_$runID"
$password = "Api9!" + [Guid]::NewGuid().ToString("N")
$tempDir = Join-Path ([IO.Path]::GetTempPath()) ("gvideo-api-acceptance-" + $runID)
$firstSubtitlePath = Join-Path $tempDir "captions-zh.srt"
$secondSubtitlePath = Join-Path $tempDir "captions-en.vtt"
$thirdSubtitlePath = Join-Path $tempDir "captions-ja.vtt"

$author = New-ApiSession "author"
$viewer = New-ApiSession "viewer"
$anonymous = New-ApiSession "anonymous"
$videoID = 0L
$commentID = 0L
$secondSubtitleID = 0L
$followActive = $false
$likeActive = $false
$favoriteActive = $false
$videoState = ""
$testSucceeded = $false

try {
    New-Item -ItemType Directory -Path $tempDir | Out-Null
    [IO.File]::WriteAllText($firstSubtitlePath, "1`r`n00:00:00,000 --> 00:00:02,000`r`nAPI acceptance subtitle`r`n", (New-Object Text.UTF8Encoding($false)))
    [IO.File]::WriteAllText($secondSubtitlePath, "WEBVTT`n`n00:00:00.000 --> 00:00:02.000`nEnglish acceptance subtitle`n", (New-Object Text.UTF8Encoding($false)))
    [IO.File]::WriteAllText($thirdSubtitlePath, "WEBVTT`n`n00:00:00.000 --> 00:00:02.000`nCSRF rejection check`n", (New-Object Text.UTF8Encoding($false)))

    Write-Step "Check service health and public categories at $script:NormalizedBaseURL"
    $health = Invoke-ApiRequest -Session $anonymous -Method GET -Path "/healthz"
    Assert-Condition ($health.Data.status -eq "ok") "health endpoint did not report ok"
    $categories = (Invoke-ApiRequest -Session $anonymous -Method GET -Path "/api/v1/categories").Data
    Assert-Condition (@($categories).Count -gt 0) "category list is empty"

    Write-Step "Verify authentication and CSRF boundaries"
    Invoke-ApiRequest -Session $anonymous -Method GET -Path "/api/v1/me/videos" -ExpectedStatus 401 | Out-Null

    Write-Step "Register author, verify cookie, log out, and log in again"
    $registered = Invoke-ApiRequest -Session $author -Method POST -Path "/api/v1/auth/register" -Body @{ username = $authorName; password = $password } -ExpectedStatus 201
    $author.User = $registered.Data.user
    $author.CSRFToken = [string]$registered.Data.csrf_token
    Assert-Condition ([long]$author.User.id -gt 0) "author registration returned no user id"
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($author.CSRFToken)) "author registration returned no CSRF token"
    Assert-Condition ($registered.Headers["Set-Cookie"] -match "HttpOnly") "session cookie is not HttpOnly"
    $me = (Invoke-ApiRequest -Session $author -Method GET -Path "/api/v1/auth/me").Data
    Assert-Condition ([long]$me.user.id -eq [long]$author.User.id) "authenticated author identity mismatch"
    Invoke-ApiRequest -Session $author -Method POST -Path "/api/v1/auth/logout" -UseCSRF | Out-Null
    Invoke-ApiRequest -Session $author -Method GET -Path "/api/v1/auth/me" -ExpectedStatus 401 | Out-Null
    $loggedIn = Invoke-ApiRequest -Session $author -Method POST -Path "/api/v1/auth/login" -Body @{ username = $authorName; password = $password }
    $author.User = $loggedIn.Data.user
    $author.CSRFToken = [string]$loggedIn.Data.csrf_token
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($author.CSRFToken)) "login returned no CSRF token"

    Write-Step "Register an isolated viewer account"
    $viewerRegistered = Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/auth/register" -Body @{ username = $viewerName; password = $password } -ExpectedStatus 201
    $viewer.User = $viewerRegistered.Data.user
    $viewer.CSRFToken = [string]$viewerRegistered.Data.csrf_token
    Assert-Condition ([long]$viewer.User.id -gt 0 -and [long]$viewer.User.id -ne [long]$author.User.id) "viewer identity is not isolated"

    Write-Step "Upload sample video with an initial SRT subtitle"
    $upload = Invoke-MultipartRequest -Session $author -Method POST -Path "/api/v1/videos" -UseCSRF -ExpectedStatus 201 -Fields @{
        title = "API acceptance $runID"
        description = "Created by scripts/acceptance-api.ps1"
        category = [string]@($categories)[0]
        visibility = "public"
        subtitle_language = "zh-CN"
        subtitle_label = "Chinese"
    } -Files @(
        [PSCustomObject]@{ Field = "video"; Path = $SampleVideo; ContentType = "video/mp4" },
        [PSCustomObject]@{ Field = "subtitle"; Path = $firstSubtitlePath; ContentType = "application/x-subrip" }
    )
    $videoID = [long]$upload.Data.id
    Assert-Condition ($videoID -gt 0) "upload returned no video id"
    Assert-Condition (@($upload.Data.subtitle_tracks).Count -eq 1) "upload did not create exactly one subtitle track"
    Assert-Condition ([bool]$upload.Data.subtitle_tracks[0].is_default) "initial subtitle is not the default"

    Write-Step "Poll processing through the submission-management API"
    $readyVideo = Wait-ForVideoReady $author $videoID
    $videoState = [string]$readyVideo.processing_status
    Assert-Condition ([int]$readyVideo.processing_progress -eq 100) "ready video progress is not 100"
    Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$readyVideo.video_url)) "ready video has no source URL"
    Assert-Condition (-not [string]::IsNullOrWhiteSpace([string]$readyVideo.hls_url)) "ready video has no HLS URL"

    Write-Step "Verify playback detail, source Range requests, HLS, and readable WebVTT"
    $detail = (Invoke-ApiRequest -Session $viewer -Method GET -Path "/api/v1/videos/${videoID}?count_view=false").Data
    Assert-Condition ([long]$detail.id -eq $videoID) "playback detail id mismatch"
    Assert-Condition (@($detail.subtitle_tracks).Count -eq 1) "playback detail does not expose the selectable subtitle"
    $range = Invoke-RawRequest -Session $anonymous -Path ([string]$detail.video_url) -Range "bytes=0-7" -ExpectedStatus 206
    Assert-Condition ($range.Bytes.Length -eq 8) "source Range response is not 8 bytes"
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($range.Headers["Content-Range"])) "source Range response has no Content-Range"
    $hls = Invoke-RawRequest -Session $anonymous -Path ([string]$detail.hls_url)
    Assert-Condition ($hls.Text -match "#EXTM3U") "HLS master playlist is invalid"
    $subtitle = Invoke-RawRequest -Session $anonymous -Path ([string]$detail.subtitle_tracks[0].url)
    Assert-Condition ($subtitle.Text -match "WEBVTT") "subtitle was not converted to WebVTT"

    Write-Step "Verify subtitle maintenance is owner-only and driven from my submissions"
    $managedPage = (Invoke-ApiRequest -Session $author -Method GET -Path "/api/v1/me/videos?page=1&page_size=100").Data
    $managedVideo = Find-Video $managedPage $videoID
    Assert-Condition ($null -ne $managedVideo) "video is absent from my submissions"
    Invoke-MultipartRequest -Session $author -Method POST -Path "/api/v1/videos/$videoID/subtitles" -ExpectedStatus 403 -Fields @{
        subtitle_language = "ja"
        subtitle_label = "Japanese"
    } -Files @([PSCustomObject]@{ Field = "subtitle"; Path = $thirdSubtitlePath; ContentType = "text/vtt" }) | Out-Null
    Invoke-MultipartRequest -Session $viewer -Method POST -Path "/api/v1/videos/$videoID/subtitles" -UseCSRF -ExpectedStatus 403 -Fields @{
        subtitle_language = "ja"
        subtitle_label = "Japanese"
    } -Files @([PSCustomObject]@{ Field = "subtitle"; Path = $thirdSubtitlePath; ContentType = "text/vtt" }) | Out-Null
    $secondTrack = (Invoke-MultipartRequest -Session $author -Method POST -Path "/api/v1/videos/$videoID/subtitles" -UseCSRF -ExpectedStatus 201 -Fields @{
        subtitle_language = "en"
        subtitle_label = "English"
    } -Files @([PSCustomObject]@{ Field = "subtitle"; Path = $secondSubtitlePath; ContentType = "text/vtt" })).Data
    $secondSubtitleID = [long]$secondTrack.id
    Assert-Condition ($secondSubtitleID -gt 0) "second subtitle upload returned no id"
    $afterUpload = (Invoke-ApiRequest -Session $author -Method GET -Path "/api/v1/videos/${videoID}?count_view=false").Data
    Assert-Condition (@($afterUpload.subtitle_tracks).Count -eq 2) "subtitle upload was not visible when the owner re-read the video"
    $tracks = (Invoke-ApiRequest -Session $author -Method PATCH -Path "/api/v1/videos/$videoID/subtitles/$secondSubtitleID/default" -UseCSRF).Data
    $defaultTrack = @($tracks) | Where-Object { $_.is_default } | Select-Object -First 1
    Assert-Condition ([long]$defaultTrack.id -eq $secondSubtitleID) "second subtitle was not made the sole default"
    $tracks = (Invoke-ApiRequest -Session $author -Method DELETE -Path "/api/v1/videos/$videoID/subtitles/$secondSubtitleID" -UseCSRF).Data
    $secondSubtitleID = 0L
    Assert-Condition (@($tracks).Count -eq 1 -and [bool]$tracks[0].is_default) "deleting the default subtitle did not restore the remaining default"
    $afterDelete = (Invoke-ApiRequest -Session $author -Method GET -Path "/api/v1/videos/${videoID}?count_view=false").Data
    Assert-Condition (@($afterDelete.subtitle_tracks).Count -eq 1 -and [bool]$afterDelete.subtitle_tracks[0].is_default) "subtitle deletion was not persisted"

    Write-Step "Follow the author and verify profile, author videos, and following feed"
    $follow = (Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/users/$($author.User.id)/follow" -UseCSRF).Data
    $followActive = [bool]$follow.active
    Assert-Condition $followActive "follow toggle did not activate"
    $profile = (Invoke-ApiRequest -Session $viewer -Method GET -Path "/api/v1/users/$($author.User.id)").Data
    Assert-Condition ([bool]$profile.followed) "creator profile does not reflect the follow"
    $creatorVideos = (Invoke-ApiRequest -Session $viewer -Method GET -Path "/api/v1/users/$($author.User.id)/videos?page=1&page_size=100").Data
    Test-PageContainsVideo $creatorVideos $videoID "creator video list"
    $following = (Invoke-ApiRequest -Session $viewer -Method GET -Path "/api/v1/me/following/videos?page=1&page_size=100").Data
    Test-PageContainsVideo $following $videoID "following feed"

    Write-Step "Exercise CSRF rejection, like, favorite, comment, and viewer state"
    Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/videos/$videoID/like" -ExpectedStatus 403 | Out-Null
    $likeActive = [bool](Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/videos/$videoID/like" -UseCSRF).Data.active
    Assert-Condition $likeActive "like toggle did not activate"
    $favoriteActive = [bool](Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/videos/$videoID/favorite" -UseCSRF).Data.active
    Assert-Condition $favoriteActive "favorite toggle did not activate"
    $comment = (Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/videos/$videoID/comments" -UseCSRF -ExpectedStatus 201 -Body @{ content = "Acceptance comment $runID" }).Data
    $commentID = [long]$comment.id
    Assert-Condition ($commentID -gt 0) "comment creation returned no id"
    $comments = (Invoke-ApiRequest -Session $viewer -Method GET -Path "/api/v1/videos/$videoID/comments").Data
    Assert-Condition ($null -ne (@($comments) | Where-Object { [long]$_.id -eq $commentID } | Select-Object -First 1)) "created comment is absent from the comment list"
    $viewerDetail = (Invoke-ApiRequest -Session $viewer -Method GET -Path "/api/v1/videos/${videoID}?count_view=false").Data
    Assert-Condition ([bool]$viewerDetail.liked -and [bool]$viewerDetail.favorited) "playback detail does not reflect viewer interactions"
    Assert-Condition ([long]$viewerDetail.comments_count -ge 1) "playback detail comment count was not updated"
    $favorites = (Invoke-ApiRequest -Session $viewer -Method GET -Path "/api/v1/me/favorites?page=1&page_size=100").Data
    Test-PageContainsVideo $favorites $videoID "favorites feed"

    Write-Step "Verify creator statistics and interaction notifications"
    $stats = (Invoke-ApiRequest -Session $author -Method GET -Path "/api/v1/me/creator/stats").Data
    Assert-Condition ([long]$stats.videos_count -ge 1) "creator stats did not count the uploaded video"
    Assert-Condition ([long]$stats.followers_count -ge 1) "creator stats did not count the follower"
    $notifications = (Invoke-ApiRequest -Session $author -Method GET -Path "/api/v1/me/notifications?page=1&page_size=100").Data
    $runNotifications = @($notifications.items) | Where-Object {
        [long]$_.actor_id -eq [long]$viewer.User.id -and (($null -eq $_.video_id) -or [long]$_.video_id -eq $videoID)
    }
    Assert-Condition ($runNotifications.Count -gt 0) "author received no notification from the test viewer"
    $notificationID = [long]$runNotifications[0].id
    Invoke-ApiRequest -Session $author -Method PATCH -Path "/api/v1/me/notifications/$notificationID/read" -UseCSRF | Out-Null
    Invoke-ApiRequest -Session $author -Method POST -Path "/api/v1/me/notifications/read-all" -UseCSRF | Out-Null

    $testSucceeded = $true
    Write-Host "`nAPI acceptance passed for video $videoID." -ForegroundColor Green
} catch {
    Write-Host "`nAPI acceptance failed: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ScriptStackTrace) { Write-Host $_.ScriptStackTrace -ForegroundColor DarkRed }
} finally {
    Write-Host "`nCleaning up API-created data..."
    if ($commentID -gt 0 -and $videoID -gt 0) {
        Invoke-BestEffort "delete test comment" {
            Invoke-ApiRequest -Session $viewer -Method DELETE -Path "/api/v1/videos/$videoID/comments/$commentID" -UseCSRF | Out-Null
            $script:commentID = 0L
        }
    }
    if ($favoriteActive -and $videoID -gt 0) {
        Invoke-BestEffort "remove test favorite" {
            $active = [bool](Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/videos/$videoID/favorite" -UseCSRF).Data.active
            if ($active) { throw "favorite remained active" }
            $script:favoriteActive = $false
        }
    }
    if ($likeActive -and $videoID -gt 0) {
        Invoke-BestEffort "remove test like" {
            $active = [bool](Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/videos/$videoID/like" -UseCSRF).Data.active
            if ($active) { throw "like remained active" }
            $script:likeActive = $false
        }
    }
    if ($followActive) {
        Invoke-BestEffort "unfollow test author" {
            $active = [bool](Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/users/$($author.User.id)/follow" -UseCSRF).Data.active
            if ($active) { throw "follow remained active" }
            $script:followActive = $false
        }
    }
    if ($secondSubtitleID -gt 0 -and $videoID -gt 0) {
        Invoke-BestEffort "delete second test subtitle" {
            Invoke-ApiRequest -Session $author -Method DELETE -Path "/api/v1/videos/$videoID/subtitles/$secondSubtitleID" -UseCSRF | Out-Null
            $script:secondSubtitleID = 0L
        }
    }
    if ($videoID -gt 0) {
        Invoke-BestEffort "delete test video and its remaining subtitle/media" {
            Remove-TestVideo $author $anonymous $videoID
            $script:videoID = 0L
        }
    }
    if ($viewer.CSRFToken) {
        Invoke-BestEffort "log out test viewer" { Invoke-ApiRequest -Session $viewer -Method POST -Path "/api/v1/auth/logout" -UseCSRF | Out-Null }
    }
    if ($author.CSRFToken) {
        Invoke-BestEffort "log out test author" { Invoke-ApiRequest -Session $author -Method POST -Path "/api/v1/auth/logout" -UseCSRF | Out-Null }
    }

    foreach ($session in @($author, $viewer, $anonymous)) {
        if ($null -ne $session.Client) { $session.Client.Dispose() }
        if ($null -ne $session.Handler) { $session.Handler.Dispose() }
    }
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force
    }
}

if (-not $testSucceeded) { exit 1 }
Write-Host "Random users retained because the API has no safe user-delete endpoint: $authorName, $viewerName" -ForegroundColor DarkGray
exit 0
