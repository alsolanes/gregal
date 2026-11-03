; Dreceres de més: «Gregal Vents» i «Gregal Jocs» obren la mateixa app a
; /vents/ i /jocs/ (main.js, --vents i --jocs).
!macro customInstall
  CreateShortCut "$SMPROGRAMS\Gregal Vents.lnk" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "--vents" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0
  CreateShortCut "$DESKTOP\Gregal Vents.lnk" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "--vents" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0
  CreateShortCut "$SMPROGRAMS\Gregal Jocs.lnk" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "--jocs" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0
!macroend

!macro customUnInstall
  Delete "$SMPROGRAMS\Gregal Vents.lnk"
  Delete "$DESKTOP\Gregal Vents.lnk"
  Delete "$SMPROGRAMS\Gregal Jocs.lnk"
!macroend
