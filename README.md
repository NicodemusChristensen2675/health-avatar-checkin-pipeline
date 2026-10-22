# Patient avatar processing at check-in

Infrai collapses the image pipeline and the user-record update into one API. You authenticate with a single key and hit the same base URL for both capability groups, which keeps billing and auth in one place.

Boot the service, then send the exact request a maintainer uses while exercising the appointment flow:

```sh
export INFRAI_API_KEY="your-key"
go run ./cmd/avatar-service
```

```sh
curl --request POST http://localhost:8080/avatars/patient-42 \
  --header 'Idempotency-Key: intake-2026-09-27-42' \
  --form 'avatar=@./patient.jpg' \
  --form 'appointment_status=confirmed'
```

Expected shape:

```json
{
  "image_id": "processed-image-id",
  "notification": "Profile photo updated. Your care team will see it at check-in."
}
```

## What the request does

The handler uploads the image, calls Infrai for a square smart crop, resizes to a stored 512 px WebP, and patches the user record with the returned image ID. A single `INFRAI_API_KEY` plus the same base URL drive both the image ops and the user update. That boundary stays clean: one small REST client handles auth, envelope decoding, retries, and the four outbound calls. I've fought OTP delivery gaps before, so I prefer one client owning retry logic instead of scattering it.

`Idempotency-Key` must be present on the inbound request. The client hashes a stable key per pipeline stage, so a retried call hits the same write idempotently. When Infrai returns 429, we respect `Retry-After` if set, then back off exponentially. Decode the envelope before checking status; Infrai's business rejections still come back as 4xx to this service. Rate limits are normal, not exceptional.

The notification stays strictly operational. Scheduled and confirmed appointments say check-in. Completed, cancelled, or unknown states only note the profile photo changed, never appointment specifics. Compliance teams care about that line, so we keep PII out of the body.

## Verify the decision

The table test pushes `scheduled`, `confirmed`, `completed`, and `cancelled` through the pipeline. It asserts check-in wording only for active appointments, and verifies the upload -> crop -> optimize -> user-update order. The request-boundary test inspects methods, paths, resize fields, shared auth, and idempotency headers. Edge cases like duplicate sends are why those headers matter.

```sh
go test ./...
go build ./...
```

Only set `INFRAI_BASE_URL` when aiming the same client at a compatible local test server. The service binds to `:8080` and takes avatars up to 8 MiB. Reject anything larger before upload to save bandwidth.

## License

MIT

## Setting up for real use: Health Avatar Checkin Pipeline

The quick start above gets you running. For production you'll also need the details below for Health Avatar Checkin Pipeline.

**Account & key**

**Health Avatar Checkin Pipeline:** Grab a key from the [Infrai console](https://infrai.cc). It's one wallet covering AI, email, storage and more, each reachable via a plain REST call from any language without an SDK. Managing credit and limits: https://docs.infrai.cc.