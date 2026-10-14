package llm

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strings"
	"syscall"
)

// Errors de transport que reintentar no arregla.
//
// postRetry reintentava qualsevol error de xarxa sis vegades (1+2+4+8+16 s)
// i el motor hi tornava dues rondes més: amb un base_url equivocat (host
// inexistent) o un servidor aturat (connection refused) el torn trigava
// ~96 s a dir el que ja se sabia al primer intent. Un DNS que no troba el
// host, una connexió refusada o un certificat invàlid són configuració, no
// un tall transitori: es falla de seguida i amb una pista accionable.

// permanentTransportHint torna una pista si l'error és un error de
// transport permanent ("" si s'ha de reintentar com sempre).
func permanentTransportHint(err error) string {
	if err == nil {
		return ""
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
		return "host not found: check the provider base_url"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused: check the provider base_url and that the server is running"
	}
	var unknownAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	var invalid x509.CertificateInvalidError
	var verifyErr *tls.CertificateVerificationError
	var recordErr tls.RecordHeaderError
	if errors.As(err, &unknownAuth) || errors.As(err, &hostErr) || errors.As(err, &invalid) || errors.As(err, &verifyErr) {
		return "TLS certificate error: check the provider base_url and certificates"
	}
	if errors.As(err, &recordErr) {
		return "TLS handshake failed: check the base_url scheme (http vs https)"
	}
	// A Windows, WSAECONNREFUSED (10061) no és syscall.ECONNREFUSED per a
	// errors.Is, i el resolver pot tornar el missatge sense *net.DNSError.
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "actively refused") || strings.Contains(msg, "connection refused"):
		return "connection refused: check the provider base_url and that the server is running"
	case strings.Contains(msg, "no such host"):
		return "host not found: check the provider base_url"
	case strings.Contains(msg, "x509:") || strings.Contains(msg, "certificate signed by unknown authority"):
		return "TLS certificate error: check the provider base_url and certificates"
	}
	return ""
}

// refusedRetries és quants reintents té un error de transport permanent: un
// de sol (curt, RetryBase) per a la connexió refusada, que també és el que
// es veu mentre un servidor local s'engega; cap per a DNS i TLS.
func refusedRetries(err error) int {
	if strings.HasPrefix(permanentTransportHint(err), "connection refused") {
		return 1
	}
	return 0
}

// PermanentTransportError marca un error de transport que postRetry no ha
// reintentat. Continua sent «recuperable» per a IsRetryable: el fallback és
// un altre proveïdor i sí que pot respondre.
type PermanentTransportError struct{ Err error }

func (e *PermanentTransportError) Error() string { return e.Err.Error() }
func (e *PermanentTransportError) Unwrap() error { return e.Err }

// IsPermanentTransport diu si l'error és d'un transport que no arribarà
// mai al proveïdor encara que es reintenti (DNS, connexió refusada, TLS).
func IsPermanentTransport(err error) bool {
	var pe *PermanentTransportError
	if errors.As(err, &pe) {
		return true
	}
	return permanentTransportHint(err) != ""
}

// IsServerError diu si la petició ha arribat al proveïdor i aquest ha
// respost amb un 5xx. És l'únic cas en què té sentit que el motor
// reintenti el pas amb guia (la crida podia ser degenerada): per a errors de
// xarxa el client ja ha esgotat els seus reintents, i un 429 ja s'ha
// esperat el que demanava el servidor.
func IsServerError(err error) bool {
	if err == nil || !IsRetryable(err) {
		return false
	}
	return strings.Contains(err.Error(), "HTTP 5")
}
