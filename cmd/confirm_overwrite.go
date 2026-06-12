package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// confirmOverwrite prompts the user to confirm overwriting an existing file.
// Returns true if the user confirms, false otherwise.
func confirmOverwrite(path string) bool {
	reader := bufio.NewReader(os.Stdin)
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
