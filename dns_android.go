//go:build android && !cgo

package main

import (
	"context"
	"net"
	"os"
	"strings"
	"time"
)

// A Android sense cgo, el resolutor de Go no té /etc/resolv.conf i pregunta
// a 127.0.0.1:53, on no hi ha ningú: cap proveïdor no resolia. La build
// normal de l'app usa cgo (getaddrinfo del sistema, amb DNS privat i VPN);
// aquesta és la xarxa de seguretat si algú compila sense l'NDK. L'app passa
// els DNS de la xarxa activa a GREGAL_DNS ("1.2.3.4,[2001:db8::1]").
func init() {
	servers := dnsServers(os.Getenv("GREGAL_DNS"))
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			var last error
			for _, s := range servers {
				c, err := d.DialContext(ctx, network, s)
				if err == nil {
					return c, nil
				}
				last = err
			}
			return nil, last
		},
	}
}

func dnsServers(env string) []string {
	var out []string
	for _, f := range strings.Split(env, ",") {
		f = strings.Trim(strings.TrimSpace(f), "[]")
		if net.ParseIP(f) != nil {
			out = append(out, net.JoinHostPort(f, "53"))
		}
	}
	return append(out, "1.1.1.1:53", "8.8.8.8:53")
}
