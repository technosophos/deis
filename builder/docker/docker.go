package docker

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/cookoo"
	"github.com/Masterminds/cookoo/safely"
)

var DockSock = "/var/run/docker.sock"

// Cleanup removes any existing Docker artifacts.
//
// Returns true if the file exists (and was deleted), or false if no file
// was deleted.
func Cleanup(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {

	// If info is returned, then the file is there. If we get an error, we're
	// pretty much not going to be able to remove the file (which probably
	// doesn't exist).
	if _, err := os.Stat(DockSock); err == nil {
		c.Logf("info", "Removing leftover docker socket %s: %v", DockSock)
		return true, os.Remove(DockSock)
	}
	return false, nil
}

// Start starts a Docker daemon.
func Start(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	dargs := []string{
		"-d",
		"--bip=172.19.42.1/16",
		"--insecure-registry",
		"10.0.0.0/8",
		"--insecure-registry",
		"172.16.0.0/12",
		"--insecure-registry",
		"192.168.0.0/16",
		"--insecure-registry",
		"100.64.0.0/10",
		//"--storage-driver=overlay", // Add this dynamically.
	}
	//mkdir --parents --mode=0700 /
	//fstype=$(findmnt --noheadings --output FSTYPE --target /)
	//if [[ "$fstype" == "overlay" ]]; then
	//DRIVER_OVERRIDE="--storage-driver=overlay"
	//fi

	// There is probably a better way to do this. I'm not sure whether the
	// original intent of the mkdir was to accomplish something, or just check
	// whether overlay is supported.
	if err := os.MkdirAll("/", 0700); err == nil {

		cmd := exec.Command("findmnt", "--noheadings", "--output", "FSTYPE", "--target", "/")

		if out, err := cmd.Output(); err == nil && strings.TrimSpace(string(out)) == "overlay" {
			dargs = append(dargs, "--storage-driver=overlay")
		} else {
			c.Logf("info", "File system type: '%s' (%v)", out, err)
		}
	}

	//# spawn a docker daemon to run builds
	//docker -d --bip=172.19.42.1/16 $DRIVER_OVERRIDE --insecure-registry 10.0.0.0/8 --insecure-registry 172.16.0.0/12
	// --insecure-registry 192.168.0.0/16 --insecure-registry 100.64.0.0/10 &
	//DOCKER_PID=$!

	c.Logf("info", "Starting docker with %s", strings.Join(dargs, " "))
	cmd := exec.Command("docker", dargs...)
	if err := cmd.Start(); err != nil {
		c.Logf("error", "Failed to start Docker. %s", err)
		return -1, err
	}
	// Get the PID and return it.
	return cmd.Process.Pid, nil
}

// Wait delays until Docker appears to be up and running.
func Wait(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	limit := 100
	timeout := time.Second

	for i := 0; i < limit; i++ {
		if _, err := os.Stat(DockSock); err == nil {
			c.Logf("info", "Docker service started.")
			return true, nil
		}
		c.Logf("info", "Waiting for docker. Elapsed time: %d", i)
		time.Sleep(timeout)
	}
	return false, fmt.Errorf("Docker doesn't seem to be waking up.")
}

type BuildImg struct {
	Path, Tag string
}

// ParallelBuild runs multiple docker builds at the same time.
//
// Params:
//	-images ([]BuildImg): Images to build
//
// Returns:
//
func ParallelBuild(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	images := p.Get("images", []BuildImg{}).([]BuildImg)

	var wg sync.WaitGroup

	for _, img := range images {
		wg.Add(1)
		safely.GoDo(c, func() {
			c.Logf("info", "Starting build for %s (tag: %s", img.Path, img.Tag)
			_, err := buildImg(c, img.Path, img.Tag)
			if err != nil {
				c.Logf("error", "Failed to build docker image: %s", err)
			}
			wg.Done()
		})

	}

	wg.Wait()
	c.Logf("info", "Docker images are built.")
	return true, nil
}

// BuildImage builds a docker image.
//
// Essentially, this executes:
// 	docker build -t TAG PATH
//
// Params:
// 	- path (string): The path to the image. REQUIRED
// 	- tag (string): The tag to build.
func BuildImage(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	path := p.Get("path", "").(string)
	tag := p.Get("tag", "").(string)

	c.Logf("info", "Building docker image %s (tag: %s)", path, tag)

	return buildImg(c, path, tag)
}

func buildImg(c cookoo.Context, path, tag string) ([]byte, error) {
	dargs := []string{"build"}
	if len(tag) > 0 {
		dargs = append(dargs, "-t", tag)
	}

	dargs = append(dargs, path)

	out, err := exec.Command("docker", dargs...).CombinedOutput()
	if len(out) > 0 {
		c.Logf("info", "Docker: %s", out)
	}
	return out, err
}
