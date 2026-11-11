package ordinador

// Pont del visor (B3). Dos trams, el mateix codi:
//
//	mòbil ──(galeta de sessió del visor)──▶ gateway de l'amfitrió
//	     ──(Bearer de l'ordinador, xarxa privada)──▶ agent /v1/viewer/
//	     ──(loopback)──▶ noVNC 127.0.0.1:6080 dins la VM
//
// Cap tram no passa galetes ni capçaleres d'autorització de l'anterior: el
// testimoni de l'ordinador només el posa l'amfitrió i la galeta del visor
// no surt mai de l'amfitrió. httputil.ReverseProxy passa el WebSocket.

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

func newViewerProxy(target *url.URL, upstreamPrefix, publicPrefix, auth string) http.Handler {
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			rest := strings.TrimPrefix(pr.In.URL.Path, publicPrefix)
			rest = strings.TrimLeft(rest, "/")
			pr.Out.URL.Scheme = target.Scheme
			pr.Out.URL.Host = target.Host
			pr.Out.URL.Path = strings.TrimRight(target.Path, "/") + upstreamPrefix + rest
			pr.Out.URL.RawPath = ""
			pr.Out.Host = target.Host
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Authorization")
			if auth != "" {
				pr.Out.Header.Set("Authorization", auth)
			}
		},
		FlushInterval: 100 * time.Millisecond,
		ModifyResponse: func(r *http.Response) error {
			// Res de la VM pot posar galetes al domini de l'amfitrió.
			r.Header.Del("Set-Cookie")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("the computer's viewer is not reachable"))
		},
	}
	return rp
}

// NewLocalViewerProxy és el pont directe cap a un noVNC de loopback (mode
// local: el visor és a la mateixa màquina, GREGAL_ORDINADOR_VISOR).
func NewLocalViewerProxy(addr, publicPrefix string) (http.Handler, error) {
	if err := CheckPrivateAddr(addr); err != nil {
		return nil, err
	}
	host, _, _ := strings.Cut(addr, ":")
	if host != "127.0.0.1" && host != "localhost" && host != "[::1]" {
		return nil, errViewerLoopback
	}
	return newViewerProxy(&url.URL{Scheme: "http", Host: addr}, "/", publicPrefix, ""), nil
}

var errViewerLoopback = &viewerErr{"the viewer must listen on loopback (127.0.0.1)"}

type viewerErr struct{ s string }

func (e *viewerErr) Error() string { return e.s }
