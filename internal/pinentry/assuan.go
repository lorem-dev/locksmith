package pinentry

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// errCancelled is returned by getPassword implementations when the user cancels.
var errCancelled = errors.New("operation cancelled")

// run implements the Assuan pinentry protocol over r/w.
// getPassword is called on GETPIN with (description, prompt) and must return
// the passphrase or errCancelled.
func run(r io.Reader, w io.Writer, getPassword func(desc, prompt string) (string, error)) {
	fmt.Fprintln(w, "OK Pleased to meet you")

	scanner := bufio.NewScanner(r)
	var desc, prompt string

	for scanner.Scan() {
		line := scanner.Text()
		cmd, arg, _ := strings.Cut(line, " ")
		cmd = strings.ToUpper(strings.TrimSpace(cmd))

		switch cmd {
		case "GETPIN":
			pin, err := getPassword(desc, prompt)
			if err != nil {
				// GPG_ERR_CANCELED = 83886179
				fmt.Fprintln(w, "ERR 83886179 Operation cancelled")
			} else {
				fmt.Fprintf(w, "D %s\n", encodeAssuan(pin))
				fmt.Fprintln(w, "OK")
			}
		case "SETDESC":
			desc = decodePercent(arg)
			fmt.Fprintln(w, "OK")
		case "SETPROMPT":
			prompt = decodePercent(arg)
			fmt.Fprintln(w, "OK")
		case "SETERROR", "SETTITLE", "SETKEYINFO", "SETREPEAT",
			"SETREPEATERROR", "OPTION", "CLEARPASSPHRASE", "NOP":
			fmt.Fprintln(w, "OK")
		case "BYE":
			fmt.Fprintln(w, "OK closing connection")
			return
		default:
			fmt.Fprintf(w, "ERR 536871187 Unknown command %s\n", cmd)
		}
	}
}

// encodeAssuan percent-encodes characters that must not appear literally in an
// Assuan data line: '%', CR, LF, and NUL.
func encodeAssuan(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '%', '\r', '\n', 0:
			fmt.Fprintf(&b, "%%%02X", c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// decodePercent decodes Assuan percent-encoding (%XX hex codes).
func decodePercent(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '%' && i+2 < len(s) {
			hi := hexVal(s[i+1])
			lo := hexVal(s[i+2])
			if hi >= 0 && lo >= 0 {
				b.WriteByte(byte(hi<<4 | lo))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}
