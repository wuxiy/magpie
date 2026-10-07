<!-- reviewed-through: 34bfb9ca (2026-10-06 03:48 +0800) -->
# Lessons from merged work

magpie's code is written, reviewed, merged and released by agents. Each night
the day's commits are reviewed for what had to be fixed again, what shouldn't
have merged and what was verified too thinly. What we learned is kept here.
Read this before changing code. Each lesson is a rule, then why, then the
evidence. "Seen" counts the days a review found it. A lesson seen on 3 or
more separate days moves into [docs/code-standards.md](docs/code-standards.md).

The day of 2026-10-05 had 220 commits and ~170 releases. About 40 of them
fixed something released earlier the same day or the day before. Most
fixes didn't need a second try. The ones that did fall into the patterns below.

## Reading the report

**Reproduce the reporter's exact case, on the surface they use, before you
call it fixed.** Use their provider, plan, agent and layout, and their GUI, TUI
or CLI. Check their goal (it drags, the title shows, the reason is readable),
not a step on the way to it.
- #834 came back 4 times (v0.1.920 → .925 → .931 → .938). Each round fixed
  only the agent in the screenshot. #792, #743, #791 and #933 were closed and
  reopened within hours.
- 4788cf2b put gnayiab's Cursor list error in the GUI. The reporter uses the
  TUI in WSL. Fixed in f3c05b5e.
- 60b910b7 made a warm /api/state faster. The report was the cold start
  (~8.5s of SyncCatalog). Fixed in 2a4042a9.
- Seen 1× (2026-10-05).

**Build fixtures from the reporter's literal bytes, not a similar sample you
made up.**
- #823: 878a8489 "proved" the Trae case with an invented `<tool_call>{json}`.
  The screenshot had no JSON. The reporter came back (c353f7bd), and the
  parser has grown 2 more formats since.
- 3f76105b's test used a short check-in reason. The real one was ellipsized
  (#821, eb5edced).
- 26706c33 used a different skills layout from #791's.
- For model-output parsers, collect several real failing turns first.
  Truncated and empty bodies go in the tests.
- Seen 1× (2026-10-05).

**When the vendor refuses magpie but serves its own client on the same
account, diff every header of the failing request against the official
client's. Do that before adding retries or fallbacks.**
- #256 Copilot Auto took four releases (719a6820, 026158b0, 251ef218, then
  4660fd69). Each added another fallback. The cause was a missing
  X-GitHub-Api-Version beside Copilot-Session-Token.
- Seen 1× (2026-10-05).

**Decide whether a refusal is about the account, the model or the request
content before you rest an account.**
- 5353ed54 counted WorkBuddy's "unapproved channel" (an agent's system
  prompt) as an account fault. It rested every WorkBuddy AI account for 5
  days (2033df0f).
- Test that the same request fails the same way on a second account.
- Seen 1× (2026-10-05).

**Fix what the error is about, not only how it is worded.**
- e63fbb0e (kkgg on Discord, "GLM 直接429了"): Zhipu answered a GLM Coding
  Plan key sent to its pay-as-you-go endpoint with 429 1113 "余额不足". The
  commit relabelled the 429 as out of credit, and nothing in magpie tells the
  user the key belongs on the Coding Plan endpoint, which is what would make
  their requests work.
- When a report shows a wrong label, also ask why the request failed. Fix
  that, or point the user to the fix in the message itself.
- Seen 1× (2026-10-06).

## Fix the class, not the sample

**When the bug is in shared logic or a repeated pattern, grep every sibling
and fix them in one change.** Siblings are every agent, every subcommand, every
guard of the same shape, every caller of a predicate.
- 6fb05259 fixed #835 for OpenHanako only. 25 agents had it. Fixed in
  165fee0d 14 minutes later.
- 31626c15 fixed the WSL LAN key for omp only. OpenCode and 14 others were
  fixed in e77a3737.
- 37ec7259 lifted the forget guard for one case. 30d952cf followed 26
  minutes later, and Codex's copy of the guard lived until 43a65950, about 50
  releases later.
- 2f135ed4 gave short plugin names to `plugin options` only. on, off and rm
  failed until db812921.
