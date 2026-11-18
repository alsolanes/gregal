# Functional Validation Checklist

Review date: 2026-10-05. This checklist applies to the current source checkout,
including uncommitted improvements after v1.7.8. It does not certify the
published installers or every provider/model combination.

## Evidence Levels

- **Automated**: exercised by local unit/integration tests, usually with synthetic model responses.
- **Browser**: exercised in a real browser using synthetic data, not a paid model.
- **Live API**: exercised against the local backend, without invoking a model.
- **Pending**: requires a real provider, platform, installation or manual review.

Passing synthetic tests verifies orchestration and safety boundaries, not a
model's reasoning, coding quality, tool-call reliability or visual judgement.
No private prompts, credentials or session transcripts belong in this report.

## Modes and Agent Lifecycle

| ID | Function to verify | Evidence / acceptance check |
|---|---|---|
| M01 | Code mode accepts a project task | Automated: agent run and mode tests; real model quality pending |
| M02 | Chat does not acquire write permissions | Automated: mode policy tests |
| M03 | Goal asks clarifying questions | Automated: goal prompt tests; conversational quality pending |
| M04 | Goal creates a persistent objective | Automated: goal save/list/update/delete tests |
| M05 | Goal records acceptance criteria | Automated: goal task/prompt tests |
| M06 | Autonomous mode preserves ask/deny rules | Automated: autonomous policy and mode tests |
| M07 | Per-turn mode can only restrict permissions | Automated: web agent mode tests |
| M08 | Mode cycling and advanced disclosure | Automated: mode-cycle JavaScript tests |
| M09 | Long tasks respect tool-step limits | Automated: agent limits and synthetic many-step scenario |
| M10 | Long tasks respect time/context cancellation | Automated: cancellation tests; multi-hour soak pending |
| M11 | Cost limit uses actual reported usage | Automated: limits/usage tests |
| M12 | Unknown cost is not fabricated | Automated: cost and noninteractive run tests |
| M13 | Context compaction preserves useful history | Automated: history compaction/window tests |
| M14 | Repeated failed calls trigger loop protection | Automated: doom detector tests |
| M15 | Retry/fallback does not invent a successful result | Automated: LLM retry/fallback tests |
| M16 | Empty responses and interrupted streams are handled | Automated: LLM and web lifecycle tests |
| M17 | Headless tool execution receives cancellation | Automated regression: functional smoke tests |
| M18 | Background processes stop with their run | Automated: agent/process context tests |
| M19 | Windows child processes stop on cancellation | Automated: Windows descendant cancellation test |
| M20 | Approvals and questions remain answerable after replay | Automated: durable interaction/run adapter tests |

## Project Tools and Deliverables

| ID | Function to verify | Evidence / acceptance check |
|---|---|---|
| T01 | Read with offsets and limits | Automated: tool and agent read tests |
| T02 | Search files and contents | Automated: glob/grep/tool tests |
| T03 | Create a new file | Automated: write/journal tests and coding smoke |
| T04 | Apply an exact edit | Automated: edit and coding smoke tests |
| T05 | Edit CRLF files without corrupting line endings | Automated: CRLF edit tests |
| T06 | Reject ambiguous/missing edit targets | Automated: edit tests |
| T07 | Apply patches with correct workspace handling | Automated: patch and web workspace tests |
| T08 | Run harmless code verification in a temporary workspace | Automated: coding smoke and verification tests |
| T09 | Expose failed verification honestly | Automated: verification gate tests |
| T10 | Run shell tools with timeout and cancellation | Automated: shell/process tests |
| T11 | Start/read/stop background commands | Automated: process and agent tests |
| T12 | Run independent read tools in parallel | Automated: parallel execution tests |
| T13 | Preserve original result order during parallel execution | Automated: parallel execution tests |
| T14 | Delegate with inherited workspace and permissions | Automated: delegation and security tests |
| T15 | Bound delegation and avoid recursive delegation | Automated: delegate tests |
| T16 | Publish todo progress without fabricated completion | Automated: todo/run summary tests |
| T17 | Checkpoint and rewind project changes | Automated: checkpoint/journal/rewind tests |
| T18 | Read images | Automated: image payload tests; real vision quality pending |
| T19 | Search/fetch web data | Automated: synthetic fetch/search tests; network/provider smoke pending |
| T20 | Browser actions apply permission checks | Automated: browser policy tests; full browser workflow pending |
| T21 | Load skills and instructions | Automated: skill tests; representative real-agent task pending |
| T22 | Read/create/edit supported Office files | Automated: Office tests; external Office rendering pending |
| T23 | Open Office files externally only with approval | Automated: Office policy tests; installed desktop integration pending |
| T24 | Connect to MCP tools without widening permissions | Automated: MCP tests; external MCP smoke pending |
| T25 | GitHub issue/PR tools handle API results | Automated: GitHub API tests; authenticated GitHub smoke pending |

