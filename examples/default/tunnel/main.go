package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	httptunnel "github.com/mediabuyerbot/go-wirenet-http-tunnel"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	var (
		addr = flag.String("addr", ":8070", "wirenet server addr")
	)
	flag.Parse()

	wire, err := wirenet.Mount(*addr,
		wirenet.WithSessionOpenHook(func(session wirenet.Session) {
			log.Println(session)
		}),
		wirenet.WithSessionCloseHook(func(session wirenet.Session) {
			log.Println(session)
		}),
	)
	if err != nil {
		log.Fatal(err)
	}

	tunnel := httptunnel.New()
	wire.Stream("tunnel", tunnel.Handle)

	go func() {
		log.Fatal(wire.Connect())
	}()

	<-terminate()

	if err := wire.Close(); err != nil {
		log.Fatal(err)
	}
}

func terminate() chan struct{} {
	done := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		done <- struct{}{}
	}()
	return done
}
