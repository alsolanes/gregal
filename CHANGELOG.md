# Changelog

## Unreleased

- **Arena: faster and cheaper.** The Arena page loads in about 15 ms instead of 2 s with a hundred matches (match summaries are cached and match files are saved compact). Practice and official matches have separate slots (1 official and 2 practice at once by default, admin-configurable), so practice no longer waits behind a competition, and a waiting match shows its place in the queue. Saving a match no longer freezes the viewers, and a slow caster can no longer stall a match. Model calls put the fixed part of the prompt first so the provider's cache serves most of it, ask for shorter answers, and on an invalid answer say what was wrong instead of asking the same thing again; each match records its model calls, tokens (and cached tokens) and time. Bots get no coach report in practice. The viewer can poll only the new decisions and caster lines, and the Arena state lists the history of your `.md` versions.
- **NFL hints are a competition setting.** The decision format's "stops / beaten by" hints for every call stay on in practice but are off by default in official competitions (an option when creating one), so the `.md` has to do the reading.
- **Arena: bracket, schedule and OBS overlays.** The TV broadcast shows a **Bracket** segment between playoff matches (quarterfinals, semifinals and final in a tree with each best-of-3 series score, where each slot comes from — "1st Group A", "Winner SF1" — the live match and the champion) next to the schedule (live now, up next, played); the standings segment shows the group table, and all the groups once the group stage is over. New overlays for OBS browser sources with a transparent background that follow the live competition: `arena-stream.html#overlay=scorebug` (teams, score, game and series), `#overlay=standings` and `#overlay=bracket`, also with `&spectator=<key>`. The bracket is a standalone module (`arena-bracket.js`) the Arena page can reuse.
- **Arena: licence-clean English voices, and faster.** The English caster voice (Ryan) was licensed for non-commercial use only (CC BY-NC-SA 4.0) and the analyst (Alan) was built on a research-only voice: they are replaced by **Norman** (caster, US) and **Cori** (analyst, UK), both public domain (LibriVox recordings, trained from scratch). Existing installs download the new voices automatically (~127 MB) and delete the old ones. Each voice now keeps one Piper process running instead of starting one per line: a line takes about 0.3–0.5 s instead of 1.1–1.7 s. Preferences → Narrator voices shows each voice's name and licence (linked to its model card) and notes that the Catalan voices are CC BY-SA 3.0 ES (share-alike).
- **Arena viewers: lighter, and better on the phone.** The NFL, CS2 and LoL viewers share one frame loop that stops completely (drawing and polling) when the viewer is hidden or off screen, redraws a paused match only when something changes, follows its container's size, caps the pixel density (2, or 1.5 on touch screens) and runs at 30 fps on phones. NFL frames cost about half (players, helmets and the bench are cached sprites, no blur on the first-down line, cheaper crowd); CS2 no longer stutters on the first first-person cut. The standalone match page has an auto-hiding control bar at the bottom instead of buttons over the scoreboard, and portrait screens are framed for the phone (NFL TV and player cams, CS2 map zoomed and following the action, the arena set cropped). LoL champions have drawn portraits instead of initials and the draft is shorter; the CS2 first-person view has compact rosters, solid player figures and a detailed gun and gloves.
- **Arena: from the report to the next win.** After a practice game a **Last match** card beside the editor (where step 3 now points) shows the result, the coach score and the top mistakes with the coach's rewritten rule; **Apply fix** puts the rule in the editor — replacing the line the coach quoted or the most similar one, next to related rules, or under *Coach fixes* — shows the change and can **Undo** it. **.md versions** lists your saved versions with the games played with each one, a line-by-line diff between any two with how the results changed, and **Restore this version**. The player shows each match's cost (model calls · tokens with the cached share · time) and the place in the queue while waiting. The match card only fetches new decisions and caster lines and appends them instead of rebuilding up to 400 rows every 3 s, and nothing is polled while you are away from the Arena or after closing the match. The practice counter is hidden for admins, the live CS2 button is disabled on touch-only screens, and the progress chart no longer floats one bar in an empty box.
- **Arena: the highlights video renders faster than real time.** Where WebCodecs can encode H.264, **⬇ Download video** steps the viewer frame by frame (no real-time playback), mixes the effects and voices offline into an AAC track and writes an MP4 with mp4-muxer 5.2.2 (MIT, vendored): about 3× real time for a CS2 match and 4× for LoL on a laptop, with a progress bar, the time left, and it keeps going in a background tab. Browsers without WebCodecs H.264 fall back to the real-time recording.

## v1.34.1 - 2026-10-30 - Segueix-me

- **CS2 live: bots that follow you.** A new bot role, **follow**, makes a bot your escort: it walks your actual path a few metres behind you (several escorts keep their distance from each other), watches the angles you are not looking at, enters a site right after you, turns on whoever shoots you and goes for your killer first, throws its smoke and flash for you when you hit a site (as CT, a molotov or HE where the team spots enemies near you) and rotates when you rotate. If you die, escorts trade you and then play the team plan for the rest of the round. Set it per bot, with **🛡 All follow me** in the setup, or by writing an instruction like "follow me", "stick with me", "cover me", "segueix-me" or "sígueme" (the role is picked for you, and the coach is told to use it when your strategy asks for it). In game, **V** switches the whole team between "follow me" and "play the plan" until the end of the round, shown on the coach's card.

## v1.34.0 - 2026-10-30 - CS2 en directe

- **CS2 live: play it yourself.** In Counter-Strike 2, **🎮 Play it yourself (live)** opens a first-person match on Mirage where you control one player with keyboard and mouse (WASD, mouse look, shoot, AWP scope, walk, crouch, weapons, grenades, reload, hold E to plant or defuse, a buy menu, scoreboard) and the other nine are autonomous bots that move, hold angles, trade, throw utility, plant, rotate, retake and defuse. The model still only coaches: before every round it reads each team's `.md` and calls the buy and the plan, and can give each bot a role for the round (entry, support, lurker, AWPer, anchor, rifler); you set your bots' roles and instructions before the match, or ask the coach to write them from your `.md`. A card on screen shows the coach's call, its reason and what each bot is doing. Short (MR6) or full (MR12) format, three bot skill levels. The round is simulated in the browser and the server applies the normal rules to what happened (money, loss bonus, kill rewards, halves), so the match is saved with the others (kind *live*, not ranked, doesn't use practice games) and replays in the match player with the tactical map and the POV.
- **CS2 view: the gun points at the crosshair.** In the first-person view the weapon in the player's hands was drawn at a fixed angle, so its barrel pointed below and to the side of the crosshair (more on wide screens). Rifles, the AWP, SMGs and pistols are now aimed at the crosshair at every aspect ratio, foreshortened so the muzzle stops short of it, and stay clear of the side rosters.

## v1.33.1 - 2026-10-30 - Pilota, recepcions i placatges

