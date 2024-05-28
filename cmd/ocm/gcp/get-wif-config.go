package gcp

import (
	"encoding/json"
	"log"
	"os"

	"github.com/openshift-online/ocm-cli/cmd/ocm/gcp/mock"

	"github.com/openshift-online/ocm-cli/pkg/arguments"
	"github.com/openshift-online/ocm-cli/pkg/dump"
	"github.com/openshift-online/ocm-cli/pkg/urls"
	"github.com/spf13/cobra"
)

var GetWorkloadIdentityConfigurationOpts struct {
	parameter []string
	header    []string
	single    bool
}

// NewCreateWorkloadIdentityConfiguration provides the "create-wif-config" subcommand
func NewGetWorkloadIdentityConfiguration() *cobra.Command {
	getWorkloadIdentityPoolCmd := &cobra.Command{
		Use:              "get-wif-config [ID]",
		Short:            "Send a GET request for wif-config.",
		Run:              getWorkloadIdentityConfigurationCmd,
		PersistentPreRun: validationForGetWorkloadIdentityConfigurationCmd,
	}

	fs := getWorkloadIdentityPoolCmd.Flags()
	arguments.AddParameterFlag(fs, &GetWorkloadIdentityConfigurationOpts.parameter)
	arguments.AddHeaderFlag(fs, &GetWorkloadIdentityConfigurationOpts.header)
	fs.BoolVar(
		&GetWorkloadIdentityConfigurationOpts.single,
		"single",
		false,
		"Return the output as a single line.",
	)

	return getWorkloadIdentityPoolCmd
}

func getWorkloadIdentityConfigurationCmd(cmd *cobra.Command, argv []string) {
	path, err := urls.Expand(argv)
	if err != nil {
		log.Fatalf("could not create URI: %v", err)
	}

	wifconfig := mock.MockWifConfig("test01", path)
	body, err := json.Marshal(wifconfig)
	if err != nil {
		log.Fatalf("failed to marshal wifconfig: %v", err)
	}

	if GetWorkloadIdentityConfigurationOpts.single {
		err = dump.Single(os.Stdout, body)
	} else {
		err = dump.Pretty(os.Stdout, body)
	}
	if err != nil {
		log.Fatalf("can't print body: %v", err)
	}
}

func validationForGetWorkloadIdentityConfigurationCmd(cmd *cobra.Command, argv []string) {
	if len(argv) != 1 {
		log.Fatalf("Expected exactly one command line parameters containing the id of the WIF config.")
	}
}
