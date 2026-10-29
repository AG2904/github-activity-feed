# Go best practices for the client/ CLI — research findings

2026-10-29 · Scope: `client/` Go CLI (`cmd/feed`, `internal/{cli,activity,render,sse}`, `test/`).
All claims mapped to primary sources with file:line evidence. Baseline verified by the parent
session: `npm run cli:lint` (go vet) exit 0, `npm run cli:test` green (client/test 37.2s), and
`gofmt -l` reports no unformatted files.

## Sources
1. Effective Go — https://go.dev/doc/effective_go (Formatting, Commentary, Defer, Errors)
2. Go Code Review Comments — https://go.dev/wiki/CodeReviewComments (Contexts, Error Strings,
   Handle Errors, Indent Error Flow, Named Result Parameters, Useful Test Failures, Doc Comments,
   Don't Panic, Declaring Empty Slices)
3. Uber Go Style Guide — https://github.com/uber-go/guide (rendered style.md: Exit in Main /
   Exit Once, Error Types, Error Wrapping, Defer to Clean Up, Test Tables, Prefer Specifying
   Container Capacity, Linting)
4. Google Go Style Guide — https://google.github.io/styleguide/go/ and /guide (reachable;
   Clarity, Simplicity, Concision, Maintainability, Consistency, Formatting, Least mechanism)
5. Go Proverbs — https://go-proverbs.github.io/ ("Make the zero value useful", "A little copying
   is better than a little dependency", "Syscall must always be guarded with build tags",
   "Don't panic", "Clear is better than clever")
6. Native Go fuzzing tutorial — https://go.dev/doc/tutorial/fuzz
7. Stdlib patterns observed in the code itself: context first parameter, http.NewRequestWithContext,
   http.Client.CheckRedirect, errors.Is/As + Unwrap, httptest, t.Helper/t.Cleanup, atomic.Bool,
   math/rand/v2, slices, strings.Cut.

## Already followed

- **Context first parameter, threaded to every I/O call** (CodeReviewComments "Contexts"):
  main.go:25,31 → cli.go:22,152 → watch.go:25,133 → stored.go:22, backfill.go:18; every request
  uses `http.NewRequestWithContext` (cli.go:152, watch.go:133). Cancellation is checked before
  error handling (watch.go:99-101, history.go:37-39).
- **Exit once, in main** (Uber "Exit in Main"/"Exit Once", which explicitly blesses
  `os.Exit(run(args))` returning an int): main.go:31-33 is the only `os.Exit`; `cli.Run` returns
  the code (cli.go:22). Nothing inside calls `os.Exit` or `panic`.
- **defer for cleanup, close next to open** (Effective Go "Defer"; Uber "Defer to Clean Up"):
  watch.go:141,151; stored.go:27; watch.go:56 (deferred session summary).
- **errors.Is / errors.As, custom error type with Unwrap, %w wrapping** (Uber "Error Types",
  "Error Wrapping"): retry.go:19-22 (`fatalError` + Unwrap), watch.go:106-107 (errors.As),
  cli.go:29,90 (flag.ErrHelp), sse.go:61 (io.EOF), activity.go:85,93 (%w), harness_test.go:265-267.
- **Error strings lowercase, no trailing punctuation** (CodeReviewComments "Error Strings"):
  activity.go:88, cli.go:162, watch.go:112,159.
- **Zero value useful** (proverb): backoff zero starts at 1s (retry.go:36-40); empty Filter passes
  everything (activity.go:55-67); `Palette{Color:false}` emits no escape (render.go:159-164).
- **Syscall guarded by build tags** (proverb): terminal_windows.go:1 (`//go:build windows`) vs
  terminal_other.go:1.
- **Zero third-party dependencies** (proverb "A little copying…"; ADR-0008): go.mod:1-3 has no
  `require`; the test suite is stdlib-only (`testing`, `net/http/httptest`, `os/exec`).