## Team and Visual Workspace

| ID | Function to verify | Evidence / acceptance check |
|---|---|---|
| V01 | Coordinator plan feeds downstream roles | Automated: team orchestration tests |
| V02 | Researcher and builder run independently in parallel | Automated: team parallel branch tests |
| V03 | Reviewer receives both branches | Automated: team merge tests |
| V04 | Rejected review triggers one revision and final review | Automated: team revision tests |
| V05 | Failure cancels the parallel sibling | Automated: team failure/cancellation tests |
| V06 | Team never claims actual tools in its tool-free workflow | Code/prompt review; real model adherence pending |
| V07 | Active workers and activities are visible | Browser: scripted demo with four concurrent workers |
| V08 | Delegates have separate visible positions | Browser: scripted delegate demo |
| V09 | Canvas pixels change while work is active | Browser: successive canvas pixel comparison |
| V10 | Stop removes working states and freezes canvas | Browser: stop + pixel comparison; automated frame test |
| V11 | Waiting for a teammate is not user attention | Automated: attention queue regression |
| V12 | Reduced-motion setting prevents continuous animation | Automated frame/position checks; visual smoke pending |
| V13 | Selecting an agent exposes its current task and output | Browser demo; keyboard/screen-reader review pending |
| V14 | English and Catalan activity/status labels | Automated bilingual progress-summary checks and shared language tests; complete activity-label/translation review pending |
| V15 | Mobile layout has no horizontal page overflow | Browser smoke; all devices and orientations pending |
| V16 | Demo is clearly separated from real execution | Browser/code review: demo labels and synthetic outputs |

## Side Preview and Documents

| ID | Function to verify | Evidence / acceptance check |
|---|---|---|
| P01 | Agent can explicitly request HTML preview | Automated native tool test; browser HTML rendering |
| P02 | Agent can request a Markdown plan/document | Automated native tool test; browser formatted Markdown |
| P03 | Agent can request a localhost development URL | Automated URL validation; live dev-server smoke pending |
| P04 | Team opens the coordinator plan | Browser: synthetic Team event stream |
| P05 | Builder and reviewer HTML update the side preview | Browser: synthetic Team event stream |
| P06 | Latest rapid update wins, not a stale iframe document | Regression + browser check against actual frame content |
| P07 | User closing preview suppresses automatic reopen | Browser and request lifecycle regression |
| P08 | New run resets automatic dismissal scope | Request lifecycle regression |
| P09 | Manual open remains available after dismissal | Request lifecycle regression |
| P10 | Changing artifact type resets title/form state | Request lifecycle regression |
| P11 | Markdown cannot inject executable raw HTML | Browser: raw script displayed as text |
| P12 | HTML iframe does not receive same-origin privileges | Automated sandbox/referrer tests |
| P13 | Generated HTML has restrictive content security policy | Automated artifact tests; not an OS/network sandbox guarantee |
| P14 | Agent preview rejects remote/credentialed/non-HTTP URLs | Automated native tool and browser validation |
| P15 | Oversized/empty preview payloads are rejected | Automated native tool/request tests |
| P16 | HTML download preserves complete deliverable | Code review; download round-trip pending |
| P17 | Preview does not imply visual inspection or passed tests | Tool description and UI limitation; actual browser inspection is separate |

## Providers, Sessions and Clients

