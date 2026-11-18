using System.Security.Cryptography;
using System.Text;
using System.Text.Json.Nodes;

namespace TokTokDataExamples;

public sealed class InvalidWebhookSignatureException(string message) : Exception(message);

/// <summary>Verifies toktokdata.live alert webhook signatures.</summary>
public static class Webhooks
{
    /// <summary>
    /// Checks <c>x-toktokdata-signature: sha256=&lt;hex HMAC-SHA256(secret, "&lt;timestamp&gt;.&lt;raw body&gt;")&gt;</c>
    /// and that <c>x-toktokdata-timestamp</c> is within the tolerance, then parses the body.
    /// </summary>
    public static JsonNode Verify(byte[] rawBody, string? timestamp, string? signature, string secret, int toleranceSeconds = 300)
    {
        if (timestamp is null || signature is null)
            throw new InvalidWebhookSignatureException("missing signature headers");
        if (!long.TryParse(timestamp.Trim(), out var ts))
            throw new InvalidWebhookSignatureException("bad timestamp");
        if (Math.Abs(DateTimeOffset.UtcNow.ToUnixTimeSeconds() - ts) > toleranceSeconds)
            throw new InvalidWebhookSignatureException("timestamp outside the tolerance");

        var signed = Encoding.UTF8.GetBytes(timestamp.Trim() + ".").Concat(rawBody).ToArray();
        var expected = "sha256=" + Convert.ToHexString(HMACSHA256.HashData(Encoding.UTF8.GetBytes(secret), signed)).ToLowerInvariant();
        // FixedTimeEquals is constant-time.
        if (!CryptographicOperations.FixedTimeEquals(Encoding.UTF8.GetBytes(expected), Encoding.UTF8.GetBytes(signature.Trim())))
            throw new InvalidWebhookSignatureException("signature mismatch");

        try
        {
            return JsonNode.Parse(rawBody) ?? throw new InvalidWebhookSignatureException("empty body");
        }
        catch (System.Text.Json.JsonException)
        {
            throw new InvalidWebhookSignatureException("body is not JSON");
        }
    }
}