- **Doc comments** on packages and declarations (CodeReviewComments "Doc Comments"): cli.go:1-3,
  sse.go:1-7, activity.go:1-3, render.go:1-4, main.go:1-5, retry.go:17-18, etc.
- **Preallocated slice capacity** (Uber "Prefer Specifying Container Capacity"): emitter.go:63,
  harness_test.go:533, e2e_test.go:940.
- **Named results as documentation, explicit returns** (CodeReviewComments "Named Result
  Parameters"): emitter.go:51 `(event, valid, shown)`, watch.go:132, backfill.go:18; no naked
  returns observed.
- **Indent error flow / early returns** (CodeReviewComments): history.go:29-42, watch.go:99-113.
- **Test seam at the process boundary with strong helpers**: t.Helper (harness_test.go:41,197,209…),
  t.Cleanup (harness_test.go:101,312), httptest fake relay (harness_test.go:50), mutex + snapshot
  reads (harness_test.go:110-122), raw-TCP relay for status-line bytes (security_test.go:154-184),
  binaries built with the production -ldflags path (main_test.go:31-34).
- **Injected writers and env-via-flag-default** instead of mutable globals (Uber "Avoid Mutable
  Globals"): cli.go:22,74-79,106-115.
- **Explicit redirect policy on the HTTP client** with a why-comment: cli.go:121-129.

## Recommendations

1. **Table-driven unit tests beside the internal packages** — effort M.
   Source: Uber "Test Tables" (give/want, `tests`/`tt` naming) —
   https://github.com/uber-go/guide/blob/master/style.md#test-tables;
   CodeReviewComments "Useful Test Failures" — https://go.dev/wiki/CodeReviewComments#useful-test-failures;
   Google guide links TableDrivenTests under Concision — https://google.github.io/styleguide/go/guide#concision.
   Why: `client/internal/**` has zero `_test.go` files; every assertion today builds and drives two
   binaries (test/main_test.go:31-34) and docs/cli-flow.md:383 puts `go test ./...` at ~2 minutes.
   The process seam protects the wire contract (docs/cli-flow.md:252-253); it does not require pure
   functions to be untestable in-process.
   Where: sse.Reader.Next (sse.go:30-65), activity.Parse (activity.go:72-107), Filter.Match
   (activity.go:55-67), render.clean (render.go:144-154), render.relative (render.go:176-190),
   backoff.delay (retry.go:36-53), printable (cli.go:136-146).
   Sketch:
   ```go
   func TestParse(t *testing.T) {
       tests := []struct{ name, give string; wantOK bool }{
           {"missing type", `{"actor":"a","subject":"s","url":"u","when":"2026-10-11T13:00:00Z"}`, false},
           {"valid", rawPayload, true},
       }
       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               _, err := activity.Parse([]byte(tt.give))
               if (err == nil) != tt.wantOK { t.Fatalf("err = %v, wantOK %v", err, tt.wantOK) }
           })
       }
   }
   ```
2. **Fuzz the two wire parsers with native fuzzing** — effort S.
   Source: https://go.dev/doc/tutorial/fuzz (stdlib, no dependency; corpus entries run on every
   plain `go test`).
   Why: both parse relay-controlled bytes; security_test.go:23-27 pins two hostile payloads by hand,
   and a fuzz target searches the rest of the space for panics/invariant breaks. Respects ADR-0008.
   Where: `FuzzNext` over sse.Reader.Next (sse.go:30), seeds from the junk frames at
   e2e_test.go:407-412; `FuzzParse` over activity.Parse (activity.go:72), seeds from rawPayload /
   historyBody (e2e_test.go:1287,1291) and hostilePayload (security_test.go:23). Property: no
   panic; Parse either errors or returns an Event whose `When` parsed.
3. **Share the "%s answered %s" refusal between fetch and stream** — effort S, low priority.
   Source: proverb "Clear is better than clever"; Google guide Simplicity.
   Why: cli.go:160-163 and watch.go:143-145 are the same rule and the same wording, and
   security_test.go:101 asserts that wording — two copies can drift away from the pinned contract.
   Caveat: a naive merge into `fetch` would make an unusable URL retryable and break the exit-1
   behavior pinned at watch.go:134-136 and e2e_test.go:532-541.
   Sketch: keep request creation where it is; extract only the message, e.g.
   `func refused(url, status string) error { return fmt.Errorf("%s answered %s", url, printable(status)) }`.
4. **One registration point for the shared `--user`/`--repo` flags** — effort S.
   Source: Google guide Consistency — https://google.github.io/styleguide/go/guide#consistency.
   Why: watch.go:28-29 and history.go:20-21 duplicate definitions and help text; the repo's own
   invariant is that the two commands cannot answer different things about the same person
   (docs/cli-flow.md:145-147), so the duplicated text is contract that can silently diverge.
   Sketch: `addFilters(fs *flag.FlagSet) (user, repo *string)` in cli.go, called by both commands.
5. **Fix the jitter clipping at the 30s ceiling** — effort S.
   Source: grounded in the code and its own comment; no style-guide citation claimed.
   Why: retry.go:42-45 computes `step + U[0, step/4)` and then caps at 30s; when
   `step == maxBackoff` the sum always exceeds 30s, so every ceiling delay is exactly 30s and the
   jitter the comment at retry.go:26-27 promises does nothing at the very step where reconnect
   lockstep matters most.
   Sketch: cap first, then jitter, e.g. at the ceiling sample in
   `[3/4·maxBackoff, maxBackoff)`; the existing assertions (e2e_test.go:658-669) only cover the
   first three steps and stay green.
6. **Run the suite with -race in CI** — effort S.
   Source: stdlib `go test -race`; article https://go.dev/doc/articles/race_detector (URL not
   fetched this session — verify before citing downstream).
   Why: the harness is genuinely concurrent — mutex-guarded fakeRelay with snapshot reads
   (harness_test.go:110-122), per-connection goroutines in rawRelay (security_test.go:163-181),
   syncBuffer (harness_test.go:513-528). Honest nuance: `go test -race` instruments the test
   process only; the CLI binaries built at main_test.go:47 are separate processes, so racing the
   CLI's own goroutines (idleReader timer vs Read) would need a -race-built test binary.
