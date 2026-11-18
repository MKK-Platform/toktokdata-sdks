package live.toktokdata.examples;

import com.fasterxml.jackson.databind.JsonNode;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.security.GeneralSecurityException;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.HexFormat;
import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;

/** Verifies toktokdata.live alert webhook signatures. */
public final class Webhooks {
    public static final long DEFAULT_TOLERANCE_SECONDS = 300;

    private Webhooks() {
    }

    public static final class InvalidSignatureException extends Exception {
        InvalidSignatureException(String message) {
            super(message);
        }
    }

    /**
     * Checks {@code x-toktokdata-signature: sha256=<hex HMAC-SHA256(secret, "<timestamp>.<raw body>")>}
     * and that {@code x-toktokdata-timestamp} is within the tolerance, then parses the body.
     */
    public static JsonNode verify(byte[] rawBody, String timestamp, String signature, String secret, long toleranceSeconds)
            throws InvalidSignatureException {
        if (timestamp == null || signature == null) {
            throw new InvalidSignatureException("missing signature headers");
        }
        long ts;
        try {
            ts = Long.parseLong(timestamp.trim());
        } catch (NumberFormatException e) {
            throw new InvalidSignatureException("bad timestamp");
        }
        if (Math.abs(Instant.now().getEpochSecond() - ts) > toleranceSeconds) {
            throw new InvalidSignatureException("timestamp outside the tolerance");
        }

        byte[] expected;
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
            mac.update((timestamp.trim() + ".").getBytes(StandardCharsets.UTF_8));
            expected = ("sha256=" + HexFormat.of().formatHex(mac.doFinal(rawBody))).getBytes(StandardCharsets.UTF_8);
        } catch (GeneralSecurityException e) {
            throw new IllegalStateException(e);
        }
        // MessageDigest.isEqual is constant-time.
        if (!MessageDigest.isEqual(expected, signature.trim().getBytes(StandardCharsets.UTF_8))) {
            throw new InvalidSignatureException("signature mismatch");
        }
        try {
            return TokTokDataHttp.JSON.readTree(rawBody);
        } catch (IOException e) {
            throw new InvalidSignatureException("body is not JSON");
        }
    }
}
