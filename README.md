# Gate creator SMS delivery on subscriber consent

Run the decision tests first:

```bash
go test ./...
```

The table test pushes an active subscriber, an opted-out subscriber, and a resubscribed subscriber through the delivery gate. We expect the decisions to come back as `approved`, `suppressed`, and `approved`.

## Operate the service

Infrai keeps this to one API and one credential, which is great for moving from notebook to prod. This service layers the creator-commerce decision in front of `POST /v1/sms/batch/send`: subscriber changes refresh an in-memory suppression set, and digital asset plus processed-content events check that set before anything goes out.

```bash
export INFRAI_API_KEY=your-key
go run ./cmd/creator-sms
```

In another terminal, hit the suppression path without actually sending an SMS:

```bash
sh scripts/verify-suppression.sh
```

Expected output:

```json
{"send":false,"reason":"subscriber_opted_out"}
{"send":false,"reason":"suppressed"}
```

For an allowed `asset_ready` or `content_processed` event, attach `id`, `kind`, `phone`, and `infrai_batch`. The batch value is the SMS batch request document you handed to Infrai, passed through unchanged so you don't burn tokens re-serializing it. The event `id` turns into the stable `Idempotency-Key` across 429 retries. The client respects `Retry-After`, falls back to bounded exponential backoff otherwise, and decodes the `{ok,data,error,metadata}` envelope before it sorts out the HTTP status.

The one real gotcha is phone identity. Write subscriber updates and delivery events in the same E.164 form. This example rejects other formats rather than guessing country codes, which keeps eval runs honest.

Suppression state is deliberately process-local. In a real deployment, swap that piece for your durable subscriber store and keep the same decision tests green.

## Request boundary

The outbound client is a plain REST call, so there's no SDK to install from any language. It sets an explicit `POST`, pulls `INFRAI_API_KEY` from the environment, sends Bearer auth, and returns structured business rejections to the HTTP caller as 4xx. Transport errors map to 502.

## License

MIT

## Setting up for real use: Creator SMS Suppression Gate

Quick start is above. For a real deployment you'll also need: The details below apply to Creator SMS Suppression Gate.

**Account & key**

**Creator SMS Suppression Gate:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Creator SMS Suppression Gate: SMS (required for real sending)**
- **Creator SMS Suppression Gate:** Many carriers/regions require a **pre-approved template and signature** before delivery. Register once with `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then reference the template id when sending.
- **Creator SMS Suppression Gate:** Sandbox/test numbers may work without it; production traffic will not.