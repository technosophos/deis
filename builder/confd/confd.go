/* confd provides basic Confd support.

Right now, this library is highly specific to the needs of the present
builder. Because the confd library is not all public, we don't use it directly.
Instead, we invoke the CLI.
*/
package confd

import (
	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/safely"

	"fmt"
	"os/exec"
	"strconv"
)

// RunOnce runs the equivalent of `confd --onetime`.
//
// Importantly, this blocks until the run is complete.
//
// Params:
// - node (string): The etcd node to use. (Only etcd is currently supported)
//
// Returns:
// - The []bytes from stdout and stderr when running the program.
//
func RunOnce(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {

	node := p.Get("node", "127.0.0.1:4001").(string)
	cmd := exec.Command("confd", "-onetime", "-node", node, "-log-level", "error")

	fmt.Println("Building confd templates. This may take a moment.")
	if out, err := cmd.CombinedOutput(); err != nil {
		c.Logf("error", string(out))
		return out, err
	} else {
		c.Logf("info", "Templates generated for %s", node)
		return out, nil
	}

}

// Run starts confd and runs it in the background.
//
// If the command fails immediately on startup, an error is immediately
// returned. But from that point, a goroutine watches the command and
// reports if the command dies.
//
// Params:
// - node (string): The etcd node to use. (Only etcd is currently supported)
// - interval (int, default:5): The rebuilding interval.
//
// Returns
func Run(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	// TODO: What should be done if confd dies at some point?

	node := p.Get("node", "127.0.0.1:4001").(string)
	interval := strconv.Itoa(p.Get("interval", 5).(int))

	cmd := exec.Command("confd", "-log-level", "error", "-node", node, "-interval", interval)
	if err := cmd.Start(); err != nil {
		return false, err
	}

	c.Logf("info", "Watching confd.")
	log := c.Logf
	safely.Go(func() {
		err := cmd.Wait()
		log("info", "confd exited: %v", err)
	})

	return true, nil
}
