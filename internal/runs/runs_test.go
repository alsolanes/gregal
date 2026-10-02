package runs

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// waitDone espera la fi d'una execució amb un topall curt: un test que penja
// ha de fallar, no bloquejar la suite.
func waitDone(t *testing.T, q *Queue, id int64) Run {
	t.Helper()
	select {
	case <-q.Done(id):
	case <-time.After(5 * time.Second):
		t.Fatalf("l'execució %d no ha acabat a temps", id)
	}
	run, ok := q.Get(id)
	if !ok {
		t.Fatalf("l'execució %d ha desaparegut", id)
	}
	return run
}

func TestSubmitExecutaICompleta(t *testing.T) {
	q := New()
	fet := false
	run, _, err := q.Submit(func(ctx context.Context, r Run) error {
		if r.State != Running {
			t.Errorf("l'executor s'ha de cridar amb l'estat running, no %s", r.State)
		}
		fet = true
		return nil
	}, Run{Session: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if run.State != Queued {
		t.Fatalf("en encuar l'estat ha de ser queued, no %s", run.State)
	}
	if fin := waitDone(t, q, run.ID); fin.State != Completed {
		t.Fatalf("estat final %s, volia completed", fin.State)
	}
	if !fet {
		t.Fatal("l'executor no s'ha cridat")
	}
	// Ja acabada: Get continua servint-la (contracte per als clients que
	// pregunten tard) i Cancel retorna fals.
	if _, ok := q.Get(run.ID); !ok {
		t.Fatal("una execució acabada s'ha de poder consultar")
	}
	if q.Cancel(run.ID) {
		t.Fatal("cancel·lar una execució acabada ha de donar fals")
	}
}

func TestExecutorQueFallitMarca(t *testing.T) {
	q := New()
	run, _, _ := q.Submit(func(ctx context.Context, r Run) error {
		return errors.New("boom")
	}, Run{Session: "s1"})
	if fin := waitDone(t, q, run.ID); fin.State != Failed || fin.Err == "" {
		t.Fatalf("estat %s err %q, volia failed amb motiu", fin.State, fin.Err)
	}
}

func TestExecutorPanicoNoTombaLaCua(t *testing.T) {
	q := New()
	run, _, _ := q.Submit(func(ctx context.Context, r Run) error {
		panic("eina explotada")
	}, Run{Session: "s1"})
	if fin := waitDone(t, q, run.ID); fin.State != Failed {
		t.Fatalf("un pànic ha de deixar l'execució failed, no %s", fin.State)
	}
	// La cua continua viva per a la mateixa sessió.
	run2, _, err := q.Submit(func(ctx context.Context, r Run) error { return nil }, Run{Session: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if fin := waitDone(t, q, run2.ID); fin.State != Completed {
		t.Fatalf("el torn següent a un pànic ha de completar, no %s", fin.State)
	}
}

func TestIdempotenciaRetornaLexistent(t *testing.T) {
	q := New()
	allibera := make(chan struct{})
	var crides atomic.Int32
	primera, _, err := q.Submit(func(ctx context.Context, r Run) error {
		<-allibera
		crides.Add(1)
		return nil
	}, Run{Session: "s1", Key: "msg-42"})
	if err != nil {
		t.Fatal(err)
	}
	rep, existing, err := q.Submit(func(ctx context.Context, r Run) error {
		crides.Add(1)
		return nil
	}, Run{Session: "s1", Key: "msg-42"})
	if err != nil {
		t.Fatal(err)
	}
	if !existing || rep.ID != primera.ID {
		t.Fatalf("la segona crida havia de retornar l'execució %d, ha donat %d (existing=%v)", primera.ID, rep.ID, existing)
	}
	if _, _, err := q.Submit(func(ctx context.Context, r Run) error { return nil }, Run{Session: "altra", Key: "msg-42"}); err != nil {
		t.Fatal(err)
	}
	close(allibera)
	waitDone(t, q, primera.ID)
	if n := crides.Load(); n != 1 {
		t.Fatalf("l'executor s'ha cridat %d vegades, volia 1", n)
	}
}

func TestUnTornPerSessioIcuaFIFO(t *testing.T) {
	q := New()
	allibera := make(chan struct{})
	var mu sync.Mutex
	ordre := []string{}
	exec := func(nom string) Executor {
		return func(ctx context.Context, r Run) error {
			<-allibera
			mu.Lock()
			ordre = append(ordre, nom)
			mu.Unlock()
			return nil
		}
	}
	primera, _, _ := q.Submit(exec("a"), Run{Session: "s1"})
	b, _, _ := q.Submit(exec("b"), Run{Session: "s1"})
	c, _, _ := q.Submit(exec("c"), Run{Session: "s1"})

	time.Sleep(100 * time.Millisecond) // la primera ha d'estar sola en marxa
	mu.Lock()
	if len(ordre) != 0 {
		mu.Unlock()
		t.Fatal("res no s'ha d'executar abans d'alliberar")
	}
	mu.Unlock()

	// S'encua darrere, però ha de passar abans que b i c per prioritat.
	d, _, _ := q.Submit(exec("d"), Run{Session: "s1", Priority: PriorityBackground})
	prio, _, _ := q.Submit(exec("prio"), Run{Session: "s1", Priority: -1})

	close(allibera)
	waitDone(t, q, primera.ID)
	waitDone(t, q, b.ID)
	waitDone(t, q, c.ID)
	waitDone(t, q, d.ID)
	waitDone(t, q, prio.ID)

	mu.Lock()
	defer mu.Unlock()
	voldria := []string{"a", "prio", "b", "c", "d"}
	for i, nom := range voldria {
		if ordre[i] != nom {
			t.Fatalf("ordre d'execució %v, volia %v", ordre, voldria)
		}
	}
}

func TestWorkspaceCompartitEsSerialitza(t *testing.T) {
	q := New()
	var vius, max atomic.Int32
	var wg sync.WaitGroup
	allibera := make(chan struct{})
	exec := func(ctx context.Context, r Run) error {
		n := vius.Add(1)
		for {
			vell := max.Load()
			if n <= vell || max.CompareAndSwap(vell, n) {
				break
			}
		}
		if n > 1 {
			t.Errorf("dos torns al mateix workspace alhora (%d)", n)
		}
		<-allibera
		vius.Add(-1)
		wg.Done()
		return nil
	}
	wg.Add(2)
	a, _, _ := q.Submit(exec, Run{Session: "s1", Workspace: WorkspaceKey(`C:\proj`)})
	b, _, _ := q.Submit(exec, Run{Session: "s2", Workspace: WorkspaceKey("c:/proj/")})
	close(allibera)
	wg.Wait()
	waitDone(t, q, a.ID)
	waitDone(t, q, b.ID)
}

func TestWorkspacesDistintsCorrenEnParal_lel(t *testing.T) {
	q := New()
	dins := make(chan struct{}, 2)
	allibera := make(chan struct{})
	exec := func(ctx context.Context, r Run) error {
		dins <- struct{}{}
		<-allibera
		return nil
	}
	a, _, _ := q.Submit(exec, Run{Session: "s1", Workspace: "C:\\a"})
	b, _, _ := q.Submit(exec, Run{Session: "s2", Workspace: "C:\\b"})
	// Els dos han d'arribar a executar-se alhora.
	for i := 0; i < 2; i++ {
		select {
		case <-dins:
		case <-time.After(2 * time.Second):
			t.Fatal("els torns de workspaces distints no corren en paral·lel")
		}
	}
	close(allibera)
	waitDone(t, q, a.ID)
	waitDone(t, q, b.ID)
}

func TestCancelEnCuaNoExecutaMai(t *testing.T) {
	q := New()
	allibera := make(chan struct{})
	execCridat := false
	primera, _, _ := q.Submit(func(ctx context.Context, r Run) error {
		<-allibera
		return nil
	}, Run{Session: "s1"})
	enCua, _, _ := q.Submit(func(ctx context.Context, r Run) error {
		execCridat = true
		return nil
	}, Run{Session: "s1"})
	if !q.Cancel(enCua.ID) {
		t.Fatal("cancel·lar un torn en cua havia de donar cert")
	}
	close(allibera)
	waitDone(t, q, primera.ID)
	if fin := waitDone(t, q, enCua.ID); fin.State != Cancelled {
		t.Fatalf("estat del cancel·lat en cua: %s", fin.State)
	}
	time.Sleep(50 * time.Millisecond)
	if execCridat {
		t.Fatal("un torn cancel·lat en cua no s'ha d'executar mai")
	}
	if llista := q.ListSession("s1"); len(llista) != 2 {
		t.Fatalf("la llista d'execucions ha de conservar les dues, té %d", len(llista))
	}
}

func TestCancelEnMarxaTallaElContext(t *testing.T) {
	q := New()
	tallat := make(chan struct{})
	run, _, _ := q.Submit(func(ctx context.Context, r Run) error {
		<-ctx.Done()
		close(tallat)
		return ErrCancelled
	}, Run{Session: "s1"})
	// Espera que estigui en marxa abans de cancel·lar.
	for {
		if r, _ := q.Get(run.ID); r.State == Running {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !q.Cancel(run.ID) {
		t.Fatal("cancel·lar un torn en marxa havia de donar cert")
	}
	select {
	case <-tallat:
	case <-time.After(2 * time.Second):
		t.Fatal("el context de l'executor no s'ha tallat")
	}
	if fin := waitDone(t, q, run.ID); fin.State != Cancelled {
		t.Fatalf("estat final del cancel·lat en marxa: %s", fin.State)
	}
}

func TestCuaPlaPerSessio(t *testing.T) {
	q := New()
	allibera := make(chan struct{})
	defer close(allibera)
	bloc := func(ctx context.Context, r Run) error {
		<-allibera
		return nil
	}
	if _, _, err := q.Submit(bloc, Run{Session: "s1"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxPerSession-1; i++ {
		if _, _, err := q.Submit(bloc, Run{Session: "s1"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := q.Submit(bloc, Run{Session: "s1"}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("volia ErrQueueFull, ha donat %v", err)
	}
	// Una altra sessió no se n'adona.
	if _, _, err := q.Submit(bloc, Run{Session: "s2"}); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceKeyNormalitza(t *testing.T) {
	// Cada plataforma amb el seu cas: la prova original era nomes de Windows
	// (barres invertides i minuscules) i a Linux fallava, perque alla la barra
	// invertida es un caracter mes i les rutes son sensibles a majuscules.
	if runtime.GOOS == "windows" {
		if WorkspaceKey(`C:\Proj\app\`) != WorkspaceKey(`c:/proj/app`) {
			t.Fatal("rutes equivalents del mateix volum no coincideixen")
		}
	} else {
		if WorkspaceKey("/proj/app/") != WorkspaceKey("/proj/app") {
			t.Fatal("la barra final no ha de canviar la clau")
		}
		if WorkspaceKey("  /proj/app  ") != WorkspaceKey("/proj/app") {
			t.Fatal("els espais de sobra no han de canviar la clau")
		}
	}
	if WorkspaceKey("") != "" || WorkspaceKey(".") != "" {
		t.Fatal("buit i directori actual no serialitzen res")
	}
}

func TestSubmitNormalitzaWorkspaceAbansDeSerialitzar(t *testing.T) {
	q := New()
	dir := t.TempDir()
	// Deliberately pass equivalent raw spellings instead of calling
	// WorkspaceKey: every queue caller must get the same invariant.
	primeraPath := filepath.Join(dir, ".")
	segonaPath := dir + string(filepath.Separator)
	if primeraPath == segonaPath {
		// Keep the test meaningful on platforms whose filepath.Join already
		// cleans the trailing separator by using a lexical dot variant.
		primeraPath = filepath.Join(dir, "sub", "..")
	}

	started := make(chan struct{})
	release := make(chan struct{})
	secondStarted := make(chan struct{}, 1)
	first, _, err := q.Submit(func(ctx context.Context, r Run) error {
		close(started)
		<-release
		return nil
	}, Run{Session: "s1", Workspace: primeraPath})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("la primera execució no ha començat a temps")
	}
	second, _, err := q.Submit(func(ctx context.Context, r Run) error {
		secondStarted <- struct{}{}
		return nil
	}, Run{Session: "s2", Workspace: segonaPath})
	if err != nil {
		t.Fatal(err)
	}
	wantWorkspace := WorkspaceKey(dir)
	if first.Workspace != wantWorkspace || second.Workspace != wantWorkspace {
		t.Fatalf("la cua no ha normalitzat els workspaces: primera=%q segona=%q volia=%q", first.Workspace, second.Workspace, wantWorkspace)
	}
	select {
	case <-secondStarted:
		t.Fatal("dos torns del mateix workspace han començat alhora")
	case <-time.After(50 * time.Millisecond):
	}
	close(release)
	waitDone(t, q, first.ID)
	waitDone(t, q, second.ID)
}
