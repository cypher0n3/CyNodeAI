package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cypher0n3/cynodeai/cynork/internal/exit"
	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Interactive REPL; run cynork commands from a prompt",
	Long:  "Reads lines, parses as cynork argv (split on spaces, respect quotes), runs the same command surface. Use -c to run a single command and exit with its exit code.",
	RunE:  runShell,
}

var shellCommand string

func init() {
	rootCmd.AddCommand(shellCmd)
	shellCmd.Flags().StringVarP(&shellCommand, "command", "c", "", "run single command and exit")
}

func runShell(_ *cobra.Command, _ []string) error {
	if shellCommand != "" {
		args := parseArgs(shellCommand)
		rootCmd.SetArgs(args)
		if err := rootCmd.Execute(); err != nil {
			code := exit.CodeOf(err)
			fmt.Fprintln(os.Stderr, err)
			os.Exit(code)
		}
		return nil
	}
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "cynork> ")
		if !scanner.Scan() {
			return nil
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		args := parseArgs(line)
		rootCmd.SetArgs(args)
		_ = rootCmd.Execute()
	}
}

// parseArgs splits line into args, respecting double-quoted segments.
func parseArgs(line string) []string {
	var args []string
	var buf strings.Builder
	inQuote := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case inQuote:
			buf.WriteByte(c)
		case c == ' ' || c == '\t':
			if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteByte(c)
		}
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args
}
