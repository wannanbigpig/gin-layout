package cmd

import (
	"github.com/spf13/cobra"
	"testing"
)

func TestReleaseCheckDoesNotBootstrapConfigOrDatabase(t *testing.T) {
	root := &cobra.Command{Use: "go-layout"}
	parent := &cobra.Command{Use: "command"}
	child := &cobra.Command{Use: "release-check"}
	root.AddCommand(parent)
	parent.AddCommand(child)
	if !shouldSkipBootstrap(child) {
		t.Fatal("offline release-check should not initialize config/logger/data")
	}
}
