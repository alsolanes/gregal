package harness

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gregal/internal/web"
)

func TestAuthenticationAndAPISurface(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	path, err := filepath.Abs("../examples/company-chat/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(context.Background(), Options{ConfigPath: path}); err == nil {
		t.Fatal("open access must require explicit opt-in")
	}
	runtime, err := New(context.Background(), Options{ConfigPath: path, Authenticate: func(r *http.Request) (Identity, error) {
		if r.Header.Get("Authorization") != "Bearer verified" {
			return Identity{}, errors.New("denied")
		}
		return Identity{AccountID: AccountID("employee-subject")}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	for _, test := range []struct {
		route, token string
		status       int
	}{
		{"/api/health", "", 401}, {"/api/health", "Bearer verified", 200}, {"/", "Bearer verified", 404},
	} {
		req := httptest.NewRequest(http.MethodGet, test.route, nil)
		req.Header.Set("Authorization", test.token)
		res := httptest.NewRecorder()
		runtime.Handler().ServeHTTP(res, req)
		if res.Code != test.status {
			t.Fatalf("%s: got %d: %s", test.route, res.Code, res.Body.String())
		}
	}
	runtime.Close() // Closing twice is safe.
	closed := httptest.NewRecorder()
	runtime.Handler().ServeHTTP(closed, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if closed.Code != http.StatusServiceUnavailable {
		t.Fatalf("closed runtime accepted a request: got %d", closed.Code)
	}
}

func TestAccountID(t *testing.T) {
	if AccountID("one") != AccountID("one") || AccountID("one") == AccountID("two") || !accountIDPattern.MatchString(AccountID("one")) {
		t.Fatal("account IDs must be stable, separate and safe")
	}
	if AccountID("") != "" || AccountID(" \t") != "" {
		t.Fatal("empty verified subjects must not share an account namespace")
	}
}

func TestIdentityPolicyIsPinnedForRuntime(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	path, err := filepath.Abs("../examples/company-chat/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	rootA, rootB := t.TempDir(), t.TempDir()
	currentRoot := rootA
	runtime, err := New(context.Background(), Options{ConfigPath: path, Authenticate: func(*http.Request) (Identity, error) {
		return Identity{AccountID: AccountID("employee-subject"), Roots: []string{currentRoot}}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	request := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
		req.Header.Set("Authorization", "Bearer verified")
		res := httptest.NewRecorder()
		runtime.Handler().ServeHTTP(res, req)
		return res.Code
	}
	if got := request(); got != http.StatusOK {
		t.Fatalf("initial identity rejected: %d", got)
	}
	currentRoot = rootB
	if got := request(); got != http.StatusUnauthorized {
		t.Fatalf("changed authorization policy reused a cached session: %d", got)
	}
}

func TestIdentityHomeMustBeWithinRootsAndEmptyRootsDenyWorkspace(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	path, err := filepath.Abs("../examples/company-chat/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root, outside := t.TempDir(), t.TempDir()
	runtime, err := New(context.Background(), Options{ConfigPath: path, Authenticate: func(*http.Request) (Identity, error) {
		return Identity{AccountID: AccountID("employee-subject"), Roots: []string{root}, Home: outside}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	unauthorized := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	req.Header.Set("Authorization", "Bearer verified")
	runtime.Handler().ServeHTTP(unauthorized, req)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("home outside allowed roots accepted: %d", unauthorized.Code)
	}
	runtime.Close()

	noRoots, err := New(context.Background(), Options{ConfigPath: path, Authenticate: func(*http.Request) (Identity, error) {
		return Identity{AccountID: AccountID("employee-without-roots")}, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer noRoots.Close()
	body := strings.NewReader(`{"id":"safe","cwd":` + jsonString(root) + `}`)
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/open", body)
	req.Header.Set("Authorization", "Bearer verified")
	denied := httptest.NewRecorder()
	noRoots.Handler().ServeHTTP(denied, req)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("identity with no roots entered a workspace: %d: %s", denied.Code, denied.Body.String())
	}
}

func TestSessionIDsAreBoundedAndNamespacedByIdentity(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	path, err := filepath.Abs("../examples/company-chat/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	accountA, accountB := AccountID("employee-a"), AccountID("employee-b")
	runtime, err := New(context.Background(), Options{ConfigPath: path, Authenticate: func(r *http.Request) (Identity, error) {
		switch r.Header.Get("Authorization") {
		case "Bearer a":
			return Identity{AccountID: accountA, Roots: []string{root}}, nil
		case "Bearer b":
			return Identity{AccountID: accountB, Roots: []string{root}}, nil
		default:
			return Identity{}, errors.New("denied")
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	open := func(token, id string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/sessions/open", strings.NewReader(`{"id":`+jsonString(id)+`}`))
		req.Header.Set("Authorization", token)
		res := httptest.NewRecorder()
		runtime.Handler().ServeHTTP(res, req)
		return res
	}
	openedIDs := map[string]string{}
	for _, token := range []string{"Bearer a", "Bearer b"} {
		res := open(token, "shared")
		if res.Code != http.StatusOK {
			t.Fatalf("open shared session for %s: %d: %s", token, res.Code, res.Body.String())
		}
		var opened struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &opened); err != nil {
			t.Fatalf("decode opened session: %v", err)
		}
		openedIDs[token] = opened.ID
	}
	if openedIDs["Bearer a"] != accountA+"-shared" || openedIDs["Bearer b"] != accountB+"-shared" {
		t.Fatalf("opened IDs were not account scoped: %#v", openedIDs)
	}
	hub := runtime.server.Hub()
	userA := &web.User{Name: accountA, Roots: []string{root}}
	userB := &web.User{Name: accountB, Roots: []string{root}}
	sessionA := hub.SessionFor(userA, "shared")
	sessionB := hub.SessionFor(userB, "shared")
	if sessionA == sessionB || openedIDs["Bearer a"] == openedIDs["Bearer b"] {
		t.Fatal("same client session ID crossed authenticated account namespaces")
	}
	if hub.SessionFor(userA, "shared") != sessionA {
		t.Fatal("same account and session ID did not resolve consistently")
	}

	maxLocal := strings.Repeat("x", 64-len(accountA)-1)
	for _, id := range []string{maxLocal, accountA + "-" + maxLocal} {
		res := open("Bearer a", id)
		if res.Code != http.StatusOK {
			t.Errorf("valid maximum-length session ID %q rejected: %d", id, res.Code)
		}
	}
	for _, test := range []struct {
		name string
		req  *http.Request
	}{
		{name: "header characters", req: httptest.NewRequest(http.MethodGet, "/api/state", nil)},
		{name: "query characters", req: httptest.NewRequest(http.MethodGet, "/api/state?session=../outside", nil)},
		{name: "conflicting invalid query", req: httptest.NewRequest(http.MethodGet, "/api/state?session=../outside", nil)},
	} {
		test.req.Header.Set("Authorization", "Bearer a")
		if test.name == "header characters" {
			test.req.Header.Set("X-Gregal-Session", "../outside")
		}
		if test.name == "conflicting invalid query" {
			test.req.Header.Set("X-Gregal-Session", "safe")
		}
		res := httptest.NewRecorder()
		runtime.Handler().ServeHTTP(res, test.req)
		if res.Code != http.StatusUnauthorized {
			t.Errorf("%s accepted: %d", test.name, res.Code)
		}
	}
	for _, id := range []string{strings.Repeat("x", 64-len(accountA)), accountA + "-" + strings.Repeat("x", 64-len(accountA))} {
		res := open("Bearer a", id)
		if res.Code != http.StatusUnauthorized {
			t.Errorf("overlong session ID accepted: %d (%q)", res.Code, id)
		}
	}
}

func TestCloseContextWaitsForBackgroundScheduler(t *testing.T) {
	t.Setenv("GREGAL_DATA_DIR", t.TempDir())
	path, err := filepath.Abs("../examples/company-chat/config.example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := New(context.Background(), Options{ConfigPath: path, AllowUnauthenticated: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runtime.CloseContext(ctx); err != nil {
		t.Fatalf("scheduler did not stop within its context: %v", err)
	}
	select {
	case <-runtime.backgroundDone:
	default:
		t.Fatal("CloseContext returned before the scheduler stopped")
	}
}

func jsonString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
