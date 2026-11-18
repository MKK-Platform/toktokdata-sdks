# toktokdata.live API quickstart for Windows PowerShell 5.1 and PowerShell 7: an official
# board, a creator profile (waiting through "202 still collecting") and the entitlements.
# Usage: .\quickstart.ps1 [-Region VN] [-Creator tiktok]
param(
    [string]$Region = 'VN',
    [string]$Creator = 'tiktok'
)
$ErrorActionPreference = 'Stop'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

if (-not $env:TOKTOKDATA_API_KEY) {
    throw 'Set TOKTOKDATA_API_KEY (dashboard Settings -> API keys at https://toktokdata.live).'
}
$Base = if ($env:TOKTOKDATA_BASE_URL) { $env:TOKTOKDATA_BASE_URL } else { 'https://api.toktokdata.live' }
$Headers = @{
    Authorization = "Bearer $($env:TOKTOKDATA_API_KEY)"
    Accept        = 'application/json'
}

function Get-RetryAfterSeconds($Response) {
    # PowerShell 7 exposes HttpResponseMessage, 5.1 a WebResponse.
    try { if ($Response.Headers.RetryAfter.Delta) { return [int]$Response.Headers.RetryAfter.Delta.TotalSeconds } } catch { }
    try { $v = $Response.Headers['Retry-After']; if ($v) { return [int]"$v" } } catch { }
    return $null
}

# Returns @{ Status; Body; RetryAfter } for 2xx. Retries 429 (honouring Retry-After) up to
# three times, prints the error envelope and returns $null for other errors.
function Invoke-TokTokData([string]$Path) {
    for ($attempt = 1; ; $attempt++) {
        try {
            $r = Invoke-WebRequest -Uri "$Base$Path" -Headers $Headers -UserAgent 'toktokdata-powershell-example' -UseBasicParsing
            # Decode as UTF-8 ourselves: 5.1 guesses ISO-8859-1 and breaks non-Latin names.
            $text = [Text.Encoding]::UTF8.GetString($r.RawContentStream.ToArray())
            return [pscustomobject]@{
                Status     = [int]$r.StatusCode
                Body       = $text | ConvertFrom-Json
                RetryAfter = "$($r.Headers['Retry-After'])"
            }
        } catch {
            $response = $_.Exception.Response
            if (-not $response) { throw }
            $status = [int]$response.StatusCode
            # PowerShell 7 puts the error body in ErrorDetails; 5.1 may leave it in the response stream.
            $raw = $_.ErrorDetails.Message
            if (-not $raw -and $response -is [Net.WebResponse]) {
                try {
                    $stream = $response.GetResponseStream()
                    if ($stream.CanSeek) { $stream.Position = 0 }
                    $raw = (New-Object IO.StreamReader($stream, [Text.Encoding]::UTF8)).ReadToEnd()
                } catch { }
            }
            $envelope = $null
            if ($raw) { try { $envelope = $raw | ConvertFrom-Json } catch { } }
            $retryAfter = Get-RetryAfterSeconds $response
            if ($status -eq 429 -and $attempt -lt 4 -and $envelope.code -ne 'daily_quota_exceeded') {
                $wait = if ($retryAfter) { [Math]::Min($retryAfter, 60) } else { $attempt }
                Write-Host "  rate limited, retrying in ${wait}s"
                Start-Sleep -Seconds $wait
                continue
            }
            if ($envelope) {
                $line = "  HTTP ${status}: $($envelope.code): $($envelope.message) (requestId $($envelope.requestId))"
                if ($status -eq 402 -and $envelope.resetAt) { $line += ", credits reset at $($envelope.resetAt)" }
                Write-Warning $line
            } else {
                Write-Warning "  HTTP ${status}: $($_.Exception.Message)"
            }
            return $null
        }
    }
}

Write-Host "== Official hourly board, $Region (scope rank:read)"
$res = Invoke-TokTokData "/v1/rankings/official?board=hourly&region=$Region&limit=10"
if ($res) {
    $board = $res.Body.data.board
    Write-Host "publishable: $($board.publishable), period $($board.periodKey)"
    foreach ($e in $board.entries) {
        $who = if ($e.masked) { "(masked, plan $($e.unmaskTier))" } else { "@$($e.uniqueId)" }
        $live = if ($e.isLive) { 'LIVE' } else { '' }
        Write-Host ("{0,3}  {1,-32} {2,12:N0}  {3}" -f $e.rank, $who, $e.score, $live)
    }
}

Write-Host ''
Write-Host "== Creator @$Creator (scope creator:read)"
$deadline = (Get-Date).AddSeconds(180)
$res = Invoke-TokTokData "/v1/creators/$([Uri]::EscapeDataString($Creator))"
while ($res -and $res.Status -eq 202) {
    # Still collecting: wait retryAfterSeconds (or Retry-After), clamped to 1..120 s.
    $wait = if ($res.Body.retryAfterSeconds) { [int]$res.Body.retryAfterSeconds } elseif ($res.RetryAfter) { [int]$res.RetryAfter } else { 5 }
    $wait = [Math]::Max(1, [Math]::Min($wait, 120))
    if ((Get-Date).AddSeconds($wait) -gt $deadline) { Write-Warning '  still collecting after 180 s, try again later'; $res = $null; break }
    Write-Host "  202 $($res.Body.status), asking again in ${wait}s"
    Start-Sleep -Seconds $wait
    $res = Invoke-TokTokData "/v1/creators/$([Uri]::EscapeDataString($Creator))"
}
if ($res -and $res.Status -eq 200) {
    $c = $res.Body.data.creator
    Write-Host "$($c.displayName) (@$($c.uniqueId)), $($c.region): $($c.followerCount) followers, $($c.awemeCount) videos, last refreshed $($c.lastRefreshedAt)"
}

Write-Host ''
Write-Host '== Entitlements (scope keys:manage)'
$res = Invoke-TokTokData '/v1/entitlements'
if ($res) {
    $d = $res.Body.data
    Write-Host "plan $($d.planCode): $($d.credits.remaining) of $($d.credits.monthlyAllotment) credits left, reset $($d.credits.resetAt)"
}
