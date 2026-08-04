# Task API

[![CI](https://github.com/SIMPLYBOYS/gogolook-task-api/actions/workflows/ci.yml/badge.svg)](https://github.com/SIMPLYBOYS/gogolook-task-api/actions/workflows/ci.yml)

RESTful task API in Go — standard library only, in-memory storage.

## Run

```bash
go run .          # listens on :8080, override with PORT=3000
go test -race ./...
```

## Docker

```bash
docker build -t task-api .
docker run --rm -p 8080:8080 task-api
```

## Endpoints

| Method | Path | Success | Body |
|---|---|---|---|
| GET | `/tasks` | 200 | `[]Task` |
| GET | `/tasks/{id}` | 200 | one `Task` |
| POST | `/tasks` | 201 | created `Task` |
| PUT | `/tasks/{id}` | 200 | updated `Task` |
| DELETE | `/tasks/{id}` | 204 | — |
| GET | `/healthz` | 200 | `ok` |

`Task`: `{"id": 1, "name": "buy milk", "status": 0}` — `status` is `0` (incomplete) or `1` (completed).

`POST` and `PUT` both require `name` and `status`; omitting either is a `400` rather than a
silent default. An `id` in the body is accepted and ignored, so a task read back from
`GET /tasks/{id}` can be `PUT` unchanged.

Errors return `{"error": "..."}` with `400` (invalid body / missing field / status outside
`[0,1]`), `404` (unknown id or unrouted path), or `405` (unsupported method).

```bash
curl -X POST localhost:8080/tasks -d '{"name":"buy milk","status":0}'
curl localhost:8080/tasks
curl -X PUT localhost:8080/tasks/1 -d '{"name":"buy oat milk","status":1}'
curl -X DELETE localhost:8080/tasks/1 -i
```

## Benchmarks

`go test -run '^$' -bench . -benchmem -cpu 1,4` — numbers below from an i7-6700HQ, darwin/amd64:

| Benchmark | 1 core | 4 cores |
|---|---|---|
| `StoreList/tasks=100` | 22 µs/op | |
| `StoreList/tasks=1000` | 234 µs/op | |
| `StoreList/tasks=10000` | 3.75 ms/op | |
| `StoreListParallel` (1000 tasks) | 228 µs/op | 55 µs/op |
| `StoreUpdateParallel` | 65 ns/op | 128 ns/op |

- **Reads scale, writes do not.** Parallel reads speed up 4.1× on 4 cores, which is what
  `RWMutex` buys over a plain `Mutex`. Concurrent updates get *slower* per op (65 → 128 ns)
  as cores contend for the exclusive lock — that is the known ceiling of one global lock.
  Aggregate write throughput is still ~8M ops/s, far past anything the HTTP layer will
  deliver, so sharding the lock would be optimising the wrong thing today.
- **`List` is the real ceiling.** It sorts on every call, so cost grows with the dataset:
  3.75 ms at 10k tasks caps `GET /tasks` around 270 req/s on one core. The fix is an
  ordered index (or a DB with an index on `id`), not a faster lock.

## Notes

- `PUT` is a full replacement, so both `name` and `status` are required.
- Unknown JSON fields are rejected so typos (`"statu": 1`) fail loudly instead of silently
  defaulting. Missing fields are rejected for the same reason — `status` is decoded through a
  pointer because `0` is a valid state and cannot double as "absent".
- The server sets read/write/idle timeouts; Go's defaults are none, which leaves a connection
  that never completes its request holding a goroutine indefinitely.
- Storage is a mutex-guarded map; data is lost on restart, per the assignment's in-memory requirement.
- `SIGINT`/`SIGTERM` stops accepting connections and drains in-flight requests (10s cap), so
  `docker stop` or a rolling update never cuts a response mid-write.
- `go.mod` targets Go 1.18, so routing is a `ServeMux` prefix + method switch rather than the
  method-aware patterns added in Go 1.22.
