package main

import (
	"os"

	"github.com/Blockdaemon/polkadotbeat/cmd"

	_ "github.com/Blockdaemon/polkadotbeat/include"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
