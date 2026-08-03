# Task API

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
| POST | `/tasks` | 201 | created `Task` |
| PUT | `/tasks/{id}` | 200 | updated `Task` |
| DELETE | `/tasks/{id}` | 204 | — |

`Task`: `{"id": 1, "name": "buy milk", "status": 0}` — `status` is `0` (incomplete) or `1` (completed).

Errors return `{"error": "..."}` with `400` (invalid body / missing name / status outside `[0,1]`),
`404` (unknown id), or `405` (unsupported method).

```bash
curl -X POST localhost:8080/tasks -d '{"name":"buy milk","status":0}'
curl localhost:8080/tasks
curl -X PUT localhost:8080/tasks/1 -d '{"name":"buy oat milk","status":1}'
curl -X DELETE localhost:8080/tasks/1 -i
```

## Notes

- `PUT` is a full replacement, so both `name` and `status` are required.
- Unknown JSON fields are rejected so typos (`"statu": 1`) fail loudly instead of silently defaulting.
- Storage is a mutex-guarded map; data is lost on restart, per the assignment's in-memory requirement.
- `go.mod` targets Go 1.18, so routing is a `ServeMux` prefix + method switch rather than the
  method-aware patterns added in Go 1.22.
