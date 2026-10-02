package agent

import "fmt"

// Textos per defecte dels events amb clau (Clau + Args). El TUI els tradueix
// amb el seu diccionari; els clients sense diccionari (el bot de Telegram)
// fan servir aquests. Si una clau nova del motor no és aquí, el test
// TestTotsElsEventsTenenText ho diu.
var textosMotor = map[string]string{
	"app.ampliaSegueix":      "continuo amb %d passos més (ara %d)",
	"app.ampliaPerChecklist": "no he entès si cal continuar (%s), però la checklist és a %d/%d: continuo amb %d passos més (ara %d)",
	"app.checkpoint":         "autònom · checkpoint %d",
	"app.passosEsgotats":     "s'han acabat els %d passos disponibles: el que ve és un resum, no la feina acabada",
	"app.senseAccionsNoves":  "el model no proposa accions noves (eines repetides): tanco la tasca…",
	"app.ratxaVermella":      "%d comprovacions vermelles seguides: demano al model que llegeixi l'error sencer i miri si és preexistent",
	"app.serverReintentCurt": "error del servidor, reintentant el pas…",
	"app.todosPendents":      "checklist %d/%d pendent: continuo sol…",
	"app.tornBuit":           "el model ha acabat el torn sense resposta (pas %d/%d): torna-ho a demanar o puja max_tokens",
	"app.verifReintent":      "l'última comprovació ha fallat: continuo…",
	"compact.fallida":        "compactació fallida (ho torno a provar al proper torn): ",
	"compact.fet":            "conversa compactada: %d → %d missatges + resum (%d caràcters)",
	"est.agentCancel":        "agent cancel·lat",
	"est.cancellat":          "cancel·lat",
}

// Missatge és el text de l'event en català, resolent la clau si n'hi ha.
func (e Event) Missatge() string {
	if e.Clau == "" {
		return e.Text
	}
	f, ok := textosMotor[e.Clau]
	if !ok {
		return e.Clau
	}
	return fmt.Sprintf(f, e.Args...)
}
