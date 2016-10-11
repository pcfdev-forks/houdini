package houdini

import (
	"os/exec"

	"code.cloudfoundry.org/garden"
)

func setUser(cmd *exec.Cmd, spec garden.ProcessSpec) error {
	// cmd.SysProcAttr for windows doesn't have a credentials struct object like unix
	return nil
}
