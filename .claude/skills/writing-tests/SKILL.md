---
name: writing-tests
description: How tests are written in this repo. Use whenever you add or change a test, and before fixing a bug (reproduce it with a test first).
---

# Writing tests

A test earns its place by failing when the behaviour breaks. Before keeping a test, ask what change to the code would make it fail; if the answer is "none" or "only a rewrite", it is not testing anything.

- **Write it first and watch it fail** for the reason you expect. A test that has never failed has never been shown to work.
- **Test behaviour through the public interface**: inputs in, outputs or visible effects out. Tests tied to private helpers and internal call order break on every refactor and catch nothing.
- **Mock only what you do not own and cannot run**: the network, the clock, paid or slow external services. Never mock the thing under test, and do not assert that a mock was called as a stand-in for checking the result. Prefer a real temporary file, a real in-memory database or a small fake.
- **One behaviour per test, named after it**: `test_rejects_expired_token`, `it("returns 404 for an unknown id")`. When it fails, the name alone should say what broke.
- **Cover the edges the task implies**: empty, missing, duplicate, boundary values, the error path. Not every combination; the ones where the behaviour changes.
- **A bug fix starts with a test that reproduces the bug**, fails, and then passes with the fix.
- **Do not edit an assertion to make it pass.** Either the code is wrong, or the requirement changed; if it changed, say so in your final message.
- **Skip the test that proves nothing**: getters, constants, the framework's own behaviour, a snapshot of a large structure nobody will read when it changes.

## Go (most of gregal)

Standard `testing` package, tests next to the code in `name_test.go`. Table-driven tests with `t.Run(tc.name, ...)` for cases, `t.TempDir()` for files, `httptest` for HTTP handlers instead of a real server. Fail with a message that says what was expected and what came instead. Use `t.Skip` only for a real platform limitation, and the diff guard will ask a human about it.

## JavaScript (internal/web, desktop, scripts)

`node --test` with `node:assert/strict`, files named `*.test.js` or `*.test.cjs` next to the code. Follow the existing suites for how they load the browser code.

## Kotlin (android)

JUnit unit tests in `app/src/test`, run with `./gradlew testDebugUnitTest`. Device tests in `app/src/androidTest` only when the behaviour needs a device.

## Python (python/)

`unittest`, in `python/tests`, like the existing client tests.