- ZCode's NAS-address fix (63f83bf1, 5daba301) was needed again in WorkBuddy.
- bf9bd9c9 (#898) changed `Running()` on Windows, and Cline's if/else chain
  took the wrong branch (3fad3d21).
- #933: 7a926dd3 signed the ChatGPT API subscription in but listed only the
  account catalog's models; the plan's Codex models followed an hour later
  (b0607ef5).
- Seen 2× (2026-10-05, six times; 2026-10-06).

**Scope a vendor rule to that vendor.**
- e950eb08 capped every model at 272K over one DeepSeek report. Claude [1m]
  models were capped too (531dc0aa).
- 71d2da1f priced Anthropic's 1-hour cache write onto OpenAI models
  (3a30ac87).
- Check the claim about the vendor's behaviour against its source or docs.
  Add a test that another vendor's model is unchanged.
- Seen 1× (2026-10-05).

**When you change what a formula or a fetched list means, re-derive
everything built on it.**
- 12cba74a changed Zed's max_tokens but kept the old clamp, which cut replies
  (69a7744a).
- d1c19131 added facts only to lists fetched afterwards. Lists saved on
  disk stayed empty (37231626).
- 10-06: e0bf3e61 changed the names the catalog gives an unlisted model.
  internal/agent's Hanako tests are built on those names and went red
  (c0c3b20b). 2a990175 (#971) made Usage and Providers ask a new
  /api/upstream. menu-scroll.test.cjs fails on any API it doesn't serve, and
  it failed 24 of 24 on main until b2b72ea0. A new endpoint the GUI calls
  means running the whole GUI suite, not only the touched page's tests.
- Seen 2× (2026-10-05, 2026-10-06).

**A restriction covers every route that reaches the thing it guards.**
- 46f03154 (#882) held keys to some models. count_tokens and System One's
  decision model weren't covered until d7a8ddc8.
- Seen 1× (2026-10-05).

## The user's own files and settings

**Never let magpie's default override a value the user set themselves.**
- CLAUDE_CODE_AUTO_COMPACT_WINDOW overrode the user's own autoCompactWindow
  for ~150 releases (602d4ec6).
- `magpie claude model default` deleted the user's own ANTHROPIC_BASE_URL and
  dropped the stash for 12 days (fe2603ff).
- Before writing a key, list everywhere the agent reads that setting from.
  Test with user-owned values present.
- Seen 1× (2026-10-05).

**A failed read, parse or hash means "unknown". It never means "empty" or
"equal".**
- relink's `fresh()` treated two unreadable folders as equal and deleted the
  copy (9fb9a8ab).
- readLogins returned an empty list on a parse error, which the next add or
  sign-out wrote back as "no accounts" (d5619d5a only retried for 15ms).
  Fixed in b363327d: the accounts read before are kept, and a file that
  doesn't parse is copied aside before it is written over.
- Seen 1× (2026-10-05).

**Before deleting or overwriting a live credential, find every record that
could match the same key.**
- 43a65950 matched Codex accounts by display name. Two Team seats of one
  email share it, so removing one deleted auth.json for both (c13f3bfb). The
  sign-out rule was then rewritten again (4c35c1b8).
- Seen 1× (2026-10-05).

**When you hand users a writable copy, test "user edits it, then sync".**
- 3c7ceb68's "Give skills as Copies" undid the user's own edits on the next
  start (20bdf1e9).
- Seen 1× (2026-10-05).

**Take another app's field types from that app's own data or validator.**
- 84554e57 wrote updatedAt as milliseconds, without trying the real Claude
  Desktop. Its skills page hung for 2 days (#863, 50d6c9a3).
- For layered vendor configs, read the vendor's merge code first. 9761248d
  guessed dsh's merge rules (#838).
- 10-06: #966's review asked that Codex's limit_reached count as "held",
  without reading the app. 38 minutes later #996 read ChatGPT.app's app.asar.
  Its composer only stops on rate_limit.allowed === false, so #996 undid part
  of #966. #996 and e2f1c6ba are how to do it: e2f1c6ba sealed its fixture
  the way OpenHanako 1.0 seals provider-catalog.json and was checked on a
  copy of the owner's real ~/.hanako.
- Seen 2× (2026-10-05, 2026-10-06).

## Verification

**"Not tried with the real thing" means don't build more on it, and don't
tell users it works.**
- Cursor Private Inference shipped four times without the real app. The
  real client's key and User-Agent never reached the new path (387afd07).
- Add to PATH was "not tried on Windows" and failed for the site's
  magpie-windows-amd64.exe for 8.5h (90b1ac79). The Windows box was there.
- The Trae CN check-in shipped on three guesses (#808, #821). Design the
  experiment that tells the guesses apart, or ask the reporter to run it.
- A platform report (WSL, Windows) is checked on that platform: ssh to the
  box. A GOOS build is not a test.
- Seen 1× (2026-10-05).

**A GUI change runs its tests in both Chromium and WebKit, at narrow widths,
in every language.**
- ~40 GUI commits ran WebKit only, without saying Chromium wasn't run.
- 3ec6b4da squeezed key names to "…" (#841). 653cb8ee shrank the ZCode
  question to 0px at 440px.
- 43a65950 broke gui-ja/gui-de placeholders for 3.5h. Run gui-ja and gui-de
  for every new `t()` string.
- When you reword a string, `git grep` the old text under internal/gui/tests.
  4a87b306 left routing-served red for 8 releases.
- Test the empty case: 6fc0afe8's price editor couldn't price a model with
  no list price (4c001cef).
- 10-06: 4c170cba's Japanese string had a third `{agent}`, and gui-ja was red
  until 34bfb9ca. 29e6d148 (#929) kept a clicked chip in view in Chromium
  only; c6e318e1 did WebKit 40 minutes later.
- Seen 2× (2026-10-05, 2026-10-06).

## Concurrency and tests

**Never send on a channel, or call anything that can block, while holding a
lock the receiver needs before it reads. Set shared state before the
handoff that lets another goroutine look at it.**
- 85fd35ab's emit sent to a full 64-slot segment while holding r.mu, and the
  reader takes r.mu first. It hung macOS -race CI for 20 minutes
  (4fb8e887). -race -count=50 locally didn't catch it. A test that fills the
  buffer first catches it every time.
- The Claude subscription run recorded run.tools after continueWith had
  already answered the agent, so a quick turn read the old tools
  (TestToolSearchLoadKeepsTheRun, c8690ca0).
- Seen 1× (2026-10-06).

**A fake or child process a test starts ends when the test does.**
- TestClaudeSignInByPaste's fake `claude auth login` polled every 50ms for 2
  days after its test, waiting on a file in a TempDir that was gone
  (38a0f470).
- After a test times out, check `ps` for its binary. An orphaned gateway.test
  spun at 96% CPU for 30 minutes.
- Size -count to -timeout. A timeout panic is not a hang until its stacks
  show one.
- Seen 1× (2026-10-06).

## Red tests and releases

**A test that fails on main is a bug to fix now, not a baseline.**
- 19 commits from 10-01 to 10-05 said "fails the same on origin/main" and
  shipped.
- TestAccountsOfAPlugin was red for 5 releases after 30d952cf.
  TestSetPortMovesTheGateway failed on contributors' clean checkouts (#872,
  #885).
- "Flaky, passes alone" hid a real data race in
  TestCloudflareDecideModelsTested for 19h (a45b3e09).
- Run `go test -race -count=20 -run X` before calling something a flake.
  Then fix it, or name the cause in an issue, in its own commit.
- 10-06: 4c170cba, ab47042b, 76dc5aed and aab93f9f each shipped with tests
  that "failed under the full run's load and pass on their own". Two of them
  weren't load: TestProvidersAnswerWhileListsComeIn depended on test order
  (bfe8bcaf), and Kiro's identity refresh had a data race (ccdd18e3).
- Later on 10-06: TestPluginSOCKSProxy timed out at 60s on ubuntu CI and was
  rerun as "load jitter". It takes 3s on a Mac every time, and a dead local
  proxy should refuse in milliseconds, so something on that path waits.
  TestUnreadLoginsNotWrittenOver needed two writes inside one wall-clock
  second (a `.bad-<second>` name) and failed on Linux CI (237d216b). Test a
  name made from time.Now() across a second boundary.
- Before fixing a red CI run, `git log origin/main` for a commit that names
  that run or test. 85fd35ab and e34e53a1 fixed the same lost tail in two
  sessions at once.
- Done right: 2bd49cc5's race was fixed at its cause in 25 minutes
  (28049bd2, -race -count=40 -cpu 1,2). 60f87223 and b1cba654 fixed two
  flakes at their cause instead of retrying.
- Seen 2× (2026-10-05, dozens of commits; 2026-10-06).

**Don't tag until a CI Test run on that commit, or one containing it, has
finished green. A cancelled run is not a pass.**
- On 10-05 nearly every CI run was cancelled by the next push. Linux-only
  TestCursorLocal failed from v0.1.877 to v0.1.892 (b7fef387).
- Batch fixes rather than tagging every commit. 294 tags in 51 hours left
  no run to finish.
- Before naming a version to a reporter, check it has all 16 assets.
  v0.1.956 was announced on #856 but never built.
- Before listing a package in market.json, check it resolves with
  `npm view`. 2f135ed4's middleware packages reached npm 5h after the
  release.
- Seen 1× (2026-10-05).

## Merging and closing

**Merge only the head you reviewed and ran.**
- Use `gh pr merge --match-head-commit <sha>`.
- #736 and #827 merged commits pushed after the last review. #807 and #838
  posted their verification after merging.
- 10-06: #974 was reviewed at 95b66f60, force-pushed 45 minutes later with a
  rebase and a new commit, and merged with nothing said about the new head.
  #966 is the model: re-checked at the new head, and the merge comment says
  so.
- Write what you ran on the PR, then merge. On 10-06, #1004, #1015, #1018,
  #1021, #1029 and #1034 each got their "verified" comment 2 to 14 seconds
  after the merge. #981, #1000 and #1030 got none. A merge with no record of
  what was run can't be checked afterwards.
- Never merge a PR whose own new test fails (#885, #826). Fix or file the
  problems a review lists before merging. #846 merged with four known-wrong
  translations.
- Seen 2× (2026-10-05, 2026-10-06).

**Keep the issue open until the reporter's case works. Reopen when they say
it doesn't.**
- On #876 the reporter couldn't reopen it themselves and had to file #888.
- Don't close as not-planned what the owner hasn't ruled on. #769 and #871
  were reversed.
- Seen 1× (2026-10-05).

**One commit, one goal.**
- 653cb8ee bundled ZCode sign-in with WSL session deletion.
- 5464a362 capped six pages at 1200px when #860 asked for a speed display.
- A fix found on the way goes in its own commit. 1e0829b1 hid the ja/de fix
  inside a Usage change, beside two issues (#925, #928).
- 76dc5aed bundled a Routing animation fix, a scrollbar fix and bringing the
  GUI tests up to date.
- Seen 2× (2026-10-05, 2026-10-06).
