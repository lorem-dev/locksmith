package main

import "os"

func main() {
	run(os.Stdin, os.Stdout, defaultGetPassword)
}
