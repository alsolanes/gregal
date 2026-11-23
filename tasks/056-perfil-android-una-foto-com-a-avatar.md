---
title: "Perfil (Android): una foto com a avatar"
status: done
depends_on: ["054", "055"]
---

## Why

Android parity with 054: choose a photo from the phone as the avatar.

## Acceptance criteria

- [x] In Preferences, «Puja una foto» opens the system photo picker and uploads the chosen image to `POST /api/profile/photo`; «Treu la foto» sends `DELETE`. A Robolectric test with a fake server checks the upload and the delete.
- [x] With a photo, the drawer avatar shows it round; without one, or if it fails to load, the initials of 055. A test of the fallback.
- [x] `ANDROID_HOME=... ./gradlew.bat testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview` passes.

## Out of scope

- Taking a photo with the camera; cropping on the phone.

## Notes

- Use the Android photo picker (`ActivityResultContracts.PickVisualMedia`), in `androidx.activity:activity-compose:1.9.0`, already a dependency; no new one.
- Tests in `UserAvatarTest`: the upload posts the raw bytes and the drawer shows the photo; «Treu la foto» sends DELETE; a photo from `/api/me` shows instead of the initials; a photo that does not load leaves the initials; a format the server does not take (stands in for HEIC) becomes JPEG, while JPEG and PNG go as they are.
- Not covered by tests: the picker itself (PickVisualMedia) and reading the content Uri; the tests call `vm.uploadUserPhoto(bytes)`. Not checked on a device, including a real HEIC photo from the gallery.
- `FakeGregal` (WindEsperitTest.kt) now reads requests as bytes, keeps the raw bodies in `bodies`, and can give a binary answer through `bytes`.
- `GregalApi.request` takes the Content-Type from the body, so a raw upload sends its image type; JSON requests still send `application/json`.
