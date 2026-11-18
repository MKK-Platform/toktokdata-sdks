// Receive toktokdata.live alert webhooks and verify their signature.
// Usage: TOKTOKDATA_WEBHOOK_SECRET=... npx tsx webhook-server.ts [PORT]
// No API key needed here; creating the alert rule that points at this server needs webhook:manage.
import { createServer } from 'node:http';
import { verifyWebhook } from '@toktokdata.live/sdk/webhooks';

interface AlertPayload {
  schemaVersion: string;
  alertId: string;
  rule: { id: string; name: string };
  event: Record<string, unknown>;
  occurredAt: string | null;
  deliveredAt: string;
}

const secret = process.env.TOKTOKDATA_WEBHOOK_SECRET;
if (!secret) {
  console.error('Set TOKTOKDATA_WEBHOOK_SECRET to the signing secret shown when you created the webhook channel.');
  process.exit(1);
}
const port = Number(process.argv[2] ?? 8080);

const server = createServer(async (req, res) => {
  if (req.method !== 'POST') {
    res.writeHead(405).end();
    return;
  }
  // Verify the exact bytes that were signed: read the raw body, do not re-serialize JSON.
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(chunk as Buffer);
  const rawBody = Buffer.concat(chunks).toString('utf8');

  let payload: AlertPayload;
  try {
    // Checks x-toktokdata-signature (HMAC-SHA256 of "<timestamp>.<body>") and a 300 s timestamp tolerance.
    payload = (await verifyWebhook(rawBody, req.headers, secret)) as unknown as AlertPayload;
  } catch (err) {
    console.warn(`rejected webhook: ${(err as Error).message}`);
    res.writeHead(401).end();
    return;
  }

  console.log(`alert ${payload.alertId} from rule "${payload.rule.name}" at ${payload.occurredAt ?? payload.deliveredAt}`);
  console.log(JSON.stringify(payload.event, null, 2));
  // Answer 2xx quickly and do slow work after responding, so the delivery does not time out.
  res.writeHead(204).end();
});

server.listen(port, () => console.log(`listening on http://localhost:${port}/ (POST only)`));
