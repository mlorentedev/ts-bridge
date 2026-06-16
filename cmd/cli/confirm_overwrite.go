package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// confirmOverwrite prompts the user to confirm overwriting an existing file.
// Returns true if the user confirms, false otherwise.
// Uses cmd.InOrStdin() to respect Cobra's input redirection.
func confirmOverwrite(path string, cmd *cobra.Command) bool {
	in := cmd.InOrStdin()
	reader := bufio.NewReader(in)
	for {
		fmt.Printf("\n%s already exists. Overwrite? [y/N]: ", path)
		line, err := reader.ReadString('\n')
		if err != nil {
			return false
		}
		answer := strings.TrimSpace(strings.ToLower(line))
		switch answer {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		default:
			fmt.Fprintf(os.Stderr, "Please enter y or n.\n")
		}
	}
}
