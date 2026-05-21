//go:build windows

package cmd

import (
	"os/exec"
)

func configureSysProcAttr(cmd *exec.Cmd) {
	// On Windows, Setsid is not supported. Leaving SysProcAttr empty
	// is sufficient for starting a background process via cmd.Start().
}
