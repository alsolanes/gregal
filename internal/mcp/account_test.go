package mcp

import (
	"path/filepath"
	"testing"
)

// Auditoria B5: les eines d'un servidor remot autoritzat amb el token de
// l'administrador no són de tothom; les d'un remot obert, sí.
func TestToolOwnerSharedOnlyWithoutAdminCredentials(t *testing.T) {
	authed := newFakeRemote(t)
	authed.auth = true
	open := newFakeRemote(t)
	store := NewTokenStore(filepath.Join(t.TempDir(), "mcp-oauth.json"))
	store.put(tokenKey("crm", Server{URL: authed.url()}), tokenRecord{AccessToken: "tok-1"})
	m := SetupWith(map[string]Server{"crm": {URL: authed.url()}, "obert": {URL: open.url()}}, Options{Store: store, NoRegister: true})
	defer m.Close()
	if st := m.ErrorsSnapshot(); len(st) != 0 {
		t.Fatalf("errors: %v", st)
	}
	if m.ToolOwnerShared(ToolName("crm", "eco")) {
		t.Fatal("una eina amb el token de l'administrador no es comparteix")
	}
	if !m.ToolOwnerShared(ToolName("obert", "eco")) {
		t.Fatal("un remot sense credencials es pot compartir")
	}
	if m.ToolOwnerShared("mcp_res_x") {
		t.Fatal("una eina desconeguda no passa")
	}
}
