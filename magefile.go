//go:build mage

package main

import "github.com/magefile/mage/sh"

// Build compiles the game to bin/vault-city.
func Build() error {
	return sh.RunV("go", "build", "-o", "bin/vault-city", ".")
}

// Run starts the game with the vault from VAULT_PATH (.env or shell).
func Run() error {
	return sh.RunV("go", "run", ".")
}
