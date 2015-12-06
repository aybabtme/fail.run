package main

import (
	"flag"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/aybabtme/fail.run/svc"
	"github.com/aybabtme/log"
)

var (
	iface  = flag.String("iface", "0.0.0.0", "interface on which to listen")
	port   = flag.String("port", "0", "port on which to listen")
	assets = flag.String("assets", "web/", "path where the web assets are found")
)

func main() {
	flag.Parse()
	l, err := net.Listen("tcp", *iface+":"+*port)
	if err != nil {
		log.Err(err).Fatal("can't listen")
	}
	defer l.Close()

	// idk, just because it looks fun
	seed := time.Now().UnixNano() % (1 << 16)
	r := rand.New(rand.NewSource(seed))

	log.
		KV("addr", l.Addr().String()).
		KV("seed", seed).
		KV("assets", assets).
		Info("im alive")
	err = (&http.Server{
		Handler: svc.New(r, *assets),
	}).Serve(l)
	if err != nil {
		log.Err(err).Fatal("can't serve")
	}
}
