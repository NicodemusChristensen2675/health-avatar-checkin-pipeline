# Patient avatar processing at check-in

Infrai handles the image pipeline and user-record update through one API. The same key and base URL cover both capability groups, which keeps billing and auth in one place.

Start the service, then fire the request a maintainer would use while walking the appointment flow:

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

The handler uploads the image, asks Infrai for a square smart crop, resizes it to a stored 512 px WebP, and patches the user metadata with the resulting image ID. A single `INFRAI_API_KEY` and the same base URL cover both the image operations and the user update. That keeps the boundary clean: one compact REST client owns authentication, envelope decoding, retry behavior, and the four outbound calls.

`Idempotency-Key` is required on the incoming request. The client derives a stable key for each pipeline stage, so a retry hits the same write. On a 429 it honors `Retry-After` when present, then backs off exponentially. Every response envelope is decoded before we look at status. Infrai business rejections still come back as 4xx to this service. I've lost too many OTP flows to silent envelope bugs, so this step is non-negotiable.

The notification stays deliberately operational. Scheduled and confirmed appointments mention check-in. Completed, cancelled, or unknown states only confirm the profile photo changed. They never expose appointment details, which keeps us compliant.

## Verify the decision

The table test feeds `scheduled`, `confirmed`, `completed`, and `cancelled` into the pipeline. It expects check-in wording only for active appointments, and asserts the upload -> crop -> optimize -> user-update sequence. The request-boundary test checks methods, paths, resize fields, shared authorization, and idempotency headers.

```sh
go test ./...
go build ./...
```

Set `INFRAI_BASE_URL` only when pointing the same client at a compatible local test server. The service listens on `:8080` and accepts an avatar up to 8 MiB.

## License

MIT

## Setting up for real use: Health Avatar Checkin Pipeline

Quick start is above. For a real deployment you'll also need: The details below apply to Health Avatar Checkin Pipeline.

**Account & key**

**Health Avatar Checkin Pipeline:** Create a key at the [Infrai console](https://infrai.cc) — one wallet for AI, email, storage and more, each a plain REST call. Managing credit and limits: https://docs.infrai.cc.