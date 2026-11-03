; Gregal (Codi i Vents). La drecera «Gregal Vents» obre la mateixa app a
; /vents/ (main.js, --vents). Gregal Jocs té el seu instal·lador
; (installer-jocs.nsh). Aquí es treu la drecera «Gregal Jocs» que posaven les
; versions 1.44-1.46 (Gregal.exe --jocs), però només si l'app Gregal Jocs no hi
; és: llavors la drecera amb aquest nom és seva.
!macro customInstall
  CreateShortCut "$SMPROGRAMS\Gregal Vents.lnk" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "--vents" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0
  CreateShortCut "$DESKTOP\Gregal Vents.lnk" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" "--vents" "$INSTDIR\${APP_EXECUTABLE_FILENAME}" 0
  IfFileExists "$LOCALAPPDATA\Programs\Gregal Jocs\Gregal Jocs.exe" +2 0
    Delete "$SMPROGRAMS\Gregal Jocs.lnk"
!macroend

!macro customUnInstall
  Delete "$SMPROGRAMS\Gregal Vents.lnk"
  Delete "$DESKTOP\Gregal Vents.lnk"
  IfFileExists "$LOCALAPPDATA\Programs\Gregal Jocs\Gregal Jocs.exe" +2 0
    Delete "$SMPROGRAMS\Gregal Jocs.lnk"
!macroend
