package main

import (
	"os"

	"github.com/nlink-jp/lite-llm/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
