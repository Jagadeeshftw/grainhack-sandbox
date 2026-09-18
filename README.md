# grainhack-sandbox

A small, self-contained job worker used for a live GrainHack test event on
Grainlify. It is not a product; it exists so two real bugs can be fixed in
two real pull requests.

```
printf 'a\nb\nc\n' | go run ./cmd/sandbox-worker --concurrency 2
```

The worker reads jobs from stdin, one per line, and processes each by
calling a downstream service. The downstream is unavailable for the first
300ms after start (`--warmup`), the way a dependency is while it restarts,
so a job that arrives early has to be retried.

## The two open issues

- **#1 — Fix the flaky retry loop in the sandbox worker.** `worker/retry.go`.
  Run the command above: every job gives up after five attempts, even though
  the downstream would have answered a few hundred milliseconds later.
- **#2 — Validate `--concurrency` instead of silently starting zero workers.**
  `cmd/sandbox-worker/main.go`. Run it with `--concurrency 0`: it logs
  `started 0 workers`, processes nothing, and exits 0 when interrupted.

## Tests

```
go test ./...
```

**This fails on `main`, on purpose.** Each failing test describes one of the
issues above and passes once that issue is fixed. There are no other tests,
so a green run means both are fixed.
