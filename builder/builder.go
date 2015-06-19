/* Packge builder provides libraries for the Deis builder.

The Deis builder is responsible for packaging Docker images for consumers.

The builder/cli package contains command line clients for this library.
*/
package builder

import (
	"github.com/Masterminds/cookoo"
	"github.com/deis/deis/builder/sshd"

	"log"
	"os"
)

const (
	StatusOk = iota
	StatusLocalError
)

// Run starts the Builder service.
//
// The Builder service is responsible for setting up the local container
// environment and then listening for new builds. The main listening service
// is SSH. Builder listens for new Git commands and then sends those on to
// Git.
//
// Run returns on of the Status* status code constants.
func Run(cmd string) int {
	reg, router, ocxt := cookoo.Cookoo()
	log.SetFlags(0) // Time is captured elsewhere.

	// We layer the context to add better logging and also synchronize
	// access so that goroutines don't get into race conditions.
	cxt := cookoo.SyncContext(DeisCxt(ocxt))
	cxt.Put("cookoo.Router", router)
	cxt.AddLogger("stdout", os.Stdout)

	// Build the routes. See routes.go.
	routes(reg)

	// Bootstrap the background services. If this fails, we stop.
	if err := router.HandleRequest("boot", cxt, false); err != nil {
		cxt.Logf("error", "Fatal errror on boot: %s", err)
		return StatusLocalError
	}

	// Set up the SSH service.
	cxt.Put(sshd.Address, "0.0.0.0:22")

	// Supply route names for handling various internal routing. While this
	// isn't necessary for Cookoo, it makes it easy for us to mock these
	// routes in tests. c.f. sshd/server.go
	cxt.Put("route.sshd.pubkeyAuth", "pubkeyAuth")
	cxt.Put("route.sshd.sshPing", "sshPing")
	cxt.Put("route.sshd.sshGitReceive", "sshGitReceive")

	// Start the SSH service.
	// XXX: We could refactor Serve to be a command, and then run this as
	// a route.
	if err := sshd.Serve(reg, router, cxt); err != nil {
		cxt.Logf("error", "SSH server failed: %s", err)
		return StatusLocalError
	}

	return StatusOk
}
