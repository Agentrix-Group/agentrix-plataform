package main

import (
	"fmt"
	"os"

	"agentrix/backend/internal/sandbox"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: agentrix-runtime-digest <runtime-root>")
		os.Exit(2)
	}
	digest, err := sandbox.RuntimeDigest(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(digest)
}
