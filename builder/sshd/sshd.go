// sshd manages SSHd services.
package sshd

import (
	"os"
	"os/exec"

	"github.com/Masterminds/cookoo"
)

func Start(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	dargs := []string{"-e", "-D"}

	sshd := exec.Command("/usr/sbin/sshd", dargs...)
	sshd.Stdout = os.Stdout
	sshd.Stderr = os.Stderr

	if err := sshd.Start(); err != nil {
		return 0, err
	}

	return sshd.Process.Pid, nil
}
