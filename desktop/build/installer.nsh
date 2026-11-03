; Segona drecera: «Gregal Vents» obre la mateixa app a /vents/ (main.js, --vents).
!macro customInstall
  CreateShortCut "$SMPROGRAMS\Gregal Vents.lnk" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "--vents" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0
  CreateShortCut "$DESKTOP\Gregal Vents.lnk" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "--vents" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0
!macroend

!macro customUnInstall
  Delete "$SMPROGRAMS\Gregal Vents.lnk"
  Delete "$DESKTOP\Gregal Vents.lnk"
!macroend
