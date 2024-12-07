// Security linting is turned off for this file as it is only used for testing.
//
//nolint:gosec
package tests

import (
	"context"
	"fmt"
	"math/rand"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sdk "github.com/openshift-online/ocm-sdk-go"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"

	"github.com/openshift-online/ocm-cli/pkg/gcp"
	"github.com/openshift-online/ocm-cli/pkg/ocm"
)

var _ = Describe("wif update test suite", Ordered, func() {

	var (
		ocmUrl          = os.Getenv("OCM_URL")
		ocmClientId     = os.Getenv("OCM_CLIENT_ID")
		ocmClientSecret = os.Getenv("OCM_CLIENT_SECRET")
		testWifId       = os.Getenv("TEST_WIF_ID")
	)

	ctx := context.Background()

	var clients struct {
		ocm *sdk.Connection
		gcp gcp.GcpClient
	}

	var wifConfig *cmv1.WifConfig

	// Holds the ocm credentials to invoke cli commands
	var ocmCliConfig string

	BeforeAll(func() {
		var err error

		// Verifying test environment variables
		Expect(ocmUrl).ToNot(Equal(""),
			"The environment variable 'OCM_URL' should be set "+
				"to specify the ocm endpoint that will be used by the tests.")
		Expect(ocmClientId).ToNot(Equal(""),
			"The environment variable 'OCM_CLIENT_ID' should be set "+
				"to specify the client id that will be used by the tests.")
		Expect(ocmClientSecret).ToNot(Equal(""),
			"The environment variable 'OCM_CLIENT_SECRET' should be set "+
				"to specify the client secret that will be used by the tests.")
		Expect(testWifId).ToNot(Equal(""),
			"The environment variable 'TEST_WIF_ID' should be set "+
				"to specify the wif-config resource that will be used by the tests.")

		// Initializing the clients
		clients.ocm, err = ocm.NewConnection().Build()
		Expect(err).ToNot(HaveOccurred(), "failed to create ocm client")

		clients.gcp, err = gcp.NewGcpClient(context.Background())
		Expect(err).ToNot(HaveOccurred(), "failed to create gcp client")

		result := NewCommand().
			Args(
				"login",
				"--client-id", ocmClientId,
				"--client-secret", ocmClientSecret,
				"--url", ocmUrl,
			).
			Run(ctx)
		Expect(result.ExitCode()).To(BeZero())
		ocmCliConfig = result.ConfigString()

		// Retreiving the user-provided wif-config resource.
		resp, err := clients.ocm.ClustersMgmt().V1().GCP().WifConfigs().WifConfig(testWifId).Get().Send()
		Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("failed to get wif-config with id '%s'", testWifId))
		Expect(resp.Body()).ToNot(BeNil(), "nil value returned when getting wif-config")
		wifConfig = resp.Body()
	})

	Describe("wif config verification and remediation", Ordered, func() {

		verifyWifConfig := func(errMsgSubstring string) func() {
			return func() {
				verifyCall := NewCommand().
					ConfigString(ocmCliConfig).
					Args(
						"gcp", "verify", "wif-config", wifConfig.ID(),
					).Run(ctx)
				Expect(verifyCall.ExitCode()).ToNot(BeZero())
				Expect(verifyCall.ErrString()).To(ContainSubstring(errMsgSubstring))
			}
		}

		repairWifConfig := func() {
			updateCall := NewCommand().
				ConfigString(ocmCliConfig).
				Args(
					"gcp", "update", "wif-config", wifConfig.ID(),
				).Run(ctx)
			Expect(updateCall.ExitCode()).To(BeZero(),
				fmt.Sprintf("failed to update wif-config '%s'", wifConfig.ID()))

			verifyCall := NewCommand().
				ConfigString(ocmCliConfig).
				Args(
					"gcp", "verify", "wif-config", wifConfig.ID(),
				).Run(ctx)
			Expect(verifyCall.ExitCode()).To(BeZero(),
				verifyCall.ErrString())
		}

		Context("disabled service accounts", Ordered, func() {
			var testServiceAccount *cmv1.WifServiceAccount
			BeforeAll(func() {
				testServiceAccount = chooseRandomServiceAccount(wifConfig)

				err := clients.gcp.DisableServiceAccount(
					ctx,
					testServiceAccount.ServiceAccountId(),
					wifConfig.Gcp().ProjectId(),
				)
				Expect(err).ToNot(HaveOccurred(),
					fmt.Sprintf("failed to disable service account '%s'",
						testServiceAccount.ServiceAccountId()))

			})
			It("detects the misconfiguration", verifyWifConfig("disabled"))
			It("repairs the issue through the update command", repairWifConfig)
			AfterAll(func() {
				serviceAccount, err := clients.gcp.GetServiceAccountv2(
					ctx,
					testServiceAccount.ServiceAccountId(),
					wifConfig.Gcp().ProjectId(),
				)
				Expect(err).ToNot(HaveOccurred(),
					fmt.Sprintf("failed to get service account '%s'",
						testServiceAccount.ServiceAccountId()))
				if serviceAccount.Disabled() {
					err := clients.gcp.EnableServiceAccount(
						ctx,
						testServiceAccount.ServiceAccountId(),
						wifConfig.Gcp().ProjectId(),
					)
					Expect(err).ToNot(HaveOccurred(),
						fmt.Sprintf("failed to re-enable service account '%s'",
							testServiceAccount.ServiceAccountId()))
				}
			})
		})
		Context("deleted service accounts", Ordered, func() {
			var testServiceAccount *cmv1.WifServiceAccount

			// The service account unique ID is needed in case the tests
			// fail and the service account must be undeleted by the
			// "AfterAll" spec.
			var serviceAccountUniqueId string

			BeforeAll(func() {
				testServiceAccount = chooseRandomServiceAccount(wifConfig)

				saData, err := clients.gcp.GetServiceAccountv2(
					ctx,
					testServiceAccount.ServiceAccountId(),
					wifConfig.Gcp().ProjectId(),
				)
				Expect(err).ToNot(HaveOccurred(),
					fmt.Sprintf("failed to get service account '%s'",
						testServiceAccount.ServiceAccountId()))

				serviceAccountUniqueId = saData.UniqueId()

				err = clients.gcp.DeleteServiceAccount(
					ctx,
					testServiceAccount.ServiceAccountId(),
					wifConfig.Gcp().ProjectId(),
					false,
				)
				Expect(err).ToNot(HaveOccurred(),
					fmt.Sprintf("failed to delete service account '%s'",
						testServiceAccount.ServiceAccountId()))
			})
			// TODO fix error message
			It("detects the misconfiguration", verifyWifConfig("verification failed"))
			It("repairs the issue through the update command", repairWifConfig)
			AfterAll(func() {
				_, err := clients.gcp.GetServiceAccountv2(
					ctx,
					testServiceAccount.ServiceAccountId(),
					wifConfig.Gcp().ProjectId(),
				)
				if err != nil {
					Expect(err.Error()).To(Equal("service account not found"),
						"service account error error should be 'not found'")
					err = clients.gcp.UndeleteServiceAccount(
						ctx,
						serviceAccountUniqueId,
						wifConfig.Gcp().ProjectId(),
					)
					Expect(err).ToNot(HaveOccurred(),
						fmt.Sprintf("failed to undelete service account '%s'",
							testServiceAccount.ServiceAccountId()))
				}

			})
		})
		Context("missing deployer service account bindings", Ordered, func() {
			var testServiceAccount *cmv1.WifServiceAccount
			BeforeAll(func() {
				testServiceAccount = findDeployerServiceAccount(wifConfig)
				Expect(testServiceAccount).ToNot(BeNil(), "failed to find deployer service account")

				err := clients.gcp.DetachImpersonator(
					ctx,
					testServiceAccount.ServiceAccountId(),
					wifConfig.Gcp().ProjectId(),
					wifConfig.Gcp().ImpersonatorEmail(),
				)
				Expect(err).ToNot(HaveOccurred(),
					"failed to detach impersonator from delpoyer service account")
			})
			It("detects the misconfiguration", verifyWifConfig("missing binding"))
			It("repairs the issue through the update command", repairWifConfig)
			AfterAll(func() {
				err := clients.gcp.AttachImpersonator(
					ctx,
					testServiceAccount.ServiceAccountId(),
					wifConfig.Gcp().ProjectId(),
					wifConfig.Gcp().ImpersonatorEmail(),
				)
				Expect(err).ToNot(HaveOccurred(),
					"failed to attach impersonator from delpoyer service account")
			})
		})
		Context("missing federated service account bindings", Ordered, func() {
			var testServiceAccount *cmv1.WifServiceAccount
			BeforeAll(func() {
				testServiceAccount = findFederatedServiceAccount(wifConfig)
				Expect(testServiceAccount).ToNot(BeNil(), "failed to find federated service account")

				err := clients.gcp.DetachWorkloadIdentityPool(
					ctx,
					testServiceAccount,
					wifConfig.Gcp().WorkloadIdentityPool().PoolId(),
					wifConfig.Gcp().ProjectId(),
				)
				Expect(err).ToNot(HaveOccurred(),
					"failed to detach federated service account '%s' from workload identity pool '%s'",
					testServiceAccount.ServiceAccountId(), wifConfig.Gcp().WorkloadIdentityPool().PoolId(),
				)
			})
			It("detects the misconfiguration", verifyWifConfig("missing binding"))
			It("repairs the issue through the update command", repairWifConfig)
			AfterAll(func() {
				err := clients.gcp.AttachWorkloadIdentityPool(
					ctx,
					testServiceAccount,
					wifConfig.Gcp().WorkloadIdentityPool().PoolId(),
					wifConfig.Gcp().ProjectId(),
				)
				Expect(err).ToNot(HaveOccurred(),
					"failed to detach federated service account '%s' from workload identity pool '%s'",
					testServiceAccount.ServiceAccountId(), wifConfig.Gcp().WorkloadIdentityPool().PoolId(),
				)
			})
		})
		Context("missing roles on service accounts", Ordered, func() {
			var testServiceAccount *cmv1.WifServiceAccount
			var testRole *cmv1.WifRole
			BeforeAll(func() {
				testServiceAccount = findFederatedServiceAccount(wifConfig)
				testRole = chooseRandomRole(testServiceAccount.Roles())

				err := clients.gcp.RemoveRoleFromServiceAccount(
					ctx,
					testServiceAccount.ServiceAccountId(),
					testRole.RoleId(),
					wifConfig.Gcp().ProjectId(),
				)
				Expect(err).ToNot(HaveOccurred(),
					fmt.Sprintf("failed to remove role '%s' from service account '%s'",
						testRole.RoleId(), testServiceAccount.ServiceAccountId()))
			})
			It("detects the misconfiguration", verifyWifConfig("missing role"))
			It("repairs the issue through the update command", repairWifConfig)
			AfterAll(func() {
				err := clients.gcp.AddRoleToServiceAccount(
					ctx,
					testServiceAccount.ServiceAccountId(),
					testRole.RoleId(),
					wifConfig.Gcp().ProjectId(),
				)
				Expect(err).ToNot(HaveOccurred(),
					fmt.Sprintf("failed to re-add role '%s' to service account '%s'",
						testRole.RoleId(), testServiceAccount.ServiceAccountId()))
			})
		})
	})
})

func chooseRandomServiceAccount(wifConfig *cmv1.WifConfig) *cmv1.WifServiceAccount {
	rand.New(rand.NewSource(GinkgoRandomSeed()))

	serviceAccounts := wifConfig.Gcp().ServiceAccounts()
	randIdx := rand.Intn(len(serviceAccounts))

	return serviceAccounts[randIdx]
}

func chooseRandomRole(roles []*cmv1.WifRole) *cmv1.WifRole {
	rand.New(rand.NewSource(GinkgoRandomSeed()))

	randIdx := rand.Intn(len(roles))

	return roles[randIdx]
}

func findFederatedServiceAccount(wifConfig *cmv1.WifConfig) *cmv1.WifServiceAccount {
	for _, sa := range wifConfig.Gcp().ServiceAccounts() {
		if sa.AccessMethod() == "wif" {
			return sa
		}
	}
	return nil
}

func findDeployerServiceAccount(wifConfig *cmv1.WifConfig) *cmv1.WifServiceAccount {
	for _, sa := range wifConfig.Gcp().ServiceAccounts() {
		if sa.OsdRole() == "deployer" {
			return sa
		}
	}
	return nil
}