| ID | Function to verify | Evidence / acceptance check |
|---|---|---|
| C01 | Provider preset assigns roles | Automated provider preset tests |
| C02 | Presets preserve existing custom URL/key | Automated provider preservation tests |
| C03 | Missing hosted key does not trigger a provider probe | Automated provider preset tests |
| C04 | Provider name is separate from recommended model | Browser provider panel smoke |
| C05 | Advanced settings are collapsed and labelled | Browser desktop/mobile smoke |
| C06 | Custom role model not in advertised list is preserved | Browser: synthetic advertised model list and preserved custom model |
| C07 | Provider errors are visible without showing keys | Browser synthetic failures/code review |
| C08 | Provider setup handles partial save/failure | Known limitation: multiple API operations, not atomic |
| C09 | New/switch/close sessions preserve correct drafts | Automated session and browser module tests |
| C10 | Session storage and per-session workspace remain isolated | Automated session/web contract tests |
| C11 | Durable SSE replay deduplicates events and cursors | Automated event/run/SSE tests |
| C12 | Python client health/session/events/cleanup | Live API: local backend, synthetic session, no model calls |
| C13 | VS Code API/SSE/localization | Automated: extension tests and compile |
| C14 | Desktop updater/download/restart confirmation | Automated: desktop tests; installed update cycle pending |
| C15 | Desktop IPC/navigation/settings/window state | Automated: desktop tests; installed app smoke pending |
| C16 | Android client | Existing Go/API contract coverage only; Android build/device smoke pending |
| C17 | Telegram client | Automated Go tests; live bot smoke pending |
| C18 | Web API authentication and remote bind rules | Automated web auth/listen tests |
| C19 | Desktop remote connection uses HTTPS | Security regression tests; remote TLS deployment smoke pending |
| C20 | Password storage repairs existing Unix file permissions | Regression added; Unix execution pending because the local Windows run skips permission-bit assertions; Windows ACL guarantees are separate |

## Local Verification Commands

### Results From This Review

- Full Go suite, forced uncached: passed. The Unix password permission assertion is skipped on Windows.
- `go vet ./...`: passed.
- Combined web, desktop and script JavaScript suite: 104 passed, zero failures.
- VS Code: 16 tests passed and extension compilation passed.
- Python: 10 tests passed, including the local backend session/event contract.
- Vendor artifact pins and XLSX round-trip: passed.
- Browser: provider selection/errors/custom model/mobile layout, Team plan/draft/review preview, actual latest iframe document, Markdown escaping, dismissal, animated canvas and Stop were exercised with synthetic data.
- Coding smoke: a real temporary Go project was read, edited and tested through a synthetic provider-driven agent run; all tool traces succeeded.
- Autonomous stress: 20 read-tool calls stopped at the configured budget; this is not a 15-30 minute or multi-hour soak.
- Cancellation: an active delegated headless task received the parent cancellation context.
- The checkout privacy scan did not pass: it found `.playwright-cli` and Python `__pycache__` runtime directories. No clean current export was generated in this review.

### Fixes Made

- Pass the headless run context into tool execution.
- Prefer the code role when the initial configured mode is autonomous.
- Coalesce rapid iframe artifact updates so the latest document actually loads.
- Respect preview dismissal across automatic updates and reset it on a new turn.
- Reset preview titles and URL controls when changing content types.
- Do not present waiting for another Team agent as a user decision.
- Require HTTPS for non-loopback desktop backend URLs without restricting ordinary external links.
- Tighten existing password-file permissions after saving; Unix assertion still needs a Unix runner.
- Shorten synthetic LLM retry waits in tests only, following the existing agent test pattern.

### Commands

```powershell
go test ./...
go vet ./...
node --experimental-vm-modules --test internal/web/*.test.js internal/web/app/*.test.cjs desktop/*.test.js scripts/*.test.cjs
npm --prefix vscode-gregal test
npm --prefix vscode-gregal run compile
$env:PYTHONPATH = "$PWD/python"
python -m unittest discover -s python/tests
node scripts/verify-vendor.cjs
node scripts/audit-public.cjs .
```

To exercise the Python live contract, set `GREGAL_CONTRACT_URL` to a local test
server and `GREGAL_CONTRACT_TOKEN` when authentication is enabled. The test
creates and closes a synthetic session. Do not point it at production data.

The source privacy scan currently detects local browser/Python runtime
directories. Those must be excluded from any fresh public source export.
Passing scanner unit tests is not equivalent to a clean checkout/export scan.

## Real-Model Validation

An authenticated configured local provider was exercised in disposable projects.
Credentials, provider addresses and private configuration are deliberately omitted.
These are individual observations, not an OpenCode speed comparison or a soak test.

