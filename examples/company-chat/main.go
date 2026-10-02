// Run from an isolated workspace with a private configuration and data profile.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"gregal/harness"
)

func main() {
	configPath := flag.String("config", "", "required deployment configuration")
	addr := flag.String("addr", "127.0.0.1:8097", "listen address")
	ui := flag.Bool("ui", true, "serve the existing chat interface")
	flag.Parse()
	if os.Getenv("GREGAL_DATA_DIR") == "" {
		log.Fatal("set GREGAL_DATA_DIR to an isolated deployment data directory")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	backend, err := harness.New(ctx, harness.Options{
		ConfigPath: *configPath, Token: os.Getenv("GREGAL_API_TOKEN"), ServeUI: *ui,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer backend.Close()
	server := &http.Server{Addr: *addr, Handler: backend.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		backend.Close()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("chat backend listening on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
