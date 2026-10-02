package inventari

// Inventari guarda les unitats de cada article.
type Inventari struct {
	unitats map[string]int
}

// Nou crea un inventari buit.
func Nou() *Inventari { return &Inventari{unitats: map[string]int{}} }

// Afegeix suma n unitats a l'article.
func (i *Inventari) Afegeix(nom string, n int) {
	if n <= 0 {
		return
	}
	i.unitats[nom] += n
}

// Unitats torna les unitats d'un article (0 si no n'hi ha).
func (i *Inventari) Unitats(nom string) int { return i.unitats[nom] }

// Articles torna quants articles diferents hi ha.
func (i *Inventari) Articles() int { return len(i.unitats) }
