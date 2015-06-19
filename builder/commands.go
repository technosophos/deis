package builder

import (
	"os"
	"os/signal"
	"time"

	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/safely"
)

// Sleep delays the execution of the remainder of the chain of commands.
//
// Params:
// 	-duration (time.Duration): Time to sleep.
func Sleep(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	dur := p.Get("duration", 10*time.Millisecond).(time.Duration)
	c.Logf("info", "Sleeping.")
	time.Sleep(dur)
	c.Logf("info", "Woke up.")
	return true, nil
}

// KillOnExit kills PIDs when the program exits.
//
// Otherwise, this blocks until an os.Interrupt or os.Kill is received.
//
// Params:
//  This treats Params as a map of process names (unimportant) to PIDs. It then
// attempts to kill all of the pids that it receives.
func KillOnExit(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, os.Kill)

	safely.GoDo(c, func() {
		c.Log("info", "Builder is running.")

		<-sigs

		c.Log("info", "Builder received signal to stop.")
		pids := p.AsMap()
		killed := 0
		for name, pid := range pids {
			pid, ok := pid.(int)
			if ok {
				if proc, err := os.FindProcess(pid); err == nil {
					c.Logf("info", "Killing %s (pid=%d)", name, pid)
					proc.Kill()
					killed++
				}
			}
		}
	})
	return nil, nil
}

// DeisContext is a Deis-specific version of the Context.
//
// It changes the logging support.
type DeisContext struct {
	cookoo.Context
}

// DeisCxt creates a new DeisContext that wraps an existing Context.
func DeisCxt(c cookoo.Context) cookoo.Context {
	return &DeisContext{c}
}

func (c *DeisContext) Log(prefix string, v ...interface{}) {
	c.Context.Log("["+prefix+"] ", v...)
}
func (c *DeisContext) Logf(prefix, format string, v ...interface{}) {
	c.Context.Logf("["+prefix+"] ", format, v...)
}
func (c *DeisContext) Copy() cookoo.Context {
	// We need to wrap this to make sure that the clone has our modifications.
	// Since all we're doing is revising the logging functions, this is a
	// simple operation.
	return DeisCxt(c.Context.Copy())
}
