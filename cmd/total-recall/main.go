package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	root := &cobra.Command{
		Use:     "total-recall",
		Short:   "Total Recall — AI-powered developer recall for the age of AI-assisted coding",
		Version: version,
	}

	root.AddCommand(
		serveCmd(),
		initCmd(),
		configCmd(),
		statusCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Total Recall daemon on localhost:7331",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented")
			return nil
		},
	}
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize Total Recall for this project and create user config",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented")
			return nil
		},
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Read and write Total Recall config values",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status and active config",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("not implemented")
			return nil
		},
	}
}