| Scenario | Observed Runtime | Evidence |
| --- | --- | --- |
| Code repair | 9.25 seconds | Four model calls; read, edit and shell verification; independent Go tests passed. |
| Autonomous functions and tests | 42.59 seconds | Six model calls; bounded to 20 tools and five minutes; independent Go tests passed. |
| Chat explanation | 16.75 seconds | Five model calls; fixture files unchanged; nonempty response. |
| Goal clarification | 15.52 seconds | Three model calls; question tool invoked; fixture files unchanged. Headless question handling is not evidence of interactive user answers. |
| Website creation | 86.23 seconds | HTML created and a preview request emitted; independent browser inspection found hidden task labels. |
| Website correction | 74.62 seconds | Model corrected the CSS; browser add, complete and delete checks passed at mobile width, with no horizontal overflow; desktop/mobile screenshots inspected. |
| Active model cancellation | 2.00 seconds | Request deadline cancelled the active provider call and the run returned an error rather than success. |
| Real Team review | 250.98 seconds | Six calls, two concurrent branches and one rejected draft/revision; final deliverable returned without error. This was tool-free collaboration, not executed coding or web research. |

The website runs intentionally left an unrelated failing Go fixture unchanged.
That fixture is not the website acceptance test. A preview request in a headless
run proves the request was emitted, not that the agent inspected a rendered page.
The browser checks were performed separately by the validation harness.

Live runs exposed two permission defects, now covered by regression tests:
`bash` restrictions could be bypassed through `bash_background`, and internal
`todowrite` bookkeeping requested approval unnecessarily. Both shell aliases now
share the strictest configured decision; hard-denied commands remain denied even
with an explicit allow grant. Todo bookkeeping is allowed without a popup.

The website correction required twelve model calls, including unnecessary
verification attempts. This remains a performance improvement target. No claim
of equal quality or greater speed than OpenCode is justified by these samples.

A separate repeated CLI comparison passed twelve immutable-fixture acceptance
tests. See [the measured conditions and limits](performance.md#local-cli-observations-2026-10-05).
An active shell cancellation test subsequently exposed a foreground descendant
leak. Foreground commands now use the existing platform process groups (Windows
jobs and Unix process groups), with marker-based cancellation and timeout
regressions. The sustained test was rerun after that fix: 900.11 seconds,
30 independently verified bounded autonomous cycles, zero errors. Cancellation
of an active shell descendant and subsequent recovery both passed. This is a
repeated-cycle soak, not one uninterrupted fifteen-minute coding task.

## Extended Acceptance Scenarios

The observations above cover a subset of this broader checklist, not every
interactive or platform-specific path. Run these in a disposable project with reviewed permissions and a configured
provider. Record model ID, runtime, steps, observed tests and final result, but
never publish credentials or private prompts. Model/API costs may apply.

1. **Chat:** ask for a comparison; assert no project writes occur.
2. **Code:** repair a small pure function with a failing unit test; inspect the diff and independently run the tests.
3. **Goal:** give an ambiguous objective; verify clarifying questions and saved acceptance criteria before implementation.
4. **Autonomous:** create a small multi-file program with tests; verify checkpoints, bounded steps and honest verification output.
5. **Long run:** use a 15-30 minute budget in a disposable project; cancel while a tool is active, verify descendants stop and the next run succeeds.
6. **Permissions:** deny writes/shell/network separately; verify neither retries nor delegation bypass the deny rules.
7. **Website:** create a standalone HTML page, preview it, revise it, and independently inspect the rendered result at desktop/mobile sizes.
8. **Complex plan:** request a multi-stage Markdown plan, update it after feedback, and verify preview matches the latest document.
9. **Team:** compare independent branches, reject an incorrect draft, and verify the revised final deliverable.
10. **Provider outage:** interrupt the endpoint mid-run; verify retry/failure, cancellation and recovery do not report false success.
11. **Restart:** restart Gregal after an interrupted task; verify session recovery without duplicate tool execution.
12. **Installation/update:** install a fresh signed/unsigned release as appropriate, verify its version and assets, then test an actual update to a newer version.

## Release Boundaries

- Release artifacts must pass checksum and packaged-version checks before publication; older local artifacts are not release verification evidence.
- The ignored `public-release` export is an older snapshot, not evidence for this checkout.
- New UI, preview and audit fixes are not in the published v1.7.8 installers.
- Six Go target cross-builds passed (Linux, Windows and macOS, amd64/arm64); cross-compilation is not runtime validation on those platforms.
- The Windows installer and portable package passed release metadata verification. The installer is unsigned.
- Broader real-provider evaluations, multi-hour reliability, Linux/macOS runtime checks, Android devices and the actual installed updater/restart cycle remain pending. The repeated OpenCode comparison is documented separately.
- Tool permissions are not an OS sandbox. Approved shell commands execute with host privileges.
- Generated content is untrusted; previewing it does not certify its security or correctness.
