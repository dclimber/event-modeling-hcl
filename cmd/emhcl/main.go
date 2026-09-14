// Command emhcl validates, formats, renders, and serves Event Modeling HCL models.
package main

import (
	"os"

	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/cli"
	"github.com/event-modeling-hcl/eventmodeling-hcl/internal/version"
)

var buildVersion = "dev"

func main() {
	os.Exit(cli.Run("emhcl", version.ToolVersion(buildVersion), os.Args[1:], os.Stdout, os.Stderr))
}
