// Package ui is the output vocabulary from CLI-STANDARD §7: progress and
// results on stdout, warnings and errors on stderr, colour only where a
// terminal is watching.
package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
)

var (
	cyan   = color.New(color.FgCyan)
	green  = color.New(color.FgGreen)
	yellow = color.New(color.FgYellow)
	red    = color.New(color.FgRed)
	faint  = color.New(color.Faint)
)

// DisableColor strips every escape sequence, for --no-color and for a pipe.
func DisableColor() {
	color.NoColor = true
}

// Step announces progress.
func Step(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "%s %s\n", cyan.Sprint("▸"), fmt.Sprintf(format, a...))
}

// Success reports the thing the command was asked to do.
func Success(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "%s %s\n", green.Sprint("✓"), fmt.Sprintf(format, a...))
}

// Warn goes to stderr, so a warning never lands in a pipe reading the result.
func Warn(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", yellow.Sprint("!"), fmt.Sprintf(format, a...))
}

// Error goes to stderr for the same reason.
func Error(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s %s\n", red.Sprint("✗"), fmt.Sprintf(format, a...))
}

// Hint explains the line above it.
func Hint(format string, a ...any) {
	fmt.Fprintf(os.Stdout, "  %s\n", faint.Sprint(fmt.Sprintf(format, a...)))
}

// JSON writes one document to stdout and nothing else, for --json. Colour is
// already off by the time this runs: a caller piping into jq must not receive
// escape codes.
func JSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// Rows is a column-aligned listing. Nothing is written until Flush, so a
// failure part way through a listing does not leave half a table behind.
type Rows struct {
	writer *tabwriter.Writer
}

// Table starts a listing with the given header.
//
// Nothing in a table is coloured, and that is not an oversight: tabwriter
// measures a cell in bytes, so an escape sequence inside one widens the column
// by characters the terminal never draws and the listing stops lining up.
func Table(headers ...string) *Rows {
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, strings.Join(headers, "\t"))
	return &Rows{writer: writer}
}

// Row adds one line. A cell count that does not match the header is not
// checked: tabwriter simply prints what it is given.
func (r *Rows) Row(cells ...string) {
	fmt.Fprintln(r.writer, strings.Join(cells, "\t"))
}

// Flush writes the aligned table.
func (r *Rows) Flush() error {
	return r.writer.Flush()
}
