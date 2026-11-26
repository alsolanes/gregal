; Gregal (Codi i Vents): una sola drecera, i Codi o Vents es trien al
; desplegable de la finestra. Aquí es treuen les dreceres «Gregal Vents»
; (Gregal.exe --vents) que posaven les versions anteriors. Gregal Jocs té el
; seu instal·lador (installer-jocs.nsh). També es treu la drecera «Gregal Jocs»
; que posaven les versions 1.44-1.46 (Gregal.exe --jocs), però només si l'app
; Gregal Jocs no hi és: llavors la drecera amb aquest nom és seva.
; La desinstal·lació no esborra cap dada: ni el perfil de Gregal
; (~/.config/gregal, ~/.local/share/gregal) ni el de Gregal Jocs
; (~/.config/gregal-jocs, ~/.local/share/gregal-jocs).
!macro customInstall
  Delete "$SMPROGRAMS\Gregal Vents.lnk"
  Delete "$DESKTOP\Gregal Vents.lnk"
  IfFileExists "$LOCALAPPDATA\Programs\Gregal Jocs\Gregal Jocs.exe" +2 0
    Delete "$SMPROGRAMS\Gregal Jocs.lnk"
!macroend

!macro customUnInstall
  IfFileExists "$LOCALAPPDATA\Programs\Gregal Jocs\Gregal Jocs.exe" +2 0
    Delete "$SMPROGRAMS\Gregal Jocs.lnk"
!macroend
