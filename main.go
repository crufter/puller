package main

import (
	"flag"
	log "github.com/cihub/seelog"
	"github.com/crufter/pauler/client"
	"github.com/crufter/pauler/daemon"
	"github.com/crufter/pauler/shared"
)

func main() {
	flag.Parse()
	if *shared.D {
		log.Critical(daemon.Start())
	} else {
		client.Start()
	}
}
