package tools

import "context"

// Variables d'entorn extra per a les ordres d'un torn (els secrets que la
// persona ha donat pel formulari segur, winds_secret.go): el valor arriba al
// procés com a variable i mai no passa pel text de l'ordre ni pel model.

type extraEnvKey struct{}

// WithExtraEnv afegeix variables «NOM=valor» a les ordres que corrin amb ctx.
func WithExtraEnv(ctx context.Context, env []string) context.Context {
	if len(env) == 0 {
		return ctx
	}
	prev := ExtraEnv(ctx)
	return context.WithValue(ctx, extraEnvKey{}, append(append([]string(nil), prev...), env...))
}

// ExtraEnv torna les variables extra del context.
func ExtraEnv(ctx context.Context) []string {
	if ctx == nil {
		return nil
	}
	env, _ := ctx.Value(extraEnvKey{}).([]string)
	return env
}
