/* builder Provides libraries for the Deis builder.

The Deis builder is responsible for packaging Docker images for consumers.

The builder/cli package contains command line clients for this library.
*/
package builder

import (
	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/fmt"
	"github.com/Masterminds/cookoo/io"
	"github.com/deis/deis/builder/confd"
	"github.com/deis/deis/builder/docker"
	"github.com/deis/deis/builder/env"
	"github.com/deis/deis/builder/etcd"
	"log"
	"os"
	"time"
)

const (
	StatusOk = iota
	StatusLocalError
)

// Run executes the boot commands for builder.
//
// Run returns on of the Status* status code constants.
func Run(cmd string, logger *log.Logger) int {
	reg, router, cxt := cookoo.Cookoo()

	out := io.NewColorizer(os.Stdout)
	cxt.AddLogger("stdout", out)

	// Build the routes
	routes(reg)

	// Boot
	if err := router.HandleRequest(cmd, cxt, false); err != nil {
		logger.Printf("Error: %s", err)
		return StatusLocalError
	}

	return StatusOk
}

// routes builds the Cookoo registry.
//
// Esssentially this is a list of all of the things that Builder can do, broken
// down into a step-by-step list.
func routes(reg *cookoo.Registry) {

	reg.AddRoute(cookoo.Route{
		Name: "boot",
		Help: "Boot the builder",
		Does: cookoo.Tasks{
			cookoo.Cmd{
				Name: "vars",
				Fn:   env.Get,
				Using: []cookoo.Param{
					{Name: "Host", DefaultValue: "127.0.0.1"},
					{Name: "ETCD_PORT", DefaultValue: "4001"},
					{Name: "ETCD", DefaultValue: "http://$HOST:$ETCD_PORT"},
					{Name: "ETCD_PATH", DefaultValue: "/deis/builder"},
					{Name: "ETCD_TTL", DefaultValue: "20"},
				},
			},
			cookoo.Cmd{
				Name:  "client",
				Fn:    etcd.CreateClient,
				Using: []cookoo.Param{{Name: "url", DefaultValue: "http://localhost:4001", From: "cxt:ETCD"}},
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
			cookoo.Cmd{
				Name: "dockerclean",
				Fn:   docker.Cleanup,
			},
		},
	})
}

// Sleep delays the execution of the remainder of the chain of commands.
func Sleep(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	dur := p.Get("duration", 10*time.Millisecond).(time.Duration)
	c.Logf("info", "Sleeping.")
	time.Sleep(dur)
	c.Logf("info", "Woke up.")
	return true, nil
}
