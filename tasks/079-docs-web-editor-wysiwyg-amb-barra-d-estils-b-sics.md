---
title: "Docs (web): editor WYSIWYG amb barra d'estils bàsics"
status: todo
depends_on: ["078"]
---

## Why

Today a text block is edited as raw markdown in a text area, which looks nothing like Word. The person wants to write on the formatted text, with a toolbar, and move the cursor through the whole document freely, while the agent keeps editing block by block and the conflict rules of 059 still hold.

## Acceptance criteria

- [ ] A text document opens in one WYSIWYG editor (built on a vendored editor library) where the person types on formatted text; each top-level node (paragraph, heading, list, quote, code, rule) is one block, read from and written back to the block's markdown (078 syntax), so the store, the agent and the export are unchanged. A node test round-trips a document with every basic style through editor → markdown → editor unchanged.
- [ ] A toolbar (and the usual shortcuts Ctrl+B/I/U, Ctrl+Alt+1–3) applies: heading 1–3 / paragraph, bold, italic, underline, strike, bullet and numbered lists, quote, link, inline code, horizontal rule; undo/redo work. A node test per action on the produced markdown.
- [ ] Edits are saved per block as the person types (debounced, with `base_version`): a changed node → PUT, a new node → POST after its predecessor, a removed node → DELETE. A 409 keeps the person's text for that block with the notice, as in 059. Agent `block` events update their node in place without moving the person's cursor, except the node being edited. Node tests of each case.
- [ ] Every string in Catalan and English; the library's licence is in `internal/web/app/vendor/THIRD_PARTY_NOTICES.md` and the vendored file is checked by `internal/web/vendor_test.go`.

## Out of scope

- Page look (080), tables and images (081), colour/alignment/font/size (082), Android (083).
- Presentations and spreadsheets keep their own views.

## Notes

- Agreed with the user: a proven editor library, vendored (no network at runtime), rather than a hand-made contenteditable. Allowed new dependency: Tiptap (MIT, on ProseMirror) or ProseMirror directly, as one prebuilt ESM/UMD bundle in `internal/web/app/vendor/` like the other vendored libraries, with a short note in `vendor/NOTES.md` on how it was built.
- Replaces the markdown text area of `internal/web/app/doc-view.js` for text documents; keep the reusable view API so the panel and the document window (`doc-page.js`) both get it. Any desktop file change must keep `desktop/package.json` `build.files` complete.
- This is the largest task of the set: if it does not fit under the diff guard (vendored file aside), split the remote-event sync into its own task.
