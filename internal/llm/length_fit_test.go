package llm

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

const fakeFitErr = "provider http://127.0.0.1:8731/v1: HTTP 400: " +
	`{"error":{"message":"max_tokens 65536 does not fit: prompt is 62192 tokens ` +
	"and the context is 122880, leaving room for 60688.\",\"type\":\"invalid_request_error\"}}"

func TestFitBudgetExtreuLaSala(t *testing.T) {
	err := fmt.Errorf("provider: HTTP 400: %s", "max_tokens 65536 does not fit: prompt is 62192 tokens and the context is 122880, leaving room for 60688.")
	budget, ok := fitBudget(err, 65536)
	if !ok {
		t.Fatalf("hauria de reconèixer l'error de fit")
	}
	// room 60688 - 512 de marge
	if budget != 60176 {
		t.Fatalf("budget esperat 60176, rebut %d", budget)
	}
}

func TestFitBudgetIgnoraAltresErrors(t *testing.T) {
	for _, err := range []error{
		errors.New("connection refused"),
		fmt.Errorf("provider: HTTP 500: internal"),
		nil,
	} {
		if _, ok := fitBudget(err, 65536); ok {
			t.Fatalf("no hauria de tractar %v com a fit", err)
		}
	}
}

func TestFitBudgetMaiMesGranQueElRol(t *testing.T) {
	// Sala reportada més gran que el max_tokens del rol: es manté el del rol.
	err := errors.New("max_tokens 8192 does not fit: prompt is 1000 tokens and the context is 122880, leaving room for 121880.")
	budget, ok := fitBudget(err, 8192)
	if !ok || budget != 8192 {
		t.Fatalf("esperat 8192 (el del rol), rebut %d ok=%v", budget, ok)
	}
}

func TestRetryFitReintentaAmbLaSala(t *testing.T) {
	calls := []int{}
	err := retryFit(context.Background(), 65536, func(budget int) error {
		calls = append(calls, budget)
		if len(calls) == 1 {
			return errors.New(fakeFitErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("no hauria de fallar: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("esperats 2 intents, fets %d", len(calls))
	}
	if calls[1] != 60176 {
		t.Fatalf("segon intent amb %d, esperat 60176", calls[1])
	}
}

func TestRetryFitNoTocaErrorNormal(t *testing.T) {
	calls := 0
	err := retryFit(context.Background(), 65536, func(budget int) error {
		calls++
		return errors.New("connection refused")
	})
	if err == nil || calls != 1 {
		t.Fatalf("un error normal no es reintenta: err=%v calls=%d", err, calls)
	}
}
