package keystore

import (
	"fmt"
	"syscall"

	"golang.org/x/term"
)

// GetPassword prompts the user to enter a password for an encrypted keystore.
func GetPassword(msg string) []byte {
	for {
		fmt.Println(msg)
		fmt.Print("> ")
		password, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			fmt.Printf("invalid input: %s\n", err)
		} else {
			fmt.Printf("\n")
			return password
		}
	}
}
