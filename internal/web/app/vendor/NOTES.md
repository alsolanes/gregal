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
| `mp4-muxer.esm.min.js` (Gregal Jocs: `jocs/web/app/vendor/`) | [mp4-muxer](https://github.com/Vanilagy/mp4-muxer/blob/v5.2.2/build/mp4-muxer.min.mjs) | 5.2.2 | MIT (`mp4-muxer.LICENSE`) | Upstream `build/mp4-muxer.min.mjs`, byte for byte, renamed to `.js` so it is served as JavaScript on every platform (Windows has no `.mjs` MIME type). ES module (`Muxer`, `ArrayBufferTarget`), loaded lazily by `arena-export.js` to mux the WebCodecs H.264/AAC video export. The upstream package is deprecated in favour of Mediabunny (MPL-2.0); 5.2.2 is kept on purpose for its MIT licence. |
| `three.module.min.js` (Gregal Jocs: `jocs/web/app/vendor/`) | [three](https://www.npmjs.com/package/three) ([r187](https://github.com/mrdoob/three.js/releases/tag/r187)) | 0.187.0 | MIT (`three.LICENSE`) | ES module (`WebGLRenderer`, `Scene`, …), loaded lazily by `arena-3d.js` for the Arena's first-person 3D view. The npm package no longer ships minified builds, so this file is `build/three.module.js` + `build/three.core.js` from [three-0.187.0.tgz](https://registry.npmjs.org/three/-/three-0.187.0.tgz) (npm integrity `sha512-Qiu7R+Cmh8CstZsY1uJOSWnVlfo7ZtNLV7mF8Jltk3LB3AtDu9cRZlcnRKkAk8Y4z6KKw35z7Y7Urd7jWY8etA==`) bundled into one file and minified, with no source changes: `esbuild@0.28.2 package/build/three.module.js --bundle --format=esm --minify --legal-comments=inline --target=es2020 --outfile=three.module.min.js` (the output is byte-for-byte reproducible; the upstream `@license` header is kept). Named `.js`, not `.mjs`, so it is served as JavaScript on every platform. No addons. |

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
