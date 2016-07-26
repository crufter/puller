package shared

import (
	"flag"
	"github.com/crufter/pauler/types"
)

var (
	Dir  = flag.String("dir", "/etc/pauler", "The dir to load service configs from")
	D    = flag.Bool("d", false, "Run as a daemon")
	Join = flag.String("join", "", "Join a cluster")
	Port = flag.Int("port", 7946, "Port is a port used for internal communication. Port + 1 is the port number of the http server")
	Node = flag.String("node", "", "Name of the node. If left empty, defaults to os value")
)

var (
	Services        = map[string]types.Service{}
	ServiceChanged  = map[string]bool{} // service definition has changed.
	ServiceOutdated = map[string]bool{} // service was launched with an image that's older than the current one locally
	BadServiceFiles = map[string]bool{}
)
