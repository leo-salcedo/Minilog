# minilog

`minilog` is a small Go HTTP service for collecting and querying log events in memory. It is intentionally simple: standard library only, a small package layout, and a clear separation between HTTP handling, validation, and storage.

## Current Scope

- Accept log events over HTTP
- Accept either a single log object or a batch of log objects
- Store logs in memory
- Query stored logs with basic filters

Because storage is in memory, data is lost when the process stops.

## Architecture

The project is split into a few focused packages:

- `main.go`
  Starts the HTTP server, creates the in-memory store, and wires routes.
- `internal/api`
  Contains HTTP handlers, request decoding, response encoding, and query parameter parsing.
- `internal/logstore`
  Contains the in-memory store and query logic.
- `internal/model`
  Defines the `LogEvent` type and its validation rules.

Current request flow:

1. A request reaches the HTTP handler in `internal/api`.
2. The handler decodes and validates input.
3. The handler calls the storage interface in `internal/logstore`.
4. The handler returns JSON responses to the client.

This keeps storage concerns out of the HTTP layer and makes behavior easier to test.

## Data Model

Each log event currently has this shape:

```json
{
  "timestamp": "10:00",
  "service": "api",
  "level": "info",
  "message": "request completed",
  "attributes": {
    "request_id": "123"
  }
}
```

Validation rules today:

- `timestamp` is required and must be in `HH:MM` format
- `service` is required
- `level` is required and must be one of `debug`, `info`, `warn`, or `error`
- `message` is required
- `attributes`, when present, must contain non-empty keys and values

## HTTP API

### `GET /`

Returns a plain text status message:

```text
minilog running
```

### `POST /logs`

Accepts either:

- A single log event object
- An array of log event objects

Single event example:

```bash
curl -i -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '{"timestamp":"10:00","service":"api","level":"info","message":"ok"}'
```

Batch example:

```bash
curl -i -X POST http://localhost:8080/logs \
  -H "Content-Type: application/json" \
  -d '[
    {"timestamp":"10:00","service":"api","level":"info","message":"one"},
    {"timestamp":"10:01","service":"worker","level":"warn","message":"two"}
  ]'
```

Typical success response:

```json
{
  "accepted": 1,
  "rejected": 0
}
```

Batch requests can partially succeed. In that case the response includes accepted and rejected counts plus per-item errors.

### `GET /logs`

Returns stored logs as JSON:

```bash
curl -i http://localhost:8080/logs
```

Example response:

```json
{
  "count": 2,
  "logs": [
    {
      "timestamp": "10:00",
      "service": "api",
      "level": "info",
      "message": "one"
    },
    {
      "timestamp": "10:01",
      "service": "worker",
      "level": "warn",
      "message": "two"
    }
  ]
}
```

Supported query parameters:

- `level`
- `service`
- `contains`
- `limit`

Example:

```bash
curl -i "http://localhost:8080/logs?level=info&service=api&contains=request&limit=10"
```

Notes on filtering:

- `level` matching is case-insensitive
- `service` matching is exact after trimming surrounding whitespace
- `contains` is a case-sensitive substring match on `message`
- `limit` must be a positive integer

## How To Run

Requirements:

- Go `1.26.1` or newer

Start the server:

```bash
go run .
```

The server listens on `:8080`.

## How To Test

Run the test suite with:

```bash
go test ./...
```

## Current Behavior And Limitations

- Storage is in memory only
- There is no persistence layer
- There is no authentication
- There is no configuration system yet
- The root route is a simple text response
- The server currently binds directly to port `8080`

## Project Direction

This README is a starting point. As the project evolves, we can expand it with:

- API reference details
- configuration and environment variables
- persistence options
- deployment notes
- example clients
