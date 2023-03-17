package main

import (
	"fmt"
	"os"

	"github.com/leofeyer/gh-changelog/api"
	"github.com/spf13/cobra"
)

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gh changelog <milestone> <version>",
		Short: "Create a changelog",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			milestone := args[0]

			version := "Unreleased"
			if len(args) > 1 {
				version = args[1]
			}

			err := api.Changelog(milestone, version)
			if err != nil {
				return err
			}

			return nil
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	return cmd
}

func main() {
	err := rootCmd().Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
