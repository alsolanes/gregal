# Vendor JS (Office preview)

These browser builds are loaded lazily by `office.js` and served from
`/app/vendor/*`. They are checked into the source so the preview also works
offline. Full upstream license texts and notices are in
[`THIRD_PARTY_NOTICES.md`](THIRD_PARTY_NOTICES.md).

| File | Upstream package | Version | License | Runtime notes |
|---|---|---:|---|---|
| `jszip.min.js` | [jszip](https://www.npmjs.com/package/jszip) | 3.10.2 | MIT (chosen from MIT OR GPL-3.0-or-later) | `JSZip`; the browser build includes pako, `lie`/`immediate`, and `setimmediate`. JSZip maps `readable-stream` to its own browser adapter. |
| `docx-preview.min.js` | [docx-preview](https://www.npmjs.com/package/docx-preview) | 0.4.0 | Apache-2.0 | `docx`; consumes the separately vendored JSZip global. |
| `xlsx.mini.min.js` | [SheetJS CE](https://cdn.sheetjs.com/xlsx-0.20.3/package/dist/xlsx.mini.min.js) | 0.20.3 | Apache-2.0 | `XLSX`; read and worksheet utilities used by the preview. |
| `chart.umd.min.js` | [chart.js](https://www.npmjs.com/package/chart.js) | 4.5.1 | MIT | `Chart`; the UMD build includes `@kurkle/color` 0.3.2. |
| `PptxViewJS.min.js` | [pptxviewjs](https://www.npmjs.com/package/pptxviewjs) | 1.1.9 | MIT | `PptxViewJS.PPTXViewer`; its UMD build takes Chart.js and JSZip as external globals. |

The npm package metadata for PptxViewJS is 1.1.9, but its published minified
browser artifact reports runtime version 1.1.8. The hash check records the
published artifact as-is; this mismatch is kept visible rather than silently
rewriting upstream code.

SheetJS 0.18.5 was affected while parsing crafted workbooks by
[CVE-2023-30533](https://cdn.sheetjs.com/advisories/CVE-2023-30533) (fixed in
0.19.3) and [CVE-2024-22363](https://cdn.sheetjs.com/advisories/CVE-2024-22363)
(fixed in 0.20.2). The current official browser build is 0.20.3. The version
and SHA-256 pins, plus an in-memory XLSX round-trip smoke check, are maintained
by [`scripts/verify-vendor.cjs`](../../../../scripts/verify-vendor.cjs).

To update a binary, obtain it from the upstream package/release linked above,
review its license and dependency notices, then update the pin and smoke check.
