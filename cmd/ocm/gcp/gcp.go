package gcp

import (
	"github.com/spf13/cobra"
)

type options struct {
	TargetDir                string
	Region                   string
	Name                     string
	Project                  string
	WorkloadIdentityPool     string
	WorkloadIdentityProvider string
	DryRun                   bool
}

// NewGCPCmd implements the "gcp" subcommand for the credentials provisioning
func NewGCPCmd() *cobra.Command {
	gcpCmd := &cobra.Command{
		Use:   "gcp COMMAND",
		Short: "Perform actions related to GCP WIF",
		Long:  "Manage GCP Workload Identity Federation (WIF) resources.",
		Args:  cobra.MinimumNArgs(1),
	}

	gcpCmd.AddCommand(NewCreateWorkloadIdentityConfiguration())
	gcpCmd.AddCommand(NewUpdateWorkloadIdentityConfiguration())
	gcpCmd.AddCommand(NewDeleteWorkloadIdentityConfiguration())
	gcpCmd.AddCommand(NewGetWorkloadIdentityConfiguration())
	gcpCmd.AddCommand(NewListWorkloadIdentityConfiguration())
	gcpCmd.AddCommand(NewDescribeWorkloadIdentityConfiguration())

	return gcpCmd
}
