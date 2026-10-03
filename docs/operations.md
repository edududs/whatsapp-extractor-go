# Operations

Run one process per linked WhatsApp device. Keep `data/session.db` on persistent storage and restrict access to the service user. Newly created directories use mode 0700 and files use 0600 on POSIX. Existing permissions are not modified. SQLite WAL files inherit the database's access constraints; Windows deployments should also set appropriate ACLs.

## Initial validation

1. Run `whatsapp-extractor-go check -config extractor.toml`.
2. Run `pair` with the same config and scan the QR code on the intended phone.
3. Run `accounts`, select an account, then run `groups -account <phone>`.
4. Configure a test group in the watchlist and run extraction with `view = "json"`.
5. Send text and an image from another device. Confirm typed output, then inspect the SQL payloads. Send one message outside the watchlist and confirm it is excluded.
6. Stop with SIGINT, restart and confirm that the paired session is reused.

The agent-created repository's automated checks cover protocol fixtures and local databases, not this phone-dependent procedure. No messages are sent by the extractor itself.

## Databases

Keep session credentials local with `EXTRACTOR_DATABASE=data/session.db`; use `EXTRACTOR_MESSAGES_DATABASE` for remote storage. PostgreSQL session tables belong to whatsmeow and use the default schema. Avoid exposing those tables through a public data API. Use a dedicated database/user with permission to create the account schemas required by migrations.

For one SQLite account:

```sql
SELECT id, chat_jid, timestamp, payload
FROM wa_5511900000001_messages
ORDER BY timestamp;
```

For PostgreSQL, the table is `wa_5511900000001.messages`. The timestamp is UTC Unix milliseconds. JSONL paths must contain `{account}`; successful writes are fsynced. Memory storage is ephemeral and intentionally unbounded, suitable for tests or controlled embedding workloads.

Do not point this program at a Python extractor's database expecting migration compatibility. Back up credentials and messages with database-aware tools; copying only the main SQLite file while WAL writes are active is insufficient.

## Failure handling

- **Buffer full:** the process exits nonzero. Investigate slow storage or handlers before increasing `buffer_size`. Already queued messages may not be saved, and restart does not ensure replay.
- **Transient disconnect:** whatsmeow attempts reconnection. Permanent logout is a terminal error; pair again only after checking the linked devices on the phone.
- **Write/mapping failure:** extraction stops before publication of that message. The CLI intentionally suppresses raw database connection errors that may contain credentials; check database-side logs and permissions.
- **Handler error:** logged and isolated. An in-process handler is not a durable subscription. Query persisted data with your own cursor if missing notifications is unacceptable.
- **SIGINT/SIGTERM:** cancellation is prompt, with no queue-drain guarantee. No exactly-once or lossless shutdown claim is made.

Use a process supervisor with restart backoff. Logs include connection changes, successful persistence metadata and received/skipped protocol-event counters at shutdown. These counters count source events, not exactly-once persisted messages. Message JSON and QR output are sensitive and should not go into shared logs.

## Builds and releases

`make build` creates a local binary. The tag-triggered release workflow builds Linux, macOS and Windows binaries for amd64/arm64, then publishes SHA256 checksums and generated release notes. Use semantic versioning; pre-1.0 minor releases can change the public API. Push an annotated `vX.Y.Z` tag only after CI has passed for that commit.

The Docker image runs as UID/GID 65532 and expects a writable `/data` volume owned by that user. Mount config read-only; pass `-config /path/extractor.toml` after the command. The default working directory is `/`, so the default relative `data/` directory maps to `/data`.
