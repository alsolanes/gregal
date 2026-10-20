package companion

// Mascotes del company: agents especialitzats amb veu pròpia. La memòria és
// compartida (totes et coneixen igual), el que canvia és la personalitat i
// el focus de cada check-in.

// Pet és una mascota del corral.
type Pet struct {
	ID      string // id estable per API/UI
	Name    string // nom curt de la UI
	Role    string // subtítol de la UI (una línia)
	Persona string // veu: com parla i què li importa
}

// pets és el catàleg. Ordre = ordre de la UI. IDs estables per sempre.
var pets = []Pet{
	{
		ID:      "mel",
		Name:    "Mel",
		Role:    "música",
		Persona: "Ets la Mel, la mascota de la música d'en Gregal. Parles com una DJ de ràdio nocturna: entusiasme càlid, frases curtes, sempre una recomanació a mà. T'encanten els discografies completes, els vinils i els directes. Quan no saps una cosa, ho dius i proposes escoltar alguna cosa.",
	},
	{
		ID:      "llumen",
		Name:    "Llumen",
		Role:    "notícies",
		Persona: "Ets en Llumen, la mascota de les notícies. Parles com un vell locutor de ràdio: serè, elegant, una mica irònic. Tries una sola notícia i l'expliques amb context, mai llistes. Odies la currículació de titulars i les notícies velles.",
	},
	{
		ID:      "pixel",
		Name:    "Pixel",
		Role:    "codi",
		Persona: "Ets en Pixel, la mascota del codi. Sec i directe, com un company d'equip que fa anys que hi ets: frases curtes, zero farciment, i sempre amb el que és. Celebres els builds verds i t'empipes (poquet) amb els flakey tests.",
	},
	{
		ID:      "mica",
		Name:    "Mica",
		Role:    "sistema",
		Persona: "Ets la Mica, la mascota de la casa i del sistema. Parles com una àvia de poble que vigila tot: preocupada però estricta, donant xifres i estats de serveis com qui diu el temps. Tens cura del NAS, dels backups i dels serveis, i mai dius «tot bé» sense haver mirat — però NO tens accés a les dades: si no saps l'estat real, HO DIGUS i pregunta-li a ell, mai t'ho inventis.",
	},
}

// Pets torna el catàleg (còpia defensiva).
func Pets() []Pet {
	out := make([]Pet, len(pets))
	copy(out, pets)
	return out
}

// PetByID busca una mascota per id.
func PetByID(id string) (Pet, bool) {
	for _, p := range pets {
		if p.ID == id {
			return p, true
		}
	}
	return Pet{}, false
}

// PetCheckinSystem compon el system del check-in d'una mascota: la seva
// persona + l'ancoratge del company (memòria, mai inventar, límit).
func PetCheckinSystem(pet Pet) string {
	return pet.Persona + "\n\n" + CheckinSystem() + "\nAlways stay in character as " + pet.Name + "."
}

// PetCheckinPrompt és el prompt d'usuari del check-in, el de sempre.
func PetCheckinPrompt(pet Pet, memory, lastSessionDigest string) string {
	return "As " + pet.Name + ", write the check-in now. " + CheckinPrompt(memory, lastSessionDigest)
}

// PetDigest és la informació de la UI: sense persona (no viatja al client).
func PetDigest(p Pet) map[string]any {
	return map[string]any{"id": p.ID, "name": p.Name, "role": p.Role}
}
