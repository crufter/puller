package main

import (
	"flag"
	log "github.com/cihub/seelog"
	"github.com/crufter/puller/client"
	"github.com/crufter/puller/daemon"
	"github.com/crufter/puller/daemon/api"
	"github.com/crufter/puller/shared"
)

func main() {
	flag.Parse()
	if *shared.D {
		go func() {
			api.Start()
		}()
		log.Critical(daemon.Start())
	} else {
		client.Start()
	}
}
