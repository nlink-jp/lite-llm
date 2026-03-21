package main

import (
	"os"

	"github.com/magifd2/lite-llm/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
