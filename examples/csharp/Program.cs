// toktokdata.live API samples (plain HttpClient + System.Text.Json).
//   leaderboard [REGION] [BOARD]   official board, top 10        (rank:read)
//   creator [HANDLE]               profile, waits through 202    (creator:read)
//   sessions [HANDLE]              LIVE sessions, cursor paging  (live:read)
//   webhook [PORT]                 verify alert webhooks         (TOKTOKDATA_WEBHOOK_SECRET)
using System.Net;
using System.Text.Json;
using System.Text.Json.Nodes;
using TokTokDataExamples;

string Arg(int i, string fallback) => args.Length > i && !string.IsNullOrWhiteSpace(args[i]) ? args[i] : fallback;
static string N(JsonNode? n) => n is null ? "-" : n.GetValue<double>().ToString("N0", System.Globalization.CultureInfo.InvariantCulture);

try
{
    switch (Arg(0, "leaderboard"))
    {
        case "leaderboard": await Leaderboard(Arg(1, "VN"), Arg(2, "hourly")); break;
        case "creator": await Creator(Arg(1, "tiktok")); break;
        case "sessions": await Sessions(Arg(1, "tiktok")); break;
        case "webhook": await Webhook(int.Parse(Arg(1, "8080"))); break;
        default:
            Console.Error.WriteLine("commands: leaderboard [REGION] [BOARD] | creator [HANDLE] | sessions [HANDLE] | webhook [PORT]");
            return 2;
    }
    return 0;
}
catch (TokTokDataException e)
{
    Console.Error.WriteLine($"HTTP {e.Status} {e.Code}: {e.Message} (requestId {e.RequestId ?? "-"})");
    switch (e.Status)
    {
        case 402: Console.Error.WriteLine($"Credits are spent; they reset at {e.Details["resetAt"]?.ToString() ?? "?"}."); break;
        case 429: Console.Error.WriteLine($"Rate limited; retry after {e.RetryAfter?.ToString() ?? "a few"} s."); break;
        case 403: Console.Error.WriteLine("The key lacks the scope (insufficient_scope) or the plan lacks the feature (module_not_entitled)."); break;
    }
    return 1;
}
catch (PendingTimeoutException e)
{
    Console.Error.WriteLine($"{e.Message}; try again in a minute.");
    return 1;
}

static async Task Leaderboard(string region, string board)
{
    using var client = new TokTokDataHttp();
    var res = await client.GetAsync("/v1/rankings/official", new Dictionary<string, object?>
    {
        ["board"] = board, ["region"] = region, ["limit"] = 10,
    });
    var b = res!["data"]!["board"]!;
    Console.WriteLine($"{b["board"]} board, {b["region"]}, period {b["periodKey"]}");
    if (!(bool)b["publishable"]!)
        Console.WriteLine($"(partial: coverage {(double)b["coverage"]!["ratio"]! * 100:0}% of the {(double)b["coverage"]!["threshold"]! * 100:0}% gate)");
    foreach (var e in b["entries"]!.AsArray())
    {
        var who = (bool)e!["masked"]! ? $"(masked, plan {e["unmaskTier"]})" : $"{e["nickname"]} @{e["uniqueId"]}";
        var live = (bool)e["isLive"]! ? "  LIVE" : "";
        Console.WriteLine($"{(int)e["rank"]!,3}  {who,-40} {N(e["score"]),12}{live}");
    }
}

static async Task Creator(string handle)
{
    using var client = new TokTokDataHttp();
    var res = await client.GetAsync($"/v1/creators/{Uri.EscapeDataString(handle.TrimStart('@'))}");
    var c = res!["data"]!["creator"]!;
    var verified = (bool?)c["verified"] == true ? " [verified]" : "";
    Console.WriteLine($"{(string?)c["displayName"] ?? (string?)c["uniqueId"]} (@{c["uniqueId"]}){verified}");
    Console.WriteLine($"  region {(string?)c["region"] ?? "?"}, {N(c["followerCount"])} followers, {N(c["awemeCount"])} videos, {N(c["totalFavorited"])} likes");
    Console.WriteLine($"  last refreshed {(string?)c["lastRefreshedAt"] ?? "-"}");
}

static async Task Sessions(string handle)
{
    const int maxSessions = 50;
    using var client = new TokTokDataHttp();
    string? cursor = null;
    var shown = 0;
    do
    {
        var res = await client.GetAsync("/v1/live/sessions", new Dictionary<string, object?>
        {
            ["creatorUniqueId"] = handle.TrimStart('@'), ["limit"] = 20, ["cursor"] = cursor,
        });
        var data = res!["data"]!;
        foreach (var s in data["items"]!.AsArray())
        {
            var minutes = s!["durationSeconds"] is { } d ? Math.Round(d.GetValue<double>() / 60).ToString() : "-";
            Console.WriteLine($"  {s["startedAt"]}  {minutes,4} min  peak {N(s["peakViewerCount"]),7} viewers  {N(s["totalGiftDiamonds"]),9} diamonds");
            if (++shown >= maxSessions) return;
        }
        // nextCursor is null on the last page.
        cursor = (string?)data["nextCursor"];
    } while (cursor is not null);
    if (shown == 0) Console.WriteLine("  no recorded sessions");
}

static async Task Webhook(int port)
{
    var secret = Environment.GetEnvironmentVariable("TOKTOKDATA_WEBHOOK_SECRET");
    if (string.IsNullOrWhiteSpace(secret))
    {
        Console.Error.WriteLine("Set TOKTOKDATA_WEBHOOK_SECRET to the signing secret shown when you created the webhook channel.");
        Environment.Exit(1);
    }
    using var listener = new HttpListener();
    listener.Prefixes.Add($"http://localhost:{port}/");
    listener.Start();
    Console.WriteLine($"listening on http://localhost:{port}/ (POST only)");
    while (true)
    {
        var ctx = await listener.GetContextAsync();
        var req = ctx.Request;
        if (req.HttpMethod != "POST")
        {
            ctx.Response.StatusCode = 405;
            ctx.Response.Close();
            continue;
        }
        // Verify the exact bytes that were signed.
        using var buffer = new MemoryStream();
        await req.InputStream.CopyToAsync(buffer);
        try
        {
            var p = Webhooks.Verify(buffer.ToArray(), req.Headers["x-toktokdata-timestamp"], req.Headers["x-toktokdata-signature"], secret!);
            Console.WriteLine($"alert {p["alertId"]} from rule \"{p["rule"]?["name"]}\" at {p["occurredAt"] ?? p["deliveredAt"]}");
            Console.WriteLine(p["event"]?.ToJsonString(new JsonSerializerOptions { WriteIndented = true }));
            ctx.Response.StatusCode = 204;
        }
        catch (InvalidWebhookSignatureException e)
        {
            Console.Error.WriteLine($"rejected webhook: {e.Message}");
            ctx.Response.StatusCode = 401;
        }
        ctx.Response.Close();
    }
}
