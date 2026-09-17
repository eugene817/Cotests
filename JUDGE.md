# Judge placeholder and professor-led design

Updated: 2026-09-08. This decision supersedes the earlier engine/worker proposals.

## Implemented now

[internal/judge/judge.go](internal/judge/judge.go) contains only:

```go
type Judge interface {
    RequestEvaluation(ctx context.Context, submissionID uint) error
}
```

`Noop` implements this interface and always returns `ErrUnavailable`. It has no dependencies, state, side effects or background activity. It does not inspect the source, touch the database, create jobs, run programs, call a service or produce a result.

The interface is a provisional application boundary, not the professor's final judge API. It refers only to a saved submission ID and context. It intentionally defines no execution transport, result schema, language or scoring contract.

There is no submission endpoint/service yet, so the stub is not wired into an HTTP route. It can be injected when source persistence is implemented. No automatic application startup evaluation or dependency on judge infrastructure is added.

## Behavior when submissions are implemented

Persist source through the web application first. Handle `errors.Is(err, judge.ErrUnavailable)` as “Saved — not judged”, retaining the saved submission.

Do not represent the stub as queued, running, Accepted, Wrong Answer, Judge Error or a zero score. “Unavailable” describes the missing service; “Judge Error” would imply a real attempted evaluation failed.

Leave verdict, points, runtime and memory absent. Do not schedule retries, start polling, create mock results, accept arbitrary result callbacks, or enable rejudging. Existing source is not automatically evaluated after a future engine installation; backfill rules need a deliberate design.

## Decisions for the professor discussion

- Responsibility split between application, judge and any external system.
- Execution safety, deployment environment and required languages/toolchains.
- Synchronous/asynchronous request semantics and request/result identifiers.
- Result schema, per-test feedback, runtime/memory units and partial results.
- Checker interfaces, legacy compatibility, custom configuration and plugin handling.
- Scoring rules, quiz/manual/external grading, ranking ties and result visibility.
- Persistence, failure recovery, cancellation, duplicate result handling and rejudging.
- Validation fixtures and criteria for enabling real grading in production.

No technology, worker protocol or scoring model is selected by this document. Earlier Isolate/Linux-worker, queue-lease and language-runner suggestions are historical discussion material only; implementing them is deferred.

## Completion boundaries

**Current branch:** interface and empty implementation only, plus plans and minor application fixes.

**Web MVP:** authoring, submission collection/history and an explicitly unavailable judge.

**Full replacement:** remaining web parity plus the jointly designed, implemented and verified judge. Checker/quiz/ranking/rejudge rows in the parity matrix cannot be marked complete just because the placeholder exists.

