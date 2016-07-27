package main

import (
	"flag"
	log "github.com/cihub/seelog"
	"github.com/crufter/puller/client"
	"github.com/crufter/puller/daemon"
	"github.com/crufter/puller/shared"
)

func main() {
	flag.Parse()
	if *shared.D {
		log.Critical(daemon.Start())
	} else {
		client.Start()
	}
}
