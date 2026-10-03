# whatsapp-extractor-go

[![CI](https://github.com/edududs/whatsapp-extractor-go/actions/workflows/ci.yml/badge.svg)](https://github.com/edududs/whatsapp-extractor-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/edududs/whatsapp-extractor-go.svg)](https://pkg.go.dev/github.com/edududs/whatsapp-extractor-go)

Read a paired WhatsApp account, select chats or senders, persist typed messages, then notify your application's handlers. Native Go implementation of the architecture in [whatsapp-extractor](https://github.com/edududs/whatsapp-extractor), using [whatsmeow](https://github.com/tulir/whatsmeow) directly.

Go **1.27+**, pure-Go SQLite, PostgreSQL, JSON Lines and memory adapters. No Python, browser, CGO or external message broker is required. This is a library with a CLI, not a message-sending bot. Media is metadata only.

## Quick start

```sh
go install github.com/edududs/whatsapp-extractor-go/cmd/whatsapp-extractor-go@latest
whatsapp-extractor-go pair
whatsapp-extractor-go accounts
whatsapp-extractor-go groups -account 5511900000001
whatsapp-extractor-go run -account 5511900000001
```

Scan the QR code in WhatsApp → Linked devices. The default session database is `data/session.db`; messages go into `data/wa_<phone>.db`. A single paired account is selected automatically. The default empty watchlist accepts all incoming and outgoing message events.

For filtering, copy [extractor.example.toml](extractor.example.toml) to `extractor.toml`, edit it, and pass it explicitly:

```sh
whatsapp-extractor-go check -config extractor.toml
whatsapp-extractor-go run -config extractor.toml -account 5511900000001
```

```toml
store = "sql"
view = "log"
buffer_size = 10000

[watchlist]
chats = ["120363000000000001@g.us"]
senders = ["5511900000002"]
```

A chat **or** a sender match accepts a message. Phone and LID aliases are considered. `[accounts."<phone>".watchlist]` replaces the global list, including when empty. Config files are never rewritten by the program.

Session credentials and message storage can be separated:

```sh
export EXTRACTOR_MESSAGES_DATABASE='postgres://user:password@localhost/extractor?sslmode=require'
whatsapp-extractor-go run -config extractor.toml
```

Precedence: `-account` > `EXTRACTOR_*` environment > explicit TOML > defaults. Supported environment suffixes: `ACCOUNT`, `DATABASE`, `MESSAGES_DATABASE`, `STORE`, `VIEW`, `BUFFER_SIZE`, `JSONL_PATH`. `.env` files are not implicitly loaded. Relative paths resolve from the working directory.

Views: `log` emits structured metadata to stderr without message bodies or phone numbers; `json` emits complete messages to stdout; `none` disables message views. Operational errors still go to stderr. Use `version` for build information and `help` for commands.

## Architecture

```mermaid
flowchart LR
    WA[whatsmeow adapter] -->|iter.Seq2| E[application.Extract]
    W[domain.Watchlist] --> E
    E -->|1. Save| S[(SQLite / PostgreSQL / JSONL / memory)]
    E -->|2. Publish| B[ordered application.Bus]
    B --> H[application handlers]
    CLI[CLI composition root] -.-> WA
    CLI -.-> S
    CLI -.-> B
```

| Package | Responsibility |
| --- | --- |
| `domain` | Validated values, identities, content and watchlist; standard library only |
| `application` | Consumer-owned interfaces, extraction and ordered event publication |
| `adapters/whatsapp` | Pairing, account selection, event mapping, connection lifecycle and bounded buffering |
| `adapters/storage` | Concrete persistence adapters and versioned SQL migrations |
| `internal/config`, `internal/cli` | Configuration and composition; not part of the public Go API |
| `storetest` | Reusable contract tests for third-party stores |

The dependency direction is enforced by a test. The core uses `context.Context`, Go iterators, explicit errors and constructor injection. Pointer-bearing messages are cloned at storage/handler boundaries. No global service locator or reflection-based dependency injection is involved.

### Embed the extraction core

```go
package main

import (
    "context"
    "fmt"
    "iter"

    "github.com/edududs/whatsapp-extractor-go/adapters/storage"
    "github.com/edududs/whatsapp-extractor-go/application"
    "github.com/edududs/whatsapp-extractor-go/domain"
)

type SourceFunc func(context.Context) iter.Seq2[domain.Message, error]
func (f SourceFunc) Messages(ctx context.Context) iter.Seq2[domain.Message, error] {
    return f(ctx)
}

func extract(ctx context.Context, source application.Source) error {
    filter := domain.NewWatchlist([]string{"123@g.us"}, nil)
    bus := application.NewBus(nil, func(ctx context.Context, e domain.MessageExtracted) error {
        fmt.Println(e.Message.Content.Text)
        return nil
    })
    return application.Extract(ctx, source, filter.Matches, storage.NewMemory(), bus)
}

func main() {}
```

For a real source: open `whatsapp.OpenSessions(ctx, path)`, select `sessions.Client(ctx, phone)`, and call `whatsapp.NewSource(client, bufferSize, logger)`. Close the session container after extraction. `Source.Messages` owns connecting and disconnecting the client. A custom writer needs only `Save(context.Context, domain.Message) error`. An optional `Store` adds `Load(ctx, chatJID, messageID)`.

## Persistence and delivery contracts

- A write must succeed before `MessageExtracted` is published. Write, source and mapping errors terminate extraction. Handler errors are logged and isolated; handlers run in registration order. Handler panics remain fatal.
- SQL and memory upsert by `(chat_jid, id)`. Redelivery can publish again; this is **not exactly-once delivery**. Edits overwrite the same identity when upstream supplies the same ID. JSONL is append-only, including edits and duplicates, and fsyncs each write.
- PostgreSQL uses a `wa_<phone>` schema. SQLite uses a separate file by default and `wa_<phone>_*` tables, also isolating accounts when an explicit shared path is used. SQL stores JSON plus chat and Unix-millisecond timestamp projections. Migrations run transactionally; newer unknown versions are rejected.
- The live queue is bounded. Overflow terminates with an explicit error instead of continuing with silent loss. Restarting does not guarantee recovery of missed events. SIGINT/SIGTERM cancels promptly; queued messages may remain unpersisted. Use SQL polling with a consumer cursor or implement an outbox when durable downstream delivery is required.
- One active extractor per linked device. The library does not enforce a distributed lease. Transient disconnections use whatsmeow's reconnect support; permanent logout terminates extraction.

## Relationship to the Python project

Preserved: hexagonal boundaries, account context outside the domain, watchlist OR semantics and aliases, persistence-before-publication, best-effort ordered handlers, content/media metadata, ephemeral/view-once/edit flags, local contact names, SQL/JSONL/memory storage and pairing/group discovery.

Deliberate changes: native Go runtime, stdlib-only core, chat-qualified message keys, explicit overflow failure, strict TOML validation, manual watchlist configuration, JSON/structured-log views instead of Rich panels, native Go tooling and cross-platform binaries. Python session/message databases are **not automatically migrated**. Pair a new device and use fresh databases. The exported JSON is similar but not guaranteed byte-compatible (optional values can be omitted).

## Development

```sh
go test -race ./...
go vet ./...
go tool staticcheck ./...
go tool govulncheck ./...
go test ./domain -run '^$' -fuzz FuzzParseJID -fuzztime 10s
go test ./adapters/whatsapp -run '^$' -fuzz FuzzMap -fuzztime 10s
go test ./domain -run '^$' -bench . -benchmem
CGO_ENABLED=0 go build -trimpath -o bin/whatsapp-extractor-go ./cmd/whatsapp-extractor-go
```

`go.mod` pins both dependencies and development tools. `make check` runs formatting checks, tests, vet and static analysis. PostgreSQL integration tests run when `TEST_POSTGRES_DSN` points to a **disposable test database**; they create unique account schemas. CI provisions PostgreSQL and runs Linux/Windows tests, security analysis, fuzz smoke tests and portable builds. See [CONTRIBUTING.md](CONTRIBUTING.md), [architecture decisions](docs/decisions.md) and the [operations guide](docs/operations.md).

Automated tests do not connect to a real WhatsApp account. Real pairing, group discovery and live event delivery must be checked with a phone after installing. This unofficial client is not affiliated with Meta/WhatsApp and can be affected by protocol changes or account restrictions. Store session files and message exports as sensitive data.

## License

[MIT](LICENSE), matching the original project. Dependencies retain their own licenses, including whatsmeow's MPL-2.0.
