package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the esmutant version",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("esmutant v%s\n", version)
		},
	}
}
