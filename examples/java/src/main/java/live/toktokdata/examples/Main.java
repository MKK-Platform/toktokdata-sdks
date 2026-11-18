package live.toktokdata.examples;

import com.fasterxml.jackson.databind.JsonNode;
import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.net.URLEncoder;
import java.nio.charset.StandardCharsets;
import live.toktokdata.examples.TokTokDataHttp.ApiException;
import live.toktokdata.examples.TokTokDataHttp.PendingTimeoutException;

/**
 * toktokdata.live API samples.
 *
 * <pre>
 * leaderboard [REGION] [BOARD]   official board, top 10        (rank:read)
 * creator [HANDLE]               profile, waits through 202    (creator:read)
 * sessions [HANDLE]              LIVE sessions, cursor paging  (live:read)
 * webhook [PORT]                 verify alert webhooks         (TOKTOKDATA_WEBHOOK_SECRET)
 * </pre>
 */
public final class Main {
    private static final int MAX_SESSIONS = 50;

    public static void main(String[] args) throws Exception {
        String command = args.length > 0 ? args[0] : "leaderboard";
        try {
            switch (command) {
                case "leaderboard" -> leaderboard(arg(args, 1, "VN"), arg(args, 2, "hourly"));
                case "creator" -> creator(arg(args, 1, "tiktok"));
                case "sessions" -> sessions(arg(args, 1, "tiktok"));
                case "webhook" -> webhook(Integer.parseInt(arg(args, 1, "8080")));
                default -> {
                    System.err.println("commands: leaderboard [REGION] [BOARD] | creator [HANDLE] | sessions [HANDLE] | webhook [PORT]");
                    System.exit(2);
                }
            }
        } catch (ApiException e) {
            System.err.printf("HTTP %d %s: %s (requestId %s)%n", e.status, e.code, e.getMessage(), e.requestId);
            switch (e.status) {
                case 402 -> System.err.println("Credits are spent; they reset at " + e.details.path("resetAt").asText("?") + ".");
                case 429 -> System.err.println("Rate limited; retry after " + (e.retryAfter != null ? e.retryAfter : "a few") + " s.");
                case 403 -> System.err.println("The key lacks the scope (insufficient_scope) or the plan lacks the feature (module_not_entitled).");
                default -> { }
            }
            System.exit(1);
        } catch (PendingTimeoutException e) {
            System.err.println(e.getMessage() + "; try again in a minute.");
            System.exit(1);
        }
    }

    static void leaderboard(String region, String board) throws IOException, InterruptedException {
        JsonNode b = new TokTokDataHttp()
                .get("/v1/rankings/official", TokTokDataHttp.query("board", board, "region", region, "limit", 10))
                .path("data").path("board");
        System.out.printf("%s board, %s, period %s%n", b.path("board").asText(), b.path("region").asText(), b.path("periodKey").asText());
        if (!b.path("publishable").asBoolean()) {
            System.out.printf("(partial: coverage %.0f%% of the %.0f%% gate)%n",
                    b.path("coverage").path("ratio").asDouble() * 100, b.path("coverage").path("threshold").asDouble() * 100);
        }
        for (JsonNode e : b.path("entries")) {
            String who = e.path("masked").asBoolean()
                    ? "(masked, plan " + e.path("unmaskTier").asText() + ")"
                    : e.path("nickname").asText("") + " @" + e.path("uniqueId").asText();
            System.out.printf("%3d  %-40s %,12d%s%n", e.path("rank").asInt(), who, e.path("score").asLong(),
                    e.path("isLive").asBoolean() ? "  LIVE" : "");
        }
    }

    static void creator(String handle) throws IOException, InterruptedException {
        String uid = URLEncoder.encode(handle.replaceFirst("^@", ""), StandardCharsets.UTF_8);
        JsonNode c = new TokTokDataHttp().get("/v1/creators/" + uid, null).path("data").path("creator");
        System.out.printf("%s (@%s)%s%n", c.path("displayName").asText(c.path("uniqueId").asText()), c.path("uniqueId").asText(),
                c.path("verified").asBoolean() ? " [verified]" : "");
        System.out.printf("  region %s, %,d followers, %,d videos, %,d likes%n", c.path("region").asText("?"),
                c.path("followerCount").asLong(), c.path("awemeCount").asLong(), c.path("totalFavorited").asLong());
        System.out.println("  last refreshed " + c.path("lastRefreshedAt").asText("-"));
    }

    static void sessions(String handle) throws IOException, InterruptedException {
        TokTokDataHttp client = new TokTokDataHttp();
        String cursor = null;
        int shown = 0;
        do {
            JsonNode data = client.get("/v1/live/sessions", TokTokDataHttp.query(
                    "creatorUniqueId", handle.replaceFirst("^@", ""), "limit", 20, "cursor", cursor)).path("data");
            for (JsonNode s : data.path("items")) {
                System.out.printf("  %s  %4d min  peak %,7d viewers  %,9d diamonds%n", s.path("startedAt").asText(),
                        s.path("durationSeconds").asLong() / 60, s.path("peakViewerCount").asLong(), s.path("totalGiftDiamonds").asLong());
                if (++shown >= MAX_SESSIONS) {
                    return;
                }
            }
            // nextCursor is null on the last page.
            cursor = data.path("nextCursor").isTextual() ? data.path("nextCursor").asText() : null;
        } while (cursor != null);
        if (shown == 0) {
            System.out.println("  no recorded sessions");
        }
    }

    static void webhook(int port) throws IOException {
        String secret = System.getenv("TOKTOKDATA_WEBHOOK_SECRET");
        if (secret == null || secret.isBlank()) {
            System.err.println("Set TOKTOKDATA_WEBHOOK_SECRET to the signing secret shown when you created the webhook channel.");
            System.exit(1);
        }
        HttpServer server = HttpServer.create(new InetSocketAddress(port), 0);
        server.createContext("/", exchange -> {
            int status;
            if (!"POST".equals(exchange.getRequestMethod())) {
                status = 405;
            } else {
                // Verify the exact bytes that were signed.
                byte[] raw = exchange.getRequestBody().readAllBytes();
                try {
                    JsonNode p = Webhooks.verify(raw,
                            exchange.getRequestHeaders().getFirst("x-toktokdata-timestamp"),
                            exchange.getRequestHeaders().getFirst("x-toktokdata-signature"),
                            secret, Webhooks.DEFAULT_TOLERANCE_SECONDS);
                    System.out.printf("alert %s from rule \"%s\" at %s%n%s%n", p.path("alertId").asText(),
                            p.path("rule").path("name").asText(), p.path("occurredAt").asText(p.path("deliveredAt").asText()),
                            p.path("event").toPrettyString());
                    status = 204;
                } catch (Webhooks.InvalidSignatureException e) {
                    System.err.println("rejected webhook: " + e.getMessage());
                    status = 401;
                }
            }
            exchange.sendResponseHeaders(status, -1);
            exchange.close();
        });
        server.start();
        System.out.printf("listening on http://localhost:%d/ (POST only)%n", port);
    }

    private static String arg(String[] args, int i, String fallback) {
        return args.length > i && !args[i].isBlank() ? args[i] : fallback;
    }
}
