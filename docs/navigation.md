# Desktop navigation

The permanent rail contains Chat, Winds, Scheduled work and More. Help and
preferences remain available at its bottom even when the contextual sidebar
is collapsed. Secondary tools and library views live in More.

The contextual sidebar contains the existing project-grouped conversation
history, search, and project actions. On the Winds page it contains the wind
and group conversation lists instead. Their DOM is moved rather than copied,
so histories, drafts, event handlers and profile controls keep their identities.

Files, changes, terminal, browser and knowledge are mounted into a right-hand
tool dock. Opening one keeps the conversation and composer visible. On narrow
screens the dock overlays the workspace and has an explicit close button.
Closing restores focus to its opener. Preferences for collapsing the context
remain local to the browser; the permanent rail stays available.

The model picker is inside the composer and retains the existing model and
role selection API. Its popup opens above or below its anchor according to
the available viewport space, and is repositioned on resize.

The native Android screen is unchanged.

## Validation

Run the frontend Node suite using the command in `.github/workflows/quality.yml`.
For browser regression checks, serve `internal/web` on port 8766, open that URL
with Playwright CLI, and run:

```
playwright-cli run-code --filename scripts/check-shell-ui.js
```

The browser check intercepts API calls. It covers tools beside the chat, model
picker placement, collapsed context, always-visible utilities, Winds context,
and mobile panel bounds. It makes no live model requests.


## Project folders and compact composer

Projects and general conversations occupy separate sidebar sections. The Projects + control opens a folder in a new session without moving an existing session. Registered folders remain listed before any conversation is saved. Each project offers + for another conversation using that folder as its working directory; saved project conversations appear underneath it. General conversations remain in date groups below.

The composer places the model, advanced options, permissions lock, attachments and send action on one row. Work modes and planning remain accessible in advanced options; the lock retains its permission menu and confirmation behavior. The model name truncates at narrow widths instead of adding a separate row.


The project and conversation list share a single scroll area. The OS-drawn scrollbar is hidden to keep it visually consistent with the sidebar; wheel, trackpad, and keyboard scrolling continue to work.
