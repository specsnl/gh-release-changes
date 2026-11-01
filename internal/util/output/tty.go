package output

import "golang.org/x/term"

// fileDescriptor is the only thing a terminal check actually needs. Asserting
// this rather than *os.File is what lets [IsTTY] accept the io.Reader that
// cmd.InOrStdin returns as readily as the io.Writer cmd.OutOrStdout does.
type fileDescriptor interface {
	Fd() uintptr
}

// IsTTY reports whether stream is an interactive terminal. It takes any stream —
// os.Stdin as readily as os.Stdout — because the non-interactive guard asks
// about both: the tag picker is only shown when stdin and stdout are terminals,
// and a missing tag otherwise fails with ErrInteractiveRequired rather than
// hanging a pipeline or an agent's shell.
//
// The check is the isatty ioctl, not the file mode: /dev/null is a character
// device too, and `gh release-changes items < /dev/null` is not someone at a
// keyboard. Anything without a file descriptor — a bytes.Buffer in a test, say —
// is not a terminal.
func IsTTY(stream any) bool {
	f, ok := stream.(fileDescriptor)
	if !ok {
		return false
	}

	return term.IsTerminal(int(f.Fd()))
}
