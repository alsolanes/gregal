# PDF

Gregal treats PDFs as first-class files: the agent and the winds can read
them (including scanned pages), edit them, and you can upload them from the
web and desktop chat, the Android app and the Chrome extension.

Everything runs in the Go backend, so the desktop app, `gregal --serve`, the
phone and the extension all get the same behaviour.

## Tools

### `pdf_read`

| `detail` | What comes back |
|---|---|
| `text` (default) | Text page by page (`— Pàgina N —`). Pages without a text layer are listed and, when a renderer is available, attached as images so the model reads them itself (up to 4 at once). |
| `info` | Page count and sizes in points, title/author/subject, encryption and signature status, form fields with their type, value and options, and existing annotations. |
| `images` | The pages as images: up to 4 at full size, more as a numbered grid (`pages="1-3"`). |

`pages="1,3-5"` limits text and images. `office_read` on a `.pdf` returns the
same text (models often reach for it), and `@file.pdf` in a chat message
attaches the text, not the bytes.

### `pdf_edit`

Edits are journaled (`/rewind` undoes them) and the result is validated before
it replaces anything: a failed edit never touches the original. By default the
file at `path` is changed in place; `out` writes elsewhere, and an existing
different file needs `overwrite=true`.

| `op` | Arguments |
|---|---|
| `merge` | `paths` (in order), `out` (required) |
| `split` | `at="3,7"` (cut before those pages) or `span=N`; `out` is a folder (default: next to the file) |
| `extract` | `pages`, `out` (default `<name>_p<pages>.pdf`) |
| `reorder` | `pages` = the new order, e.g. `"3,1,2"` (pages can repeat) |
| `delete_pages` | `pages` |
| `rotate` | `degrees` (90/180/270), `pages` (default all) |
| `add_text` | `text`, `pages`, `position` (`tl tc tr l c r bl bc br`), `x`/`y` offset in points, `size`, `color`, `opacity`, `rotation` |
| `stamp` | Same as `add_text`, with big red semi-transparent defaults |
| `watermark` | Same, behind the content, grey and diagonal |
| `fill_form` | `fields` `{name: value}`; checkboxes take `true/false`; radio and combo values must be one of the options |
| `annotate` | `kind` (`note`, `highlight`, `box`, `link`), `pages`, `x`, `y`, `w`, `h` in points from the bottom-left, `text`, `url`, `color`, `author` |
| `set_info` | `fields` `{title, author, subject, keywords, creator}` |

Page selections accept `"1-3,5"`, `"2-last"`, `"odd"`, `"even"`.

PDFs protected only with an owner password ("no copying/printing") are read
and edited; the result is saved unencrypted. PDFs that need a password to open
are refused with a clear message. Editing a signed PDF invalidates the
signature (`detail=info` says when a file is signed).

### Permissions

`pdf_read` is allowed like `read` (and runs in parallel with other reads).
`pdf_edit` is treated like `write`: inside the project it runs without asking
in code and autonomous modes, outside it asks, and read-only modes (chat,
goal, inspect) deny it. `permissions.tools.pdf_read` / `pdf_edit` override the
defaults as for any other tool.

## Uploading a PDF

| Where | How |
|---|---|
| Web and desktop chat | Paperclip → **Document (PDF, Word, Excel…)**. The file is uploaded and `@path` is added to the message. |
| Office page (web/desktop) | Drop or pick a `.pdf`: a side pane shows the pages (text when no renderer is available) with quick actions (summary, key data, fill the form, stamp). |
| Winds desk | **📎 Attach a document** next to *Let's go*: the path goes into the task and the wind reads it with `pdf_read`. |
| Android | Chat **+** → **Attach a document**, or the Office screen. The phone draws the pages itself with Android's `PdfRenderer`. |
| Chrome extension | Open a PDF in Chrome and open the side panel: the PDF is uploaded to Gregal and its text becomes the context, with the path so the wind can read the whole file. |

Uploads go through `/api/office/upload` (base64, 25 MB) to the server's temp
folder, expire after two hours and belong to the user who uploaded them.
`/api/office/read`, `/api/office/render` and `/api/office/download` work for
PDFs as for Office files; `/api/office/edit` accepts the `pdf_edit` operations
that change the same file (`rotate`, `delete_pages`, `reorder`, `add_text`,
`stamp`, `watermark`, `fill_form`, `annotate`, `set_info`).

## Libraries and the optional renderer

- Text: Gregal's own dependency-free extractor (`internal/pdfdoc/text.go`, also
  used by knowledge indexing).
- Editing: [pdfcpu](https://github.com/pdfcpu/pdfcpu) (Apache-2.0, pure Go).
  Gregal always runs it with a stateless configuration, so it never writes a
  `pdfcpu` folder in the user's config directory.
- Page images: there is no pure-Go PDF rasterizer with a compatible licence,
  so this part is optional. Gregal uses `pdftoppm` (Poppler) when it is on the
  `PATH`, or the `Windows.Data.Pdf` API built into Windows 10/11 (nothing to
  install). Without either, `pdf_read` returns text and says which pages have
  none; Android draws pages on the phone either way.

## Tests

`go test ./internal/pdfdoc ./internal/tools ./internal/web ./internal/agent`
cover every operation with the small fixtures in
`internal/pdfdoc/testdata` (three pages of text, a form, a scanned page). They
are regenerated with
`GREGAL_PDF_FIXTURES=1 go test ./internal/pdfdoc -run TestMakeFixtures`.
