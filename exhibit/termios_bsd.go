// +build darwin freebsd netbsd openbsd

package exhibit

import (
	"syscall"
)

const ioctlReadTermios = syscall.TIOCGETA
const ioctlWriteTermios = syscall.TIOCSETA
