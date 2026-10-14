package web

import (
	"context"
	"errors"
	"sync"
	"time"
)

// retroStandupBudget és el temps màxim per al resum d'obertura del #retro;
// passat aquest temps queda el resum automàtic a partir de l'evidència.
var retroStandupBudget = 75 * time.Second

// retroReplyBudget és el temps màxim per a les respostes a un missatge.
var retroReplyBudget = 60 * time.Second

// errRetroQuick: l'usuari ha demanat el resum ràpid (sense model).
var errRetroQuick = errors.New("quick summary requested")

// Generacions en curs per canal: un segon clic espera la primera en comptes
// de tornar a cridar el model.
type retroJob struct {
	done   chan struct{}
	cancel context.CancelFunc
}

var (
	retroJobsMu sync.Mutex
	retroJobs   = map[string]*retroJob{}
)

func retroJobFor(key string) *retroJob {
	retroJobsMu.Lock()
	defer retroJobsMu.Unlock()
	return retroJobs[key]
}

func retroBeginJob(key string, cancel context.CancelFunc) *retroJob {
	j := &retroJob{done: make(chan struct{}), cancel: cancel}
	retroJobsMu.Lock()
	retroJobs[key] = j
	retroJobsMu.Unlock()
	return j
}

func retroEndJob(key string, j *retroJob) {
	retroJobsMu.Lock()
	if retroJobs[key] == j {
		delete(retroJobs, key)
	}
	retroJobsMu.Unlock()
	close(j.done)
}
