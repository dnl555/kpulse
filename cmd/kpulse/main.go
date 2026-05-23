package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	fmt.Printf("kpulse %s starting\n", version)
	os.Exit(0)
}
