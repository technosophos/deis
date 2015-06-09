package main

import (
	"log"
	"os"

	"github.com/deis/deis/builder"
)

const LOG_PREFIX = "builder» "
const LOG_FLAGS = 0

func main() {

	cmd := "boot"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	// We just set up the outputs and then pass to Run.
	logger := log.New(os.Stdout, LOG_PREFIX, LOG_FLAGS)
	os.Exit(builder.Run(cmd, logger))
}
