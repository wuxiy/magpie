# Code standards

These rules come from what PR reviews keep asking for. A reviewer checks
each one, and a PR that doesn't meet one gets a comment saying which. How to
describe and review a change to a subsystem is in the
[subsystem references](subsystems/README.md).

## What a change contains

| Rule | What a reviewer looks at |
| --- | --- |
| A change does only what its goal needs. | Every change in behavior has a reason. Unrelated refactors, extra features and policy changes go in a PR of their own. |
| A fix covers exactly the cases it means to. | Which cases change, and which neighboring ones stay the same. Different errors are not handled as one. |
| A change holds on the whole path. | Follow it from the user's input to the end result, through the callers, the state it changes and the side effects. One correct function doesn't make the whole operation correct. For example, an agent's model picks are saved under one id and read under another (#926). An update button doesn't update the binary that actually runs (#930). |
| A fix reaches every sibling of the bug. | Grep for the same pattern: every agent, subcommand, guard of the same shape and caller of a changed predicate gets the fix in the same change. On 2026-10-05 six fixes needed a second release because they covered only the case in the report (see [LESSONS.md](../LESSONS.md)). |
| Checks cover the edges the change touches. | For numbers, check the boundaries. For paths, check each platform (macOS, Linux, Windows). For concurrency, check races and stale state. Pick checks by risk. |

## Tests

| Rule | What a reviewer looks at |
| --- | --- |
| A fix comes with a test that fails without it. | The reviewer puts the old code back and runs the test, and the test must fail. A PR description says what it failed with. |
| Tests use real inputs. | Fixtures look like real requests, real serialized output and real files on disk. That includes missing fields, empty values and defaults. |
| Tests check behavior, not implementation. | Assert what the user or caller needs. Formatting changes and internal refactors shouldn't break a test, unless exact bytes are the requirement. |
| Tests stay out of the real machine. | Run them under a temp HOME with Go's caches pinned (see the snippet in [provider-plugins.md](subsystems/provider-plugins.md)). Never touch a real agent's config or `~/.config/magpie`. An agent's variable goes in `agentenv.Vars`, so the sandbox clears it. |

## Acceptance criteria

A change is merged or released only when it shows each of the following.
Say which ones were run and what they printed. A check that wasn't run is
listed as not run, never left out.

1. **A test that fails without the change.** Put the old code back and run
   the new test. It must fail with a real `FAIL` that names the behavior. A
   compile error or a skipped test doesn't count. Say which test fails and
   what it failed with. Then restore the change and check the test passes.
2. **Tests in a sandbox.** Run Go tests under a temporary HOME with Go's
   caches pinned: `GOPATH`, `GOMODCACHE` and `GOCACHE` taken from `go env`
   before HOME is swapped. Remove the HOME afterwards with
   `chmod -R u+w "$h"; rm -rf "$h"`, because the module cache in it is
   read-only. The snippet is in [provider-plugins.md](subsystems/provider-plugins.md#verification).
   Without the pinned caches, every run downloads 1–2 GB into a new temp
   HOME. A test never reads or writes a live agent config (`~/.codex`,
   `~/.claude`, `~/.claude.json`, `~/.gemini` and the rest) or
   `~/.config/magpie`. Packages set this up in their `TestMain`, most with `testenv`.
3. **The build and the suite.** Run `go vet ./...`, `go build`,
   `GOOS=linux go build -tags nogui`, `GOOS=windows go build` and
   `go test -tags nogui ./...`. In a chain of commands, set
   `set -o pipefail`, so that `go test | tail && git push` stops on a
   failure.
4. **GUI tests in both engines.** A change to `internal/gui/assets` runs the
   Playwright tests it touches in Chromium and WebKit (`make test-ui`, or
   `BROWSER=webkit node --test …`). The suite must also pass in Chinese,
   English, and `gui-ja`/`gui-de` (every string has its Japanese and German,
   with the same placeholders). CI doesn't run this suite, so it is run
   locally.
5. **Real use where possible.** A change to a provider, subscription or
   agent is also checked against the real thing: a real account or key, the
   agent's real config format, the vendor's real reply. Do this in a sandbox
   HOME with copies of the credentials. Never touch the live gateway or a
   running magpie. If a real check isn't possible, say so.
6. **The UI rules** under [GUI](#gui) hold: `t()` strings, the app's own
   menu instead of `<select>`, no scroll on click, no colored left-border
   stripes.

A PR is merged only after a maintainer has pulled it, merged it onto
current `main` and run the checks above on the result. The merge names the
commit that was reviewed: `gh pr merge --match-head-commit <reviewed sha>`.
A push made during the review then stops the merge, rather than shipping
code nobody ran. A Draft PR can't be merged until it is marked Ready for
review.

A release counts only when all of these hold:

- A CI Test run on the tagged commit, or a later one containing it, finished
  green. A run cancelled by the next push is not a pass.
- The tag `vX.Y.Z` is on `main`, an ancestor of `origin/main`.
- The release in `yetone/magpie-releases` has all 16 assets.
- That release is marked Latest, the highest version.

Check `git tag` for the next free version first, because releases can be
made from more than one place. A PR that only adds tests doesn't get a
release of its own.

## Reviewing a PR

These patterns come from recent reviews. A reviewer checks each one that
applies:

- **Run it.** Pull the head, merge it onto current `main` (resolving
  conflicts if needed) and run the acceptance checks. Reading the diff alone
  isn't a review.
- **Break it on purpose.** Revert the fix, or break the code it guards in
  several ways, and check that the new tests catch each break (#901 broke
  `backup.go` four ways). A reviewer may add review tests of their own
  (#829–#831).
- **Check the vendor, not the PR's account of it.** Read the upstream source
  for a vendor's behavior (#870 read Codex's
  `uses_openai_actor_authorization`). Try the change against a real upstream:
  #853 found that most Chat upstreams don't continue a trailing assistant
  message.
- **Check each platform and architecture.** An `int` conversion that is safe
  on arm64 can wrap on `GOARCH=amd64` (#877). Windows reports a refused
  connection as `WSAECONNREFUSED`, which `syscall.ECONNREFUSED` doesn't
  match (#805).
- **Follow the whole path.** Check that a pick is saved and read under the
  same id (#926). Check that a button works on the thing that actually runs
  (#930). Check that aliases and User-Agents don't collide with another
  agent's (#926). Every name in `agentenv.Vars` is treated as a folder path
  unless it is in `agentenv.NotPaths` (#922).
- **Classify narrowly.** A mark or error class covers only the case it
  describes: #873 counted every failure on a turn as "turned away".
- **A preview writes nothing.** A dry run, plan or preview has no side
  effects (#827).
- **Don't reverse deliberate decisions.** The scroll guard, the button
  cursor and other product decisions are the maintainer's to change (#820,
  #921). A PR that changes one is left for the maintainer.
- **GUI suites.** Compare pass counts with `main`. A test that fails on
  `main` too is not a baseline: fix it, or open an issue naming its cause,
  in a commit of its own. "Passes alone" is not a pass; run a Go test with
  `-race -count=20` before calling it a flake (a45b3e09 found a data race
  that was waved through for 19 hours).
- **Docs match the source.** A reference is checked against the code it
  links, and every link must resolve (#889).
- **Keep the PR to its goal.** Unrelated tests and refactors go in a PR of
  their own. Comments name functions, not line numbers. Fixture names match
  what they stand for (#901).

## Go

- Write `any`, not `interface{}`.
- Read environment variables that name a folder with `appdir.Getenv`, which ignores a relative path. A variable in `agentenv.Vars` that names no folder goes in `agentenv.NotPaths`.
- A comment says what the code is for, in a sentence or two. Name functions instead of giving line numbers, which go stale.

## GUI

- Every string the user sees goes through `t()` and gets its translations in `i18n.js`. Tests run in Chinese and English.
- Use no native `<select>`. Dropdowns use the app's own menu (`openProtoMenu`).
- A click never scrolls the page (`scrollOnPurpose`).
- Don't use colored left-border stripes. Mark state with a dot or a swatch.

## Commits

A commit message names the areas it changes, then says what the user now
sees, written as plain behavior: `gateway: a 429 that says the balance is
spent is out of credit, not a rate limit`. Give the issue number or the
reporter. The body says how it was verified, and which test fails without
the change.
