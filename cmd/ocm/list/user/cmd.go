/*
Copyright (c) 2020 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package user

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	c "github.com/openshift-online/ocm-cli/pkg/cluster"
	"github.com/openshift-online/ocm-cli/pkg/config"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
)

var args struct {
	clusterKey string
}

// Cmd Constant:
var Cmd = &cobra.Command{
	Use:     "users --cluster=mycluster [flags]",
	Aliases: []string{"user"},
	Short:   "List cluster users",
	Long:    "List administrative cluster users",
	Args:    cobra.RangeArgs(0, 1),
	RunE:    run,
}

func init() {
	fs := Cmd.Flags()
	fs.StringVarP(
		&args.clusterKey,
		"cluster",
		"c",
		"",
		"Name or ID of the cluster to add the IdP to (required).",
	)
	//nolint:gosec
	Cmd.MarkFlagRequired("cluster")
}

func run(cmd *cobra.Command, argv []string) error {
	// Check that the cluster key (name, identifier or external identifier) given by the user
	// is reasonably safe so that there is no risk of SQL injection:
	clusterKey := args.clusterKey
	if !c.IsValidClusterKey(clusterKey) {
		return fmt.Errorf(
			"Cluster name, identifier or external identifier '%s' isn't valid: it "+
				"must contain only letters, digits, dashes and underscores",
			clusterKey,
		)
	}
	// Load the configuration file:
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("Can't load config file: %v", err)
	}
	if cfg == nil {
		return fmt.Errorf("Not logged in, run the 'login' command")
	}

	// Check that the configuration has credentials or tokens that haven't have expired:
	armed, err := cfg.Armed()
	if err != nil {
		return fmt.Errorf("Can't check if tokens have expired: %v", err)
	}
	if !armed {
		return fmt.Errorf("Tokens have expired, run the 'login' command")
	}

	// Create the connection, and remember to close it:
	connection, err := cfg.Connection()
	if err != nil {
		return fmt.Errorf("Can't create connection: %v", err)
	}
	defer connection.Close()

	clusterCollection := connection.ClustersMgmt().V1().Clusters()

	cluster, err := c.GetCluster(clusterCollection, clusterKey)
	if err != nil {
		return fmt.Errorf("Failed to get cluster '%s': %v", clusterKey, err)
	}

	if cluster.State() != cmv1.ClusterStateReady {
		return fmt.Errorf("Cluster '%s' is not yet ready", clusterKey)
	}

	groups, err := c.GetGroups(clusterCollection, cluster.ID())
	if err != nil {
		return fmt.Errorf("Failed to get users for cluster '%s': %v", clusterKey, err)
	}

	// Create the writer that will be used to print the tabulated results:
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "GROUP\t\tUSER\n")

	for _, group := range groups {
		groupName := group.ID()
		for _, user := range group.Users().Slice() {
			fmt.Fprintf(writer, "%s\t\t%s\n", groupName, user.ID())
			err = writer.Flush()
			if err != nil {
				return nil
			}
		}
	}

	return nil
}