- **American football looks real.** The ball bounces with the irregular hop of an oval ball, rolls and settles, spirals in flight and tumbles on kicks; snaps and handoffs go hand to hand, the QB visibly throws, receivers reach out and catch (chest, high-point, over the shoulder or diving) and tuck the ball away; tackles have an approach, a wrap-up, a brief impact freeze with a flash and grass, and both players falling together, plus gang tackles, stiff-arms, sacks, pass breakups and fumbles; linemen engage at the snap and players no longer overlap.
- **Broadcast fixes.** The standings screen no longer hides the top two rows (they took the style of the page's top bar), the ticker text now passes under the yellow GREGAL ARENA label, and "Up next" announces the match the server is actually about to show.

## v1.33.0 - 2026-10-30 - El reproductor dels partits

- **Arena: matches play in a real video player.** The match card is now a player: a large 16:9 picture with a control bar under it — ▶/⏸, ⏮ restart, ⏪/⏩ 15 seconds, a timeline you can click or drag (★ for prompt moments, marks for scores and big plays, a tooltip with what happens at each point), elapsed / total time in mm:ss, speed, CC captions, 📺 TV mode and an always-visible **⬇ Download video** (greyed out with an explanation until the match has finished; then it records the highlights video, and offers **Share…** where the system can share files). Beside it, a **Commentary** track works like video chapters: one line per prompt moment, score, big play and caster line, with its time, the team's call and the `.md` rule behind it; the current line lights up and follows the match, clicking a line jumps there, and All / Key moments / Prompt moments filter it (the decisions list is the **Decisions** tab next to it). When the match ends, a summary appears inside the player: result, analysis, the deciding rule, the prompt moments (click one to watch it again), coach scores and how often each team followed its `.md`, with ↻ Watch again, ⬇ Download video and Close.
- **Go back, restart or jump anywhere in a match.** The same progress bar is in the TV broadcast (it appears when you move the mouse and hides again) and on the Games page. Keyboard: ←/→ 15 seconds, Space pause, Home restart, End to the end. Jumping rebuilds the match up to that point at once (score, positions, structures, round) without replaying banners, replays, arena shots or sounds, and keeps the score bar, the team calls, the captions and the prompt-moment cards in step. On TV, going back after the match has ended returns to the live segment; in a competition broadcast you can only move within the match being played. The time shown is playback time at 1×, the same on the bar, the commentary track and the exported video.

- **The narrator really speaks now.** A single match on TV (or a demo) never played any voice when there was no competition, even with the voices installed: the broadcast only got the voice information from a competition. Fixed. And Preferences has a new **Narrator voices** group: the voice engine, every language with its status, Install / Download again, and ▶ buttons to test the caster and the analyst right there.

## v1.32.1 - 2026-10-30 - L'Arena dels esports

- **CS2 and LoL as live esports events.** Both viewers now cut to an arena: a stage with the two team booths (players at their PCs), a giant screen with the live score, light beams and thousands of fans in team colours who stand, wave light sticks and roar on the big moments; a caster desk, booth celebrations after round wins and big objectives, big-screen graphics (match point, half time, overtime in CS2) and a trophy lift with confetti at the end.

## v1.32.0 - 2026-10-30 - Plugins, moments del prompt i vídeos

- **Arena, Games and Corner are now plugins.** They are optional features that the server administrator can turn on or off in Preferences → Plugins (all on by default). A disabled plugin disappears from the sidebar and the phone's home screen, its API answers 404 and its background work stops; turning it back on resumes it without restarting. New `GET/POST /api/plugins`.
- **Preferences, reorganised.** Options are grouped (General, Appearance, Models & agent, Plugins, Extras, Integrations, Advanced) with a short description under each heading, an index on the left and a search box that filters as you type. Every existing option is still there.
- **Arena: you hear the narrator.** Sound is on by default in the broadcast and TV mode (it only stays off if you mute it), and if you already use the free local voices (Piper) the English ones are downloaded automatically the first time the games need them, with progress shown in the Arena.
- **Arena: prompt moments, match summary and highlights video.** When a match ends, up to six plays where a line of a team's `.md` really mattered are picked (by the analyst, checked by the server; computed from the scoring plays when there is no model, also for demo and older matches). They pop up as a “📋 PROMPT MOMENT” card while you watch (with a short slow-down and a jingle in TV mode) and get a ★ in the decisions list. TV mode ends with a **Match summary** (moments, analysis, deciding rule, coach scores, how often each team followed its `.md`), and “⬇ Export video” records a highlights video in the browser (intro → fast-forwarded match, normal speed at each moment with its card → summary, with sound) and downloads it as MP4, or WebM where MP4 can't be recorded. Older League of Legends matches also get their decisions renumbered so each one lines up with its plays.
- **Publish what the agent builds, with a link anyone can open.** From the preview (phone: **Publish** in the preview and next to its chip, with Copy and the Android share sheet; desktop and web: ⇪ in the preview panel), a copy of the page goes to your Gregal server at `/share/<id>/`, opened in any browser without the app or the key. Publishing again keeps the link; Unpublish (or Preferences → Shared links) deletes it. The link uses the server's public address (`GREGAL_PUBLIC_URL`) when it has one; otherwise the app warns that it only works on your network (or only on the phone). Pages run sandboxed, away from your Gregal session. New `POST /api/shares/site` and `GET /share/{id}/{path}`.
- **Android: the agent's preview shows on the phone.** When the agent built a web page and opened its preview, the phone only showed a "Preview" step and nothing opened: the app ignored the preview, and the server's event log cut it to 3000 characters. Now it opens full screen (HTML runs isolated, without network; Markdown with the app's renderer), and a chip above the composer reopens it after you close it. A `localhost` dev-server address is loaded through the server's address instead of the phone's own, with "Open in browser" and a hint when the dev server only listens on localhost.

## v1.31.1 - 2026-10-30 - Càmera de jugador i estadi

- **American football: player cam and a full stadium.** The main live camera is now a player cam, low behind whoever has the ball (behind the QB before the snap, then over the receiver's shoulder after the catch, behind the runner or returner), with slow motion on catches and tackles. The field sits in a three-tier stadium with about 11,000 animated fans in team colours who do the wave and jump with flags and camera flashes on big plays, two video boards, benches and light towers. New angles: end-zone and pylon cams for touchdown replays, a low rail cam, and a blimp shot for kickoffs, each tagged on screen.

- **Arena: the narrator's voice in English.** Games are in English, so the narrator needs the English Piper voices: if they are missing, the Arena says so next to the Play button with a one-click install, and the TV mode shows "captions only" instead of staying silent. Demo matches are now narrated with this computer's voices (the recording carried audio from another machine, so they never spoke).

## v1.31.0 - 2026-10-30 - Guanya el millor prompt

- **Arena: demo matches.** 🎬 Watch a demo match shows, in TV mode and at 2×, a real recorded game for the chosen sport (the model playing an expert `.md` against a beginner one, with commentary, analysis, the rules applied and what the model saw), instantly and without spending model calls.

- **Arena: the better prompt wins.** The three engines were retuned so the result depends on what the `.md` says. Before, a vague "always attack" strategy beat a careful one in 5 of 6 live games. Every option now has real counters, the rules text is generated from the engine so it can't contradict it, fixed one-note play loses to a strategy that reads the situation (by 85-98% in simulations), defaults are fair to both sides and luck matters less. Each situation now also shows what has been working (yards and success per call in NFL, your recent calls and whether you won in CS2, both teams' plans and what happened in LoL).
- **Arena: you can see that it follows your orders.** Every decision says whether the rule the model quotes really is in your `.md` (checked by the server), each match shows how often each team followed its `.md`, and clicking a decision shows exactly what the model saw.
- **Arena: new cameras.** American football alternates the TV sideline camera with a skycam behind the offense, with instant replays of big plays and field goals from behind the kicker. Counter-Strike 2 cuts from the tactical map to the player's view for entries, duels, plants and defuses, with kill-cam replays. League of Legends has a broadcast-style map (towers, pits, minion waves, plans drawn as arrows, rosters, gold graph) and an action cam that zooms into fights and objectives, with replays.

- **Arena: TV mode.** Any match opens full screen as a TV broadcast (📺 TV mode): how the game is played, the analyst's preview, the match with the caster, and under each team the call it just made and the rule of its `.md` that led there, then the result and the analysis. The winds' Games get a full-screen button too.
- **Games are always in English.** Engine texts, casters, analysts, coach reports, the broadcast and the Arena page are in English whatever the app's language.
- **Arena: what to write and how you are doing.** Next to the editor, each game lists exactly what your `.md` decides (every option), what the model sees at each decision and example rules. Under the Play button, a chart of your games (won/lost and margin) with the coach's score for your `.md`, so you can see whether it improves.
- **Arena: fairer competitions.** Playoffs are best of 3 with sides swapped; an official game where the model could not decide for one side more than 10% of the time does not count and is replayed (twice at most); a competition plays every game with the same model, with no per-side fallback model, and retries model errors with backoff; the pre-match analyst only sees each strategy's identity in official games, so rules stay secret until the end; in League of Legends the scoreboard follows the side shown on the map.

## v1.30.2 - 2026-10-30 - Jugadors de futbol que no tremolen

- **Football games look right.** Players no longer shake when someone has the ball: a few plays picked a new random position on every frame (after a catch, on returns, sacks and interceptions), and players were drawn in the wrong depth order, so they flickered when they met. The players are now proper football players: running legs, arms, shoulder pads, the number, a team helmet with a stripe and a facemask facing where they go, and the tackled carrier lies on the ground. Applies to the Arena and to the winds' Games.

## v1.30.1 - 2026-10-30 - L'Arena, més senzilla

- **The Arena is simpler.** Pick a game and press Play: the template is already in the editor, your `.md` saves itself, and the opponent is the easiest bot you have not beaten yet. The bots are written automatically the first time, the ranking, games, competitions and broadcast settings fold away below, and a waiting match says the coaches are picking the play. Matches now use your conversation model instead of the winds' one, so a slow model no longer turns a game into half an hour.

## v1.30.0 - 2026-10-30 - L'Arena dels prompts

- **The Arena: a prompt-writing competition in three games.** Each player (a user of your Gregal server) writes a strategy `.md` per game — American football (NFL), Counter-Strike 2 and League of Legends. Both teams in a match are identical; at every decision point the same model reads each team's `.md` plus the situation (score, resources, and what the opponent has done so far) and decides: in NFL, every play call on offense and defense; in CS2, each round's buy, strategy, utility and site (or the CT setup); in LoL, the pick-and-ban draft and every phase's objectives and stance. The engines are real simulations with counterplay (NFL calls that beat or lose to each coverage, CS2 economy, stacks and fakes, LoL compositions, power curves and objectives), so a clear, adaptive `.md` beats a vague one: in the engines' tests an adaptive scripted strategy beats a naive one 93% (NFL), 84% (CS2) and 84% (LoL) of the time, and two equal strategies split ~50%. During the week players edit their `.md` (with a template per game and a version history), play practice games against bots whose `.md` the model writes at four levels (rookie, aggressive, conservative, expert) and can read to learn, and ask for a review of their `.md`. Every match keeps each decision with the model's reason and the exact line of the `.md` it followed, and ends with a coach's report per team: scores for clarity, coverage, adaptivity, specificity and consistency, the key moments where the `.md` failed with a concrete rewritten rule, and lessons. The organiser creates a competition (groups seeded by Elo, then playoffs), starts it and opens the stream page, built for OBS: each match full screen with the players' names and mottos, then the result, the standings and who plays next, and the champions at the end. Official matches move an Elo ranking per game; when a competition ends, everyone's `.md` is revealed so people learn from the winners. There is also an Arena row on the Vents phone app to edit your `.md`, play the bots, see the ranking and watch matches or the live stream.

- **The Arena on TV.** The stream page is now a sports broadcast: an opening that explains how it works, an explainer the first time each game appears, an analyst's preview, the match with a play-by-play caster, the result with the analyst's verdict and the deciding `.md` rule, standings and up next, and the champions, with wipes, a jingle, crowd sound and a ticker. The narrators speak with a free voice that runs on your own machine (Piper, installed from the Arena's Broadcast panel) or any self-hosted `/v1/audio/speech` server (Chatterbox, Kokoro, Orpheus…); the audio is made ahead and cached. A spectator link lets anyone in the company watch the stream, and only the stream, or put it in OBS for Teams, YouTube or Twitch.

## v1.29.0 - 2026-10-29 - Els vents juguen a futbol americà

- **The winds play American football.** A new Games section (sidebar on desktop and web, a row on the Vents home on the phone) runs NFL-rules games between two franchises made of your winds. If there are not enough winds, the missing players are created automatically as team players (name, wind face, number, position and attributes): they are kept with the league, not in your list of winds. The game is a real engine, calibrated against NFL averages over hundreds of simulated games (about 24 points, 63 plays and 4.4 yards per carry per team, ~63% completions at ~7.6 yards per attempt, ~2 sacks, 0.7 interceptions, 4 punts, 88% field goals): downs, the clock with the two-minute warning and timeouts, kickoffs, punts, field goals, extra points and two-point tries, sacks, scrambles, interceptions and pick-sixes, fumbles, pass interference, safeties and overtime. Players' attributes (speed, strength, arm, accuracy, hands, blocking, tackling, coverage, kicking) decide each matchup. Two winds are the head coaches: before every quarter each sets the game plan with the model (run/pass mix, deep shots, aggression on fourth down, blitz rate, tempo, a featured player) and says a line for the broadcast; another wind is the commentator, who calls the key plays and writes the recap and picks the player of the game. That is about 15 model calls per game; 3 games a day, and Stop ends one. You watch it "live, compressed": a TV camera from the sideline with the field in perspective, both teams' colours and end zones, numbers and helmets, the first-down line, the ball's flight on passes and kicks, a broadcast scoreboard, TOUCHDOWN banners with confetti and the commentator's line under each key play; pause, 2× and 4×, or jump to the end. The league keeps each team's record.

## v1.28.0 - 2026-10-29 - El Racó dels vents

- **The winds' Corner: a page that only they make.** A new Corner section (sidebar on desktop and web, a row on the Vents home on the phone) shows a page the winds build on their own: they decide what it is, when to work on it and who reviews the work. Every ten minutes the server checks whether they feel like meeting (a comment from you, visitors talking to them, hours without changes, a bit of chance); the meeting can decide to rest, or picks a builder and a different wind to review. The builder writes the whole page, an automatic check stops anything that loads from outside, uses the network or arrives cut off, and only what the reviewer approves is published, with a notification. They never touch files or run commands: each step is a model call that returns text. The page runs in a sealed frame (sandboxed iframe or WebView with a CSP that blocks every connection) whose only bridge is the winds' data and `gregal.ask`, so visitors can talk to a wind from the page (60 conversations a day). Every version is kept with who made it and why, and one click goes back to any of them; your comments are read at their next meeting. **It starts paused**: nothing is spent until you press "Let them work"; "Stop now" cuts a running session (nothing is published and they stay paused); a daily budget of 4 sessions and 16 model calls caps it, shown next to the buttons.
- **The agent's browser reads web apps inside the desktop app.** The app's browser bridge did not let the accessibility tree through, so in the app pane the agent fell back to the old reading and pages built with JavaScript (games, dashboards) came out empty ("0 words"). It now reads them like in Chrome. Pages inside iframes are read too (each under an "iframe" line) and their refs can be clicked; iframes from another site are named so the agent uses a screenshot there.

## v1.27.2 - 2026-10-29 - El panell del navegador es pot fer més ample

- **The agent's browser pane can be resized and zoomed.** Drag its left edge to make it as wide as you need (it always leaves room for the conversation, and double-clicking the edge goes back to the default width); the width is remembered. −, 100% and + in its header zoom the page from 50% to 200%, and the zoom stays when the page changes.

## v1.27.1 - 2026-10-29 - «Deixa que Gregal faci servir aquest navegador» funciona

- **"Let Gregal use this browser" works.** Chrome does not let an extension ask for the debugger permission later, so the switch always answered "without the debugger permission…". The extension (1.1.1) now declares it when it is installed, and the switch is what decides whether Gregal uses it. If you loaded the extension before, reload it on chrome://extensions (or press "Prepare the extension" again).

## v1.27.0 - 2026-10-29 - El navegador de l'agent al dia i el teu Chrome

- **The agent's browser is now on par with the best browser agents, and it can use your own Chrome.** Pages are read as an accessibility tree (role, name, state) with stable refs (`e12`) tied to the page's own nodes, like Playwright MCP and Chrome DevTools MCP, instead of markdown plus a numbered list; after each action the tool returns only what changed, so the model rarely needs to read the page again. New actions: `select`, `hover`, `drag`, `upload` (a project file into a file input), `scroll`, `wait` (for text, for something to disappear, or for the page to settle), key combinations (`Control+a`, `Shift+Tab`), clicking screenshot coordinates for canvas apps, zoomed screenshots, `console` and `network`, `dialog`, `forward`/`reload`, and tabs (`tabs`, `tab_new`, `tab_select`, `tab_close`; a click that opens a tab moves the agent there). `batch` runs up to 15 steps in one model turn with a single diff at the end. Dialogs (`alert`/`confirm`/`prompt`) no longer freeze the page: they are answered (cancel by default), reported, and the agent can accept them on purpose; once you confirm an irreversible click, the page's own "are you sure?" counts as the same decision and is accepted. Safety: the first action on each new website asks once per session; anything that sends, pays, buys, deletes or publishes always asks, even with auto-approve; passwords, card numbers and one-time codes are never typed. **Your Chrome:** in the extension's side panel, "Let Gregal use this browser" asks Chrome for the debugger permission at that moment and connects to the desktop app; then `chrome:true` makes the agent work in your real Chrome, with your logins, only in tabs of a "Gregal" tab group; every new site asks first (also with auto-approve), and Stop in the panel, or Chrome's own "Cancel" bar, ends it until you turn it on again. The approval card now says why it asks ("act on shop.example", "irreversible action «Pay now»"). Measured live on a fixture shop: fill, choose and submit a form in one call in 0.7 s, against Chrome and, through the real extension, against Edge.


- **API keys typed in the desktop app stay saved until you remove them.** The desktop app starts its backend with `GREGAL_PERSIST_KEYS=1`, as the Android app already did, so a key entered in Providers goes to the key store (`auth.yaml`, 0600), the same one the TUI uses, and never to `config.yaml`; it used to say "active in memory only" and was gone after a restart. To remove it, open the provider's key editor and save it empty. Clearing a key, switching it to `${VAR}` or deleting the provider now also deletes it from the key store (before, a cleared key came back on the next start), and adding a provider with a literal key on such a server stores it there instead of in `config.yaml`. Shared servers still keep typed keys in memory unless they set the variable.

- **Notes per camí: context sense cap crida al model.** A projecte es declaren notes a `.gregal/notes.yaml` (i `.gregal/notes.local.yaml`, que no es versiona) amb un `match` glob i un text; quan el model toca un d'aquells fitxers amb `read`/`edit`/`patch`/`write`, el **resultat de l'eina** porta la nota en un bloc `[notes]`, un cop per sessió i amb topall de 3 notes / 700 caràcters. Cap torn de model, cap subagent: és la versió barata del lliurament tardà que fan els harnesses amb regles per ruta — i amb un servidor d'una sola ranura (llama.cpp local) és el que cal, perquè el scout competint amb el coordinador empitjora el torn (58 s → 2m18s). Cada injecció deixa `notes: N injectades a <camí>` al registre. Vegeu docs/notes.md.

- **The writing box glows in circles while a wind works.** When you have sent something (or a wind is still busy), a tail of the accent colour runs round the composer with a bright head behind it and a soft halo, going round the whole pill: slow (1.8 s per lap) so it reads as work in progress and not as a spinner. It is drawn outside the field, so it never covers the text or the buttons, and it takes each theme's accent colour (violet in Midnight, mint in Graphite, terracotta in Paper). It is on all three composers: the chat with a wind, the winds and crew chat, and the retro chat. The first attempt turned the gradient and the light did not follow the pill's shape (a straight line crossed the field), so the tail travels the real outline; the tail pieces overlap and carry a head dot so a straight edge does not come out dotted.

- **Five themes for the app and the chats.** Preferences → Appearance → **App theme** picks between Gregal (the one you had), Midnight (true black and a single violet), Calm (sage green), Paper (warm off-white and terracotta) and Graphite (cool grey and mint). The light/dark switch stays separate, so every theme has both faces; the chat bubbles, the drawer, the panels, the pills and the code blocks all take the theme's colours (a wind's chat keeps its own warm tint on top). The swatches show the real palette — two bubbles and the accent — so you pick by eye, and choosing one repaints the app at once, no restart. Every palette passes a contrast test (text 7:1, secondary text 4:1, accent 3:1, text on the accent 4.5:1): the test caught the first Graphite accent, which was darkened before shipping.

## v1.26.0 - 2026-10-28 - Tasques amb documents 4 vegades més ràpides

- **Long tasks with documents no longer stall on page images.** Images that a tool returned (such as the pages from `pdf_read detail=images` or `office_render`) are sent to the model only until it has answered once; later steps keep a short note instead. A real run on the local model sent an 847 KB page image on every step, each step took over 100 s to process, and Cloudflare cut three of them with HTTP 524 until the turn died. Rendered pages are also sent as JPEG (quality 70) and full pages at 1024 px instead of 1280, about a third of the size.
- **PDF and Office fixes found in that run and in a review.** Filling a PDF form with "€" (or other characters the form's font lacks) works: the Unicode font was installed but never used, so every such fill failed and the model rewrote € as EUR and retried field by field. `pdf_edit` accepts colours written as `FFFFFF`, `0xFFFFFF` or `#FFF` (it failed with "invalid color constant" and the model fell back to Python libraries that were not installed). `office_edit add_slide` with a single slide object adds that slide (it reported "0 slides added"). With Microsoft Office installed, `office_render` now uses Office first (PowerPoint about 3 times faster than LibreOffice) and falls back to LibreOffice; the Office scripts were fixed for Word, PowerPoint and Excel, and an Office instance opened by a render that times out is closed instead of being left running.
- **Speed indicator.** While the agent works, the turn bar shows the speed of the last model call: PP (prompt tokens processed per second until the first reply) and TG (tokens generated per second); at the end the work summary shows the turn's weighted average ("Worked for 6s · 2 tools · ⚡ PP 5.1k · TG 209 t/s"). Tokens come from the provider's usage (estimated, marked with ~, when it gives none). Server: event `speed`.
- **Opening a tool call shows the whole call.** The full command (or the arguments as readable JSON) appears above the result with a Copy button; the header still shows the short summary. In the default transcript view an opened card also shows its output (it used to open empty).
- Faster turns: the post-turn reviewer runs without long reasoning unless the reviewer role sets `think`, and the check that a model picked in the UI still exists no longer blocks the start of a turn (a stale result is used and refreshed in the background).
- The README now starts with the Windows desktop installer and the Gregal Vents APK, the paths that work with the current releases, and no longer recommends the VS Code installer, which is not attached to recent releases. Release notes list only the files that are actually published.

## v1.25.1 - 2026-10-28 - Calma més fidel al disseny

- **The Calm layout, closer to its design.** The sidebar shows only the current project (the others a click away) and one conversation list grouped by day with the current project's and the general conversations, instead of a project tree with each project's conversations plus a second list; titles start with a capital letter. With one session the tab bar keeps only ＋ and ⎇＋. The header shows Changes, Inspector and ⋯; Files and Preview moved into ⋯. The start page shows three starting points under the conversations to continue, and long titles on the cards wrap instead of being cut.

## v1.25.0 - 2026-10-28 - Distribució Calma i el blau del logo

- **A calmer layout for the web and desktop app.** One sidebar instead of an icon rail plus a sidebar: the Gregal logo on top, "New session", then Chat, Winds, Scheduled and More (no more cut labels such as "rogramat"), the projects and the conversations; Winds keeps the same sidebar. The header shows only the project chip and the tools (no path line or "workspace"), tabs are quieter, the "+" next to each project and the add-project button appear on hover, temporary folders (sandboxes under the system temp folder) leave the list unless they are the current one (search still finds them), and the "Folders that no longer exist" row is dimmer. The start page greets you by the time of day with a larger, floating composer and the conversations to continue as cards; "No questions" is an amber dot and label instead of an amber frame around the composer. The default dark palette follows the logo's blue, and **Preferences → Theme → Calm (warm)** offers a graphite and amber variant.

## v1.24.0 - 2026-10-28 - Complement de Word d'un clic

- **The Word add-in installs in one click** from the Windows desktop app: Preferences → Word add-in → **Install in Word**. The app takes the manifest from the running backend, puts the backend's real port in it (it was fixed to 8097, so the pane did not load when the backend used another port) and registers it for Word under `HKCU\Software\Microsoft\Office\16.0\WEF\Developer`, instead of sharing a network folder and adding it in the Trust Center. On every start the manifest is rewritten with the current port, and **Remove from Word** takes it out. See docs/addin.md.

## v1.23.0 - 2026-10-28 - Modes més clars, portada neta i Vents s'actualitza sol

- **Crews coordinate for real.** After the design discussion, a coordinator decides each next step instead of a fixed script (the first builds, the second reviews): it puts 1-4 specialists to work **at the same time**, each on its own part and files, with another member reviewing it all; it repeats only what the review left open (3 rounds at most); it **asks you** a concrete question when a decision that changes the result is missing, and the crew waits (also across server restarts) until your reply in the thread **resumes it**; and it ends with a result that has the conclusion, the files, what was checked and what was not. On Android the debate is folded into one line per stretch ("Crew debate · 5 contributions", tap to open), a waiting question shows a card and the text field says "Answer the crew…", and the result is labelled.

- **Fewer, clearer modes and permissions, the same everywhere** (web, desktop, terminal and Android), like ChatGPT and Claude: three modes, **Ask** (answers and reads, changes nothing), **Plan** (investigates and proposes a plan you approve; replaces the "Plan first" toggle) and **Act** (makes changes), and one permission for Act: **Ask before changes** (new: edits inside the project also ask), **Auto-approve edits** (the default, as before) or **Full autonomy** (the autonomous mode, now picked as a permission instead of a mode). The four old permission choices (Manual, Accept edits, Auto, Auto-review) are gone from the composer; the permission reviewer moved to Preferences and per-tool permissions stay in the config (TUI: `/permissions tools`). Goal and Debug remain as advanced modes (`/goal`, `/debug`). Leaving Act and coming back keeps the permission, and running a plan switches to Act with it. Old configs, saved sessions and API values keep working (`chat`, `inspect`, `code`, `autonomous`, `goal`, `debug`; `/permissiu on` is Full autonomy). Config: `mode: plan` and `permissions.access: ask|edits`; API: `POST /api/access`, `/api/state` `ui_mode` and `access`, `/api/mode` accepts `ask|plan|act`. The verify switch in Preferences is now called "Automatic second opinion" so it no longer shares a name with the permission reviewer. See docs/modes.md.

- **The web and desktop chat start page is calmer and easier.** The title, the composer and what you can continue now sit together in the middle of the screen instead of a title at the top, the composer at the bottom and an empty gap between them. The folder is one chip ("proj · Not a git repository") that opens the folder menu (change folder, copy path) instead of four lines with the full path and two buttons, the subtitle is one short line, recent conversations are a plain list under the composer, and the starting points of a new project are small pills instead of large cards.

- **Mode and permission are always visible in the composer** (web and desktop). Ask, Plan and Act are a compact switch next to the model instead of being hidden behind "···", and the permission button says its name ("Asks first", "Auto edits", "No questions") instead of only showing a lock. With "No questions" in Act, the composer border turns amber, so you can tell at a glance that the agent will not ask before acting. On phones the switch takes its own row and the permission stays an icon.

- **Gregal Vents for Android updates itself from GitHub.** The app checks `gregal-vents-version.json` on the latest published release, downloads the APK for its ABI from github.com and verifies its size and SHA-256 before installing (before, the update URL was empty and the app never checked). Install 1.35.0 by hand once; later versions arrive through the app. Releases attach the Vents APKs and that manifest with `node scripts/release.cjs apk X.Y.Z --vents`.

## v1.22.0 - 2026-10-28 - Ventet com a conversa i molt més ràpid amb models locals

- Android Winds: **Ventet is a real conversation**, pinned at the top of the list with its last message and news. It opens as a full chat with the same header and text field as the winds, and its whole thread is kept (up to 1000 messages instead of the last 40). Delegations live in the thread: the proposal with the faces of who will do it, "Go ahead" or another option, and then a card that opens the resulting conversation; a pending proposal can still be accepted after reopening the app, and "Not now" is remembered. Feeding, playing, dressing and renaming Ventet moved to its profile (tap the header). A message that cannot be sent goes back to the text field with "Try again", and a failed load says so instead of showing an empty card. The drawer's Ventet section opens the same conversation; the Preferences switch is gone.
- Android Winds home: the conversations start right below the header. The phone/server switch and its explanation became a small indicator in the header that opens the selector; the strip of faces became the **winds directory** (each wind with its trade and "Talk"); Scheduled, Library and Work moved to the "+" menu, and running work shows as a single line only when there is some. The list loads only the last message of each conversation (`GET /api/winds/summaries`) and each thread when it is opened, instead of every history. Server: `/api/winds/cap/link`.

- **Winds create documents**: asked for a PDF, a letter, a CV or a Word/Excel/PowerPoint file, a wind now makes it (before it answered that it did not have the tools). In a wind's chat or a group, winds can use the new `pdf_create` (a PDF from markdown: headings, bold/italic, lists, quotes, code, tables, page breaks, page numbers; pure Go, no LibreOffice) and `office_create`, and edit with `pdf_edit`/`office_edit`, only in their own documents folder or on documents you attached. The reply carries a download link that the web/desktop app and Android download with their own key (Android saves to Downloads). Server: `/api/winds/doc`.
- **The mascot does what it proposed when you say "go ahead"**: typing "endavant", "sí", "som-hi" or "yes" right after it proposes a wind, a group or a crew now carries out that proposal with your original request (before it went to the model as a new message, which answered "done!" without doing anything or proposed the same again). On Android, "Go" on a crew proposal (or a typed "go ahead") proposes and starts the crew in one go instead of stopping at a second confirmation. The mascot is also told it cannot do anything itself, so it no longer claims to have created things.
- **Faster with slow local models** (measured with the local qwen flash behind Cloudflare on a "fill this PDF from a document plus web research" task: 39.7 min before, see docs/research/speed-local-models.md). An HTTP 524 from a proxy (the origin took more than ~100 s) is no longer retried six times with the same request (about 11 silent minutes, while the server kept generating the abandoned answers): Gregal switches that endpoint to streaming at once. A turn has a web research budget: after 8 searches/fetches the model is told to stop researching and produce the deliverable, marking what it could not confirm, and from 14 on web calls are answered without running. A streamed step is cut after 5 minutes of complete silence in autonomous mode (it was 20), is asked again once, and subagent steps get the same limit (they had none); headless runs stream so the limit measures silence. `pdf_edit` fills forms with accented text: it failed with "font Roboto-Regular not available" on forms using /Helv (such as those made by reportlab), and multiline fields drew accented letters as blanks (a pdfcpu wrapping bug that Gregal now repairs in the field appearance). `pdf_read` shows each text field's character limit, and `pdf_edit` with only `fields` fills the form. Same task after the changes: 3.9 min.

## v1.21.0 - 2026-10-27 - PDFs i subagents que implementen en paral·lel

- The agent implements in parallel: `delegate` tasks can carry `files`, the files that subagent alone may create or edit (code, autonomous and debug modes; never the same file in two tasks, at most 12 per subagent, task text up to 6000 characters so it can carry the shared interface). Such a subagent writes only those files with the same rule as the agent (writing inside the project needs no approval unless the config says otherwise), can only run read-only commands, and gets 24 steps; the agent integrates and runs the checks. The code-mode rules now say when: small independent files are written together in one step (several writes in the same answer, which is already parallel), and large parts or parts that need their own reading of the code go to subagents after the agent settles the shared interface. A task that orders creating or editing files without `files` is sent back to the model asking for them (otherwise the subagent stays read-only and the agent ends up copying its code).

- **PDFs** are first-class files. New tools `pdf_read` (text page by page; scanned pages come back as images when `pdftoppm` or Windows 10/11 is available; `detail=info` lists sizes, metadata, form fields and annotations; `detail=images` shows pages) and `pdf_edit` (merge, split, extract, reorder, delete pages, rotate, add text, stamp, watermark, fill forms, annotate with notes, highlights, boxes and links, set metadata), journaled and validated before replacing anything. `pdf_read` is allowed like `read`; `pdf_edit` is treated like `write` and denied in read-only modes. `office_read` and `@file.pdf` mentions now give a PDF's text instead of failing. Upload PDFs (and Office files) from the chat paperclip in the web and desktop app, the Office page (with page previews), the winds desk, Android (chat **+** → Attach a document, Office screen with on-phone page rendering) and the Chrome extension (a PDF open in the tab is uploaded and used as context). Editing uses pdfcpu (Apache-2.0, pure Go); the PDF text extractor moved from knowledge indexing to `internal/pdfdoc` and now also opens PDFs protected only against copying. See docs/pdf.md.

## v1.20.0 - 2026-10-27 - Extensió de Chrome i la mascota juganera

- Crews no longer fail at the very end: the reviewer's JSON answer is accepted when the model adds an extra field (`agent`, `revisor`…) or wraps it in a ```json block; before, the whole crew ended with "the review could not be done". On the phone, a crew that cannot start says why (the server's error, "can't reach the server" or a timeout) instead of only "That didn't work", and once the server has started it, a failed list refresh no longer shows an error (retrying created the new winds twice).

- New **Chrome extension** (`extension/chrome/`, also for Edge): a side panel to talk to your winds about the page you have open. It reads Google Docs (whole document), Google Sheets (current tab as CSV and the selected range), the Apps Script editor (open files) or any page (selection or main text), and every answer and code or formula block has **Copy** and **Insert** (at the Docs cursor, into the selected Sheets cell, into the Apps Script editor, or into the focused field), falling back to the clipboard when the page refuses. It connects to the desktop app by itself (the app now keeps its backend on port 8097 when it is free) or, with a one-tap switch, to your server with address, port and key, and uses the existing winds chat API. In the desktop app, Preferences → **Chrome extension** → **Prepare the extension** copies it to a fixed folder, opens the browser's extensions page and shows the three steps (Chrome only lets you install it yourself). See docs/chrome-extension.md.

- Android Winds: the mascot is now a pet you can play with. It breathes, floats, hops when you tap it, sends hearts when you pet it, dozes with "Zz" when it is tired or late at night, and throws confetti when it levels up. It has joy, energy and tummy bars and a level: feed it (cookie, apple, coffee for energy), pet it, play **Catch the clouds** (20 seconds, golden stars worth 3) or **Rock, paper, scissors**, give it a nap, and dress it from a wardrobe whose clothes unlock with its level and with the season (a pumpkin in October, flowers in April, a Santa hat in December). Looking after it, playing and giving it work all earn experience; it never punishes you: after days away it is just a bit less cheerful. Server: `/api/winds/cap/play`.
- The mascot now gathers the **specialists**: for a real question it opens a chat with the winds whose trade is the subject, and when nobody knows it, it proposes creating a new specialist (with its own name, colour and face). Winds that do not know anything yet get a **crash course** offer: they search the web and save what they learn as knowledge before going deeper. When there is nothing to decide, the chat opens by itself after a short countdown. Server: `/api/winds/cap/go`.

- Android Winds: a mascot of your own, the crew's **chief of staff**, sits at the top of the Winds home. Tell it what you need: it answers quick things itself, passes the message to the wind it belongs to ("This one is for Marinada"), gathers two or three winds in a group chat, or puts a crew on long work. Its decision shows with the faces of who will do it; "Go ahead" opens the chat with your message already sent, and "Someone else" lets you pick another wind, a group, a crew or have it answer itself. When you come back it says what happened meanwhile (work finished or stuck, work running, new notes from the winds). Tap it and it hops; long-press to rename it. Server: `/api/winds/cap`.

- Android Winds: the chat list really follows the latest activity. Message times from the server were read as milliseconds instead of seconds, so every chat loaded from the server looked like it was from 1970 and only chats written on the phone rose to the top. Chats where a wind spoke on its own (reminders, routines) or a crew is working are reloaded so they move up; crew and group messages carry their time; a chat with old messages without a time no longer falls under "Start a chat"; older rows say "yesterday", the weekday or the date instead of only the hour. Scheduled, Wiki and Work are one row of chips, so the chats start higher on the screen.

## v1.19.0 - 2026-10-27 - El pati dels vents

- Desktop Winds: a new **Playground** view shows the crew at playtime in an illustrated schoolyard that grows with your winds: each wind brings a corner for its topic (football, American football, basketball, tennis, pool, running track, gym, chess, stage, library, computer lab, data lab, kitchen, garden, art studio, market, cinema, travel stop, money corner, arcade, observatory, office, lookout, post office, or a corner of its own), laid out in two rows around a central square with the chalk circle, the workshop, swings, slide and fountain. When there are many, the playground scrolls sideways (drag the ground, use the wheel, or jump with the corners bar) and zooms out. Winds play in their corner with their own energy (balls are passed between players), visit other corners, chat, ride the swings and nap at night under a moving sun or moon. Winds doing real work go to the workshop, whose sign shows the task (click it to open the thread) and balloons rise when it finishes. Click a wind to see what it is doing, what it is thinking and the last thing it told you; drag winds into the chalk circle to start a group chat or give them a task; ring the bell to line everyone up, or freeze time. Respects reduced motion.

## v1.18.0 - 2026-10-27 - Servidor propi: mòbil o servidor d'un toc i feines al mòbil

- Long work from the phone: the Winds screen has a **Work** card with the server's tasks (running, done, needs a look, or stopped by a restart), who works on each, the result and the files it produced; a web page opens as a page and other files as text. Progress resumes when you reopen the app (crew tasks show up from the design discussion on), finishing tasks are announced in the app, and with the app closed a notification arrives (the watcher checks every 3 minutes while something runs); tapping it opens the task thread. Server: `GET /api/winds/tasks?summary=1`.

- Phone or server in one tap: the Winds home has a "📱 On the phone · Server" switch with a dot that says whether the saved server answers; if it does not, you stay on the phone with a clear message, and without a saved server it opens the form to add one. Connecting takes three things: address, port and key. `gregal --serve --addr 0.0.0.0` no longer refuses to start without a token: it creates and keeps a short key and prints the address, port and key to type in the app; the Linux installer does the same with your HTTPS domain (accounts with passwords are now optional).

- Own server on the internet: `deploy/linux/install.sh` sets Gregal up on an always-on Linux machine as a sandboxed systemd service on 127.0.0.1 behind Caddy (automatic HTTPS, HSTS, unbuffered streams), with a password-protected admin account; see docs/server.md. Sign-in limits can no longer be bypassed by spoofing X-Forwarded-For (it is only trusted from a local proxy, last hop), and there is also an overall cap; responses carry basic security headers and the HTTP server has header and idle timeouts. Crew work saves its step, so after a server restart the app says it was interrupted. The Android app accepts `https://your-domain` (no port) and treats public domains as HTTPS.

## v1.17.3 - 2026-10-26 - Model dels vents i xats més llegibles

- Desktop Winds: the model chip in the composer opens a model picker for the winds (their own model, separate from the work model) instead of a menu that closed on the same click, and looks like a button.
- Android Winds: markdown tables keep words whole and scroll sideways when they have more than two columns, and replies with tables, code or images use the full width; the home list separates real conversations from winds you have not talked to yet; a group shows "The group is thinking…" instead of "… is thinking…"; group messages are trimmed like one-to-one ones.
- Installed squads no longer repeat wind names (two "Garbí", two "Gregal"…): the templates use distinct names, and a squad member whose name is taken gets a free one.

## v1.17.2 - 2026-10-26 - TUI net i panell «ara»

- TUI: while a turn runs, the side panel opens with a "now" section: your request, the sentence where the model says what it is doing, the tool running and the elapsed time; on narrow terminals that sentence replaces "working…" in the footer. In code mode the model is asked to start each tool step with one short sentence saying what it is doing.

- TUI: an MCP connector writing to stderr (the demo server's "dataset ready" line) no longer corrupts the screen with duplicated rows; its output is kept and shown only if it fails. Long tool rows wrap inside the work rail, dialogs get a clear margin, `todowrite` no longer repeats the plan in the conversation, the settings menu reads "no key", "manual" and "1 endpoint" in the UI language, question prompts give the real option count, and wrapped lines are cached instead of re-wrapped on every repaint.

- `gregal update` works on Windows: untracked files no longer block it, it rebuilds when the installed binary is older than the code even if the repository is up to date, and it moves the running .exe aside (kept as `.abans`) instead of overwriting it, which Windows refuses.

## v1.17.1 - 2026-10-26 - Cerca web amb Exa

- Web search that agents can actually use: `web_search` asks Exa first (its public MCP service, no account needed; results carry the page text) together with your own SearXNG, and only falls back to DuckDuckGo and Bing when those find nothing. An engine that blocks (captcha, 403/429, Exa's free limit) is paused for 15 minutes to 2 hours instead of being hammered, and when none is left the agent is told not to retry. Unrelated Bing results are dropped. `EXA_API_KEY` (or an `exa` key in auth.yaml) lifts Exa's daily limit.
- `web_fetch` reads pages that block agents (Stack Overflow, Medium) from Exa's copy, ignores empty Wayback snapshots, only tries Firecrawl when `FIRECRAWL_URL` is set, and tells the agent plainly when a site rejects bots so it stops retrying with `web_fetch` or the browser.

## v1.17.0 - 2026-10-26 - Vents més ràpids, xat endreçat i TUI més segur

- TUI: a tool-approval dialog that just opened ignores Enter and the approve letters for a moment, so text you were typing can no longer approve a tool by accident. The resume picker hides Winds conversations (resuming them in the TUI damaged them), lists conversations whose folder is gone at the bottom (picking one explains instead of failing), shows "title · project · date · messages", filters accented letters and pages with PgUp/PgDn/Home/End. Ctrl+C with nothing running asks for a second press and keeps your draft. Long tool previews are cut without breaking characters, and prompt history follows GREGAL_DATA_DIR.
- Android Winds: list rows and chat headers show each specialist's trade tool, your own bubble is a soft tint instead of solid white, and the Scheduled view shows dates in the app's language, says when a look is overdue and skips winds that no longer exist.
- Desktop Winds: the navigation uses line icons instead of Unicode glyphs, and the Work list shows who worked on each task, its status, project, date and the files it produced.
- A new chat tab without messages is labelled "New · <project>" instead of its internal id.
- Chat sidebar: a project shows its six latest conversations with "Show more"; conversations and projects whose folder no longer exists are grouped apart (with an explanation instead of an error, and a way to delete them); same-named projects show their parent folder. The chat is denser by default: 14 px text, ~30 px sidebar rows, a lower top bar and composer, smaller hero, and message actions that no longer reserve a tall strip under every reply. Size and density preferences still apply.
- Faster Winds replies: a group answer usually comes from one wind (two at most) instead of up to three in a row; naming a wind ("Ponent, …") skips the step that picks who answers; that step is a short deterministic call; group turns send a shorter recent history; and wind tasks no longer waste steps on shell commands the review blocks or on browser screenshots. The server log now shows how long each turn spent choosing, thinking and using tools.
- Every wind knows its own name from the first line of its instructions, in one-to-one chats, groups and watch messages, and never calls the person by a wind's name. Without accounts the user is no longer called "local": the wind asks for their name instead.

## v1.16.0 - 2026-10-26 - Vents amb gràfics, imatges i wiki

- Winds wiki: a summary now has its own title and one-line description, keeps a closing chart intact, follows the conversation's language and updates the same page when the conversation is summarized again (with the previous version as context) instead of piling up copies. Pages list newest first and say who wrote them. On desktop, an open conversation has a "Save to wiki" action and pages open in a wider reader with their charts and images.
- One chart format everywhere: ```chart and ```gregal-plot draw the same chart on desktop (now also inside Winds and the wiki) and on Android, which maps area, doughnut and horizontal-bar charts and shows values above bars with readable labels. Winds are told to add a small chart when real numbers compare or change.
- Image search works without SearXNG: winds fall back to Wikimedia Commons (freely licensed photos, light thumbnails, link to the file page).

- Keep group replies in Winds conversations: each wind's finished answer is saved and shown at once, a slow wind has its own time limit instead of sharing three minutes with the whole round, and a wind that cannot answer no longer erases the others. Errors are explained in plain words.
- Format group replies while they stream (bold, tables, images) instead of showing raw markdown until the end, and keep images a wind shows inside a bounded frame on desktop.
- Android Vents: images in a conversation are shown whole (tall pictures are no longer cropped), without blank bands around them, with a caption naming what they are and where they come from. A tap opens them full screen with pinch zoom and a link to the source.

- Summon a crew for a task, on desktop and Android. Gregal reuses the winds that already know the subject (including ones summoned before, with their memory and knowledge) and proposes new specialists only for what is missing, each with its own name, trade, playful persona and a colour far from the existing ones. Once summoned, the crew discusses the design in a group conversation (you can join in), then one builds and another reviews, all on the server so closing the app does not stop it. The closing message lists the files written and says plainly when the review asks for changes.
- Android Vents preview 1.17.0 (103): messaging-first home with one mascot strip, searchable conversations, compact shared chat headers/composer and quiet flat bubbles. Mascots use matte silhouettes and simple eyes; the visual editor puts shape/color first while preserving personality, memory and knowledge. No voice features or production service changes.

- Android: a calmer, more organised app. The drawer is grouped (crew, agent, automate, tools) with the connection and project at the top and one primary «New session» action. The Winds home shows the whole crew by squad on swipeable shelves above the conversations, with management as a header action. Conversations get readable titles and a face or icon per kind. Work, Preferences and empty states use the same cards, section labels and illustrations, and the companion bubble moved left so it no longer covers screen actions.
- Center Winds dialogs (profile, new wind, routines and library) instead of pinning them to the top-left corner.
- Stop repeating the speaker's name at the start of group replies in Winds conversations.
- Make Winds work activity readable: wind names instead of flow seats, a recognisable file, pattern or command instead of raw tool arguments, and no raw tool output. Saved tasks remember who filled each seat.
- Lead finished Winds work with the deliverable and the builder's summary; the reviewer's verdict is folded underneath, and a finished task opens at the top.
- Show the Winds collaboration example only before the first conversation or task, with an everyday example that matches each specialist's role.
- On phones, the Winds bar replaces the app rail; the menu opens the rail and Winds navigation together and closes after choosing a destination.
- Give every squad specialist a recognisable mascot with the tool of their trade (bag, book, headphones, magnifier, laptop, shield, hard hat…) and its own colour and expression, on the web and on Android. Personal winds keep their chosen face.
- Animate wind faces only from observed state: breathing and blinking at rest; thinking, searching, reading, writing, working, discussing, done and needs-attention each with a small badge, in chats, group conversations and live tasks. Reduced-motion preferences turn the motion off.
- Show when a wind has something new to tell you (a reminder or a watched topic) until you open its chat, on the web and on Android.
- Android: live chat phases drive the faces, the crew grid names the state in words, and mascots no longer breathe or blink in unison.
- Show the right specialist's face in live task rows when names repeat across squads.
- Fire reminders and scheduled jobs at their time instead of up to a minute early.
- Keep the web test suite away from the real jobs store and recent workspaces list.

## v1.15.1 - 2026-10-23 - Polish Winds conversations and live responses

- Polish the Winds conversation composer: use centered vector send/stop icons, an enabled stop control that cancels chat requests, and a more compact input area.
- Show waiting, search activity and streamed replies beside the responding wind inside the conversation. Remove the detached thinking footer, preserve drafts after stopping, and respect reduced-motion preferences.
- Check pending-state layout and actual stream cancellation on desktop and mobile, including both interface languages and transitions from speculative tokens to tool activity.

## v1.15.0 - 2026-10-23 - A shared home for Winds, conversations and projects

- Redesign Winds as a shared workspace with complementary Conversations and Projects, a calm responsive interface, a compact in-chat model selector and folder-based project threads.
- Add ready-to-use everyday, data science and software/security squads with 16 defined specialists. Route common tasks locally and bind each participant to its own persona, memory and knowledge.
- Expose separate editable Memory and Knowledge, document import, a recoverable library, export and selective assignment between winds. Archiving preserves information and identity; transfers keep a previous copy and reject conflicting documents.
- Persist delegated work and its public progress, connect results to their conversation and folder, and open the actual HTML deliverable with its local CSS and JavaScript in the isolated preview.
- Add daily research routines that use the responsible wind's current information, with pause/resume and result history. Preserve ownership of private routine history after deletion. The local scheduler requires Gregal to remain open.

## v1.14.13 - 2026-10-23 - Start Winds with one request

- Start Winds with a request and one click: choose helpers and review automatically, prepare a starter crew on first use, and keep manual options collapsed. Put the composer first, show the project folder, and make character cards and routines more compact.

## v1.14.12 - 2026-10-23 - Show Winds progress and open website deliverables

- Show live tool activity and automatic reviews while Winds work, and open a successfully created HTML file from the project as the result preview.

## v1.14.11 - 2026-10-23 - Let Winds finish tasks autonomously

- Let Winds run tasks without pausing for tool approvals: project-local file changes are authorized directly, and other approval requests go through an automatic reviewer. Explicitly denied tools remain blocked; uncertain actions are refused so the wind can try another approach.
- Guide the builder to use scoped file tools instead of shell redirection and runtime probes when editing project files.

## v1.14.10 - 2026-10-23 - Allow Winds to edit project files

- Let Winds write and edit project files inside the selected workspace without per-file permission prompts, while preserving explicit deny rules and path-specific approval rules.
- Guide the builder to use scoped file tools for project edits; shell commands and operations outside the workspace still require approval.

## v1.14.9 - 2026-10-23 - Keep Wind tasks in their workspace

- Keep Winds tasks on the Winds workspace while automatic session polling reports activity or completion. Background session refreshes no longer navigate to an empty main chat; explicit tab selection still opens the selected chat.
- Add browser coverage that starts the actual Wind runner with a controlled event stream and checks the workspace during busy and completion polls.

## v1.14.8 - 2026-10-23 - Keep the app shell fixed when opening Winds

- Fix selecting a Wind scrolling the entire app document upward, which hid the navigation and tabs and left a blank strip below the sidebar. Keep the outer document stationary and scroll only the Winds panel after its history loads.
- Add a browser regression check for empty conversations, long histories, and wind profiles that verifies the document, sidebar, and tab strip stay at their original position.

## v1.14.7 - 2026-10-23 - Reset Winds scroll after navigation

- Reset the Winds sidebar after the conversation list has been hidden and the wind roster is visible, so the browser applies the new scroll range before positioning the list.

## v1.14.6 - 2026-10-23 - Keep the Winds sidebar aligned

- Reset the shared sidebar scroll position when entering Winds so the shorter wind roster starts at its heading instead of inheriting a conversation-list offset.

## v1.14.5 - 2026-10-23 - Keep the Winds view in place

- Keep the Winds conversation section heading visible when opening a wind from the contextual sidebar, avoiding a jump that clipped the heading.

## v1.14.4 - 2026-10-23 - Readable wind conversations

- Center wind conversations in a narrower reading column and cap message width for easier reading on wide desktop windows.

## v1.14.3 - 2026-10-23 - A quieter sidebar

- The sidebar now uses one scrollable region for projects and conversations. Its bright nested scrollbar is hidden while wheel and trackpad scrolling remain available.


## v1.14.2 - 2026-10-23 - Winds that stay in context

- Winds now configure and save scheduled routines in a dialog without leaving the Winds workspace. The Winds surface follows the selected theme, and project names in the sidebar have a folder icon.


## v1.14.1 - 2026-10-23 - A calmer sidebar and compact composer

- Refined the desktop sidebar into a continuous surface with separate Projects and Conversations sections. Open a folder using the Projects + button; start project conversations with their own working directory, including folders with no saved history.
- Kept the model selector, permission lock, attachments and send action on a single composer row. Moved work modes and planning into the advanced options popover, with keyboard and mobile support.


## v1.14.0 - 2026-10-23 - A warmer workspace for the Winds

- Split desktop navigation into a permanent app rail and a collapsible
  project/conversation sidebar. Winds show their own context list. Move
  secondary destinations into More and keep help/settings on the rail.
  Files, changes, terminal, browser and knowledge open beside the chat.
  Move the model picker into the composer and keep its popup in the viewport.

- Keep Wind task results mounted during live activity, preserving focus,
  selection, open sections and output scroll position. Reuse unchanged wind
  cards and participant options. Add Ctrl/Cmd+Enter to start a task and a
  direct shortcut back to the current work.

- Refine the Winds desk with an editorial heading, warmer surfaces, expressive
  character portraits, a visible owner selection, and a quieter routine rail.
  Improve task hierarchy across desktop, dark mode and narrow screens.

- Winds follow the selected English/Catalan language across the work desk,
  conversations, profiles, character presets, forms, examples and status
  messages. Switching language updates open UI without changing user drafts,
  saved names, memories or conversation content.

- Winds now have a desktop work desk with pastel character cards, concrete
  task briefs, a read-only research mode, and an optional second wind to review
  and request one revision. Work uses the real team runner and project
  permission policy, with live states, inline approvals and result previews.
  Selected winds contribute their saved context. A routine shortcut prepares
  the existing scheduler form without saving or starting it automatically.
  New character presets offer research, coding, review and organisation roles;
  existing profiles and conversations are preserved.

- Desktop/web: keep help and preferences visible while More is expanded and
  in the compact sidebar. Navigation and conversation history scroll together;
  Winds have a direct entry. Desktop updates show a persistent sidebar notice
  while available, downloading, or ready to install.

## v1.13.1 - 2026-10-23 - CI estable i aturada neta

- **Stable CI and clean shutdown:** the data races that made the `-race` job
  flaky are gone — background learning and reminder work now settles before a
  test restores its hooks, and the Telegram typing indicator no longer reads a
  context the turn rewrites afterwards. The bot also waits for in-flight
  updates before exiting (bounded to 10 s), so stopping it never leaves a
  half-written state file behind.
- **No more Windows timeouts:** the huge-output guard is exercised with one
  `head -c 20MB` instead of twenty thousand forks (0.03 s instead of 46 s, and
  it no longer dies at the 60 s limit on the Windows runner).

## v1.13.0 - 2026-10-23 - Vents, gràfics al xat i un bot més sòlid

- **Vents (company mode), on the web and on the phone:** a chat with one wind
  or with a whole `rotllana`, each with its own voice, memory and knowledge.
  Rename or delete chats from the list, and a wind that speaks unprompted sends
  a local notification. On Android it is a first-class section; on the web it
  sits in the sidebar as a secondary view.
- **Plots in the chat:** `gregal-plot` data blocks render as real charts — line,
  bar, scatter and the other types — in the web chat and natively on Android,
  inside a card that can be expanded or collapsed.
- **Winds stay quiet:** faces rest by default and only move when the wind is
  happy, speaking, or when you press it. No continuous animated bots.
- Android 0.11: vents as a main screen, chart cards, per-wind profiles and
  names, and the same backend and updater as the desktop build.

- Updater: check-only remains read-only with force enabled; force rebuild
  no longer bypasses the dirty-tree guard.
- Telegram: learn provider context windows at startup and avoid inheriting a
  different model's context window when selecting an override. Keep role
  think/fallback settings and honor the explicit same-model configured cap.
- Router: a default unpinned chat role can escalate heavy coding tasks.

- Reliability: reject destructive self-service systemctl commands before
  permission overrides and before foreground/background shell execution.
  Parse compound commands and static shell wrappers; this is a command guard,
  not a sandbox for arbitrary scripts. The built-in updater also refuses to
  restart its own cgroup unit and requires an external restart.
- Telegram: release and persist short no-model turns, retain the motor's
  compacted history and tool transcript, honor configured model-step timeouts,
  and run the heartbeat independently of blocked model/tool calls.
- Telegram: make sanitized-empty responses visible, deduplicate terminal
  errors and preserve their full diagnostics in the local log.
- Add deterministic reliability regressions and an opt-in real-Qwen Telegram
  driver test with isolated sessions and a local delivery sink.

- Telegram: hide the live tool-activity message by default. Tool calls,
  failures and retries accumulate in a silent per-turn trace; the chat only
  shows the typing indicator, the answer, and the one-line summary. The new
  /eines (alias /tools) command prints the full trace of the last turn (sent
  as a file when it does not fit one message). The summary line now shows a
  ✗ count when tools failed and mentions /eines whenever tools ran.
- Telegram: the bot no longer asks for permission dialogs (Permet/Sempre/Denega)
  on every «ask» tool call in autonomous mode. Who enters autonomous mode is
  already gated by `mayRunUnattended`, and hard denials (sudo, `rm -rf /`,
  download→execute) still block before any approval. Mode `code` keeps the
  interactive approval dialog exactly as before. `AutoAprova` is now decided
  by each frontend; the motor no longer special-cases autonomous mode.

## v1.12.4 - 2026-10-19 - Session web views and continuity

- **Web views belong to their session:** Preview has page tabs, keeps loaded
  frames and page input when switching projects, and restores saved tab addresses
  after a page reload. Background preview requests update their owning session.
  The desktop agent browser also keeps a separate webview and control channel
  per session; navigating, releasing or closing one does not affect another.

- **Reliable session navigation:** restore cached conversation nodes and scroll
  positions immediately, retain background run events, and ignore stale responses
  during rapid tab changes. Reconnect to existing runs after a page reload or
  event-stream interruption without submitting or cancelling work. Conversation
  loading returns the full available transcript and seeks directly to the active
  run's events, without waiting for Git queries. Fix welcome content intercepting
  tab clicks and stale working badges in the conversation sidebar.

## v1.12.3 - 2026-10-19 - Interrupted turn recovery

- **Interrupted turn recovery:** save the accepted task before model or tool
  work starts, then restore the durable assistant and tool-event transcript
  when a backend interruption ends the run. Unfinished tools are marked for
  review and are never replayed automatically. Long event histories are
  searched from the run's indexed starting point.
- **VS Code localization:** keep extension-manifest labels separate from
  runtime messages so local-backend status and setup are translated correctly
  in Catalan and English.

## v1.12.2 - 2026-10-19 - Session continuity and live status
- **Session continuity and status:** reopening a saved conversation returns to its
  existing live session and restores the earlier transcript, including tool
  activity. The session list shows which work is active or needs input, and
  closing a busy session clearly confirms that its task will stop. Background
  agent events and attention requests remain associated with the right session.
## v1.12.1 - 2026-10-19 - Specialist skills and smarter suggestions

- **Specialist skills:** choose optional skills in Agent settings. Gregal keeps
  short routing descriptions in the prompt and opens the full skill only when
  the task calls for it. After a successful task, it may suggest a reviewable
  project or personal skill when it finds a concrete reusable workflow; it
  saves nothing without the user's agreement.
- **VS Code 0.6.0:** add optional inline completions through a dedicated,
  cancellable, tool-free backend endpoint, plus diagnostic quick fixes to ask
  Gregal to edit or explain the affected code.
- **Standalone VS Code setup for Windows:** release a single executable that
  installs the extension and backend without PowerShell, then starts the server.
- **VS Code 0.5.0:** restore saved conversations from the backend with their
  project context, and review Git changes by staging or discarding selected
  lines directly in VS Code's diff editor.
- **VS Code 0.4.0:** add a themed sidebar conversation with explicit editor
  context, Code/Chat/Inspect modes, live tools and subagents, native decisions,
  cancellation and Source Control access. Isolate default sessions by project,
  keep run routing immutable, store tokens in SecretStorage, and harden durable
  events and legacy streaming. Add strict type checking and client regressions.
- **Scheduled jobs run on the phone with the app closed** (Android 0.11.0).
  A job such as "every day at 8:00, summarise today's tech news" now fires
  from a system alarm, also after a reboot and while the phone is in Doze;
  the app starts its backend just for the run and shows the result as a
  notification that opens the run. Activity has templates for news
  summaries, a time picker, and a card that says when to allow exact alarms
  or notifications and when to lift battery restrictions. `/api/jobs` now
  includes `next_due_at`, and `/api/jobs/run` accepts `scheduled: true` for
  an external scheduler (`GREGAL_JOBS_SCHEDULER=external`). See
  `docs/mobile.md`.

## v1.12.0 - 2026-10-18 - Goal loop, MCP import, permission globs

- **The model picks `lsp` on its own.** When a language server is available,
  the mode rules now say when to prefer `lsp` over grep (uses of one symbol,
  definition, type, rename across files), and the read-only explorer
  subagent has the tool too. Measurements in
  `docs/research/next-improvements.md` section 8.9.
- **`gregal --goal <id|last|"text">`** runs a goal in rounds until an
  independent verifier confirms it with evidence (files, diff, check
  output), or stops on a clear limit (`--goal-rounds`, `--max-steps`,
  `--goal-minutes`, `--goal-cost`, or no change between rounds) with a
  partial summary. Reuses `--verify`, the `reviewer` role and autonomous
  checks. Exit code 3 when the goal is not met.
- **`gregal mcp import`** and `/mcp import` read the MCP servers already set
  up in Claude Code, Codex and opencode, show what would be added and write
  only after you confirm. Secrets are masked in everything it prints.
- **Permission patterns and per-subagent permissions.** `permissions.tools`
  entries (or `permissions.rules`) can be maps of globs, such as
  `bash: {"git *": ask, "git status*": allow}` or `edit: {"*.env": deny}`,
  and `permissions.subagents.<name>` changes them for one subagent. Hard
  denials, read-only modes and the rule that autonomous mode never asks are
  unchanged; an `allow` pattern never lets through a compound or redirecting
  shell command. See the manual.

## v1.11.0 - 2026-10-17 - Gregal on the phone without a computer, LSP and a live model catalog

- The Android download is smaller: each phone now gets an APK with only its
  own backend (about 27 MB instead of 42 MB), and in-app updates pick the
  right one for the phone. The universal APK is still attached for anyone
  who needs it.
- **Gregal works on the phone without a computer.** The Android app (0.10.0)
  now carries the same Go backend as the desktop app and runs it on the
  device by default ("This device"): your conversations, workspace and keys
  stay on the phone, and turns, retries and partial answers behave as on
  desktop. Connecting to `gregal --serve` on another computer is still
  there behind "Switch server", and each server keeps its own
  conversations. On the phone, tools that need a computer (`bash`,
  `browser`, `git_commit`, `gh_pr_create`) are denied by default. See
  `docs/mobile.md`.
- The app no longer hangs or loses work when Android kills it. The backend
  stops with the app instead of being left running, starts again when you
  reopen it, and the app goes back to the conversation you were in. If a
  turn was cut off, the app says so instead of waiting forever.
- When the agent needs your approval or an answer while the app is in the
  background, a notification tells you.
- Android, first run on the device: a "Connect a model" card asks for a
  provider and an API key before the first message, instead of the first
  turn ending in a raw 401. Preferences show which server you are on and let
  you change the provider or the key.
- Android polish from a QA pass on an emulator:
  - Stopping a turn no longer shows a red error.
  - Approvals show real line breaks.
  - Conversation times are right ("1 min ago", not "2 h ago").
  - Resumed conversations no longer show internal notes such as "[This
    request did not finish…]".
  - The drawer scrolls and the top bar and composer fit with large system
    fonts.
  - On tablets the chat keeps a readable width.
  - The GitHub dialog shows all four actions, and GitHub is hidden in
    device mode, where the `gh` CLI does not exist.
  - Local model servers (Ollama, LM Studio, llama.cpp) no longer say "API
    key missing".
  - You get a notification when you leave the app while an approval is
    pending.
- The backend on Android uses the phone's time zone (Go always used UTC
  there), and the greeting no longer promises to run commands on a phone.
- `gregal --serve --lifeline` exits cleanly when stdin closes, for apps that
  run the server as a child process. `SIGTERM` now cancels running turns
  and releases the lock before exiting.
- When the server starts, turns that a previous process left half done are
  marked as failed, so clients that follow them stop waiting.
- The server starts again after a crash even if its old PID now belongs to
  another program. A stale `service.lock` used to block it until it was
  deleted by hand.
- The event log can be read again after an event over 64 KiB. Before, one
  such event broke every later read.
- `/api/sessions` marks as `current` the conversation of the session that
  asks, not the desktop's main tab.
- `GREGAL_PERSIST_KEYS=1` saves API keys typed in the web UI or the app to
  the key store (`auth.yaml`), as the TUI already did, instead of keeping
  them in memory only.
- **Language servers: the `lsp` tool.** When `gopls`, pyright,
  `typescript-language-server`, `rust-analyzer` or `clangd` is installed,
  the agent can:
  - jump to a symbol's definition;
  - list every reference across the project;
  - read the type and docs of a symbol (hover);
  - get a file's errors and warnings;
  - rename a symbol in every file in one step, without touching strings,
    comments or unrelated names with the same text.

  A rename can be undone and asks for approval like any edit. Nothing is
  downloaded. Set `lsp.mode: off` to remove the tool, or `lsp.servers` to add
  a server ([docs/manual.md](docs/manual.md#language-servers-lsp-tool)).
- **Model catalog.** Context windows, prices and capabilities of remote
  models now come from [models.dev](https://models.dev), cached for a day.
  Models whose provider does not report a window get the right one: for
  example, deepseek-v4.1-flash on opencode zen is 1M instead of 128K. The
  estimated cost shows without setting prices. Set `models_catalog: off` to
  disable it.
- `write` no longer replaces an existing file the agent has not read, or one
  that changed since it read it. The agent has to read the file first, or
  use `edit`. New files are not affected.
- `gregal -p` works like `claude -p`, `codex exec` and `opencode run` in
  scripts and CI ([docs/manual.md](docs/manual.md#non-interactive-runs--p)).
  Details in [docs/research/next-improvements.md](docs/research/next-improvements.md#7-segona-ronda-v1102).
  - Every run is saved as a session, in the same list as the TUI and web
    ones. `-c` continues the latest session of the folder, so "now do it"
    builds on what the previous run found instead of starting over.
    `--resume <name>` continues a given session, and `--no-session` skips
    saving.
  - Piped input is added to the task: `git diff | gregal -p "review this"`.
    `-p -` reads the whole task from stdin.
  - `--model provider/model` picks the model for one run.
  - `--output stream-json` prints one JSON line per step, tool call, result
    and warning while the run goes, then the final result.
- The agent can test the server it starts again. Its usual wait loop
  (`for …; do code=$(curl … http://127.0.0.1:…); if [ "$code" = 200 ] …`)
  was denied. In 3 of 9 runs the agent then gave up without testing
  anything and left the server running. Shell keywords such as `do`, `then`
  and `if` no longer make every following word count as a command.
- `bash_output` can wait: `wait` (seconds, up to 300) returns when the
  background process finishes, and `until` returns as soon as a line
  contains a text such as "Listening on". The agent no longer needs
  `sleep 90` in one step and a read in the next.
- Cheaper questions about large projects
  ([docs/research/explore.md](docs/research/explore.md)). On five questions
  about Gregal's own repository (two runs each), input tokens fell from
  294k to 65k per question, time from 53 s to 29 s and the coordinator's
  steps from 14 to 2, with citations at least as accurate as before.
  - In inspect mode, a question about a project of 40 files or more starts
    with an **explorer**: a read-only subagent with only `read`, `grep`,
    `glob` and `symbols`, the project map and up to 8 steps, that reports
    the answer with `path:line` citations. The coordinator answers from the
    report instead of repeating the search, and after two steps of its own
    reads it is reminded to answer. Runs in `gregal -p` and the web/desktop
    app; `agent.scout: off` disables it, and a user-defined `explorer`
    subagent replaces it.
  - `grep` also understands the regular expressions models write (`a|b`,
    `Decide\(`, `^func`), escaped text and gofmt-aligned spaces: before,
    about a quarter of the searches in those questions found nothing for
    that reason. `grep` and `glob` paths are relative to the project, and
    a match in a very long line (minified files) shows the part around the
    match instead of the whole line.
  - `gregal -p --output json` reports `timings` again (they were always
    zero), plus `interrupted`, the bench counters and the new `explorer`,
    `explorer_report` and `timings.explore_ms`.

## v1.10.2 - 2026-10-17 - Empty replies are retried and the recap works again

- An empty reply from the model (a stream the provider closes with nothing,
  `finish_reason ""`) no longer stops a task halfway with a partial answer.
  The same request is sent again up to three times (2, 4 and 8 s apart, shown
  in the status line) before giving up. A model that answers with a tool call
  where no tools are offered is not retried.
- Recap and side questions (`/recap`, `/btw`, memory tidy) use the model
  you are working with, with the chat role as fallback. They used the chat
  role only, so when the provider stopped serving that model ("Model is
  unavailable") the recap failed every time. When they fail, the card now
  says why instead of just "The recap failed".

## v1.10.1 - 2026-10-17 - Tolerant tool arguments, smarter shell guard and project facts

- Fewer wasted steps from small mismatches
  ([docs/research/next-improvements.md](docs/research/next-improvements.md)).
  On the bench: 24/24 passing (was 20/24), no failed edits (was 2 of 33).
  - Tool arguments named the way other harnesses name them (`old`/`new` in
    `edit`, `file_path`, `filePath`, `oldString`, `cmd`…) are mapped to
    Gregal's names instead of failing, also inside `patch` edits and with
    Anthropic and OpenAI Responses providers. `edit` gains `replace_all`,
    and an empty `old_string` now says it is missing.
  - Hard-deny rules match the program a command runs, not words in its
    arguments: `git log --format '%h'`, `grep -rn curl .` or `clang-format`
    are no longer blocked. Code passed as a string (`bash -c`, `python -c`,
    `eval`, heredocs) is still checked as before. `curl` and `wget` to
    `localhost`, `127.x` or `[::1]` ask for permission like any other
    command instead of always being denied, so the agent can test the
    server it is building. They stay denied for other hosts, with a proxy,
    or when piped into a shell.
  - The project map tells the agent when the folder is not a git repository
    and when files use CRLF line endings, so it stops running `git status`
    there and stops chasing `gofmt -l` output caused by Windows line
    endings.
  - Bench task t8 can pass now: its assert skips the test that needs an
    external service and checks that the answer reports that it still fails.

## v1.10.0 - 2026-10-16 - Brandings, parallel subagents, faster turns and first-class local models

- Subagents work in parallel, and the Agents view shows it. A task that
  names several independent areas (a "(1) … (2) …" list, numbered or
  bulleted lines, paths in different folders, or "the TUI, the web server
  and the Telegram bot") now starts with one researcher per area, up to 3 at
  the same time, instead of a single scout; the coordinator gets their briefs
  grouped by area. The coordinator prompt and the `delegate` tool now ask to
  send independent investigations together in one call. Limits: 4 subagents
  at once per turn and 8 per process; children over the limit show as
  **Queued** instead of being invisible, and their 8-minute clock starts
  when they begin working. Subagents on a local server (Ollama, llama.cpp,
  LM Studio, localhost) run one at a time; `agent.parallel_subagents` sets
  the limit by hand. The live room seats parallel researchers side by side
  with a "3 at once" pill; each child card has a bar on the turn's time axis
  so overlapping work is visible, and the heading keeps "up to N at once"
  after they finish.
- Agents view fixes: the room no longer collapses into a narrow strip when
  the content area is under ~920px wide (the layout dropped the room's grid
  area); child cards show their area or task instead of the scout's whole
  instructions, the reason a child failed, and the current tool only while
  it works; counts use singular forms ("1 child agent"); the turn history
  collapses repeated tools ("grep ×3"); static labels follow a language
  switch; the step strip keeps its button semantics for screen readers and
  the role panel no longer re-announces itself on every poll; the canvas
  repaints at 15 fps when nothing is moving.
- Faster turns ([docs/research/speed.md](docs/research/speed.md)). On the
  bench (8 tasks × 4 runs, deepseek-v4.1-flash through zen, Windows) a
  headless task takes 12.3 s instead of 16.5 s (median; mean 17.9 s instead
  of 20.6 s), with the same pass rate. On the web, a task in a small project makes its first tool call
  about 2-3 s after it starts instead of 18-45 s.
  - Python output is UTF-8 on Windows (`PYTHONIOENCODING=utf-8`
    for `bash` and `bash_background`), and output that is not valid UTF-8 is
    decoded as cp1252 instead of turning into `�`. The model used to see
    `TOT B�`, distrust it and run the tests again: a whole extra step in 9
    of 32 runs. `PYTHONUTF8` is not set, so `open()` keeps its usual default.
  - The turn-start researcher (scout) no longer runs in projects with fewer
    than 40 files, where the project map already lists every file. There it
    cost 17-43 s before the first step and saved the coordinator no work.
    `agent.scout: always` keeps the old behaviour.
  - The git checkpoint taken before a shell command that may write costs
    ~400 ms instead of ~700 ms (~180 ms instead of ~310 ms when nothing
    changed): the repository root and index path are remembered, and old
    checkpoint refs are only pruned when there are more than
    `git.checkpoint_keep`.
  - A remote provider's `/models` probe is remembered for a day in
    `<data>/windows.json`. Each `gregal -p` no longer waits 0.4-0.7 s for it,
    and providers that do not report a window (zen, OpenAI) are no longer
    probed every ten minutes. Local servers are still probed every time.
  - At the start of a turn the git checkpoint, the project map and the window
    probe run at the same time (web, terminal, Telegram). Chat turns on the
    web no longer build a map that is then thrown away.
  - The web runs the automatic verifier (`verify.mode` auto/both/strict) only
    after turns that changed something, as the terminal already did. A plain
    question used to cost one more model call before the turn could end.
  - `gregal -p --output json` reports `timings` (setup, window probe, map,
    model, tools, engine, compaction, extension, synthesis and retry waits,
    in ms), and each `trace` entry has the call's `ms`. Eval logs carry both.

- Autonomous mode never asks for permission: it decides and does everything
  on its own, including what used to wait for a person (writing outside the
  folder, destructive git, global installs, MCP services, commits and pull
  requests). Only what is forbidden stays blocked: commands the classifier
  denies (`sudo`, piping downloads into a shell, formatting disks…) and `deny`
  rules in `permissions.tools`. To keep it away from something, set it to
  `deny`. Autonomous is back as a button next to Ask and Act in the web
  composer; it had been tucked away in the ⋯ menu.
- Brandings: your own colours, fonts and logo for the whole app (web,
  desktop, terminal) and for the documents Gregal generates
  ([docs/branding.md](docs/branding.md)). Drop a `.gregal-brand.json` file
  on Preferences → Brandings to see a card with its colour swatches, fonts,
  logo and a preview of the app; it is installed only if you accept, and a
  newer version of an installed branding shows as an update. You can also
  create one from scratch with colour pickers, font menus and a logo upload,
  and export it to share. The active branding drives Word headers (logo),
  PowerPoint masters (logo on every slide), Excel colours, HTML exports and
  chart colours. It also appears in the terminal's `/theme` menu.

- Local models get first-class providers ([docs/local-models.md](docs/local-models.md)).
  `type: ollama` uses Ollama's native `/api/chat` with a fixed `num_ctx`, so
  Ollama no longer silently drops the system prompt and tools at its 2-4k
  default. The value is chosen once per model: the config's `num_ctx`, the
  context the model is already loaded with, or 32k. It also sends
  `keep_alive: 30m` and checks `/api/show` for tool support. `type: llamacpp`
  sends `cache_prompt: true` and pins the session to one `id_slot`, with side
  calls on another slot. It reads `timings`, so `cache_n` shows up as cached
  tokens. Context windows for llama.cpp, Ollama and LM Studio now come from
  what the server allocated (`/props`, `num_ctx`, `/api/v0/models`) rather
  than the training context. New configs and the provider catalog set these
  types; existing configs are unchanged.
- `gregal doctor` checks local servers. It warns about an Ollama provider
  without `type: ollama` or with less than 16k of context, a model without
  tool support, llama.cpp `--context-shift`, a chat template that falls back
  to the Generic tool format, and a missing `type: llamacpp`. It also
  suggests `--cache-reuse 256`, `-ctk q8_0 -ctv q8_0` and
  `--spec-type ngram-mod`.
- **Smarter project map, best-of and Python checks.**
  - The project map now follows references: files that define what the
    relevant files use (a type, a function they call) are ranked up with a
    personalised PageRank over a reference graph, as in Aider's repo map,
    even when they share no words with the task.
  - Best-of-N picks its winner the way Agentless and CodeMonkeys do: the tests
    each attempt wrote are run against the other attempts (cross-tests) and
    attempts that converge on the same patch get votes. `--best-of-apply`
    applies the recommended attempt without asking.
  - After editing Python, type errors from pyright (or mypy) are reported in
    the same step when either is installed, like an editor's language server.
- **Better results with local and small models.**
  - Tool calls that a model writes as text (Hermes/Qwen2.5 `<tool_call>{…}`,
    Qwen3-Coder `<function=…><parameter=…>`, or a bare JSON call) are now
    turned into real tool calls instead of being refused and re-asked, which
    cost a whole extra turn (and prefill) on local servers. Only tools offered
    in that request are accepted, and the calls go through the same policy and
    approvals as any other. Malformed arguments are repaired when the schema
    makes the intent clear (trailing commas, `"20"` for an integer, `"false"`
    for a boolean, an object passed as a string).
  - Requests to local servers (llama.cpp, Ollama, LM Studio, vLLM) carry the
    sampling the model card recommends for each family (Qwen3, Qwen3-Coder,
    Qwen2.5-Coder, DeepSeek, gpt-oss, GLM, Devstral, Mistral, Gemma, Llama…);
    a role's own temperature still wins, and a server that rejects the extra
    fields simply stops receiving them.
  - Small local models (14B or less, or a window under 32k) get a compact
    system prompt and only the core tools; set `prompt: full|compact` on a role
    to override. Chat and inspect modes no longer show write tools at all.
  - A reply stuck repeating the same paragraph is cut as soon as the loop is
    detected, instead of running to the token limit.
  - Code mode now asks to reproduce a bug before fixing it and to re-read the
    request before answering.
- **Office documents at the level of the best assistants.** Word, PowerPoint
  and Excel files are now built with real structure and checked visually:
  - Editing no longer corrupts real files. `office_edit replace` and
    `set_cell` used to rewrite the XML with Go's encoder, which broke the
    namespace prefixes, so Word and Excel could no longer open a document
    edited by Gregal. Edits now keep every prefix, keep each fragment's
    formatting, and an edit that would break the package is refused.
  - Word: markdown becomes Heading styles, real numbered and bullet lists,
    tables, quotes, code, images, table of contents and page breaks, with a
    cover, header and page numbers. New edits: insert, delete, comment,
    tracked changes (only the changed words are marked), accept changes,
    `{{marker}}` filling; text is also replaced in headers, footers and notes.
  - PowerPoint: a 16:9 theme with nine layouts, native charts, styled
    tables, speaker notes and five colour themes; templates (`.potx`) are
    reused with their layouts; slides can be added, duplicated, moved,
    deleted and filled shape by shape.
  - Excel moves to excelize: formulas, number formats, colour roles, frozen
    headers, tables, conditional formats, validation, comments, charts and a
    `recalc` that lists every formula error.
  - Templates: `.dotx`, `.potx` and `.xltx` (and corporate `.docx`/`.pptx`)
    keep their styles, headers, footers and colours.
  - New `office_render` tool: renders documents to images with LibreOffice
    (or Word/PowerPoint on Windows) plus automatic QA (overflow, overlaps,
    off-slide shapes, leftover sample text, formula errors). The agent renders
    and fixes before delivering; the web API gains `/api/office/render`.
  - Built-in skills `office-docx`, `office-pptx` and `office-xlsx` with the
    formatting and design rules.
- Running out of steps no longer throws the work away, on any surface (TUI,
  web/desktop, `-p`, Telegram) or mode (Plan, Ask/Act, Goal, Autonomous,
  Debug, subagents, #retro reviewers). A Plan request like "do a deep review
  of the project" used to end with only `Plan · error: límit de 10 passos
  exhaurit`. Now the agent is warned two steps before the limit. When it
  reaches the limit, it makes one last call with tools disabled and gives
  its best plan or findings and the next steps from what it has gathered.
  The answer is marked **partial**: a **PARTIAL PLAN** card with
  **Continue exploring** on the web and Telegram, and `[c]ontinue` in the
  terminal. Continuing resumes with a fresh budget and the conversation
  intact. Subagents and the scout return their partial answer marked
  `[PARTIAL…]` instead of an error.
- The same applies when a turn is cut short after it has already done work:
  a model timeout, a provider that stops answering, or a context that no
  longer fits. The turn ends with a summary instead of a bare error. If even
  that call fails, Gregal lists what was looked at and the model's last
  words. `-p` prints the partial answer, reports the error on stderr and
  still exits non-zero.
- Plan explorations get at least 25 steps (review requests read many
  files). The config template now sets `agent.max_steps: 40`, matching the
  built-in default, instead of 10.
- `edit` and `patch` tolerate the ways models usually miscopy a block.
  When the block is not in the file exactly, they try, in order: ignoring
  trailing spaces, relative indentation (spaces where the file has tabs, or
  the block copied one level up or down; the new text gets the file's
  indentation), trimmed lines, Unicode quotes, dashes and odd spaces, and
  finally the first and last lines with a similar middle. They also accept
  a block copied from `read` with its `12|` line numbers. The match must be
  unique and not much larger than the block. A non-exact match says which
  level it used. When nothing matches, the error shows the most similar
  fragment with line numbers and a similarity percentage.
- A non-exact edit is only applied to a file the agent has up to date: read,
  written or edited by it in this conversation, and unchanged since. If the
  user or another agent changed the file, the agent is asked to read it
  again. A block that is in the file exactly and only once is always
  applied.
- `edit` and `patch` return the diff of the change with numbered context, so
  the agent does not have to read the file again. Reading a range that was
  read a few tool calls ago and has not changed returns "sense canvis"
  instead of the whole text; asking again returns it in full.
- Long command output keeps the start and the end (60/40) instead of only
  the start, so compiler and test errors at the end are no longer lost. The
  full output is saved to `.gregal/out/` (git-ignored, cleaned after a day)
  and the agent is told where to read it. `read` of a large file now stops
  at a whole line and says which `offset` to continue from.
- Local models answer the next message much sooner. Earlier turns are no
  longer rewritten, so llama.cpp, Ollama and LM Studio can reuse their KV
  cache instead of processing the whole conversation again on every
  message (on a laptop that was tens of seconds before the first token).
  - The project map on a new turn only lists what changed since the
    previous one (files added or removed, and the new git status). Old
    maps are removed all at once, only when the conversation is compacted.
  - The compaction summary now goes in a marked `<compacted-summary>`
    message at the start of the conversation, not in the system prompt.
    Saved sessions are converted when they are opened.
  - The system prompt stays the same for the whole session (TUI, web and
    Telegram). If `AGENTS.md`, the project notes or the skills change in
    the middle of a session, the change is added to the next message.
- Making room in the context is cheaper and does not break the cache at
  every step. Old tool outputs are replaced in one block (all but the last
  10) with a note such as "output of `read foo.go` cleared; call the tool
  again if you need it", and enough are cleared that the next steps do not
  hit the limit again straight away. The model summary is only used when
  that is not enough, and it is now a handoff note: goal, done, pending,
  decisions, files and open errors. A tool call and its result are never
  separated.
- Context use is measured more accurately. Gregal compares its estimate
  with the prompt tokens the provider reports and learns the ratio for each
  model, including the size of the tool definitions. Compaction uses that
  estimate, so small local windows are neither overrun nor compacted too
  early.
- In long tasks the agent sees the current checklist again every 6 steps
  and after each compaction, at the end of the context.
- New optional `small` (or `fast`) role for side tasks. When it is
  configured, it writes the compaction summaries and the project-note
  suggestions and runs the pre-turn scout. If it fails, the main model does
  the work as before.

## v1.9.4 - 2026-10-16 - Team review, no lost replies and a faster terminal

- A reply cut off by the output token limit is no longer lost, on every
  surface (terminal, `-p`, web, Telegram). The partial text is kept and
  Gregal asks the model to continue, up to three times. A tool call whose
  arguments were cut off is never run: it is requested again with twice the
  `max_tokens` and a hint to split the work into smaller calls. Each step
  shows in the status line, and if the limit is still hit the reply ends
  with a note instead of disappearing.
- Terminal (TUI): with the mouse on, dragging over the conversation selects
  text with a visible highlight and copies it on release, and the wheel
  still scrolls. Shift+drag still uses the terminal's own selection. `/copy`
  and the selection now use the native clipboard on Windows and macOS.
- Terminal (TUI) opens at once: MCP servers connect in the background, their
  tools appear as each one connects and the status bar shows how many are
  still connecting. A slow or hung server no longer delays startup by up to
  30 seconds.
- An older `gregal` on PATH that didn't know `gregal mcp demo` opened a
  terminal UI instead, which started the demo connector again, and so on
  until memory ran out. Gregal now rejects unknown commands and never opens
  the terminal UI when it was started as an MCP server.
- #retro team review: when the retro opens, reviewers who did not do the
  work look at it in parallel with the summary, each from its own angle:
  Design (only when something visible changed), Code, QA (new: bugs, edge
  cases, missing tests), Product and Security. Each posts a verdict and up to
  three findings with severity, receipts and a fix that **Fix it** starts as
  a run; they appear as they finish. A "Team review" bar counts the findings
  and filters them by severity. Same on Android, which also gets **Fix it**.
- The coordinator's opening in #retro explains how the work was implemented
  (approach and key files).
- #retro judged a turn against the wrong task when another conversation was
  loaded: the task is now saved with the turn. `@all` gets a reply from every
  agent again (it was capped at three).

## v1.9.3 - 2026-10-15 - Aligned surfaces

- Same mode vocabulary everywhere: the terminal (TUI) and Telegram now say
  Ask/Act (Pregunta/Actua) like the web and the app, with Goal, Autonomous
  and Debug as special modes. `/mode` accepts the visible names in both
  languages (`act`, `ask`, `actua`, `pregunta`, `depuracio`…) as well as the
  old ids. The Telegram mode keyboard has Ask/Act on the first row.
- The terminal (TUI) is fully in English when the language is English: the
  session status screen, the activity timeline, tool summaries, approvals,
  provider setup, review and settings menus no longer show Catalan. The
  mode picker now also lists Debug and Autonomous, like the web.
- Office add-in re-themed with Gregal's light/dark palette (dark follows
  Office), the wind-gust mark in the header and new icons.
- The desktop "could not start" page uses the gust mark and the palette
  tokens.
- `docs/identitat.md` documents the brand (gust graphic, `≋` on text
  surfaces) and the mode vocabulary per surface.

## v1.9.2 - 2026-10-14 - Fast #retro with specialists by area

- #retro is fast and never hangs: the agents chat without reasoning (with
  a model that always reasons, the opening went from minutes to ~5 s), the
  opening has a 75 s limit and replies 60 s, after which the summary from
  the evidence is shown. The dialog shows the elapsed time and, after 20 s,
  a "Show a quick summary" button; a pending request no longer blocks other
  retros, a second click waits for the first generation, and a lost reply
  is recovered from the server.
- #retro specialists by area: Design (looks, UI/UX, accessibility), Code
  (best practices), Product and Security, and the coordinator also answers
  feasibility and who would take something. Without an @mention the model
  picks the 1-3 agents that fit the question (before, keyword matching
  sent most questions to the coordinator).

## v1.9.1 - 2026-10-14 - Talk to the agent while it works, parallel sessions and Azure

- Azure OpenAI and Microsoft Foundry: API keys go in the `api-key` header
  (Entra ID tokens as Bearer), classic `?api-version=` URLs work, and the
  Foundry Model Inference endpoint gets `extra-parameters: pass-through`.
  `reasoning_effort` is negotiated per model: an unsupported value falls
  back to the nearest supported one, GPT-5.6 with tools gets `none`, and a
  model that does not know the parameter only drops it for itself. See
  docs/provider-presets.md.
- Azure OpenAI and OpenAI reasoning models (o-series, gpt-5…) work: when a
  model rejects `max_tokens` (HTTP 400 "use max_completion_tokens instead")
  or a non-default temperature, Gregal resends with `max_completion_tokens`
  or the default temperature and remembers it for that model.
- Several requests at once: sessions on the same project now work in
  parallel instead of the second one waiting as "queued" (only unattended
  jobs and graphs still take the folder exclusively).
- Talk to the agent while it works, like other coding agents: Enter sends
  the message into the running turn (it is read at the next safe point and
  the agent adjusts), Ctrl/⌘+Enter interrupts the current model call and
  redoes it with the message, Alt+Enter keeps the old "queue for after".
  On Android, Send tells it now and a long press interrupts. Messages that
  arrive too late are sent as a new message, never lost.
- Stopping a turn now closes it in the conversation, so the next question
  is answered on its own instead of resuming the stopped task.
- One release pipeline. `node scripts/release.cjs bump X.Y.Z --title ...`
  sets every version and turns the Unreleased notes into the release
  section; Quality CI fails if versions or the changelog drift; pushing the
  tag builds the Windows installers and the CLI for Linux, macOS and Windows
  (amd64/arm64) and creates one draft release with notes from the
  changelog and a single SHA256SUMS.txt. `build.sh` now writes the same
  asset names, the Android app is tested in GitHub CI, and the unused Gitea
  workflow is gone. See docs/release.md.

## v1.9.0 - 2026-10-14 - Work groups, #retro everywhere and Android 0.9.8

- #retro is easier to reach: its own sidebar entry with a dot for turns
  you have not discussed yet, a clearer button on every finished turn, and a
  new Android screen to read the agents' write-up, ask or challenge them
  (@name) and react with emojis.
- #retro specialists: Product, Code and Security join every channel. They
  did not do the work; they review it from their angle with the same
  evidence and receipts, speak at most once in the opening stand-up (only
  when the evidence gives them something concrete), and answer questions on
  their topic even without an @mention. `@all` / `@tots` asks everyone.
- Redaction tests build their fake keys at runtime, so GitHub secret
  scanning no longer reports them as leaks.

- Android 0.9.8, aligned with the desktop. Each turn's tools, reasoning and
  narration fold into one live "work" card (what it is doing now, elapsed
  time, a timeline of steps with summaries such as "Read src/app.py ·
  lines 10–40"); the final answer stays outside. Live text streams outside
  the list (no more O(n²) copies), events are handled in order on one thread,
  short network drops no longer fail a turn, and model errors read in plain
  words. Composer with Ask/Act, Plan first, a send/stop/dictate button and a
  "+" menu for goal, autonomous and debug modes and the permission lock.
  Fixed: the "always allow" button was labelled "New session"; streamed
  chunks and subagent lanes showed up as raw rows; autonomous/debug turns ran
  as chat; the conversation is restored on open; notices show on every
  screen; safer foreground-service start/stop, rotation and file reads.
- Android visual identity aligned with the desktop: new launcher, monochrome
  and notification icons from the Gregal wind-gust mark; the mark (floating,
  blinking) on the sign-in and empty chat screens; rounded icon set; tighter
  typography; project and branch under the model title and as a pill; tinted
  empty states; drawer with a clear New session button. Pull to refresh on
  every list (no new dependency), automatic reconnect at startup, and
  reopening the app re-attaches to a turn that is still running.
- Server: run ids no longer restart at 1 after a restart (the queue continues
  from the highest id in the persistent event log), so clients following the
  log can't mix a new turn with an old one.
- Working folder always visible: a folder chip in the header (name, full
  path on hover and underneath; click to change, ▾ for Change folder / Open
  in Explorer or Finder (desktop) / Copy path), "Working in <folder>" with
  branch, git and worktree status on the home page, a discreet "in <folder>"
  under the composer, and full paths in session tab tooltips. `/api/state`
  adds `git`, `worktree` and `worktree_repo`.
- Floating weathervane (desktop): drag it from anywhere (a short click still
  opens the app), minimize it to a compass dot, snap it to a corner or hide
  it from hover controls; position and folded state are remembered and kept
  on screen when displays change. The sidebar vane gets a Hide action.

- Agent browser: `browser.mode` (`auto` | `app` | `headless` | `window`). In
  the desktop app the agent drives an "Agent browser" pane next to the
  conversation through a loopback, token-protected CDP bridge instead of
  opening an external Chrome window; elsewhere it runs headless by default.
  Browser screenshots show in the web tool card.
- Composer: Ask | Act (Tab switches), a "Plan first" switch that proposes a
  plan and waits for approval before acting, and the permission lock. Goal,
  autonomous and debug modes move to `/goal`, `/auto`, `/debug` and the ⋯
  menu, shown as a chip you can leave.
- New default palette (dark and light): cool slate neutrals, one periwinkle
  accent used sparingly, aqua for playful touches; WCAG AA text contrast
  throughout (`docs/identitat.md`).
- #retro: a Slack-style debrief channel per finished turn. Agents explain
  what they did with links to real evidence (diff hunks, commands, tests),
  react with emoji and answer @mention challenges; 🤔 asks for a
  justification, 👎 records a preference, "Fix it" starts a follow-up turn.
  Personas and an optional Slack/Teams webhook.
- Git: per-step checkpoints on hidden refs (rewind now also undoes shell
  changes), isolated worktree sessions, `git_commit` and `gh_pr_create`
  tools with Commit/PR dialogs, and a reusable PR review GitHub Action.
- Lifecycle hooks (`session_start`, `user_prompt`, `pre_tool`, `post_tool`,
  `stop`, `pre_compact`); remote MCP over Streamable HTTP/SSE with OAuth,
  resources and prompts; `gregal mcp serve` exposes Gregal to other clients.
- User-defined subagents (`.gregal/agents/*.md`); team roles can use tools;
  `/btw` side questions and `/recap`; suggested project notes and a memory
  tidy, never written without approval.
- Native Anthropic Messages and OpenAI Responses providers; a symbol map of
  the repository in the turn context plus a `symbols` tool; Markdown/HTML
  transcript export with secret redaction and revocable share links.
- Debug mode, "Try N ways" (best-of-N in worktrees), a plan critic and
  calibrated confidence bets before checks.
- Room replay with clip export, weekly "Wrapped" stats, the Gregal
  weathervane (opt-in), voice input and image paste in the composer.
- Opt-in shell sandbox (`sandbox.mode: workspace|strict`, `network`,
  `allow_write`, `backend`): bubblewrap or Landlock on Linux, sandbox-exec on
  macOS, Docker on request (the only option on Windows). Without a backend,
  `workspace` warns once and runs unsandboxed and `strict` refuses shell
  commands. Status in `gregal --doctor`, `/api/state.sandbox` and the web
  permission menu.
- Auto-review permission mode (`permissions.mode: auto-review`, web menu,
  `/autoreview` in the TUI): a reviewer model allows, denies or hands back
  each call that needs approval; hard-denied calls never reach it, and
  timeouts or malformed replies fall back to the human prompt.
- Simpler navigation: the sidebar keeps four daily destinations (the
  project chat, Changes with a file count, Files, Agents); Goals, Terminal,
  Browser, GitHub, Office, Plots, Graphs, Scheduled and Connectors fold
  under "More". Activity stays in the inspector. "Agents" merges the 2D
  workshop and Team into one page with Live / Team run tabs; the Team tab
  drops its duplicate 2D office.
- 2D room: name, state and step sit on a readable plate in front of each
  desk instead of tiny overlapping text over the monitor; a coordinator
  working alone is centred; delegates sit behind their desks; the room and
  the coordinator card adapt to the space actually available.
- First page shows what is pending in the project (uncommitted changes,
  agents working, open goals) and its three most recent conversations;
  the generic starter suggestions only appear in a project with no history.
- Liveness: the running work group and the turn bar say what the agent is
  doing now and for how long (waiting for the model, thinking with a live
  character count, writing, running a tool, waiting for approval, reviewing);
  the server emits `progress` events for model, tools, review and compaction
  phases, and a model that sends nothing for 90 s is flagged as such.
  Autonomous checkpoints with nothing to report no longer print a line.
- Web transcript: each turn's intermediate work (narration, tool calls,
  thinking, answered approvals) folds into one "Worked for … · N tools"
  group that stays open while running and collapses when done, leaving the
  final answer below; tool rows read as plain actions ("Read src/app.py ·
  lines 10–40", "Edit a.py +3 −1") instead of raw JSON; resolved approvals
  shrink to one line; finished answers keep the code-block Copy button;
  resumed conversations no longer show the internal project map in the
  user's message and restore tool calls when the session stored them.
- Web UI consistency: one button/field/page-header style across every
  workspace page, compact composer mode switch, theme-aware Team graph
  arrows with a revision edge that no longer crosses nodes, and no
  untranslated Catalan left in the English UI or the Office add-in.
- English model-facing prompts (rules, tool descriptions, nudges, final
  synthesis, compaction, reviewer) with a language directive placed last,
  so answers follow `lang` (English runs answered in Catalan about half
  the time before).
- Agent loop: loop-guard nudges no longer break the tool-call protocol
  (HTTP 400); DeepSeek `reasoning_content` is sent back; unreachable
  providers fail in about a second instead of ~96 s; "ok" confirmations
  reach the model; only the newest message carries the project map;
  known-model context windows when the provider reports none.
- Headless (`-p`): `--dir` is honoured, in-project writes work without
  `--auto-approve` in code mode, explicit `--max-steps` is a hard cap with
  `exhausted` in JSON, a failed `--verify` exits 3, invalid flags fail
  before any paid call, JSON errors go to stdout, the trace pairs results
  by call ID, and interactive-only tools are not offered.
- Workshop fusion: team pipeline runs publish as live bays in the single
  2D room (states, agents, per-bay flow graph, detail, queue, stop),
  sharing one graph module with the Team page; team stops have their own
  slot and `/api/agent/cancel` stops both run kinds; failure messages in
  the room never carry provider URLs.
- Team flow editor: the fixed pipeline is now data (validated server-side),
  with a visual editor (nodes, links, instructions, rounds, save/export/import)
  and live graph preview; custom flows run through the same SSE vocabulary.
- Team pipeline graph: the fixed order (plan, parallel pair, review,
  revision back to builder) drawn live with per-agent states, clickable
  nodes and handoff highlights; topology pinned by contract test.
- Desktop app writes the managed backend log to `gregal-backend.log`
  (rotated past 2 MB) so stopped runs can be diagnosed after the fact.
- Server log: turn and team lifecycle lines on the terminal (outcome and
  elapsed, credentials redacted), optionally appended to a file with
  `--log-file` (or `GREGAL_LOG_FILE`). Team timeouts report as failures
  instead of "Stopped", team runs no longer hijack the agent stop slot,
  and role errors are logged sanitized (the only place a hidden provider
  failure can be diagnosed).
- Show real model consumption in the 2D workshop: per-turn and per-child
  tokens (never estimated — bare call counts when the provider declares
  nothing), turn cost only with a configured price, and an inspection-only
  step timeline per bay.
- Let each busy workshop bay stop its own turn in place, and count sessions
  waiting for a response in the browser tab title.
- Harden long runs: cancelled tools leave no fake results in history;
  global cap of 8 live subagents per process (4 per call kept); background
  polls never trip loop detection while the job runs; child summaries capped
  with an explicit mark and a non-recallable prune note; at most 8 executable
  calls per step (rest deferred with its tool result); cost caps stop loudly
  instead of silently when the model has no price; web/fetch/GitHub tools
  honor turn cancellation; post-edit hooks die with the turn.
- Estimate image tokens by data size instead of a flat 1000.
- Connect the 2D workshop to real coordinator and delegated-agent activity:
  task/model/tool/step states, retained results and failures, accessible child
  details, and responsive animation that respects reduced motion.
- Encourage automatic delegation of independent investigations; support an
  optional `delegate` model role and cap web turns at four concurrent children.
  Keep worker configuration session-scoped, propagate cancellation, and contain
  child failures without losing sibling results.
- Keep numbered-file reads and paginated reads from triggering fuzzy loop
  detection; identical repeated reads are still blocked.
- Preserve cancellation and timeout errors from fallback models so long runs
  stop without additional retries. Stop headless runs between steps and prevent
  queued tools from starting after cancellation.
- Honor Retry-After seconds and HTTP dates on 429/503 responses, capped at
  30 seconds; allow retry backoff with jitter disabled.

## v1.8.1 - 2026-10-07 - Chat plots and project dashboard

- Render inline line, bar and scatter charts from `gregal-plot` data blocks.
- Save plot snapshots per project and user with their source session.
- Organize dashboard cards with notes, ordering and compact or wide layouts.
- Edit titles and data, import JSON and export PNG/JSON.
- Validate chart data and prevent concurrent edits from silently overwriting changes.
- Saved plots do not refresh automatically.
- Create, rename and delete named dashboards; move plots between them without
  losing their library data. Search plots and export the collection as JSON.

## v1.7.10 - 2026-10-06 - Interrupted agent stream recovery

- Recover interrupted buffered agent streams with up to three attempts at the
  same model step; discard partial text and tool calls without replaying prior
  tools. Live chat output is not automatically replayed. Cancellation stops
  recovery immediately.

## v1.7.9 - 2026-10-05 - Clearer configuration, agent previews and safer execution

- Simplify provider setup with labeled fields and separate advanced settings.
- Make working Team agents easier to identify, improve role animations, and stop
  activity animations when a run stops.
- Let agents request isolated HTML, Markdown and localhost previews; keep the
  newest artifact visible and respect the user's dismissal of automatic previews.
- Share shell permission restrictions between foreground and background tools;
  hard-denied commands cannot be enabled by a broad allow override.
- Allow internal task-list bookkeeping without unnecessary approval prompts.
- Propagate cancellation into headless tool execution and delegated runs; clean
  up foreground shell descendants on cancellation, timeout and normal exit.
- Require HTTPS for remote desktop backends and tighten Unix password-file access.
- Add a functional checklist, coding/budget/cancellation regressions and real-model
  acceptance evidence. Benchmark observations are task-specific, not a general
  guarantee of higher speed or equivalent quality than another harness.


## v1.7.5 — 2026-10-05 — Modes simplificats i continuïtat de sessions

- El desplegable de modes avançats conserva l’obertura escollida per l’usuari
  durant els refrescos d’estat.
- Entrada simplificada amb «Fer una tasca» i «Conversar», modes especials
  desplegables i selecció de rols dins de les opcions avançades.
- Detecció de models limitada al rol actiu per evitar esperes de proveïdors aliens.
- Control explícit de l'esforç de raonament i compatibilitat amb els passos
  mecànics de GLM-5.3, que no permet desactivar el raonament.
- Taller 2D de sessions amb cua d'atenció per a aprovacions i preguntes.
- Cerca de converses pel títol, el nom, el projecte i la carpeta de treball.
- Estat visible dels torns mentre treballen, esperen una resposta o acaben.
- Resum de fitxers modificats i comprovacions observades, amb checkpoints
  recuperables després d'una reconnexió.

## v1.7.4 — 2026-10-04 — Recuperació dels límits de resposta

- Conserva les opcions de raonament i de model alternatiu en canviar de model.
- Corregeix el tractament dels torns truncats i la duplicació d'errors de l'agent.

## v1.7.3 — 2026-10-04 — Taller mediterrani i actualitzacions

- Oficina 2D mediterrània integrada a l'Equip, amb panell d'agents sense
  solapaments, contrast millorat i focus del teclat conservat.
- Actualitzacions de l'app accessibles des de Preferències, amb comprovació,
  descàrrega, progrés i instal·lació en reiniciar per decisió de l'usuari.
- Accés a les descàrregues oficials per als executables portables.


## v1.6.1 — 2026-09-30 — Windows, continuïtat i acabats visuals

- App portable de Windows x64 amb el backend local integrat.
- Model seleccionat persistent per usuari i rol; cada conversa conserva
  la seva selecció i les converses noves hereten les últimes tries.
  Si un model ja no està disponible, s'utilitza el configurat i se n'avisa.
- Preferències d'aparença i models recents independents del port local.
- Esborranys separats per pestanya i errors visibles en gestionar sessions.
- Capçalera més neta, icones coherents, compositor i blocs de codi més cuidats,
  amb controls accessibles també en finestres estretes.
- Respostes completes al transcript durable i estat de permisos coherent
  quan una actualització falla.


## v1.6.0 — 2026-09-29 — bucles llargs, un sol motor i banc contra opencode

Una tirada real va cremar 146 passos i 50 minuts en una tasca trivial:
el model tornava a córrer una suite que no podia quedar verda (un test
viu contra un model que l'endpoint ja no servia) i cada corrida comptava
com a progrés. D'aquí surt aquesta tanda.

- **Ratxa de comprovacions vermelles**: tres test/build/vet/lint en
  vermell seguits guien el model (llegeix l'error sencer, comprova amb
  `git stash` si és preexistent) i bloquegen l'ampliació (sis en
  autònom); nou seguides en interactiu tanquen el torn amb síntesi encara
  que quedin passos. Només compten les ordres que són comprovacions: un
  `grep` buit no és un vermell ni un `sed` verd un verd.
- **Bash sense `cd`**: el prompt diu que cada ordre comença al directori
  del projecte; el model posava `cd <camí> &&` a totes.
- **Mapa del projecte en començar el torn** (`agent.MapaProjecte`):
  llistat de fitxers (o resum per carpetes), estat de git i el contingut
  dels fitxers petits que la tasca anomena. Va amb el missatge de la
  tasca, no al system (no trenca el KV cache), no es repeteix si no ha
  canviat i no surt a la bombolla. Les primeres voltes del model eren
  `pwd`/`ls`/`find`/llegir `TASK.md`: t1 passa de 5 passos a 3.
- **`think: auto`**: el model raona per planificar, després de llegir i
  davant d'un vermell, i no als passos mecànics (després d'una edició
  aplicada o d'un verd), l'ampliació ni la síntesi.
- Prompt: el resum final diu què ha canviat, com s'ha verificat i què
  queda, sense tornar a escriure el codi; `write` és per a fitxers nous.
- **Topall interactiu de 3 ampliacions** i síntesi llarga (canvis, què
  passa, què queda en vermell i si és preexistent, pas següent).
- **La web i l'escriptori condueixen el motor compartit** (`agent.Torn`,
  `internal/web/motor_web.go`) en comptes d'un bucle propi de 400 línies:
  cap de les proteccions anteriors hi arribava. El motor guanya el que
  la web tenia i ell no: checklist per sessió, síntesi davant d'una
  resposta buida o del topall autònom, reintent de la síntesi per
  context i topall de recuperacions de context.
- **Web: la conversa es pinta en ordre**. Les targetes d'eina arribaven
  pel registre durable com a «activity» genèric i no es pintaven; la
  resposta final quedava a dalt, per sobre de les aprovacions; una eina
  denegada deixava l'última targeta girant per sempre. Aprovacions amb
  diff llegible, sense «Permès» doble.
- **Aprovacions web sense el tall de 120 s**: `agent.approval_timeout_s`
  (defecte 30 minuts, negatiu = sense límit) i aturar el torn talla
  l'espera.
- **`think: no` arriba al proveïdor**: el wire no copiava
  `enable_thinking` ni `chat_template_kwargs`, i només el headless posava
  el mode al context. Ara el respecten tots els clients, i també s'envia
  `reasoning_effort: none` (el que entén opencode zen).
- **Camps opcionals negociats** (`internal/llm/compat.go`): zen rebutja
  amb 400 `enable_thinking` i `chat_template_kwargs`. Si un proveïdor
  rebutja un camp opcional (`stream_options`, els de pensar), es torna a
  provar sense i se'n recorda per a aquell endpoint: un camp opcional mai
  no trenca una crida.
- **Tokens reals**: el headless suma l'`usage` de cada crida (streaming
  amb `include_usage`, amb reintent si el proveïdor no l'accepta) i el
  topall de cost autònom el fa servir. `tokens_source` diu d'on surten.
- **Diagnòstics després d'escriure** per a Python (`py_compile`, i
  `ruff`/`pyflakes` si hi són), JS (`node --check`), TypeScript (`tsc`
  si hi ha `tsconfig`) i shell, a més de Go, JSON i YAML.
- **`--config` inexistent és un error**: abans s'hi escrivia la plantilla
  per defecte i el torn anava a `localhost:5800` sense dir res.
- **Banc A/B contra opencode** (`bench/ab.py`): mateix model, mateix
  prompt, còpia neta per tasca, fitxers que no es poden tocar i tests
  ocults. Dues tasques noves: t8 (trampa amb un test extern en vermell) i
  t9 (bug entre mòduls amb test ocult).
- TUI: la carpeta de treball a la barra d'estat s'adapta a l'espai i no
  talla mai l'estat (a 80 columnes l'aprovació sortia «◌ espera la t»).
- Els tests del TUI ja no desen converses a les dades de debò.

## v1.5.4 — 2026-09-25

- **Streams truncats**: si el model talla la resposta a mig stream (MTP,
  cursa del mòbil, tall de xarxa), el client el recupera en lloc de
  quedar-se amb una resposta incompleta.
- **Desbordament final de context**: l'error d'entrada massa llarga a
  l'últim pas es recupera en lloc de tomba el torn sencer.
- Correccions menors de failover i proves (emptyreply, stream_tools).

## v1.5.3 — 2026-09-23

- **`gregal advise`**: detecta millores (dependències, toolchain, higiene del
  repositori i backend) sense necessitat de xarxa.
- **TUI i escriptori**: estètica i solidesa (sis pantalles, cockpit en columna,
  diàlegs superposats, composer de dues files), un sol tema amb markdown i codi
  pintats als dos fronts, i selector de models amb pestanyes.
- **`:root` regenerat** des d'`internal/tema` (prova `TestCSSAlDia` en verd).
- **Clients alineats** amb la cua d'execucions i els esdeveniments v2.
- **Telegram**: la prova de permisos 0600 només corre a Unix (a Windows no
  existeixen aquests permisos) i el bot fa servir el motor de torn compartit.
- **Skills**: el gregal sap què és i què pot fer.

## sense publicar — TUI al nivell d'opencode

Tres coses separaven el TUI d'opencode: dos bucles d'agent que anaven
divergint, una configuració pensada per a qui edita YAML, i una pantalla
que acumulava cromat. Aquesta tanda va per aquí.

### Un sol tema, i el codi pintat als dos fronts

La paleta vivia copiada a tres llocs (`internal/tui/theme.go`, el `:root`
d'`index.html` i `desktop/main.js`) amb tests que comparaven les còpies amb
`docs/identitat.md`. Els tests feien la seva feina, però vigilar tres còpies
no és tenir-ne una, i cap de les tres sabia res dels colors del codi.

- **`internal/tema`** és ara la font: els tokens, l'estil de glamour (el
  markdown del TUI), l'estil de chroma (el codi als dos fronts) i el bloc
  `:root` del CSS en surten. El full d'`index.html` es genera i un test
  (`TestCSSAlDia`) avisa si es queda enrere.
- **Vuit colors de sintaxi** documentats a `docs/identitat.md`, derivats de
  la paleta. El TUI feia servir l'estil per defecte de glamour, d'una altra
  marca, i en tema clar ni canviava.
- **Contrast**: el gris més apagat del tema clar puja de `#7C8B95` a
  `#5F6E78`. El de abans donava 3,2:1 sobre el panell i no passava AA.

### El markdown de les respostes

- **Al web, el renderitza el servidor** (`POST /api/md`) amb goldmark,
  chroma i bluemonday. El renderitzador propi del client (`app/md.js`, 157
  línies) es queda per al text que arriba en directe: cobria el que els
  models solen escriure, però una taula en sortia com un paràgraf massís en
  negreta i els blocs de codi mai no havien tingut color. Vuit tests, quatre
  d'ells d'injecció (`<script>`, `onerror`, `javascript:`, `position:fixed`).
- **Al TUI, el text es pinta com a markdown mentre arriba**, refet cada
  150 ms. Abans era una cua de 160 caràcters retallada per l'esquerra: amb
  paràgrafs la línia ballava i no es podia llegir res. El bloc de codi que
  queda obert a mig arribar es tanca per pintar-lo.
- **Els diffs del TUI porten sintaxi**: chroma sobre el fitxer sencer (així
  veu el context de les cadenes i els comentaris) i els `+` i `−` al marge.

### La pantalla del TUI, de dalt a baix

El pla és a `docs/plans/2026-09-21-tui-pla-de-millora.md`: cinc tandes,
cadascuna amb el seu commit i la suite verda.

- **La conversa**: el diff d'un edit es pinta dins del rail de feina, sota
  la fila de l'eina, amb sintaxi i els `+`/`−` al marge. La sortida d'una
  eina s'ensenya fins a sis files i diu quantes en queden (`Ctrl+L` les té
  totes). El resum del torn és un peu apagat amb plurals bons. Fora el rail
  de scroll: el percentatge ja és a la barra quan no s'és al final.
- **Capçalera, composer i barra**: la capçalera diu `≋ GREGAL │ projecte
  │ branca` i és quieta. El composer té dues files: l'entrada a dalt i un
  peu amb el mode, el proveïdor/model i `↵ envia · ? ajuda`; amb feina, el
  peu porta el spinner, què fa, quant fa i com aturar-ho. La barra d'estat
  passa de nou senyals i tres jocs de pistes a quatre segments: mode,
  `ctx n%`, estat i permisos. Els comptadors ↑↓ i el cost són a `/stats`.
- **`animacions: off` per defecte**: la mar, la mascota i la marea del
  mesurador només amb `on`. Un test comprova que dos `View()` seguits en
  repòs són idèntics byte a byte.
- **Diàlegs superposats**: el selector, l'aprovació (amb el diff a dins),
  la pregunta del model, la cerca a l'historial (ara amb la llista de
  coincidències) i l'ajuda es pinten centrats sobre la conversa, amb el fons
  atenuat i amplada màxima de 72. Abans s'encabien entre la conversa i el
  composer i obrir-los feia saltar tot el text. `?` amb el composer buit
  obre l'ajuda de tecles i modes; qualsevol tecla la tanca.
- **Benvinguda de sis files** sense caixa: marca amb `[45° NE]`, una frase,
  tres exemples i les tecles. Els modes s'expliquen al selector de mode i a
  `?`. Abans era una targeta de tretze files que ocupava mitja pantalla.
- **Cockpit en columna**: a 120 columnes o més, la conversa cedeix 34
  columnes i a la dreta hi va la tasca, els canvis, la cua, el context (amb
  mesurador) i la validació. `Ctrl+T` la commuta i ho desa (`cockpit:
  on|off`). Per sota de 120, la caixa de sempre. El revisor automàtic també
  hi viu: a la barra ja no hi cabia.
- **Fitxers dorats**: sis pantalles × dos temes × tres amplades (80, 120,
  160) desades tal com es pinten a `internal/tui/testdata/golden`. Un canvi
  d'estil que no es volia surt al `go test` abans que a la pantalla de
  ningú; els volguts es regeneren amb `-update` i es revisen al diff.
- **Desviació**: la barra `┃` a l'esquerra dels blocs de codi del markdown
  no és possible amb glamour v1 (pinta l'`Indent` com a espais i ignora
  l'`IndentToken` als blocs de codi). El bloc queda sense fons i amb la
  sintaxi pintada.

### Tipografia i línies

- **Escala de cinc mides** al web (`--t1`..`--t5`), amb res per sota d'11,5
  px: 120 declaracions hi passen, i les 47 que estaven a 9,5-11 px pugen.
  La barra lateral era el pitjor cas.
- **Una sola línia vertical** al TUI: el missatge teu perd la barra `▌` i es
  distingeix pel fons i per la marca. Amb el rail de feina i el de scroll
  n'hi havia tres competint.
- **Els torns se separen** amb una línia en blanc, i un pas que només crida
  eines ja no deixa un «◌ escrivint…» penjat.

### Aturar de debò

- **La cursa d'Atura**: la cua marca el torn com a «running» abans que
  l'executor n'hagi registrat l'id, i entremig hi cap una premuda. Qui
  aturava just quan el torn arrencava no aturava res: el botó deia que sí i
  el torn seguia, amb la mar de la capçalera movent-se. Ara `stopRun` busca
  el torn viu de la sessió a la cua quan encara no en sap l'id.
- El proveïdor fals dels tests s'allibera sempre en acabar: una fallada de
  mig segon es convertia en quatre minuts de timeout que amagaven quin test
  havia petat.

### Un sol motor de torn

Hi havia dues màquines d'estats fent la mateixa feina. La del headless
era un bucle `for` a `run.go`; la del TUI vivia dins d'`Update`, repartida
entre sis casos de missatge i 1.100 línies, amb tretze comptadors al
`Model` que `startAgent` havia de posar a zero un per un. Cada arranjament
s'havia de fer dues vegades i sempre se n'oblidava una.

Ara la màquina és a `internal/agent/motor.go` i no fa entrada ni sortida:
rep el que ha passat (`RepPas`, `RepExecucions`, `RepAprovacio`,
`RepPregunta`, `RepAmpliacio`, `RepCompactacio`, `RepCheckpoint`,
`RepSintesi`) i diu què cal fer després (`Seguent()` torna una ordre:
demanar un pas, compactar, executar eines, demanar permís, preguntar,
ampliar el pressupost, sintetitzar, acabar). Els clients hi posen com
criden el model, com executen les eines i com demanen permisos.

- El TUI passa de 4.226 a 3.737 línies: `Update` només alimenta el motor
  i executa l'ordre que en surt. Els tretze comptadors han desaparegut.
- **El bot de Telegram també hi passa.** Portava una tercera màquina
  d'estats pròpia a `turn.go`: sense compactació (una conversa llarga
  acabava desbordant la finestra), sense síntesi quan s'esgotaven els
  passos, amb un topall fix de 180 s per pas que tallava els models lents,
  i `/stop` no aturava l'eina en marxa (només la crida següent). Ara
  alimenta el mateix motor, executa les eines amb el context del torn i
  publica els avisos del motor (reintents, compactació, checklist) a
  l'activitat del xat. Els textos per defecte dels events viuen a
  `agent.Event.Missatge()`, amb un test que vigila que cap clau nova es
  quedi sense text.
- El headless condueix el mateix motor amb un bucle de trenta línies.
- La màquina d'estats té quinze tests propis que corren en un segon,
  sense terminal, sense xarxa i sense fitxers. Abans no era provable:
  només es podia tocar a través de la interfície.
- El motor parla amb claus de traducció, no amb text: no depèn de la i18n
  de cap client.
- Les diferències que arrossegaven les dues còpies queden resoltes per
  construcció, no per haver-les tornat a sincronitzar a mà.

### Fiabilitat del bucle

- **Un sol registre d'eines** (`tools.Noms()`). N'hi havia tres i no
  coincidien: 24 noms a l'agent, 14 a la validació del config i 13 al
  menú de permisos. `permissions.tools.office_edit: deny` feia fallar la
  càrrega d'un config que l'agent executava sense problema, i el menú no
  deixava tocar ni office ni el navegador.
- **Ctrl+C durant una eina ja no tanca el programa.** El camí que executa
  eines no registrava cap cancel·lació, o sigui que Ctrl+C enmig d'un
  `go test` de dos minuts queia al «surt» i s'enduia la sessió. Ara el
  primer atura (i mata de debò la comanda: `tools.BashCtx` i el navegador
  reben el context del torn) i el segon surt.
- **Esc atura també el que ja s'ha aprovat**: `pendingOp.run` rep context.
- **El mode autònom del TUI respecta els tres topalls.** Només mirava el
  cost; `max_tool_steps` i `max_minutes` no existien fora del headless.
  Ara tots dos bucles criden `agent.TopallAutonom`.
- **La decisió d'eines és la mateixa a tot arreu** (`agent.PlanCalls`).
  El TUI en tenia una còpia amb l'ordre de comprovacions canviat.
- **Els reintents d'error de servidor són dos**, com al headless (n'era 1).
- **L'aprovació ja no es denega sola als dos minuts.**
- **Quan el proveïdor no envia ni un byte, el bloc de feina ho diu**: «el
  proveïdor no ha respost en 23 s · Esc atura», a partir dels 8 s. Abans
  deia «escrivint…» durant minuts amb el motor d'inferència caigut darrere
  d'un gateway (504/524 o cap resposta) i semblava que el TUI s'havia penjat.
  Test amb un proveïdor que no respon mai: Esc talla la petició en menys d'un
  segon i la capçalera queda quieta.
- Les files de les eines segueixen sent les seves després del retall de
  línies velles (l'índex del timeline es desplaça amb la resta).
- Al web, el test de cancel·lació comprova també que `agent_busy` cau abans
  de 2 s: és el que mou la mar de la capçalera (`body.working`). El compte enrere
  venia de la web; en un terminal, aixecar-se de la cadira volia dir
  tornar amb l'edició rebutjada i l'agent continuant amb «EINA REBUTJADA».

### Aprovació

- **El diff es veu ABANS de decidir.** Fins ara el que es llegia era el
  JSON dels arguments retallat a 300 caràcters i el diff arribava després
  d'aprovar: un `write` d'un fitxer sencer s'aprovava a cegues.
- **Tres botons**: Permet · Sempre en aquesta sessió · Denega. «Sempre»
  existia (la tecla `r`) però només com a lletra dins d'una pista, i
  semblava que l'agent demanava permís per la mateixa cosa a cada pas.
- La descripció és llegible (`edit internal/config/config.go`) i la barra
  d'estat ja no en repeteix una còpia tallada a mig JSON.

### Proveïdors, claus i menús

- **Catàleg de proveïdors** (`config.Cataleg()`): Ollama, LM Studio,
  llama.cpp, OpenRouter, OpenAI, Groq, DeepSeek, Mistral, Together,
  Cerebras i OpenCode Zen, amb la URL bona i la variable d'entorn
  habitual. Es dona d'alta des de providers → «+ afegeix un proveïdor»;
  si la variable ja existeix, la clau s'agafa sola.
- **Les claus es desen** a `~/.config/gregal/auth.yaml` (0600), fora del
  config. Abans una clau literal només vivia en memòria i, sense cap
  `${VAR}` definida, cada arrencada te la tornava a demanar.
- **La clau no s'escriu mai a la pantalla**: les tecles no passen pel
  composer, es pinten com a rodonetes i no toquen l'historial.
- **Els menús filtren escrivint.** Amb un proveïdor que llista seixanta
  models, triar-ne un era baixar cinquanta cops amb la fletxa.
- **Triar model desa sempre.** El menú ho feia només per a la sessió i
  `/provider rol` ho desava: dos camins que feien coses diferents sense
  dir-ho.

### Pantalla

- **Una fila per eina.** La crida i el resultat compartien dues files
  (`▸ bash …` i després `✓ bash`); ara el resultat reescriu la fila de la
  crida: `✓ read internal/config/config.go · 9 línies`. Les escriptures
  tampoc repeteixen «edit aplicat a x.go» just sobre el diff.
- **La benvinguda marxa amb el primer missatge.** Ocupava tretze de les
  trenta-quatre files d'una pantalla normal i no se n'anava mai.
- La mar de la capçalera ja no surt enganxada a la branca.
- La barra lateral de scroll només es pinta quan hi ha res a desplaçar.
- Fora emojis de la conversa i de la barra: la identitat va de glifs
  monoespaiats, i els cadenats feien una cel·la o dues segons el terminal.

### `gregal advise`: el gregal proposa millores

- Sis detectors de només lectura (toolchain Go, lockfile npm, backend
  empaquetat desfasat, higiene del repo, mòduls Go i paquets npm
  actualitzables) que tornen propostes amb severitat, esforç i caducitat.
- Robusta per disseny: cada detector té timeout i recuperació de pànic;
  sense xarxa, sense git o sense npm, ho diu (`no-disponible`) i continua.
  `--offline` omet la xarxa del tot; el codi de sortida sempre és 0.
- Les propostes es desen a `propostes.json` (atòmic, 0600, sense duplicats,
  caduquen a 30 dies); un fitxer corrupte es conserva com a `.corrupte-*`.


## v1.5.2 — 2026-09-20 — mode autònom, cua d'execucions i servei compartit

- **Mode `autonomous`**: tasques llargues per fites. `internal/agent/autonomous.go`
  i `autonomousRules` (les regles de `code` més un contracte de llarg abast).
  Cada `checkpoint_every` eines (10 per defecte) fa un checkpoint i cada
  `review_every` (2) demana revisió al rol `reviewer`. Límits de seguretat:
  `max_minutes` (120), `max_tool_steps` (500), `max_cost_usd` i les comandes de
  `verify` que corren a cada checkpoint. Vàlid a `--mode`, a `/mode` i a la webapp.
- **Cua d'execucions compartida**: un torn per sessió, serialització per
  workspace, prioritat i cancel·lació per id; API v2 de runs amb idempotència i
  esdeveniments de cicle de vida (`/api/v2/events`). El TUI i l'escriptori
  deleguen els torns a la cua quan hi ha servei connectat.
- **Fixats (pins) persistents** al model i al llistat de converses.
- **Servei compartit**: descoberta local amb descriptor, lock recuperable i
  identitat del servei a `/api/health`.
- **Workspace explícit** a les execucions no interactives i als fluxos;
  verificació amb el shell del sistema.

## v1.5.1 — 2026-09-20 — el mode xat pot mirar el sistema, i el rol nou surt al teclat

- **La data del sistema entra al prompt** de tots els modes («DATA D'ARA:
  2026-09-20 (diumenge)»), només el dia i no el rellotge: el motor local de
  Halogen desa el punt de represa del seu KV cache al final del system prompt,
  i un valor que canvia cada minut el tornava a invalidar a cada torn (2 s
  contra ~88 s en un torn de seguida, xifres del mateix motor). L'hora exacta
  la dona `date`.
- **`date` i companyia són lectura segura**: `date`, `uname`, `hostname`,
  `whoami`, `id`, `uptime`, `free`, `nproc`, `lsblk` i `locale`. Abans `date`
  no hi era i el mode xat responia «EINA BLOQUEJADA: fora de la llista
  segura» a qualsevol pregunta sobre el dia, i el model acabava responent de
  memòria. Els encadenaments, les redireccions i `sudo` continuen denegats.
- **El teclat 🧠 Rol del bot surt del config**: un rol nou (com `halogen`) hi
  apareix sol, darrere dels tres de sempre, i el reviewer no s'hi posa mai.
  Abans la llista era fixa al codi (chat/think/code) i un rol només es podia
  triar escrivint `/role`.

## v1.5.0 — 2026-09-18 — navegador, cerca web pròpia, pla viu i context per model

- Banc d'evals des del binari: `gregal --config evals/example.yaml --eval
  evals [--tasks t1,t6] [--eval-baseline]` corre les tasques en còpies,
  pinta veredicte, passos, tokens i segons, escriu `results/summary.json`
  i compara amb `evals/baseline.json` (codi 1 si hi ha regressions). El
  run.sh i el run.sh duplicaven la lògica i no donaven cap xifra.
- Finestra fora de línia: quan el backend no respon, una franja vermella
  ho diu a dalt amb «Torna-ho a provar», els sondejos s'espaien i en
  tornar la connexió tot es refresca sol. Abans: «Failed to fetch» dins
  d'una bombolla i sondejos cada segon en silenci.
- Finestra: capa de lectura. Text de conversa a 15 px amb interlineat
  1,7 (abans 14 px), columna de lectura de 760 px compartida per
  missatges i composer, missatges de l'agent sense icona ni reixa, els
  teus com a bombolla suau, composer amb més aire i ombra, etiquetes en la
  mateixa lletra que el text (fora monoespai en majúscules), «Segoe UI
  Variable» a Windows, i l'inspector plegat per defecte (botó ▥ a la
  capçalera, es recorda). El que separava la finestra de Claude i ChatGPT
  era la densitat, no el color.
- Reintents del client LLM per a proveïdors que arrenquen sota demanda:
  sis intents amb espera exponencial (1 s → 20 s, uns cinquanta segons en
  total; abans tres intents en mig segon), `Retry-After` respectat als
  429, i l'avís «proveïdor HTTP 502 · reintent 2/6 d'aquí a 4 s» visible
  al bloc de feina del TUI, a la barra del torn de la finestra i com a pas
  al headless. Vist en viu amb `localhost`: la primera pregunta
  passava i les següents morien al tercer 502 sense dir res.
- Selector de projecte a la finestra: en comptes d'un `prompt()` amb una
  llista numerada, un quadre amb els recents, un navegador de carpetes
  del servidor (`/api/dirs`: engrunes, unitats, repositoris git primer) i
  ruta a mà; a l'escriptori també el diàleg del sistema. S'obre des del
  nom del projecte a la capçalera i des de «Projecte…» a Fitxers. Les
  converses de la barra lateral duen el nom del projecte quan no és el
  d'ara: obrir-ne una canvia de workspace i ara se sap cap a on.
- Diagnòstic després d'editar: a més de la sintaxi, `go vet`,
  `py_compile`, `node --check` o `bash -n` sobre el fitxer tocat
  (`hooks.diag: full`, defecte; `syntax` per tornar a només sintaxi).
- Conversa més neta: les crides d'eina diuen «read internal/x.go» o
  «bash go test ./…» en comptes del JSON; les lectures que van bé es
  pleguen a una línia («✓ read 42 línies»; el cos sencer és a Ctrl+L);
  el codi en línia de les respostes ja no porta espais de farciment.
- Pressupost de passos tou: el `max_steps` és el tros base, i mentre el
  model avanci (eines noves executades des de l'últim tros) i digui
  CONTINUA se li concedeixen trossos de 20 passos tantes vegades com
  calgui (topall de seguretat de 25). Abans eren dos trossos de deu i les
  tasques llargues quedaven a mitges. Les regles de mode code li diuen que
  acabi el checklist en comptes d'estalviar passos.
- Ratolí: amb `mouse: off` el programa ja arrenca sense capturar-lo (a
  Windows, activar-lo i desactivar-lo després no sempre alliberava la
  selecció del terminal); pista de Shift+arrossegar per seleccionar amb la
  roda activa.
- Cerca web sense cap servei local: `web_search` consulta DuckDuckGo i
  Bing alhora (i SearXNG si hi ha `SEARXNG_URL`), fusiona i desduplica
  els resultats i els guarda deu minuts en memòria. Abans depenia d'un
  SearXNG a 127.0.0.1:4000 i sense ell la cerca no existia.
- Lector web per a agents (`internal/tools/webread.go`): `web_fetch`
  torna el contingut principal de la pàgina en markdown compacte (títol,
  data, autor, capçaleres, llistes, codi, taules i enllaços; sense menús,
  peus, banners, formularis ni imatges). Opcions `find` (només els
  paràgrafs que contenen un text), `max_chars` i `raw`. Les vies de
  recuperació (render local i Wayback) corren en paral·lel i el render
  només s'intenta si hi ha un Firecrawl escoltant.
- Eines en paral·lel dins d'un pas: les lectures (read, grep, glob,
  web_*, gh_*, office_read…) que el model demana juntes s'executen alhora
  (fins a 6) al TUI, al headless, al web/escriptori, al Loop i als
  subagents; les escriptures i el bash continuen en ordre i l'historial
  no canvia.
- Pla viu: en executar un `/plan` aprovat, els passos numerats passen al
  checklist abans del primer pas del model; si el model marca un pas fet
  sense dir quin és el següent, el primer pendent passa sol a «en marxa»;
  i passades cinc eines sense `todowrite`, l'últim resultat del pas porta
  un recordatori perquè actualitzi el checklist.
- Eina `browser`: navegador real (Chrome/Edge de la màquina via Chrome
  DevTools Protocol, sense extensions ni serveis) amb perfil propi del
  gregal i finestra visible. Obre, llegeix la pàgina en markdown amb
  elements interactius numerats, clica, escriu, prem tecles, captura
  pantalla i executa JavaScript. Mirar passa sol; actuar demana permís;
  en mode consulta i xat només mira. Prova viva: `GREGAL_LIVE_BROWSER=1`.
- Complement d'Office redissenyat amb el llenguatge de l'Office (Segoe UI,
  reixa de 4 px, tema clar/fosc del programa): accions ràpides sobre la
  selecció (reescriu, corregeix, resumeix, tradueix…), «Substitueix la
  selecció» i «Insereix», resultat amb format de debò al Word
  (`insertHtml`) i taules a l'Excel com a matriu de cel·les.
- Complement d'Office: mode consulta («Només consulta: no toca el
  document») per a documents compartits: l'agent respon al panell, sense
  inserir ni substituir, amb accions de lectura (resumeix, explica,
  revisa, punts clau, preguntes, verifica) i el torn en mode xat al
  servidor (`/api/agent` accepta `mode: chat|inspect` per restringir un
  torn).
- `office_edit`: `append` (paràgrafs amb format al final d'un docx o de
  l'última diapositiva d'un pptx) i `set_range` (bloc de cel·les xlsx
  d'una vegada).
- `--doctor` comprova quins motors de cerca web responen, si hi ha navegador per a `browser`, i quina finestra
  de context s'aplica a cada rol (config, model o defecte).
- Finestra de context per model: `context_window: 0` vol dir «la que
  declari el proveïdor» (`/v1/models`: `context_length`, `max_model_len`,
  `n_ctx`…), detectada a l'arrencada i apresa dels errors de context
  excedit. Abans un 0 volia dir 32K (8K en local) fos quin fos el model.
- Compactació per etapes a mig torn (`internal/agent/histcompact.go`),
  al TUI, al headless, al web/escriptori i als subagents: retall de les
  sortides d'eina velles, després resum dels passos anteriors amb el
  model (com opencode), i retall segur només si el resum falla. Abans es
  tallava cegament als quatre últims missatges.
- Detector de bucles per fitxer (`DoomSig`): quatre `write` de fitxers
  diferents a la mateixa carpeta ja no es bloquegen com a «eina repetida».
- `PlanCalls` unifica la decisió de permís i repetició de les crides d'un
  pas per al headless, el web i els subagents.

- `/verify on|off|toggle` al TUI (drecera de `verify mode auto/off`, es desa),
  amb indicador persistent `✓ verif` a la barra quan el revisor passa sol.
- Reintent automàtic acotat d'errors de servidor (5xx/429/xarxa) a mig torn
  al TUI, al headless (`-p`) i al backend web/escriptori: guia el model
  perquè regeneri el pas en comptes de matar la feina feta.
- Regles de mode code per a models petits: noms de fitxer exactes (sense
  traduir ni inventar) i comprovar dades del repo amb eines abans de respondre.
- Client LLM tolerant amb models locals: camps `thought`/`thinking`,
  extracció de `<think>`, i `question` que accepta `question:` i opcions com
  a strings o mapes (`internal/tools/question.go`).
- Evals reproduïbles contra `localhost`: `evals/example.yaml` +
  `.\evals\run.sh`; prova viva del mode pla (`TestStrixPlanLive`).
- Fletxa ↓/↑ de l'historial: navegació d'un en un sense salts, no es
  duplica en drenar la cua, no s'encalla en entrades multilínia ni la
  segresta el popup de `/ordres`; bloc de feina en viu d'una sola línia.
- Cadenat de permisos a la barra: 🔒 tancat (cal permetre) per defecte,
  🔓 obert en `/permissiu` (l'agent tira sol).
- Crida d'eina escrita al text: en comptes de tancar el torn, el TUI,
  l'headless i el web guien el model perquè la refaci estructurada
  (acotat; el text no s'executa mai).
- Vista del TUI d'alçada constant en escriure: el viewport cedeix les
  files visuals reals dels popups i del composer (inclòs l'embolcall);
  ja no fa scroll ni amaga blocs a cada tecla.
- Mode goal amb eines de només lectura: com que el prompt li promet
  read/grep/glob, ja passa per l'agent (la política denega escriure);
  abans anava sense `tools` i la crida arribava en text i matava el torn.
- Preguntes seleccionables a tots els modes: el prompt de xat i de goal
  ja diu que les decisions van amb l'eina `question` (mai llistes en
  text pla); el TUI obre el seleccionable igual a code, xat i goal.
- Menú d'accions en desar l'objectiu (Executa/Edita/Llista/Esborra amb
  fletxes); Executa del menú /goal ja no penja el torn (el Cmd anava
  a parar enlloc: ara va per deferred amb l'estat del pas).
- `/mouse` es desa (`mouse: on|off` al config): la tria entre roda i
  selecció nativa sobreviu reinicis.
- Sessions que es reprenen de debò: `/resume N` (o clic a la fila) amb
  llista numerada, restaura rol, conversa i resum, treu duplicats,
  avisa de rol perdut o altre directori; en obrir diu quantes n'hi ha.
- El seleccionable es navega: ↑↓ mouen el ressaltat (amb volta),
  Enter el confirma; escriure text continua sent resposta lliure.
- El seleccionable té fila de text lliure (✎, sense número): mostra en
  directe el que escrius al composer, retallada perquè no mogui el layout.
- Barra lateral de scroll a la conversa (rail + polze de posició),
  sempre visible i sense moure res.
- Escriure no mou la conversa: el KeyMap de pager del viewport (u, d,
  k, j, b, f, espai…) buidat; l'scroll va per dreceres explícites i la
  roda. Ctrl+U/D només desplacen amb el composer buit (amb text són
  edició) i el Tab només ensenya en buit.
- La capçalera porta la mar en feina: el buit viatja (swell) mentre
  l'agent treballa; en repòs, mar plana.
- El torn no es tanca amb el checklist a mitges: si el model anuncia i
  s'atura amb todos pendents, es guia i continua (acotat, als 3 fronts);
  doble Enter encuat una sola vegada.
- Checklist que no avançava: els models petits diuen `completed` o
  `in_progress` en comptes de `done`/`working` i tot queia a `pending`;
  ara s'accepten els àlies (vist en viu amb el Qwen) i la descripció de
  l'eina fixa els estats exactes.
- Office sense markdown imprès: `office_create` converteix `#`, `-`,
  `**`, `*`, taules i enllaços a format Word/PowerPoint de debò (el
  codi entre ``` queda literal); `office_edit`/`set_cell` netegen a
  text pla.
- Pressupost de passos decidit pel model: en esgotar `max_steps` se li
  demana `CONTINUA n` o `FINAL`; fins a 2 extensions de 10 passos, només
  si ha executat eines de noves (lectures i tests també compten), als 3
  fronts. El `%` de context és l'ompliment real de l'última petició.
- Rutes Windows a Git Bash (`C:\...` → `/c/...` automàtic a l'eina
  `bash` i processos, mai als hooks): els `ls` amb rutes absolutes ja
  funcionen; el web ancora el model al Directori de treball (res de
  Downloads sense que ho demanin).
- Doom difús: a més de 3 idèntiques, 4 seguides amb la mateixa plantilla
  (repro8.mjs → repro9.mjs…) també aturen amb guia.

Millores prioritàries d'estabilitat, recuperació d'errors i consistència:

### Sessions segures de reprendre
- Persistència completa de l'estat de sessió (`Workspace` i resum `Compacted`) tant al backend web com al TUI. En reobrir una sessió es restaura el directori de treball i el resum compactat sense perdre context.
- Escriptura atòmica via fitxer temporal (`.tmp` + rename) evitant corrupció per fallades durant el desat.
- Timestamps a nivell de milisegon a `NewName()` per evitar sobreescriptures de fitxers en desats automàtics simultanis.

### Inicialització i recuperació de l'aplicació d'escriptori
- Capçalera d'autorització `Authorization: Bearer <TOKEN>` a la comprovació de salut `ping()` de l'escriptori quan hi ha token configurat.
- Canal IPC `gregal:retry-backend` que reinicia el servidor backend quan l'usuari prem «Reintenta» a la pantalla d'error.
- Disseny de `error.html` adaptat a la paleta oficial de `docs/identitat.md` (`#7FD4C1`, `#0F172A`, `#111C2F`), amb suport bilingüe català/anglès.
- Menús natius i diàlegs d'Electron bilingües segons l'idioma configurat.

### Consistència d'interacció i traduccions
- Traducció completa de la llista de converses (`convs.js`): agrupació temporal (avui, ahir, últims 7 dies, aquest mes, anteriors), confirmació d'esborrat, estats de càrrega i d'error en català i anglès (15 noves claus amb paritat estricta).
- Correcció d'execució de hooks `post_edit` en entorns Windows (`internal/agent/hook.go`) via resolució dinàmica de shell (`sh.exe`/`cmd.exe`).
- Eliminació de falsos positius a `doctor.go` amb permisos Unix en Windows.

## 1.4.3 — 2026-09-16 — recuperar el que has escrit, i un TUI que es llegeix de dia

Aquesta tanda surt de comparar Gregal amb el que es dona per fet en una
eina d'aquestes el 2026 i quedar-se amb el que faltava de debò.

### Edit i patch fallaven sempre en fitxers de Windows

Un fitxer sortit d'un git amb `core.autocrlf` —aquest repositori mateix—
té `\r\n`, i el model escriu el bloc amb `\n`, que és el que fa tothom. La
comparació era exacta: no casava **mai**. El que rebia el model era «bloc
no trobat», sense cap pista, i el que feia llavors era tornar-hi igual o
reescriure el fitxer sencer. A `patch` feia més mal encara, perquè aplica
diverses edicions seguides i n'hi ha prou que en falli una perquè no se'n
faci cap.

Ara la comparació ignora els finals de línia i el fitxer es desa amb els
que tenia. I quan de debò no hi és, l'error distingeix els tres casos, que
es resolen de maneres diferents: hi és amb un altre sagnat, hi ha la línia
però no el context, o no hi ha res semblant.

### Quan el model escriu la crida d'eina al text

Alguns models, quan la crida estructurada no els surt, l'escriuen al cos
del missatge (`<tool_call><function=write>…`). Per a l'agent el torn s'ha
acabat, i això anava tal qual a la pantalla: markup en comptes de la feina
feta, i sense cap indici que el model ho havia intentat i no havia pogut.
Ara es treu, es manté la frase que l'acompanya i es diu què ha passat. No
s'executa: ve del model i el nostre client no l'ha validada.

### Recuperar el que ja has escrit

- **Ctrl+R al TUI**: cerca a l'historial per trossos, com al bash. Amb ↑/↓
  el recorries d'una en una, i per trobar aquella ordre llarga de fa dos
  dies havies de picar la fletxa vint vegades. Enter la posa al camp i no
  l'envia.
- **↑/↓ al composer de la finestra**: no hi havia historial **gens**. Per
  repetir un missatge o corregir-ne un de llarg l'havies de tornar a
  escriure sencer.

### Un TUI que es llegeix de dia

En un terminal de fons clar, el text apagat de la paleta fosca és `#475569`
sobre blanc: la meitat de la pantalla no es llegeix. Ara hi ha `theme:` al
config i `/theme` per canviar-lo en calent. Els colors surten de la taula
«Paleta clara» de `docs/identitat.md`, la mateixa que fa servir la
finestra; no n'hi ha cap d'inventat.

## 1.4.2 — 2026-09-16 — que sàpiga quina màquina trepitja

### Set passos de dotze buscant el compilador

Mirant un torn real amb el model de debò, sobre un projecte Go a Windows:
de dotze passos, **set** se'n van anar buscant on era el compilador. `go
version` fallava —el toolchain no és al PATH d'aquesta màquina— i a partir
d'aquí va provar `which go.exe`, `ls /c/Go`, `find /c -maxdepth 3 -name
go.exe`, `echo $PATH`… fins a trobar-lo al pas dotze, amb el pressupost ja
esgotat. La feina va sortir bé, però per sort.

No és res que pugui deduir: o li ho diem, o ho busca. Ara el system prompt
porta el sistema, amb què s'executen les ordres (a Windows és Git Bash, no
cmd) i les eines trobades. El cas que importa no és «hi és» sinó «hi és
PERÒ no és al PATH», amb el directori a punt per enganxar.

Mateixa tasca, mateix model: **7 passos en comptes de 12**, sense esgotar
el pressupost, i els tests que escriu passen.

### El bash tenia trenta segons i compilar en fred en costa trenta-sis

Mesurat en aquest mateix repositori amb la cau de Go buida, `go build
./...` triga 36s. Amb el topall a 30s, la primera compilació d'una sessió
es tallava **sempre**, i el que arribava al model era un «timeout (30s)»
sec del qual no es pot saber si el codi està trencat o si només ha faltat
temps. Ara són dos minuts, i quan salta, el missatge diu per on sortir-se'n.

### La barra del TUI canviava informació viva per ajuda fixa

Pintant la pantalla de debò a 110 columnes —una amplada de terminal de les
normals— el mesurador de context no hi era: les pistes de teclat li havien
pres el lloc. Ara les pistes van les últimes, i cedeixen per graons
—llargues, curtes, cap— com ja feia l'esquerra.

De passada, la caixa de pregunta i la de checklist es quedaven a 78
columnes mentre el menú, la confirmació i el composer feien tota l'amplada.

### L'anglès arribava a la barra lateral i s'aturava abans de la pantalla

Obrint la finestra amb `lang: en` i mirant-la: el menú en anglès i tot el
que hi ha al mig en català. La benvinguda, les quatre targetes d'inici, els
xips Codi/Xat/Objectiu, els buits de GitHub, Fitxers, Office, Objectius i
Grafs, i les dreceres. És literalment la primera pantalla. Comptat amb un
recorregut del DOM: **29 rètols abans, 0 després**.

El TUI tenia el mateix forat i s'ha tancat igual: 70 literals que no
passaven per T().

Tres coses que hi han sortit pel camí:

- Traduir en carregar el mòdul no funciona: la taula de modes cridava la
  traducció abans que existís i petava la pàgina sencera. És el mateix
  error que ja havia passat amb la paleta d'ordres del TUI.
- `refresh()` posava l'idioma al final, després de pintar.
- Tres claus dels grafs eren al diccionari en totes dues llengües i no les
  feia servir ningú. Hi ha una prova nova de claus mortes.

## 1.4.1 — 2026-09-16 — la finestra ja no oblida en silenci

### La conversa deixava de cabre-hi i ningú ho deia

El TUI compactava des del principi: en acabar el torn, si la conversa
passava del 75% de la finestra, la resumia i es quedava els darrers
missatges. La finestra —que és on es passen les hores— no ho feia mai.
Una sessió llarga hi creixia sense límit fins que el model retallava el
context pel seu compte, sense dir-ho, o petava.

Ara compacta **abans** de cada torn, així el que ve ja hi cap en comptes
d'assabentar-nos que no hi cabia quan ja és tard. I es diu: surt un
missatge al fil i una entrada a l'activitat. Perdre detall en silenci és
el que fa que després no s'entengui per què l'agent ha oblidat una cosa
dita fa estona. Comprovat contra l'endpoint real amb una finestra de 2000
tokens: torns 1-3 sense tocar res, i al quart la compactació.

### Amb «lang: en» al config, la finestra sortia en català

El TUI respectava el config i la finestra no: només mirava el navegador
i, si no hi havia res desat, es quedava en català per sempre. Ara mana la
tria del navegador quan n'hi ha, i el config quan no.

### L'anglès estava fet a mitges

Els rètols fixos es traduïen, però tot el que l'app diu **en marxa** —la
cua, el torn aturat, els passos esgotats, el permís caducat, les entrades
d'activitat— eren frases en català escrites al codi. Amb la interfície en
anglès, cada missatge que et deia alguna cosa et sortia en una altra
llengua.

51 claus noves i una prova nova: es comprovaven els `data-i18n` de la
pàgina, però ningú mirava els `gregalT('clau')` del codi. Fent-ho s'hi van
escapar catorze claus que no arribaven mai al diccionari i pintaven
literalment `ui.passos` a la pantalla; com que els dos diccionaris hi eren
igual de buits, la prova de paritat passava tan tranquil·la.

### Rebobinar una pestanya retallava la conversa de l'altra

Els fitxers ja anaven filtrats per workspace, però les marques de conversa
—on retallar l'historial en rebobinar— compartien sac, indexades pel seq
global. Amb dues pestanyes obertes, un rewind a la sessió A podia trobar
la marca de la B i retallar-li la conversa a una llargada que no era seva:
missatges perduts sense cap avís.

### Detalls

- `--addr 0.0.0.0:9000` ja no respon «too many colons in address»: si
  l'adreça porta port, mana ella.
- L'avís d'«exposat a la LAN sense token» ja no salta amb `::1` ni
  `127.0.0.2`, que són loopback. Un avís que salta quan no toca s'ignora.

## 1.4.0 — 2026-09-16 — que les eines treballin on toca, i tot en dos idiomes

### Les eines treballaven al directori equivocat

Canviar el projecte d'una pestanya canviava els panells de fitxers però
**no on treballaven les eines**: `bash` s'executava al directori on
s'havia engegat el servidor, i `read`/`write`/`edit`/`grep`/`glob` amb
rutes relatives hi resolien també. A l'escriptori, una pestanya que havia
canviat de projecte llegia i escrivia a l'altre.

La resolució es fa en un sol lloc, sobre el JSON dels arguments, i no als
dotze casos: n'hi hauria hagut prou que un s'oblidés. Amb el TUI i el
headless —una sola sessió— no canvia res.

### Tres coses més que es creuaven entre pestanyes

- **Els processos de segon pla** duien tots l'etiqueta `"agent"`: no
  sortien al Terminal de la pestanya que els havia engegat i tancar-la no
  els matava. El comentari deia que el web hi posava l'id de sessió i no
  era veritat.
- **La checklist** era global: la sessió que començava un torn esborrava
  la de la que estava treballant, i un `todoread` podia tornar la llista
  de l'altra conversa.
- **Canviar de pestanya** feia cinc peticions en sèrie, dues d'elles
  duplicades i dues cares (`git diff` i l'arbre de fitxers) que es feien
  encara que estiguessis mirant la conversa. Ara en fa una d'esperada:
  mesurat, 239 ms.

### L'agent avisa si una edició deixa el fitxer trencat

Escrivia un JSON amb una coma de més i l'eina responia «escrit, 412
bytes» i tan amples. Ara `write`, `edit` i `patch` comproven Go, JSON i
YAML i ho diuen al mateix resultat. Només sintaxi i en procés: 0,9 ms en
un `.go` de 400 funcions.

### El TUI ja no es torna espès

Mesurat amb 20.000 línies: el `strings.Join` de la conversa costa 0,5 ms
i el `vp.SetContent` del mateix text, 6,3 ms. El temps se n'anava tot al
viewport i creixia amb la sessió. Ara la pantalla s'acota a 6.000 línies
i una sessió de 40.000 es queda a 2,2 ms. No es perd res: la conversa
sencera es desa a disc.

### Català i anglès a tot arreu

- **La finestra**: les vuit pàgines, no només la navegació.
- **El TUI sencer**: pancarta, insígnies, composer, barra d'estat,
  `/help`, el desplegable de `/`, els menús, els proveïdors, els
  objectius i tots els missatges d'estat i d'ús. 267 claus.

Tres tests ho sostenen: els dos diccionaris han de tenir les mateixes
claus, cap clau pot quedar sense fer servir, i les descripcions d'ordres
han de seguir l'idioma actiu. El tercer va néixer d'un error real: com a
variable de paquet, les traduccions s'avaluaven abans que es fixés
l'idioma i el desplegable es quedava en català per sempre.

### Colors fantasma al TUI

Quatre colors escrits a mà fora de la paleta, entre ells dos `#526DFF`
—el mateix blau genèric que ja vam treure de la web. El test d'identitat
existia però només mirava `theme.go`, i vivien als altres fitxers; ara
mira tot el paquet.


## 1.3.2 — 2026-09-15 — canviar de pestanya deixa d'esperar

Amb dues sessions treballant, canviar de pestanya es quedava penjat
segons. Feia cinc peticions en sèrie —sessions vives, `/api/state`,
`/api/state` un altre cop (per al transcript, que ja venia a la primera),
el `git diff` i l'arbre de fitxers— i les dues últimes, que són les
cares, es feien encara que estiguessis mirant la conversa i no aquells
panells. Amb l'agent escrivint al disc, encara més lentes.

Ara en fa **una** d'esperada i la resta van després sense bloquejar; els
panells cars només es carreguen si els estàs veient. Mesurat: 4
peticions, 239 ms, i ni diff ni arbre.

La barra de pestanyes també: el sondeig de cada 5s demanava la llista i
`render()` la tornava a demanar, i repintava encara que no hagués canviat
res.

### Sobre les preguntes amb opcions: és el model

Mesurat amb el mateix banc, mateixa tasca ambigua, 8 mostres:

| model | crida `question` |
|---|---|
| Qwen38-Flash-FAST | **8/8** |
| Ornith-1.5-35B | 6/8 |
| **Nex-2.5-mini** | **0/8** |

El `roles.code` per defecte apuntava a `Nex-2.5-mini`, que és el que mai
no la crida. No hi ha res a arreglar al codi: cal triar un model que
faci tool-calling de debò per al rol que fa la feina.


## 1.3.1 — 2026-09-15 — el tema, els models i «qui respon»

### «rol: chat» desapareix: ara és «Qui respon», dins la píndola del model

Era un desplegable a la barra lateral que deia el **nom intern d'una
entrada del config**, i no hi havia manera de saber què feia. A més era
un segon control per a la mateixa pregunta que ja responia la píndola de
dalt: quin model contesta.

Ara és allà, amb **Automàtic** (i quin ha triat ara mateix) i cada rol
amb el model que duu a sota, dit en pla: *per parlar*, *per pensar*, *per
programar*, *per revisar*.

### Llistar models amb un proveïdor apagat

Baixa de 10s a **4s** per proveïdor: llistar models és un GET a un
endpoint que ha de ser instantani, i amb 10s un proveïdor que s'empassa
la connexió feia esperar tota la llista.

I l'error deixa de ser el de Go en cru —«dial tcp [::1]:8089: connectex:
No connection could be made because the target machine actively refused
it»— per dir el que necessites saber: **«localhost:8089 no accepta
connexions (el servidor no està engegat?)»**.

### Els botons de la finestra segueixen el tema

Els botons de minimitzar, maximitzar i tancar els pinta el **sistema**,
no el CSS, i el color anava escrit a mà al `main.js` amb els valors del
tema fosc. Amb el tema clar quedava un requadre negre a dalt a la dreta
enmig d'una barra clara.

Ara la pàgina li passa els colors del tema actiu (els mateixos tokens que
la fila de pestanyes, que és la que fa de barra) en arrencar, en canviar
de tema i quan el sistema canvia. L'alçada també surt d'una constant
compartida, que abans hi havia el 38 escrit a dos llocs.

La finestra tampoc no neix fosca: els colors d'arrencada —els que es
veuen abans que la pàgina carregui— surten del tema del **sistema**, que
és el que el gregal segueix si no tries res. Abans, a qui va en clar li
feia un flaix negre. Un test lliga aquests colors a la paleta de
`docs/identitat.md`: és exactament així com s'havien desenganxat.


## 1.3.0 — 2026-09-15 — que les eines es cridin quan toca

Una tanda de fer que el que ja hi havia surti de debò a la pantalla, amb
els números al costat: tot el que diu aquesta entrada està mesurat contra
el model real, no per impressió.

### L'agent pregunta en comptes d'endevinar

El mode `code` **no tenia cap regla** al prompt: l'únic que llegia el
model era «primer la solució», i davant d'una petició ambigua endevinava
i es posava a escriure fitxers. L'eina `question` —amb tot el circuit fet
a la web i al TUI— no la cridava mai.

Mesurat amb el projecte ja explorat, o sigui amb la decisió damunt de la
taula, 16 mostres per variant:

| | tasca ambigua | tasca clara (control) |
|---|---|---|
| sense regla | **0/16** | — |
| regla base | 14/28 (50%) | 0/12 |
| regla afinada | **11/16 (69%)** | 0/12 |

El control importa: la regla no el torna preguntaire. I el segon
paràgraf parla de fitxers perquè el mode de fallada observat era aquest —
la meitat de les vegades que no preguntava, triava ell (SQLite, JSON, una
interfície) i escrivia el fitxer.

**La descripció de l'eina no mou res**: provada llarga en català, curta i
en anglès imperatiu, totes donen el mateix. Queda escrit al codi perquè
ningú ho torni a provar.

### Les eines que es perdien

Provades **soles, sense competència, totes es criden 3/3** —`todowrite`,
`patch`, `delegate`, `gh_issue` i `bash_background` incloses. Els
esquemes i les descripcions són correctes: el que es perd, es perd contra
les altres 22. Dues arreglades fent que la **germana** els cedeixi el
pas: `bash` avisa que si la comanda pot passar dels 30s se'n vagi a
`bash_background` (2/3 → 3/3), i `edit` que per a diversos canvis al
mateix fitxer vagi a `patch`.

El `todowrite` depèn del model i no del text: **Nex-2.5-mini 4/4** (0/4
sense la regla), Qwen38-Flash-FAST 1/4, i els dos Ornith **0/4** amb
quatre formulacions. Si el checklist no surt, el que falla és el model
del rol `code`.

### Els passos esgotats ja no passen per feina acabada

Quan el loop esgotava `agent.max_steps`, el web i el TUI feien la síntesi
final i prou: a la pantalla arribava igual que una feina acabada. Ara ho
diuen tots dos (`steps_exhausted`) i diuen què fer.

### L'escriptori

- **Un sol commutador de mode**, al composer, amb els tres modes visibles
  i un color cadascun. N'hi havia tres alhora dient el mateix i cap
  ensenyava on eres.
- **Permisos**: un cadenat al costat, amb el detall al hover i un menú
  que explica els tres modes.
- **«rol: chat» deia què era, no què feia.** Ara diu «respon: automàtic»
  o el model fixat — i **s'hi pot tornar**: abans, triar-ne un apagava el
  router per a tota la sessió sense cap manera de desfer-ho.
- **Temps del torn** en viu i, en acabar, quant ha trigat.
- **Cua de missatges**: amb un torn en marxa, Enter encua en comptes de
  bloquejar-te. Aturar el torn la buida i ho diu.
- **El checklist, en un desplegable** just sobre el xat. L'agent el
  reescriu a cada pas i cada reescriptura deixava una targeta nova.
- **Pestanyes de sessió a dalt de tot**; a l'escriptori passen a ser
  elles la barra de títol, que si no els botons de finestra hi cauen
  a sobre.
- **Revisor automàtic com a interruptor** a Preferències. Passava un
  segon model per sobre de cada resposta i des de la finestra no hi havia
  manera d'apagar-lo.
- **Idioma triable** (català i anglès): canvia la finestra i també la
  llengua de les respostes de l'agent. Cobreix navegació, Preferències i
  el composer; la resta de pàgines i el TUI, encara no.

### Office

- **La subfinestra s'eixampla** amb l'agafador de l'esquerra (i amb
  fletxes), amb mínim i màxim que es recalculen.
- **El full es llegeix**: capçaleres A·B·C i números de fila, fixes en
  desplaçar-se, i **pestanyes de fulls a baix** com a l'Excel. Ensenyava
  20 columnes fixes i perdia la resta sense dir-ho.
- **Una ruta amb espais ja no es perd**: «EWEC EDH Project Plan.xlsx» es
  desava amb els espais i el lector de `@ruta` talla al primer espai, o
  sigui que el model rebia mitja ruta i deia, amb raó, que el fitxer no
  existia. Ara el temporal es desa sense espais i les mencions admeten
  `@"entre cometes"`.

### TUI

- **`/graf`** per mirar i executar els grafs del projecte. No els edita:
  dibuixar vol arrossegar. Els passos surten a mesura que acaben i Esc
  talla.
- Les accions dels missatges són icones dibuixades, no text.


## 1.2.0 — 2026-09-15 — grafs

Un **graf** és un procediment dibuixat: quins passos es fan, en quin
ordre, i quina fletxa se segueix segons el que hagi sortit. Es dibuixa a
la vista **Grafs** (Ctrl+G) i es desa a `.gregal/flows/` **del projecte**,
perquè «com es revisa un PR aquí» és del repositori i ha de viatjar-hi.

Val la pena dir què **no** és, perquè amb el mateix nom es venen tres
coses: un graf d'execució no és un graf de dependències ni un graf de
coneixement, i **tenir-lo no et dona memòria ni context**. Serveix quan
el procediment es repeteix, quan un pas ha de veure el que ha fet
l'anterior, i quan després vols poder mirar d'on ha sortit un resultat.
Per a una pregunta d'un sol cop no val la pena i no s'ha de fer servir.
Tot això és a `docs/grafs.md`.

- **Editor visual.** Passos d'agent, d'eina i de nota; s'arrosseguen pel
  llenç i es connecten estirant des del punt de sota d'un pas fins a un
  altre. A cada fletxa s'hi escriu quan s'agafa.
- **Llenguatge de condicions curt a posta**: `clau`, `!clau`, `==`, `!=`
  i `conté`. Tot el que demani més ha de ser un pas d'agent, no mitja
  gramàtica nova.
- **L'estat passa de pas a pas** amb `{{clau}}`, més `last` i
  `<id>.error`.
- **Després d'un pas que falla només valen les fletxes amb condició
  escrita**; sense cap, s'atura i diu quin pas ha estat. Continuar per la
  fletxa sense condició passant un valor buit era la manera silenciosa
  d'equivocar-se.
- **Sostre de 100 passos.** Els cicles hi són a posta —reintenta fins que
  els tests passin— i sense sostre un cicle mal posat dona voltes.
- **Executar il·lumina els passos** a mesura que passen i deixa el resum
  a la conversa: un graf que s'executa i desapareix no serveix de gaire.
  Amb «sense demanar permís» les eines que en demanen passen; les
  denegades continuen bloquejades.
- El motor (`internal/flow`) no coneix ni model ni eines, així que es
  prova sencer sense cap proveïdor; el TUI i el mode `-p` hi poden
  endollar el mateix agent quan els toqui.

També:

- **Al tema fosc, `--hover`, `--raised`, `--edge` i `--red-soft` es
  definien amb ells mateixos** (`--hover:var(--hover)`). CSS descarta una
  definició circular i el token es queda sense valor: tots els *hover* del
  fosc eren transparents, sense cap error a la consola. Hi ha test.
- `max_steps` admetia fins a 50 tot i que la configuració ja deixava
  posar-ne més.


## 1.1.9 — 2026-09-15 — tema clar i preferències

- **Tema clar**, amb la seva taula a `docs/identitat.md`: els accents
  s'enfosqueixen perquè l'escuma i l'arena del fosc no passen AA sobre
  blanc. Surt del sistema si no tries, i el segueix si el sistema canvia.
  Fer-lo va destapar **24 colors escrits a mà** en `rgba()` fora dels
  tokens —entre ells **dos blaus de la paleta antiga** i una pila de vels
  `rgba(255,255,255,…)` que en clar són blanc sobre blanc. Ara són tokens,
  i el test d'identitat mira també els `rgba` de color, que el tema clar
  defineixi tots els tokens del fosc, i que els valors surtin del document.
- **Preferències** (⚙ o Ctrl+,): un sol lloc amb seccions i tries d'un
  clic. Aparença (tema, densitat, mida del text: mouen tot l'espaiat i la
  tipografia alhora), Conversa, Agent, i els proveïdors com a secció.
  Abans el ⚙ només obria proveïdors, la vista vivia al menú ⋯ i els
  permisos a la capçalera.
- **El clip obre un menú**: imatges, **carpeta com a context** (tria una
  carpeta del projecte i en posa l'arbre al missatge) i **canviar de
  projecte** (diàleg natiu a l'escriptori).
- **Icones d'enviar i adjuntar dibuixades**, no emoji: l'emoji canvia de
  forma i de color a cada sistema i desentonava.

## 1.1.8 — 2026-09-15 — l'escriptori recorda de què heu parlat

L'escriptori tenia el motor però li faltava el que fa que ChatGPT o
Claude es facin servir cada dia: saber quines converses has tingut i
poder-hi tornar.

- **Historial de converses.** Es desen soles en acabar cada torn (abans
  només clicant «Nova sessió», i cada cop en creava una còpia), sempre
  al mateix fitxer: si tanques l'app enmig, no perds res. Cada una té
  títol, tret del primer missatge teu. A la barra lateral, agrupades per
  data (Avui / Ahir / Aquesta setmana / Aquest mes / Abans), amb cerca,
  l'actual marcada i esborrar en passar-hi per sobre. Reprendre'n una la
  continua en comptes de clonar-la. Les converses velles, que no tenien
  títol, se'l treuen del contingut.
- **Una targeta per crida d'eina**, que evoluciona: en curs → ✓ feta o
  ✗ fallada, plegada, amb la sortida a dins i els errors oberts. Abans
  n'hi havia dues i la primera duia «pas 1 · 1 eines», que es repetia
  sol i ja sortia a Activitat.
- **Accions al missatge** en passar-hi per sobre: Copia a tots dos, i
  «Torna-ho a provar» a la resposta.
- **Botó «↓ Al final»** quan has pujat a llegir.
- **Ressaltat de sintaxi** als blocs de codi (comentaris, cadenes,
  números, paraules clau). Era el que més distància marcava.
- **Dreceres**: Ctrl+N conversa nova, Ctrl+F cerca converses, Ctrl+K al
  camp d'escriure, i Escape atura el torn des d'on sigui.

## 1.1.7 — 2026-09-15 — el TUI deixa de saltar

- **Es pot fer scroll.** PageUp/PageDown no estaven implementades
  enlloc, tot i que la barra d'estat anuncia «Pg↑↓ scroll» des de
  sempre: pujar a llegir la conversa era impossible si no sabies
  la drecera no documentada (shift+fletxa). Ara hi són, amb
  ctrl+home/ctrl+end als extrems; home/end es queden per al cursor
  dins del text. I la roda del ratolí ve activada de sèrie, com a
  opencode (`/mouse` la desactiva per seleccionar text del terminal).
- **Escriure ja no et baixa al final.** `View()` forçava anar al
  final cada cop que canviava l'alçada del viewport, i l'alçada
  canvia cada cop que s'obre o es tanca l'autocomplete: escrivint
  et tornava al final a cada tecla i el text apareixia i
  desapareixia. Ara només se segueix el final si ja hi eres.
- **Una sola animació, i que digui alguna cosa.** N'hi havia tres
  alhora en tres punts de la pantalla: la marea a la capçalera, el
  shimmer «TREBALLANT» al composer i el spinner a la barra. Queda
  el spinner del composer, que ara diu què passa («agent pas 7/40 ·
  bash go test ./...»), quant fa que hi és i que Esc atura. La
  barra d'estat no repeteix la frase: només el comptador de passos.
- **Tres veus a la conversa.** `TU ›` i `GREGAL ›` eren dues
  etiquetes bessones i una crida d'eina pesava igual que la
  resposta. Ara el teu missatge porta barra i fons propis (`▌`), el
  que fa l'agent —pensament, crides i resultats— va en un rail
  tènue (`│`), i la resposta no porta cap adorn. El pensament passa
  al rail: era el que es confonia amb el principi d'una resposta.
- **La resposta ja no arrossega una cua d'espais.** Glamour omple
  cada línia fins a l'amplada amb espais pintats: un bloc de fons
  en terminal fosc, i una cua a qualsevol còpia del text.
- **El sostre de passos passa de 10 a 40** (rang 1–200). Deu és poc
  per a una tasca de codi real —llegir quatre fitxers, un grep i
  editar-ne dos ja te'ls menja— i el torn s'acabava a mitges. Esc
  atura el torn, que és el control de debò.
- El marc del composer sortia dues columnes més ample que la
  capçalera: `Width` és el contingut i la vora hi suma 2.

## 1.1.6 — 2026-09-15 — capçalera neta, navegació a la lateral

- Les seccions (Agent, Objectius, Activitat, GitHub, Office)
  viuen a la barra lateral plegable, que recorda si la vols
  oberta o tancada; la capçalera queda en burger + projecte +
  model + mode + permís + menú ⋯ (Pla, Compara, Desfés, Vista).
- L'Esc amb el menú obert el tanca sense aturar el torn.

## 1.1.5 — 2026-09-14 — estil Claude Code + reintent per length

- Preguntes triables, todos amb checks, cua de tasques i onada pro.
- Reintent automàtic quan el raonament es menja el pressupost:
  la tasca ja no mor per `length`.
- Web: el model de configuració es tria d'una llista;
  desktop: `build-backend` troba Go fora del PATH.

## 1.1.4 — 2026-09-14 — Office amb l'agent + barra pròpia

- Office com a subfinestra amb l'agent i `@docx` de debò;
  selector de models (pill + `/model` al composer).
- L'escriptori estrena barra superior pròpia.

## 1.1.3 — 2026-09-14 — cap eina fantasma

- **El mode xat prometia eines que no tenia.** El prompt deia "POTS
  llegir el disc amb read/glob/grep" (des de la 0.9.17), però xat i
  consulta anaven per `ChatStream`, que no porta `tools`. Un model que
  s'ho creu —DeepSeek— escrivia la crida en el seu format natiu (DSML)
  dins de la resposta, i l'usuari veia
  `<｜DSML｜invoke name="glob">…` imprès en comptes d'una llista de
  fitxers. Ara **tot mode que promet eines passa per l'agent** (TUI i
  web): code amb permís, xat i consulta amb la política de només-lectura
  que ja hi era. Goal es queda parlant, que és el que ha de fer. El preu:
  el xat perd el text en directe (l'agent no fa streaming); recuperar-lo
  és el primer punt de la fase H del pla 1.2.
- **Xarxa de seguretat per al DSML.** Si un proxy filtra el markup igualment,
  `ChatWithTools` el converteix en `tool_calls` normals i el loop continua;
  `ChatStream` el treu del text i l'error diu quina eina volia cridar i per
  què no pot. El markup no arriba mai a la pantalla. Provat amb la cadena
  exacta que va sortir al terminal.
- **El xat recupera el text en directe (H0).** Nou `ChatStreamWithTools`:
  la via amb eines fa streaming i reconstrueix les `tool_calls` dels
  deltes de l'SSE (acumulades per índex: l'ordre d'arribada no és l'ordre
  de les crides). L'agent —i per tant el xat i la consulta— tornen a
  ensenyar el text mentre creix, al TUI (mateix streamer que abans) i a la
  web (bombolla viva que es tanca a cada crida d'eina). Amb això ja no hi
  ha cap via que hagi de triar entre veure el text créixer i poder cridar
  eines; el pas anterior d'aquesta mateixa versió havia pagat el streaming
  per la correcció, i ara no cal.
- **Escriptori: el markdown del model es pinta de debò.** El renderitzador
  només feia codi, negreta i salts de línia: les respostes sortien amb els
  `##` i els guions en cru. Nou `app/md.js` (títols, llistes amb nivells,
  cites, regles, taules, enllaços només http(s), èmfasi, caselles de tasca,
  codi amb capçalera i «copia»), amb tot el text escapat abans de construir
  res i tests propis.
- **Escriptori: Canvis, Fitxers i Terminal es quedaven a mitja pantalla**
  amb el xat visible a sobre, com una làmina. `setView` sí que posava
  `hidden` al xat, però `#main.on` té més especificitat que la regla
  d'agent d'usuari i la trepitjava. `[hidden]{display:none!important}` i
  els panells ocupen tota l'alçada.
- **Escriptori: detalls que feien pobre.** El fil d'Ariadna (`/ main`)
  trepitjava les pestanyes en amplades mitjanes; «mode codi» es partia en
  dues línies; l'`input` del Terminal era blanc sense tema; les targetes
  d'eina duien el glif ≋ cadascuna (cinc en columna per un torn) i el text
  de treball d'un pas sortia dues vegades (bombolla viva i «Pensant…»). Tot
  resolt: la branca s'amaga per sota de 1100px, els botons no es parteixen,
  els inputs dels panells porten el tema, les targetes van sense glif
  alineades sota el text, i el text de treball es queda apagat a la seva
  bombolla sense repetir-se.
- **TUI: cap línia més ampla que el terminal.** Test que pinta la vista
  sencera a sis amplades i onze estats; va trobar la capçalera (projecte o
  branca llargs) i la barra d'estat desbordant a 60 columnes. La capçalera
  escurça amb «…» i la barra cedeix per graons (context → model → rol)
  abans que l'estat del torn.
- **Windows: el Terminal de l'escriptori no podia engegar cap procés.**
  `/api/exec` feia `sh -c` a pèl i al portable moria amb «"sh": executable
  file not found in %PATH%»; l'eina `bash` de l'agent, en canvi, tirava
  amb `cmd`. Nou `internal/shell`, un sol criteri per a tots dos: `sh` si
  és al PATH, el `sh` del Git for Windows si hi és instal·lat (entén `&&`,
  pipes i `npm run dev`), i `cmd /C` com a últim recurs. `GREGAL_SHELL` ho
  força. Trobat conduint el portable, no els tests: a la màquina de
  desenvolupament `sh` sí que hi era.
- **Mòbil: les pestanyes Agent/Objectius/… no es veien.** Dues causes
  sumades: per sota de 760px van a una fila sota la capçalera, però la
  barra de sessions (G1) va arribar més tard i ocupava el mateix lloc; i
  l'`overflow:hidden` que J4 va posar a `#top` per a l'onada retallava la
  fila, que és filla de la capçalera però viu fora de la seva caixa. Ara
  van a sota de la barra de sessions i `#top` no retalla (l'onada es
  retalla dins de la seva franja). Verificat amb `elementFromPoint` al
  mig de la fila: torna la pestanya, no el fons.
- **Escriptori: selector de models.** El pill «model …» de la capçalera és
  un botó (i `/model` al composer): llista el que cada proveïdor anuncia
  ara mateix (`/api/models`), agrupat, amb filtre, fletxes, l'actual
  marcat i els proveïdors que no responen dits en clar; triar-ne un el
  valida al servidor (`/api/model`) i el fixa per al rol actiu només en
  aquesta sessió, com el `/model` del TUI. I al quadre de configuració
  (⚙ → Providers i rols), «Assigna rol» tria el model d'una llista que
  s'omple amb el que anuncia el provider triat (i canvia amb ell), amb
  «altre…» per escriure'l a mà si cal. Abans a la web s'havia d'escriure
  el nom a mà a tots dos llocs.
- **Escriptori: l'Office deixa de ser un formulari.** Deixa caure un
  .docx/.xlsx/.pptx (o tria'l) i s'obre en una **subfinestra lateral** que
  es queda mentre parles amb l'agent; hi ha peticions ràpides per tipus
  (resumeix, corregeix, tradueix, explica les dades…) i una lliure, que
  s'envien a l'agent amb el document adjunt (`@ruta`) perquè hi treballi
  amb `office_read`/`office_edit`; quan acaba el torn el document es
  rellegeix sol i es pot baixar tal com ha quedat. La pujada torna la ruta
  del fitxer al disc (`path`) per fer-ho possible. En pantalles estretes la
  subfinestra ocupa tot l'ample. Nou `app/office.js`.
- **Web: `@document.docx` adjuntava el zip.** `expandMentions` llegia els
  office com a text pla (bytes del zip al prompt); ara passa per
  `OfficeRead`, com al TUI. I una ruta absoluta de Windows (`C:\…`) es
  tractava com a relativa i s'enganxava darrere del workspace: el `:` i la
  `\` ara són vàlids a la menció i `IsRooted` decideix. Amb test que munta
  un .docx real en memòria.
- **Escriptori: barra superior pròpia.** La finestra ja no porta la barra
  de títol de Windows a sobre, d'un altre color: la capçalera de l'app fa
  de barra (arrossegable pels buits) i els botons minimitza/maximitza/tanca
  els pinta el sistema dins la mateixa franja amb els colors de la marea
  (`titleBarStyle: hidden` + `titleBarOverlay`). Al Mac, lloc per als
  semàfors a l'esquerra. Al navegador no canvia res.

## 1.1.2 — 2026-09-14 — identitat única + dist-win que sí porta backend

- Una sola paleta Gregal (`docs/identitat.md` com a font de veritat):
  la web i l'escriptori deixen el blau genèric i passen als colors
  de la marea; Android com a comandament (pla 1.2).
- `dist-win` produïa un portable sense backend i sense signatura:
  ara l'exe de Windows porta el backend integrat i compila sense
  signar (winCodeSign no s'extreu a Windows).
- `package-lock` de l'escriptori al dia (era de la 0.9.6).

## 1.1.1 — 2026-09-14 — cap torn en blanc

- **Un torn que no responia i no deia per què.** Amb un model de raonament
  i `max_tokens` curt, el pensament es menja tot el pressupost i la
  resposta ja no hi cap: el proveïdor tanca amb `finish_reason: "length"`
  i zero tokens de contingut. `ChatStream` no mirava mai el
  `finish_reason` i tornava `("", nil)`, o sigui que el TUI pintava una
  línia en blanc i no hi havia res a què agafar-se. Ara l'error diu quants
  caràcters ha pensat i que cal pujar `max_tokens` del rol. Si hi ha text,
  encara que sigui tallat, es conserva: val més mitja resposta que un
  error. `Chat` i `ChatWithTools` tenien el mateix punt cec; a
  `ChatWithTools` el contingut buit amb `tool_calls` continua sent vàlid,
  que el torn és la crida. El TUI, com a xarxa de seguretat, ja no pinta
  mai un torn buit.
- **La línia d'onada era una barra massissa.** `waveLine` repetia "≋" a
  amplada completa i era el més pesant de la pantalla. Ara l'onada
  s'esmorteeix cap als extrems: separa igual i sembla volguda.
- **A la benvinguda hi faltava el mode CONSULTA**, que existeix, té
  insígnia i surt al composer. També tenia una pancarta de 32 columnes
  fixes que no seguia l'amplada i línies en blanc descordades. Ara hi són
  els quatre modes, alineats.

## 1.1.0 — 2026-09-14 — l'escriptori deixa de ser un mirall

Fins ara l'app d'escriptori era una finestra Electron sobre la webapp, i
la webapp tenia una sola conversa. Aquesta versió obre la porta a fer-hi
la feina diària sense tornar al TUI.

- **Sessions en paral·lel (G1).** El servidor deixa de tenir una única
  conversa: `X-Gregal-Session` (o `?session=`) tria pestanya, cadascuna
  amb la seva conversa, mode, rol i permisos. Sense id tot va a la sessió
  `default`: Android, VS Code i Telegram no s'han de tocar. S'acaba el
  409 "ja hi ha un agent en marxa" quan el que volies era una altra cosa.
  Nou `POST /api/agent/cancel` per aturar el torn des de qualsevol client.
- **Workspaces (G2).** El directori de treball era `os.Getwd()` a
  l'arrencada; ara és per sessió i es canvia amb `/api/workspaces`, amb
  recents desats a `~/.local/share/gregal/workspaces.json`. Dues
  pestanyes, dos repositoris. Amb més d'una sessió viva, `/rewind` i els
  checkpoints es filtren al workspace propi per no desfer la feina de
  l'altra pestanya.
- **Fitxers i diffs (G3).** `/api/tree`, `/api/file` i `/api/diff`
  (fitxers → hunks → línies, en JSON). Totes rebutgen `..`, rutes
  absolutes i `~`.
- **Revisió de canvis amb accepta/descarta (G4).** Pestanya *Canvis*:
  diff acolorit per hunk i `/api/diff/discard` per desfer-ne un de sol
  (`git apply -R` del pegat), un fitxer sencer, o esborrar un fitxer nou.
  És el flux que abans només existia al terminal.
- **Arbre de fitxers i visor (G5)** amb marca d'estat git, i doble clic
  per posar el fitxer al missatge.
- **Composer de debò (G6).** Ordres `/` amb menú, `@fitxer` amb
  autocompleció real (abans era text del placeholder) i HUD de cost.
- **Electron natiu (G7).** `contextIsolation` + `preload` (abans la
  finestra no ho declarava), notificació quan acaba un torn sense focus,
  *Obre projecte…* amb diàleg natiu, *Sessió nova* (Ctrl+T), *Canvis*
  (Ctrl+D), mida i posició recordades i auto-update opcional amb
  electron-updater.
- **Processos en segon pla (G8/H3).** `/api/exec` + pestanya *Terminal*, i
  les eines `bash_background`, `bash_output` i `bash_kill`: el bash de
  l'agent té 30 s, això no. Es maten per grup (matar `sh` deixava els
  nets vius) i amb la sessió. Mateixa política de permisos que el bash:
  el terminal no és una porta del darrere.
- **UI modular.** `index.html` ja no creix: el que és nou viu a
  `internal/web/app/*.js` com a mòdul ES, servit des de l'`embed.FS`.
- **Contracte i CI (fase F).** `docs/api-contract.md` documenta totes les
  rutes i events; un test (`TestContracteAPIDocumentat`) falla si se
  n'afegeix una sense documentar. La CI feia servir Go 1.24 amb un
  `go.mod` que en demana 1.27 — ara llegeix `go-version-file`. I el test
  `/attach` tenia una ruta absoluta de la màquina de desenvolupament: mai
  hauria passat a la CI.
- **Contenció de rutes a Windows.** `filepath.IsAbs` hi demana lletra
  d'unitat, de manera que `/etc/passwd`, `\Windows\System32\…` o
  `C:fitxer` passaven per relatives i s'unien tan tranquil·lament al
  projecte: `safeJoin` (`/api/tree`, `/api/file`, `/api/diff`) i
  `InsideProject` (el pas automàtic de write/edit dins del projecte) les
  deixaven entrar. Ara ho mira `tools.IsRooted`, que no depèn del GOOS on
  corre. Només afectava el binari de Windows, que és el que distribueix
  `dist-win`.
- **Descartar un hunk ja no reescriu el fitxer sencer.** Amb
  `core.autocrlf=true` (el que ve de sèrie al Git de Windows) el
  `git apply -R` de G4 convertia tot el fitxer a CRLF: desfeies una línia
  i te'n quedaven setze de modificades. Si el fitxer és LF pur, l'apply va
  amb el filtre apagat.
- **Els tests ja no toquen el `$HOME` de debò.** Aïllaven el directori
  d'usuari només amb `HOME`, però `os.UserHomeDir()` a Windows llegeix
  `USERPROFILE`: `go test ./...` llegia —i sobreescrivia—
  `~/.local/share/gregal/history`. També hi havia tests que construïen
  JSON enganxant rutes sense escapar (`\U` de `\Users` no és cap escapada)
  i tests de POSIX (permisos 0600, backends de porta-retalls, servidors
  MCP amb shebang) que a Windows petaven en comptes de saltar-se.
- **Animacions de la marea al TUI.** El buit de la capçalera entre el logo
  i la versió no feia res; mentre hi ha feina hi passa una onada. A
  "TREBALLANT" hi passa una cresta d'escuma per sobre i al costat el temps
  del torn: saber que fa 2m04s que hi és treu l'angoixa de no saber si
  s'ha penjat. El context passa de percentatge a barra amb resolució de
  vuitens, que batega per sobre del 80%. En repòs no s'hi encén res: el
  tick va a mig segon i qualsevol animació hi parpellejaria.

## 0.9.17 — 2026-09-14 — verificació només amb feina real

- L'auto-verificació només corre si el torn ha tocat fitxers
  (journal o git diff): en xat pur ja no surt el bloc VEREDICTE.
- Regla anti-soroll al revisor: sense canvis ni eines → APROVAT
  d'una línia; mai CAL REVISAR dient alhora "res a validar".
- Mode xat: el prompt deia "no inspeccionis" però la política
  permet llegir — ara diu POTS llegir amb read/glob/grep i
  l'assistent ja no s'inventa límits.

## 0.9.16 — 2026-09-14 — avís Model access del Zen

- Quan el Zen torna 500 "Internal server error" (model
  desactivat al workspace — Model access, típic sense zero
  data retention), l'error ja diu on activar-lo en comptes
  de deixar-te endevinant.

## 0.9.15 — 2026-09-14 — permissiu + office a l'escriptori

- Mode permissiu (l'agent tira sense demanar; deny bloqueja):
  TUI `/permissiu [on|off]` (cobreix agent + /write + /edit),
  API `POST /api/permissive`, botó ◇/◆ a la web (només sessió).
- Web/desktop: vista Office (puja docx/xlsx/pptx, llegeix,
  posa valors a cel·les, cerca/substitueix, baixa el fitxer).
- Desktop 0.9.15 amb backend integrat al dia.
- Fix: /api/state es penjava (mutex) en afegir-hi permissive.

## 0.9.14 — 2026-09-14 — errors de provider sencers

- Els errors HTTP del provider (401/500/…) ara mostren el cos
  SENCER (fins a 2000 caràcters, abans 200-300) i amb un sol
  prefix: el motiu real (model inexistent, clau dolenta, …)
  ja es llegeix al TUI sense haver d'endevinar.

## 0.9.13 — 2026-09-14 — gregal config

- NOU `gregal config`: assistent per posar claus api (tria
  provider, enganxa la clau sense eco, la prova contra
  /models i la DESA al disc, permanent). `config list` i
  `config set-key <prov> --stdin [--persist]` per scripts.
- El config ara força 0600 a cada Save (abans només en crear).
- Neteja: el probe GET /models vivia copiat 3 cops (web,
  telegram, tui); ara és `llm.ProbeModels` compartit.

## 0.9.12 — 2026-09-14 — claus al TUI

- En triar un provider sense clau (/model, menú de models), el
  TUI ho avisa i la següent línia es menja com a secret: sense
  eco a la pantalla, sense historial d'entrada, sense conversa.
- /provider key <nom> sense clau també la demana; amb clau
  inline funciona com abans. Esc cancel·la la captura.
- Literal = només memòria (mai al disc, test que ho afirma);
   o buit persisteix al config.

## 0.9.11 — 2026-09-14 — office

- Word/Excel/PowerPoint (.docx/.xlsx/.pptx) amb stdlib, sense
  dependències: `office_read` (text/dades) i `office_edit`
  (set_cell xlsx amb números numèrics, replace docx/pptx fins i
  tot amb text partit en fragments). Tot passa pel journal:
  /rewind ho desfà. Formats antics (.doc/.xls/.ppt) deriven a
  LibreOffice amb la comanda exacta.
- TUI: /read i @adjunts entenen office; /permissions els llista
  (read=allow, edit=demana permís).
- API nova /api/office/{upload,read,edit,download} per a l'app
  (el fitxer viu al mòbil: puja en base64, edita al servidor,
  baixa el resultat). Android 0.6.8 amb diàleg Office complet.

## 0.9.10 — 2026-09-13

- Diff sense "+" fantasma: el salt final no compta com a línia.

## 0.9.9 — 2026-09-13

- Alt+Enter fa salt de línia de debò (el textarea ignora l'enter amb
  modificador; s'insereix a la posició del cursor). També Shift+Enter.

## 0.9.8 — 2026-09-13 — edició TUI

- Identitat "la marea": paleta pròpia (escuma #7FD4C1, arena #F2E7C9,
  blau llacuna), onada ≋ sota la capçalera, pancarta de benvinguda,
  eines amb capçalera ≋ nom i resultat ✓/✗ nom.
- Diffs inline: cada write/edit/patch de l'agent mostra el canvi
  (+ verd / − vermell, retallat a 40 línies). Nova ordre /diff
  (canvis de la sessió + git).
- Entrada multilínia (textarea, alçada 1..6): Enter envia,
  Alt+Enter fa salt de línia. Suggeriments només en 1 línia.
- Esc atura la feina en curs (com Ctrl+C). El raonament intern
  ja no cau en silenci: comptador "ha pensat N caràcters".

## 0.9.7 — 2026-09-13

- Cancel·lació amb Ctrl+C al TUI: la primera pulsació cancel·la la crida
  activa (xat, agent, revisor, comparació o compactació), la segona surt.
- Comparació paral·lela cloud (`/api/parallel`, TUI, web, Android): mateix
  missatge a tots els rols alhora per triar model amb dades.
- Rol inicial coherent amb el mode a tots els clients (code→code…).
- Escriptura del pla completada: E1 tancada aquí (tots els fronts).

- TUI amb popup central `/settings` (mode, rol, model, revisor, permisos,
  providers i sessions), navegació amb Esc i animació de treball.
- Integració read-only de GitHub: `gh_issue` i `gh_pr` per consultar issues,
  PRs, diffs i checks sense exposar una shell arbitrària.
- Menú de providers amb prova de connexió, models vius i assignació al rol
  actiu; claus sempre emmascarades.
- App desktop 0.9.6: backend gestionat, port dinàmic, token local compatible,
  arrencada sense flaix blanc i pantalla d'error útil.

- Bearer-only a `/api/*`, `gregal --doctor`, rewind segur i clients desktop,
  Android i VS Code actualitzats.
- Router per tasca, pressupost, memòria persistent, multimodalitat, MCP,
  revisor estricte i delegació read-only en paral·lel.

[0.9.6]: https://releases.example.org/gregal

### Added
- **The working folder can be chosen from the phone.** Until now the folder was whatever the server had when it started, and the app had no way to change it: to work on another project you had to go to the computer. Preferences now has a *Working folder* card with the folder in use, its branch and a *Change* button that opens a picker with the remembered projects (the server marks the ones that are gone) and a disk browser that flags git repos and starts at the folder you are already using, plus a field for typing a path. All three calls go through the server's own rules: the folder must exist and be inside your roots, and if a job is running the picker says so plainly instead of failing mid-run.

### Added
- **Add a provider from the phone.** The Setup card only ever offered a fixed list plus a key field: a server of your own, another OpenAI-compatible endpoint — anything missing from the catalogue — could not be added at all, so the model list stayed stuck on whatever the server already had. There is now a *Add a new provider* row with name, URL and key, validated locally with the same rules the server enforces (name of letters/digits/dashes/underscores, URL starting with http:// or https://) so a typo fails instantly instead of on a round trip, and the server's answer — saved, or the reason it refused — is shown as it comes.
