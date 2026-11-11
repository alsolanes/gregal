package web

// Contracte compartit de l'assistent (pla v2, §9; docs/assistant-contract.ca.md).
//
// Aquí només hi ha els vocabularis d'estat que els paquets B2–B7 han
// d'implementar igual al servidor i a Android. Els models complets els
// afegeix cada paquet quan els fa servir; les fixtures de
// docs/assistant-contract/fixtures els exerciten i un test les valida.

// Estats d'un encàrrec. waiting_* alliberen recursos: no hi ha cap goroutine
// esperant; la represa la fa el runner quan arriba la resposta o la connexió.
var encarrecStates = []string{"queued", "running", "waiting_user", "waiting_connection", "completed", "failed", "cancelled"}

// Estats d'una decisió pendent. L'aprovació és d'una revisió concreta de
// l'efecte proposat: si l'efecte canvia, es demana de nou.
var decisioStates = []string{"pending", "approved", "rejected", "expired", "superseded"}

// Estats d'una connexió. needs_session = el compte hi és però cal tornar a
// iniciar sessió (caducada); mai es mostra «connectat» sense una comprovació.
var connexioStates = []string{"available", "connecting", "connected", "needs_session", "error", "disconnected"}

// Salut de l'ordinador personal i qui en té el control interactiu.
var ordinadorHealth = []string{"starting", "running", "stopped", "unreachable"}
var ordinadorControl = []string{"agent", "user", "none"}