7. **Expand static analysis and add a CI job for the client** — effort M.
   Source: Uber "Linting" (minimum set errcheck, goimports, revive, govet, staticcheck; recommends
   golangci-lint as runner) — https://github.com/uber-go/guide/blob/master/style.md#linting;
   Google guide Formatting (gofmt enforced) — https://google.github.io/styleguide/go/guide#formatting.
   Why: today the only check is `go vet` (package.json:24) and docs/cli-flow.md:339-340 admits no
   CI compiles the client.
   Caveats: (a) ADR-0008 governs the built binary — a linter is dev tooling and adds no go.mod
   require; (b) errcheck will flag the deliberate ignored stderr narration writes (watch.go:43-50,
   78, 91-96, 108, 116) — exclusion config with a comment, not a behavior change; (c) run from
   `client/` (nested module, ADR-0009).

## Suggested order of application
1 → 2 (unit tests first; fuzz seeds reuse them) → 5 (small, independent correctness fix) →
4 → 3 (consistency/DRY cleanups) → 6 (-race, worth it once unit tests exercise in-process
concurrency) → 7 (golangci-lint + CI job last, institutionalizing the rest).

## Uncertainty
- Baseline verified after research: `npm run cli:lint` (go vet) exit 0; `npm run cli:test` green
  (client/test ok, 37.247s); `gofmt -l` in `client/` reports zero unformatted files.
- The race-detector article URL above was not fetched this session.
- In-package (`package cli`) vs external (`package cli_test`) unit-test placement is a repo-policy
  call; in-package reaches unexported symbols like `backoff` (retry.go:28).
