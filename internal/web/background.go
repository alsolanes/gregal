package web

import "sync"

// bgWork compta els treballs de fons que poden llegir els ganxos
// substituïbles (windLearnLoop, windHeartbeatLoop, petChatter…). Els tests
// canvien i restauren aquests ganxos; si un aprenentatge o un recordatori
// encara corre, escriure-hi alhora és una cursa de dades que el detector
// marca. Amb el comptador, cada neteja espera que el fons s'assequi abans
// de tocar res, i una aturada també pot esperar-los.
var bgWork sync.WaitGroup

// goBackground executa fn en segon pla comptant-la a bgWork.
func goBackground(fn func()) {
	bgWork.Add(1)
	go func() {
		defer bgWork.Done()
		fn()
	}()
}

// waitBackground espera que no quedi cap treball de fons viu.
func waitBackground() { bgWork.Wait() }
