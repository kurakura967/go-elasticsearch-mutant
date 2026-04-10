package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "esmutant",
		Short: "Mutation testing for go-elasticsearch Typed API queries",
		Long: `esmutant detects weaknesses in your Elasticsearch query tests
by applying small mutations to your query-building code and checking
whether your test suite catches them.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newVersionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
