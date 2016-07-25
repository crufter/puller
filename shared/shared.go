package shared

import (
	"flag"
)

var (
	Dir          = flag.String("dir", "/etc/pauler", "The dir to load service configs from")
	D            = flag.Bool("d", false, "Run as a daemon")
	Join         = flag.String("join", "", "Join a cluster")
	InternalPort = flag.Int("internal-port", 7946, "Port to use for internal daemon communication")
	HttpPort     = flag.Int("http-port", 7500, "Port to expose http interface on")
	Node         = flag.String("node", "", "Name of the node. If left empty, defaults to os value")
)
