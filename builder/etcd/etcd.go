/*etcd is a library for performing common Etcd tasks.

*/
package etcd

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Masterminds/cookoo"
	"github.com/coreos/go-etcd/etcd"
)

var (
	retryCycles = 2
	retrySleep  = 200 * time.Millisecond
)

// EtcdGetter describes the Get behavior of an Etcd client.
//
// Usually you will want to use go-etcd/etcd.Client to satisfy this.
//
// We use an interface because it is more testable.
type EtcdGetter interface {
	Get(string, bool, bool) (*etcd.Response, error)
}

// EtcdDirCreator describes etcd's CreateDir behavior.
//
// Usually you will want to use go-etcd/etcd.Client to satisfy this.
type EtcdDirCreator interface {
	CreateDir(string, uint64) (*etcd.Response, error)
}

// CreateClient creates a new Etcd client and prepares it for work.
//
// Params:
// 	- url (string): A server to connect to.
// 	- retries (int): Number of times to retry a connection to the server
// 	- retrySleep (time.Duration): How long to sleep between retries
//
// Returns:
// 	This puts an *etcd.Client into the context.
func CreateClient(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	url := p.Get("url", "http://localhost:4001").(string)

	// Backed this out because it's unnecessary so far.
	//hosts := p.Get("urls", []string{"http://localhost:4001"}).([]string)
	hosts := []string{url}
	retryCycles = p.Get("retries", retryCycles).(int)
	retrySleep = p.Get("retrySleep", retrySleep).(time.Duration)

	// Support `host:port` format, too.
	for i, host := range hosts {
		if !strings.Contains(host, "://") {
			hosts[i] = "http://" + host
		}
	}

	client := etcd.NewClient(hosts)
	client.CheckRetry = checkRetry

	return client, nil
}

// Get performs an etcd Get operation.
//
// Params:
// 	- client (EtcdGetter): Etcd client
// 	- path (string): The path/key to fetch
//
// Returns:
// - This puts an `etcd.Response` into the context, and returns an error
//   if the client could not connect.
func Get(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	cli, ok := p.Has("client")
	if !ok {
		return nil, errors.New("No Etcd client found.")
	}
	client := cli.(EtcdGetter)
	path := p.Get("path", "/").(string)

	res, err := client.Get(path, false, false)
	c.Logf("debug", "Result: %V", res)
	return res, err
}

// MakeDir makes a directory in Etcd.
//
// Params:
// 	- client (EtcdDirCreator): Etcd client
//  - path (string): The name of the directory to create.
// 	- ttl (uint64): Time to live.
// Returns:
// 	*etcd.Response
func MakeDir(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	name := p.Get("path", "").(string)
	ttl := p.Get("ttl", uint64(0)).(uint64)
	cli, ok := p.Has("client")
	if !ok {
		return nil, errors.New("No Etcd client found.")
	}
	client := cli.(EtcdDirCreator)

	if len(name) == 0 {
		return false, errors.New("Expected directory name to be more than zero characters.")
	}

	res, err := client.CreateDir(name, ttl)
	c.Logf("debug", "Result: %V", res)
	return res, err
}

// checkRetry overrides etcd.DefaultCheckRetry.
//
// It adds configurable number of retries and configurable timesouts.
func checkRetry(c *etcd.Cluster, numReqs int, last http.Response, err error) error {

	if numReqs > retryCycles*len(c.Machines) {
		return fmt.Errorf("Tried and failed %d cluster connections: %s", retryCycles, err)
	}

	switch last.StatusCode {
	case 0:
		return nil
	case 500:
		time.Sleep(retrySleep)
		return nil
	default:
		return fmt.Errorf("Unhandled HTTP Error: %s %d", last.Status, last.StatusCode)
	}
}
