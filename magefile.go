//go:build mage

package main

import "github.com/magefile/mage/sh"

// Run starts the game with the vault from VAULT_PATH (.env or shell).
func Run() error {
	return sh.RunV("go", "run", ".")
}
