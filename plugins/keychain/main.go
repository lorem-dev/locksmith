package main

import "github.com/lorem-dev/locksmith/sdk/vault"

func main() { vault.Serve(&KeychainProvider{}) }
