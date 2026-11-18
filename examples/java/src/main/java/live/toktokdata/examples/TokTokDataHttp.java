package live.toktokdata.examples;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.StringJoiner;

/** A minimal toktokdata.live client: auth, query encoding, 202 waits, retries and the error envelope. */
public final class TokTokDataHttp {
    static final ObjectMapper JSON = new ObjectMapper();
    private static final int MAX_RETRIES = 2;
    private static final Duration PENDING_DEADLINE = Duration.ofSeconds(180);

    private final HttpClient http = HttpClient.newBuilder().connectTimeout(Duration.ofSeconds(10)).build();
    private final String baseUrl;
    private final String apiKey;

    public TokTokDataHttp() {
        String key = System.getenv("TOKTOKDATA_API_KEY");
        if (key == null || key.isBlank()) {
            throw new IllegalStateException("Set TOKTOKDATA_API_KEY (dashboard Settings -> API keys at https://toktokdata.live).");
        }
        String base = System.getenv("TOKTOKDATA_BASE_URL");
        this.baseUrl = base == null || base.isBlank() ? "https://api.toktokdata.live" : base.replaceAll("/+$", "");
        this.apiKey = key;
    }

    /** Thrown for any 4xx/5xx answer; fields mirror the error envelope. */
    public static final class ApiException extends RuntimeException {
        public final int status;
        public final String code;
        public final String requestId;
        public final boolean retryable;
        public final Integer retryAfter; // seconds, from the Retry-After header
        public final JsonNode details;   // the whole envelope (402 carries resetAt)

        ApiException(int status, JsonNode envelope, Integer retryAfter) {
            super(envelope.path("message").asText("HTTP " + status));
            this.status = status;
            this.code = envelope.path("code").asText("http_error");
            this.requestId = envelope.path("requestId").asText(null);
            this.retryable = envelope.path("retryable").asBoolean(false);
            this.retryAfter = retryAfter;
            this.details = envelope;
        }
    }

    /** Thrown when a 202 "still collecting" answer did not turn into data within 180 s. */
    public static final class PendingTimeoutException extends RuntimeException {
        PendingTimeoutException(String message) {
            super(message);
        }
    }

    public static Map<String, Object> query(Object... keyValues) {
        Map<String, Object> q = new LinkedHashMap<>();
        for (int i = 0; i + 1 < keyValues.length; i += 2) {
            q.put((String) keyValues[i], keyValues[i + 1]);
        }
        return q;
    }

    /** GET a JSON resource. Waits through 202, retries 429 and retryable 5xx. */
    public JsonNode get(String path, Map<String, Object> query) throws IOException, InterruptedException {
        URI uri = URI.create(baseUrl + path + encode(query));
        Instant deadline = Instant.now().plus(PENDING_DEADLINE);
        int attempt = 0;
        while (true) {
            HttpRequest request = HttpRequest.newBuilder(uri)
                    .timeout(Duration.ofSeconds(30))
                    .header("Authorization", "Bearer " + apiKey)
                    .header("Accept", "application/json")
                    .header("User-Agent", "toktokdata-java-example/0.1.0")
                    .GET()
                    .build();
            HttpResponse<String> response = http.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
            int status = response.statusCode();
            Integer retryAfter = response.headers().firstValue("Retry-After").map(TokTokDataHttp::parseIntOrNull).orElse(null);

            if (status == 202) {
                // Still collecting: wait retryAfterSeconds (or Retry-After), clamped to 1..120 s, and ask again.
                JsonNode body = JSON.readTree(response.body());
                int wait = clamp(body.path("retryAfterSeconds").asInt(retryAfter != null ? retryAfter : 5), 1, 120);
                if (Instant.now().plusSeconds(wait).isAfter(deadline)) {
                    throw new PendingTimeoutException("Still collecting after 180 s (job " + body.path("jobId").asText("?") + ")");
                }
                System.err.printf("  202 %s, asking again in %d s%n", body.path("status").asText("resolving"), wait);
                Thread.sleep(wait * 1000L);
                continue;
            }
            if (status >= 200 && status < 300) {
                return status == 204 || response.body().isEmpty() ? null : JSON.readTree(response.body());
            }

            JsonNode envelope = parseEnvelope(response.body());
            ApiException error = new ApiException(status, envelope, retryAfter);
            boolean retry = (status == 429 && !"daily_quota_exceeded".equals(error.code))
                    || (status >= 500 && (!envelope.has("retryable") || error.retryable));
            if (!retry || attempt >= MAX_RETRIES) {
                throw error;
            }
            long waitMs = retryAfter != null ? Math.min(retryAfter, 60) * 1000L : (long) (500 * Math.pow(2, attempt) + Math.random() * 250);
            Thread.sleep(waitMs);
            attempt++;
        }
    }

    private static JsonNode parseEnvelope(String body) {
        try {
            JsonNode node = JSON.readTree(body);
            if (node != null && node.isObject() && node.has("code")) {
                return node;
            }
        } catch (IOException ignored) {
            // not JSON: fall through
        }
        ObjectNode node = JSON.createObjectNode();
        node.put("code", "http_error");
        node.put("message", body.length() > 200 ? body.substring(0, 200) : body);
        return node;
    }

    private static String encode(Map<String, Object> query) {
        if (query == null || query.isEmpty()) {
            return "";
        }
        StringJoiner joiner = new StringJoiner("&", "?", "");
        query.forEach((k, v) -> {
            if (v != null) {
                joiner.add(URLEncoder.encode(k, StandardCharsets.UTF_8) + "=" + URLEncoder.encode(String.valueOf(v), StandardCharsets.UTF_8));
            }
        });
        return joiner.length() == 1 ? "" : joiner.toString();
    }

    private static Integer parseIntOrNull(String s) {
        try {
            return Integer.parseInt(s.trim());
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private static int clamp(int v, int min, int max) {
        return Math.max(min, Math.min(max, v));
    }
}
