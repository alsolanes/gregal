---
title: "Perfil (Android): una foto com a avatar"
status: todo
depends_on: ["054", "055"]
---

## Why

Android parity with 054: choose a photo from the phone as the avatar.

## Acceptance criteria

- [ ] In Preferences, «Puja una foto» opens the system photo picker and uploads the chosen image to `POST /api/profile/photo`; «Treu la foto» sends `DELETE`. A Robolectric test with a fake server checks the upload and the delete.
- [ ] With a photo, the drawer avatar shows it round; without one, or if it fails to load, the initials of 055. A test of the fallback.
- [ ] `ANDROID_HOME=... ./gradlew.bat testDebugUnitTest -Pgregal.skipBackend -Pgregal.plotsPreview` passes.

## Out of scope

- Taking a photo with the camera; cropping on the phone.

## Notes

- Use the Android photo picker (`ActivityResultContracts.PickVisualMedia`), in `androidx.activity:activity-compose:1.9.0`, already a dependency; no new one.
