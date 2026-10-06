package awake

import (
	"os"
	"strconv"

	"github.com/yetone/magpie/internal/proc"
)

// takeHold runs caffeinate -i, which keeps the Mac from idle sleep while it
// runs; -w ends it with magpie should magpie end without letting go.
func takeHold() (func(), error) {
	cmd := proc.Command("/usr/bin/caffeinate", "-i", "-w", strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}, nil
}
