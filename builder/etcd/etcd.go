/*etcd is a library for performing common Etcd tasks.

*/
package etcd

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
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

type EtcdWatcher interface {
	Watch(string, uint64, bool, chan *etcd.Response, chan bool) (*etcd.Response, error)
}

type EtcdSetter interface {
	Set(string, string, uint64) (*etcd.Response, error)
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
	if err != nil {
		return res, err
	}
	if !res.Node.Dir {
		return res, fmt.Errorf("Expected / to be a dir.")
	}
	//c.Logf("debug", "Result: %V", res)
	return res, err
}

func Set(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	key := p.Get("key", "").(string)
	value := p.Get("value", "").(string)
	ttl := p.Get("ttl", uint64(20)).(uint64)
	client := p.Get("client", nil).(EtcdSetter)

	res, err := client.Set(key, value, ttl)
	if err != nil {
		c.Logf("Failed to set %s=%s", key, value)
	}

	return res, err
}

func UpdateHostPort(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {

	base := p.Get("base", "").(string)
	host := p.Get("host", "").(string)
	port := p.Get("port", "").(string)
	client := p.Get("client", nil).(EtcdSetter)
	sshd := p.Get("sshdPid", 0).(int)

	// If no port is specified, we don't do anything.
	if len(port) == 0 {
		return false, nil
	}

	var ttl uint64 = uint64(20)

	if err := setHostPort(client, base, host, port, ttl); err != nil {
		c.Logf("error", "Etcd error setting host/port: %s", err)
		return false, err
	}

	go func() {
		ticker := time.Tick(10 * time.Second)
		for range ticker {
			c.Logf("info", "Setting SSHD host/port")
			if _, err := os.FindProcess(sshd); err != nil {
				c.Logf("error", "Lost SSHd process: %s", err)
				return
			} else {
				if err := setHostPort(client, base, host, port, ttl); err != nil {
					c.Logf("error", "Etcd error setting host/port: %s", err)
					return
				}
			}
		}
	}()

	return true, nil
}

func setHostPort(client EtcdSetter, base, host, port string, ttl uint64) error {
	if _, err := client.Set(base+"/host", host, ttl); err != nil {
		return err
	}
	if _, err := client.Set(base+"/port", port, ttl); err != nil {
		return err
	}
	return nil
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
	if err != nil {
		// FIXME: Do we want this to be recoverable?
		return res, &cookoo.RecoverableError{err.Error()}
	}

	return res, nil

}

// Watch watches a given path, and executes a git cehck-repos for each event.
//
// It starts the watcher and then returns. The watcher runs on its own
// goroutine. To stop the watching, send the returned channel a bool.
//
// Params:
// - client (EtcdWatcher): An Etcd client.
// - path (string): The path to watch
//
// Returns:
// 	- chan bool: Send this a message to stop the watcher.
func Watch(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	// etcdctl -C $ETCD watch --recursive /deis/services
	path := p.Get("path", "/deis/services").(string)
	cli, ok := p.Has("client")
	if !ok {
		return nil, errors.New("No etcd client found.")
	}
	client := cli.(EtcdWatcher)

	receiver := make(chan *etcd.Response)

	stop := make(chan bool)
	// Buffer the channels so that we don't hang waiting for go-etcd to
	// read off the channel.
	stopetcd := make(chan bool, 1)
	stopwatch := make(chan bool, 1)

	// Watch for errors.
	go func() {
		// When a receiver is passed in, no *Response is ever returned. Instead,
		// Watch acts like an error channel, and receiver gets all of the messages.
		_, err := client.Watch(path, 0, true, receiver, stopetcd)
		if err != nil {
			c.Logf("info", "Watcher stopped with error %s", err)
			stopwatch <- true
			close(stopwatch)
		}
	}()
	// Watch for events
	go func() {
		for {
			select {
			case msg := <-receiver:
				c.Logf("Received notification %s for %s", msg.Action, msg.Node.Key)
				git := exec.Command("/home/git/check-repos")
				if out, err := git.CombinedOutput(); err != nil {
					c.Logf("error", "Failed git check-repos: %s", err)
					c.Logf("info", "Output: %s", out)
				}
			case <-stopwatch:
				return
			}
		}
	}()
	// Fan out stop requests.
	go func() {
		<-stop
		stopwatch <- true
		stopetcd <- true
		close(stopwatch)
		close(stopetcd)
	}()

	return stop, nil
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
