/* builder Provides libraries for the Deis builder.

The Deis builder is responsible for packaging Docker images for consumers.

The builder/cli package contains command line clients for this library.
*/
package builder

import (
	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/fmt"
	//"github.com/Masterminds/cookoo/io"
	"github.com/deis/deis/builder/confd"
	"github.com/deis/deis/builder/docker"
	"github.com/deis/deis/builder/env"
	"github.com/deis/deis/builder/etcd"
	"github.com/deis/deis/builder/sshd"
	"log"
	"os"
	"os/signal"
	"time"
)

const (
	StatusOk = iota
	StatusLocalError
)

// Run executes the boot commands for builder.
//
// Run returns on of the Status* status code constants.
func Run(cmd string) int {
	reg, router, ocxt := cookoo.Cookoo()
	cxt := &CxtMunger{ocxt}

	log.SetFlags(0) // Time is captured elsewhere.

	//out := io.NewColorizer(os.Stdout)
	cxt.AddLogger("stdout", os.Stdout)

	// Build the routes
	routes(reg)

	// Boot
	if err := router.HandleRequest(cmd, cxt, false); err != nil {
		cxt.Logf("error", "Fatal errror on boot: %s", err)
		return StatusLocalError
	}

	return StatusOk
}

// routes builds the Cookoo registry.
//
// Esssentially this is a list of all of the things that Builder can do, broken
// down into a step-by-step list.
func routes(reg *cookoo.Registry) {

	// The "boot" route starts up the builder as a daemon process. Along the
	// way, it starts and configures multiple services, including etcd, confd,
	// and sshd.
	reg.AddRoute(cookoo.Route{
		Name: "boot",
		Help: "Boot the builder",
		Does: cookoo.Tasks{

			// ENV: Make sure the environment is correct.
			cookoo.Cmd{
				Name: "vars",
				Fn:   env.Get,
				Using: []cookoo.Param{
					{Name: "HOST", DefaultValue: "127.0.0.1"},
					{Name: "ETCD_PORT", DefaultValue: "4001"},
					{Name: "ETCD_PATH", DefaultValue: "/deis/builder"},
					{Name: "ETCD_TTL", DefaultValue: "20"},
				},
			},
			cookoo.Cmd{ // This depends on others being processed first.
				Name: "vars2",
				Fn:   env.Get,
				Using: []cookoo.Param{
					{Name: "ETCD", DefaultValue: "http://$HOST:$ETCD_PORT"},
				},
			},

			// ETCD: Make sure Etcd is running, and do the initial population.
			cookoo.Cmd{
				Name:  "client",
				Fn:    etcd.CreateClient,
				Using: []cookoo.Param{{Name: "url", DefaultValue: "http://127.0.0.1:4001", From: "cxt:ETCD"}},
			},
			cookoo.Cmd{
				Name: "ls",
				Fn:   etcd.Get,
				Using: []cookoo.Param{
					{Name: "client", From: "cxt:client"},
					{Name: "retries", DefaultValue: 20},
				},
			},
			cookoo.Cmd{
				Name:  "-",
				Fn:    Sleep,
				Using: []cookoo.Param{{Name: "duration", DefaultValue: 21 * time.Second}},
			},
			cookoo.Cmd{
				Name: "newdir",
				Fn:   fmt.Sprintf,
				Using: []cookoo.Param{
					{Name: "format", DefaultValue: "%s/users"},
					{Name: "0", From: "cxt:ETCD_PATH"},
				},
			},
			cookoo.Cmd{
				Name: "mkdir",
				Fn:   etcd.MakeDir,
				Using: []cookoo.Param{
					{Name: "path", From: "cxt:newdir"},
					{Name: "client", From: "cxt:client"},
				},
			},

			// CONFD: Build out the templates, then start the Confd server.
			cookoo.Cmd{
				Name:  "once",
				Fn:    confd.RunOnce,
				Using: []cookoo.Param{{Name: "node", From: "cxt:ETCD"}},
			},
			cookoo.Cmd{
				Name:  "confd",
				Fn:    confd.Run,
				Using: []cookoo.Param{{Name: "node", From: "cxt:ETCD"}},
			},

			// DOCKER: start up Docker and make sure it's running.
			cookoo.Cmd{
				Name: "dockerclean",
				Fn:   docker.Cleanup,
			},
			cookoo.Cmd{
				Name: "dockerstart",
				Fn:   docker.Start,
			},
			cookoo.Cmd{
				Name: "waitfordocker",
				Fn:   docker.Wait,
			},
			cookoo.Cmd{
				Name: "slugbuilder",
				Fn:   docker.BuildImage,
				Using: []cookoo.Param{
					{Name: "tag", DefaultValue: "deis/slugbuilder"},
					{Name: "path", DefaultValue: "/usr/local/src/slugbuilder/"},
				},
			},
			cookoo.Cmd{
				Name: "slugrunner",
				Fn:   docker.BuildImage,
				Using: []cookoo.Param{
					{Name: "tag", DefaultValue: "deis/slugrunner"},
					{Name: "path", DefaultValue: "/usr/local/src/slugrunner/"},
				},
			},
			// Start SSHD.
			cookoo.Cmd{
				Name: "sshdstart",
				Fn:   sshd.Start,
			},

			// ETDCD: Now watch for events on etcd, and trigger a git check-repos for
			// each. For the most part, this runs in the background.
			cookoo.Cmd{
				Name: "Cleanup",
				Fn:   etcd.Watch,
				Using: []cookoo.Param{
					{Name: "client", From: "cxt:client"},
				},
			},
			// If there's an EXTERNAL_PORT, we publish info to etcd.
			cookoo.Cmd{
				Name: "externalport",
				Fn:   env.Get,
				Using: []cookoo.Param{
					{Name: "EXTERNAL_PORT", DefaultValue: ""},
				},
			},
			cookoo.Cmd{
				Name: "etcdupdate",
				Fn:   etcd.UpdateHostPort,
				Using: []cookoo.Param{
					{Name: "base", From: "cxt:ETCD_PATH"},
					{Name: "host", From: "cxt:HOST"},
					{Name: "client", From: "cxt:client"},
					{Name: "sshdPid", From: "cxt:sshd"},
				},
			},

			// DAEMON: Finally, we wait around for a signal, and then cleanup.
			cookoo.Cmd{
				Name: "listen",
				Fn:   KillOnExit,
				Using: []cookoo.Param{
					{Name: "docker", From: "cxt:dockerstart"},
					{Name: "sshd", From: "cxt:sshdstart"},
				},
			},
		},
	})
}

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

	c.Log("»", "Builder is running.")

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
	return killed, nil
}

// CxtMunger fixes the spacing brokenness in the Go logger.
type CxtMunger struct {
	cookoo.Context
}

func (c *CxtMunger) Log(prefix string, v ...interface{}) {
	c.Context.Log(prefix+" ", v...)
}
func (c *CxtMunger) Logf(prefix, format string, v ...interface{}) {
	c.Context.Logf(prefix+" ", format, v...)
}
