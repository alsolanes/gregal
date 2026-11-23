---
title: "Docs (web): editor WYSIWYG amb barra d'estils bàsics"
status: done
depends_on: ["078"]
---

## Why

Today a text block is edited as raw markdown in a text area, which looks nothing like Word. The person wants to write on the formatted text, with a toolbar, and move the cursor through the whole document freely, while the agent keeps editing block by block and the conflict rules of 059 still hold.

## Acceptance criteria

- [x] A text document opens in one WYSIWYG editor (built on a vendored editor library) where the person types on formatted text; each top-level node (paragraph, heading, list, quote, code, rule) is one block, read from and written back to the block's markdown (078 syntax), so the store, the agent and the export are unchanged. A node test round-trips a document with every basic style through editor → markdown → editor unchanged.
- [x] A toolbar (and the usual shortcuts Ctrl+B/I/U, Ctrl+Alt+1–3) applies: heading 1–3 / paragraph, bold, italic, underline, strike, bullet and numbered lists, quote, link, inline code, horizontal rule; undo/redo work. A node test per action on the produced markdown.
- [x] Edits are saved per block as the person types (debounced, with `base_version`): a changed node → PUT, a new node → POST after its predecessor, a removed node → DELETE. A 409 keeps the person's text for that block with the notice, as in 059. Agent `block` events update their node in place without moving the person's cursor, except the node being edited. Node tests of each case.
- [x] Every string in Catalan and English; the library's licence is in `internal/web/app/vendor/THIRD_PARTY_NOTICES.md` and the vendored file is checked by `internal/web/vendor_test.go`.

## Out of scope

- Page look (080), tables and images (081), colour/alignment/font/size (082), Android (083).
- Presentations and spreadsheets keep their own views.

## Notes

- Agreed with the user: a proven editor library, vendored (no network at runtime), rather than a hand-made contenteditable. Allowed new dependency: Tiptap (MIT, on ProseMirror) or ProseMirror directly, as one prebuilt ESM/UMD bundle in `internal/web/app/vendor/` like the other vendored libraries, with a short note in `vendor/NOTES.md` on how it was built.
- Replaces the markdown text area of `internal/web/app/doc-view.js` for text documents; keep the reusable view API so the panel and the document window (`doc-page.js`) both get it. Any desktop file change must keep `desktop/package.json` `build.files` complete.
- This is the largest task of the set: if it does not fit under the diff guard (vendored file aside), split the remote-event sync into its own task.
- BLOCKED on the diff guard: the whole task is 440 changed lines of non-test code (limit 400); without the remote-event sync it is still 417, because replacing the textarea editor removes ~160 lines of `doc-view.js`. Proposed split: 079a adds the editor (`doc-view.js` `editor()`/`view()`, markdown mapping, toolbar, shortcuts, per-block saves, 409 notice with «Desa el meu text» / «Fes servir el de l'agent») and the vendored bundle; 079b wires it into the panel, deletes the textarea editor and adds the agent `block` events (`apply`, the waiting queue and the focus rule). The branch has the whole task working and tested, so both can be cut from it.
- Editor library: ProseMirror directly (allowed by the notes), not Tiptap: `prosemirror-markdown` already maps markdown to the basic nodes, so the only glue is the `id` attribute, `++u++`/`~~s~~` marks and `-` bullets. One IIFE bundle `internal/web/app/vendor/prosemirror.min.js` (global `gregalPM`, `module.exports` under Node so the tests load it), pinned in `scripts/verify-vendor.cjs`; how to rebuild is in `vendor/NOTES.md`. Not counted by the diff guard (`vendor/` and `.min.js` are excluded).
- Block ↔ node: each top-level node carries its block id as a node attribute. Consecutive nodes with the same id are one block (an agent block like "Text:
- a" parses to a paragraph and a list). Enter in the middle of a node gives the second half no id (a new block); a plugin gives a node rebuilt by a command (paragraph → heading, wrapped in a list or quote) the id of the block it replaced, so that is a PUT, not DELETE + POST. Pasted nodes lose their ids (ProseMirror does not put attributes in the clipboard HTML), so they are new blocks.
- Saving: 700 ms after the last keystroke, one request at a time in document order (a new node is POSTed after the block before it, under a temporary id until the answer). An untouched block is compared with its own markdown as the editor writes it, so a block the editor would format differently is not saved until the person changes it. After a 409 the block is not saved again until the person picks «Desa el meu text» (PUT against the version shown) or «Fes servir el de l'agent».
- Agent events: they wait while a save is running (so the stream's copy of a POST is not inserted twice) and, for the block under the cursor while the editor has the focus or a block with unsaved changes, until the cursor leaves or the editor loses the focus.
- Page look (080): the editor's page is `.doc-editor` (ProseMirror's element `.ProseMirror` inside it), the toolbar `.doc-toolbar`, the 409 notices `.doc-conflict`, the empty-document placeholder `.doc-empty` with `data-placeholder`.
- Checked in headless Chrome on a sandboxed server (`%TEMP%/g079`, isolated HOME/data, fake provider), doc window `/app/doc/{id}?scope=mine` with a synthetic document with every style: it renders as formatted text, typing and the Bold button sent one `PUT {md: "… **Revisat**", base_version}` and the store had it. Screenshots `output/t079/dark.png`, `light.png` (git-ignored). Not checked in the panel next to a chat or in the packaged desktop app; no desktop file changed.
- Follow-up: markdown the editor does not know yet (tables, images: 081) shows as plain text; a softbreak inside a paragraph becomes a space if the person edits that block.
- Follow-up: if the server splits a POSTed node into several blocks, the editor only keeps the first id.
