package docker

import (
	"os"

	"github.com/Masterminds/cookoo"
)

// Cleanup removes any existing Docker artifacts.
//
// Returns true if the file exists (and was deleted), or false if no file
// was deleted.
func Cleanup(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {

	path := "/var/run/docker.sock"

	// If info is returned, then the file is there. If we get an error, we're
	// pretty much not going to be able to remove the file (which probably
	// doesn't exist).
	if _, err := os.Stat(path); err != nil {
		return true, os.Remove(path)
	}
	return false, nil
}

// Start starts a Docker daemon.
func Start(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	return nil, nil
}
