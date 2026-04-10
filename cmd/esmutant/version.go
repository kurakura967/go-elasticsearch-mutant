package main

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

func getVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" {
		return "dev"
	}
	return v
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the esmutant version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("esmutant %s\n", getVersion())
		},
	}
}
