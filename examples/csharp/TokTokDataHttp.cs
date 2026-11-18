using System.Net;
using System.Net.Http.Headers;
using System.Text.Json;
using System.Text.Json.Nodes;

namespace TokTokDataExamples;

/// <summary>Thrown for any 4xx/5xx answer; properties mirror the error envelope.</summary>
public sealed class TokTokDataException(int status, JsonObject envelope, int? retryAfter)
    : Exception((string?)envelope["message"] ?? $"HTTP {status}")
{
    public int Status { get; } = status;
    public string Code { get; } = (string?)envelope["code"] ?? "http_error";
    public string? RequestId { get; } = (string?)envelope["requestId"];
    public bool Retryable { get; } = (bool?)envelope["retryable"] ?? false;
    /// <summary>Seconds from the Retry-After header, when present.</summary>
    public int? RetryAfter { get; } = retryAfter;
    /// <summary>The whole envelope; a 402 carries <c>resetAt</c>.</summary>
    public JsonObject Details { get; } = envelope;
}

/// <summary>Thrown when a 202 "still collecting" answer did not turn into data within 180 s.</summary>
public sealed class PendingTimeoutException(string message) : Exception(message);

/// <summary>A minimal toktokdata.live client: auth, query encoding, 202 waits, retries and the error envelope.</summary>
public sealed class TokTokDataHttp : IDisposable
{
    private const int MaxRetries = 2;
    private static readonly TimeSpan PendingDeadline = TimeSpan.FromSeconds(180);
    private readonly HttpClient _http;

    public TokTokDataHttp()
    {
        var key = Environment.GetEnvironmentVariable("TOKTOKDATA_API_KEY");
        if (string.IsNullOrWhiteSpace(key))
            throw new InvalidOperationException("Set TOKTOKDATA_API_KEY (dashboard Settings -> API keys at https://toktokdata.live).");
        var baseUrl = Environment.GetEnvironmentVariable("TOKTOKDATA_BASE_URL");
        _http = new HttpClient
        {
            BaseAddress = new Uri((string.IsNullOrWhiteSpace(baseUrl) ? "https://api.toktokdata.live" : baseUrl.TrimEnd('/')) + "/"),
            Timeout = TimeSpan.FromSeconds(30),
        };
        _http.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", key);
        _http.DefaultRequestHeaders.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        _http.DefaultRequestHeaders.UserAgent.ParseAdd("toktokdata-csharp-example/0.1.0");
    }

    /// <summary>GET a JSON resource. Waits through 202, retries 429 and retryable 5xx.</summary>
    public async Task<JsonNode?> GetAsync(string path, IDictionary<string, object?>? query = null, CancellationToken ct = default)
    {
        var url = path.TrimStart('/') + Encode(query);
        var deadline = DateTimeOffset.UtcNow + PendingDeadline;
        for (var attempt = 0; ; )
        {
            using var response = await _http.GetAsync(url, ct);
            var body = await response.Content.ReadAsStringAsync(ct);
            var status = (int)response.StatusCode;
            int? retryAfter = response.Headers.RetryAfter?.Delta is { } delta ? (int)delta.TotalSeconds : null;

            if (response.StatusCode == HttpStatusCode.Accepted)
            {
                // Still collecting: wait retryAfterSeconds (or Retry-After), clamped to 1..120 s, and ask again.
                var pending = JsonNode.Parse(body)?.AsObject();
                var wait = Math.Clamp((int?)pending?["retryAfterSeconds"] ?? retryAfter ?? 5, 1, 120);
                if (DateTimeOffset.UtcNow.AddSeconds(wait) > deadline)
                    throw new PendingTimeoutException($"Still collecting after 180 s (job {pending?["jobId"]})");
                Console.Error.WriteLine($"  202 {(string?)pending?["status"] ?? "resolving"}, asking again in {wait} s");
                await Task.Delay(TimeSpan.FromSeconds(wait), ct);
                continue;
            }
            if (response.IsSuccessStatusCode)
                return string.IsNullOrEmpty(body) ? null : JsonNode.Parse(body);

            var error = new TokTokDataException(status, ParseEnvelope(body), retryAfter);
            var retry = (status == 429 && error.Code != "daily_quota_exceeded")
                || (status >= 500 && (!error.Details.ContainsKey("retryable") || error.Retryable));
            if (!retry || attempt >= MaxRetries)
                throw error;
            var waitFor = retryAfter is { } s
                ? TimeSpan.FromSeconds(Math.Min(s, 60))
                : TimeSpan.FromMilliseconds(500 * Math.Pow(2, attempt) + Random.Shared.Next(250));
            await Task.Delay(waitFor, ct);
            attempt++;
        }
    }

    private static JsonObject ParseEnvelope(string body)
    {
        try
        {
            if (JsonNode.Parse(body) is JsonObject obj && obj.ContainsKey("code"))
                return obj;
        }
        catch (JsonException)
        {
            // not JSON: fall through
        }
        return new JsonObject { ["code"] = "http_error", ["message"] = body.Length > 200 ? body[..200] : body };
    }

    private static string Encode(IDictionary<string, object?>? query)
    {
        if (query is null) return "";
        var parts = query
            .Where(kv => kv.Value is not null)
            .Select(kv => $"{Uri.EscapeDataString(kv.Key)}={Uri.EscapeDataString(Format(kv.Value!))}")
            .ToList();
        return parts.Count == 0 ? "" : "?" + string.Join("&", parts);
    }

    // Booleans go out as true/false, numbers without culture-specific separators.
    private static string Format(object value) => value switch
    {
        bool b => b ? "true" : "false",
        IFormattable f => f.ToString(null, System.Globalization.CultureInfo.InvariantCulture),
        _ => value.ToString() ?? "",
    };

    public void Dispose() => _http.Dispose();
}
