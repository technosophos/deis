// sshd manages SSHd services.
package sshd

import (
	"github.com/Masterminds/cookoo"
	"os/exec"
)

func Start(c cookoo.Context, p *cookoo.Params) (interface{}, cookoo.Interrupt) {
	dargs := []string{"-e", "-D"}

	sshd := exec.Command("sshd", dargs...)

	if err := sshd.Start(); err != nil {
		return 0, err
	}

	return sshd.Process.Pid, nil
}
