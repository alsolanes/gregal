//go:build !windows && !linux && !darwin

package pty

// Start no està disponible en aquesta plataforma.
func Start(spec Spec) (PTY, error) { return nil, ErrUnsupported }
