package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	httptunnel "github.com/mediabuyerbot/go-wirenet-http-tunnel"

	"github.com/mediabuyerbot/go-wirenet"
)

func main() {
	var (
		target     = flag.String("url", "", "some URL address")
		tunnelAddr = flag.String("tunnelAddr", ":8070", "tunnel addressj")
	)
	flag.Parse()

	init := make(chan wirenet.Session)
	wire, err := wirenet.Join(*tunnelAddr,
		wirenet.WithSessionOpenHook(func(session wirenet.Session) {
			init <- session
		}))
	if err != nil {
		log.Fatal(err)
	}
	defer wire.Close()
	go func() {
		log.Fatal(wire.Connect())
	}()
	sess := <-init

	client := httptunnel.NewClient(wire, "tunnel")
	ctx := httptunnel.WithSession(context.Background(), sess.ID())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *target, nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	name := "download"
	filename := filepath.Join(os.TempDir(), name)
	tmpfile, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer tmpfile.Close()
	io.Copy(tmpfile, resp.Body)

	log.Println("open ", filename)
}
