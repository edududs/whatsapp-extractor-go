# Architecture decisions

The reference is `edududs/whatsapp-extractor` at `7cd93ecc2f3f3f576e4de44f1e981b42a5d36a9c`. These decisions adapt its contracts to Go rather than mechanically translating Python frameworks.

| Decision | Reason and tradeoff |
| --- | --- |
| Go 1.27, direct whatsmeow | One runtime, access to native context and event APIs. Pin the pseudo-version because upstream has no stable semantic release contract. |
| Standard-library-only core | Domain/application code remains usable without SQL or protocol packages. Consumer-defined interfaces and executable import checks protect this boundary. |
| `iter.Seq2[Message,error]` source | Idiomatic synchronous consumption with early-break cleanup. The adapter owns its queue, handlers and connection; callers do not close producer channels. |
| Explicit constructors and composition | Wiring remains visible and compile-time checked; no DI container. Public packages support embedding while CLI implementation stays internal. |
| Value messages with defensive clones | Go structs are not immutable. Clone pointer fields before persistence and each handler so one consumer cannot mutate another's input. |
| Persist before publish | Events describe completed writes. An ordered in-process bus isolates returned errors, not panics. No promise of atomicity between persistence and notification. |
| `(chat_jid,id)` primary key | An ID collision across chats must not overwrite a message. Account context remains outside the domain and is represented by SQL namespaces. |
| SQL JSON payload with indexed projections | Flexible content schema with cheap per-chat/time queries. Payload is text for SQLite/PostgreSQL parity; timestamps are Unix milliseconds. |
| Versioned transactional migrations | Reject unknown future schemas and serialize PostgreSQL migration startup using an advisory transaction lock. Whatsmeow's own migrations remain separate. |
| Pure-Go SQLite and pgx | Portable binaries with `CGO_ENABLED=0`, while retaining race-test support on native CI runners. SQLite uses WAL, foreign keys, one connection and a busy timeout. |
| Fail on overflow | No silent successful-looking extraction after losing a queue item. This is still a live feed, not a replayable log; operators must monitor failures. |
| Prompt cancellation | `context` terminates connection and persistence operations. Shutdown can abandon buffered messages; durable spooling would require a separate explicit delivery contract. |
| Declarative strict TOML | Unknown keys fail early. Environment carries deployment secrets; CLI does not rewrite files or silently load `.env`. Per-account lists replace global lists. |
| No background history backfill or media downloads | Keep parity with live extraction and metadata-only handling; retrieving files/history changes resource usage and product scope. |
| Tools in `go.mod` | Native `go tool` gives pinned, reproducible staticcheck/govulncheck versions without a separate tools module. CI pins third-party actions by commit. |

Possible future extensions should respond to a measured requirement: transactional outbox, durable ingestion queue, consumer offset API, explicit history import, OpenTelemetry, or distributed account leases. They are not implied guarantees of this version.
