package main

import sdk "github.com/lorem-dev/locksmith/sdk"

func main() { sdk.Serve(&KeychainProvider{}) }
