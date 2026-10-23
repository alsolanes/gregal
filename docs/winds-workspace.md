# The Winds work desk

Winds are personal helpers with a practical work surface as well as their
existing conversations. The desktop/web Winds page gives each saved wind a
compact character card below a request composer. Writing a brief and pressing
Start is enough: owner and work type are selected locally, with an independent
reviewer for website/code creation. Manual overrides and text-only execution
live in the collapsed Task options menu. An empty catalogue is populated with
Tramuntana, Marinada and Mestral on the first submitted task. Existing profiles,
memories and conversations are not migrated or
overwritten. The specialist descriptions apply to newly created presets.

## Execution

`app/wind-desk.js` builds a bounded team flow. Creation uses a builder seat;
research uses a read-only researcher seat. An optional reviewer receives the
deliverable and can request one revision. Before starting, the desk reads back
the selected profiles and supplies their character, knowledge and personal
memory to their respective seats.

`app/team.js` exposes a narrow `gregalWindRunner` bridge onto the same team
controller and `/api/v2/team/run` stream used by the workshop. It prevents
overlapping team runs, preserves project permissions and routes approval
requests to the automatic reviewer during wind work. Progress, stop controls, deliverables
and previews are driven by actual stream events. There is no simulated work
in the production desk.

The routine shortcut prepares the existing scheduler form; the user reviews
the instruction and timing before saving. The current scheduler runs read-only
research and summaries, not unattended code changes. Its service must be running.

## Visual direction

The desk takes visual cues from [Things](https://culturedcode.com/things/features/)
for hierarchy and restraint, [Headspace](https://developer.apple.com/news/?id=fkfnhq8u)
for friendly character illustration, and [Finch](https://finchcare.com/about-finch)
for giving a companion a central place. These are references, not copied assets.
The result uses an editorial serif heading, warm neutral surfaces, softly shaded
SVG portraits and a restrained character shelf. Selecting an owner marks the
matching character card. The task is the primary working sheet; routines sit
in a collapsed secondary section. The composer shows the current project folder.
Colours continue to follow the application's theme.

## Current scope

- Desktop and web share the desk. The native Android Winds screen is unchanged.
- Live team work requires the app/connection to stay open. It does not resume
  after restarting the app and this view does not add durable task history.
- A task's output is not automatically added to a wind's long-term memory.
- The routine's wind identity is included in its instruction, rather than a
  new scheduler ownership field.
- HTML and document previews use the existing preview controls. File writes
  use the existing team's tools and permission policy.

## Validation

Run the wind desk, wind conversation, desktop updater and team UI Node tests,
plus Go tests for `internal/team` and the web wind/team handlers. Browser smoke
checks use an intercepted API and SSE stream to exercise the real UI and
runner without spending model tokens. They cover selecting winds, reading
their context, starting, receiving the reviewed result, preparing a routine,
inline permission approval, stopping and mobile layout. This validates the
integration, not the quality of a live model's generated website.

`scripts/check-winds-ui.js` is the repeatable browser check. Serve
`internal/web` on port 8766, open that URL in Playwright CLI, then run
`playwright-cli run-code --filename scripts/check-winds-ui.js`.
It also sends 50 activity updates and checks that result nodes, focus,
selection and scroll position survive without mutations to the output text.
Character cards are reused when the catalogue has not changed. Ctrl/Cmd+Enter
starts the validated form; the current-task shortcut moves keyboard focus to
the progress heading. Work badges reflect real participant states.

`scripts/check-winds-quick-start.js` covers an empty catalogue, automatic crew
creation and assignment, mobile layout, language changes and opening an actual
HTML file rather than the agent's written summary. API/model responses are
controlled fixtures; no model is invoked by these browser checks.


Scheduled routines are configured in a dialog in the Winds workspace. Saving refreshes the scheduled task list without navigating away. The page background uses the active theme palette, including dark themes.
