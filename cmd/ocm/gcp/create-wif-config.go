package gcp

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/googleapis/gax-go/v2/apierror"

	"cloud.google.com/go/iam/admin/apiv1/adminpb"
	"github.com/openshift-online/ocm-cli/cmd/ocm/gcp/models"
	"github.com/openshift-online/ocm-cli/pkg/gcp"
	"github.com/pkg/errors"

	"github.com/spf13/cobra"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iam/v1"
	iamv1 "google.golang.org/api/iam/v1"
	"google.golang.org/grpc/codes"
)

var (
	// CreateWorkloadIdentityPoolOpts captures the options that affect creation of the workload identity pool
	CreateWorkloadIdentityConfigurationOpts = options{
		Name:      "",
		Project:   "",
		TargetDir: "",
	}

	impersonatorServiceAccount = "projects/sda-ccs-3/serviceAccounts/osd-impersonator@sda-ccs-3.iam.gserviceaccount.com"
)

const (
	poolDescription = "Created by prototype CLI"

	openShiftAudience = "openshift"
)

// NewCreateWorkloadIdentityConfiguration provides the "create-wif-config" subcommand
func NewCreateWorkloadIdentityConfiguration() *cobra.Command {
	createWorkloadIdentityPoolCmd := &cobra.Command{
		Use:              "create-wif-config",
		Short:            "Create workload identity configuration",
		Run:              createWorkloadIdentityConfigurationCmd,
		PersistentPreRun: validationForCreateWorkloadIdentityConfigurationCmd,
	}

	createWorkloadIdentityPoolCmd.PersistentFlags().StringVar(&CreateWorkloadIdentityConfigurationOpts.Name, "name", "", "User-defined name for all created Google cloud resources (can be separate from the cluster's infra-id)")
	createWorkloadIdentityPoolCmd.MarkPersistentFlagRequired("name")
	createWorkloadIdentityPoolCmd.PersistentFlags().StringVar(&CreateWorkloadIdentityConfigurationOpts.Project, "project", "", "ID of the Google cloud project")
	createWorkloadIdentityPoolCmd.MarkPersistentFlagRequired("project")
	createWorkloadIdentityPoolCmd.PersistentFlags().BoolVar(&CreateWorkloadIdentityConfigurationOpts.DryRun, "dry-run", false, "Skip creating objects, and just save what would have been created into files")
	createWorkloadIdentityPoolCmd.PersistentFlags().StringVar(&CreateWorkloadIdentityConfigurationOpts.TargetDir, "output-dir", "", "Directory to place generated files (defaults to current directory)")

	return createWorkloadIdentityPoolCmd
}

func createWorkloadIdentityConfigurationCmd(cmd *cobra.Command, argv []string) {
	ctx := context.Background()

	gcpClient, err := gcp.NewGcpClient(ctx)
	if err != nil {
		log.Fatalf("failed to initiate GCP client: %v", err)
	}

	log.Println("Creating workload identity configuration...")
	wifConfig, err := createWorkloadIdentityConfiguration(models.WifConfigInput{
		DisplayName: CreateWorkloadIdentityConfigurationOpts.Name,
		ProjectId:   CreateWorkloadIdentityConfigurationOpts.Project,
	})
	if err != nil {
		log.Fatalf("failed to create WIF config: %v", err)
	}

	poolSpec := gcp.WorkloadIdentityPoolSpec{
		PoolName:               wifConfig.Status.WorkloadIdentityPoolData.PoolId,
		ProjectId:              wifConfig.Status.WorkloadIdentityPoolData.ProjectId,
		Jwks:                   wifConfig.Status.WorkloadIdentityPoolData.Jwks,
		IssuerUrl:              wifConfig.Status.WorkloadIdentityPoolData.IssuerUrl,
		PoolIdentityProviderId: wifConfig.Status.WorkloadIdentityPoolData.IdentityProviderId,
	}
	if err = createWorkloadIdentityPool(ctx, gcpClient, poolSpec, CreateWorkloadIdentityConfigurationOpts.DryRun); err != nil {
		log.Fatalf("Failed to create workload identity pool: %s", err)
	}

	if err = createWorkloadIdentityProvider(ctx, gcpClient, poolSpec, CreateWorkloadIdentityConfigurationOpts.DryRun); err != nil {
		log.Fatalf("Failed to create workload identity provider: %s", err)
	}

	if err = createServiceAccounts(ctx, gcpClient, wifConfig, CreateWorkloadIdentityConfigurationOpts.DryRun); err != nil {
		log.Fatalf("Failed to create IAM service accounts: %s", err)
	}

}

func validationForCreateWorkloadIdentityConfigurationCmd(cmd *cobra.Command, argv []string) {
	if CreateWorkloadIdentityConfigurationOpts.Name == "" {
		panic("Name is required")
	}
	if CreateWorkloadIdentityConfigurationOpts.Project == "" {
		panic("Project is required")
	}

	if CreateWorkloadIdentityConfigurationOpts.TargetDir == "" {
		pwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("Failed to get current directory: %s", err)
		}

		CreateWorkloadIdentityConfigurationOpts.TargetDir = pwd
	}

	fPath, err := filepath.Abs(CreateWorkloadIdentityConfigurationOpts.TargetDir)
	if err != nil {
		log.Fatalf("Failed to resolve full path: %s", err)
	}

	sResult, err := os.Stat(fPath)
	if os.IsNotExist(err) {
		log.Fatalf("Directory %s does not exist", fPath)
	}
	if !sResult.IsDir() {
		log.Fatalf("file %s exists and is not a directory", fPath)
	}

}

func createWorkloadIdentityConfiguration(input models.WifConfigInput) (*models.WifConfigOutput, error) {
	// TODO: Implement the actual creation of the workload identity configuration
	return mockWifConfig(), nil
}

func createWorkloadIdentityPool(ctx context.Context, client gcp.GcpClient, spec gcp.WorkloadIdentityPoolSpec, generateOnly bool) error {
	name := spec.PoolName
	project := spec.ProjectId
	if generateOnly {
		log.Printf("Would have created workload identity pool %s", name)
	} else {
		parentResourceForPool := fmt.Sprintf("projects/%s/locations/global", project)
		poolResource := fmt.Sprintf("%s/workloadIdentityPools/%s", parentResourceForPool, name)
		resp, err := client.GetWorkloadIdentityPool(ctx, poolResource)
		if resp != nil && resp.State == "DELETED" {
			log.Printf("Workload identity pool %s was deleted", name)
			_, err := client.UndeleteWorkloadIdentityPool(ctx, poolResource, &iamv1.UndeleteWorkloadIdentityPoolRequest{})
			if err != nil {
				return errors.Wrapf(err, "failed to undelete workload identity pool %s", name)
			}
		} else if err != nil {
			if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 404 && strings.Contains(gerr.Message, "Requested entity was not found") {
				pool := &iamv1.WorkloadIdentityPool{
					Name:        name,
					DisplayName: name,
					Description: poolDescription,
					State:       "ACTIVE",
					Disabled:    false,
				}

				_, err := client.CreateWorkloadIdentityPool(ctx, parentResourceForPool, name, pool)
				if err != nil {
					return errors.Wrapf(err, "failed to create workload identity pool %s", name)
				}
				log.Printf("Workload identity pool created with name %s", name)
			} else {
				return errors.Wrapf(err, "failed to check if there is existing workload identity pool %s", name)
			}
		} else {
			log.Printf("Workload identity pool %s already exists", name)
		}
	}

	return nil
}

func createWorkloadIdentityProvider(ctx context.Context, client gcp.GcpClient, spec gcp.WorkloadIdentityPoolSpec, generateOnly bool) error {
	if generateOnly {
		log.Printf("Would have created workload identity provider for %s with issuerURL %s", spec.PoolName, spec.IssuerUrl)
	} else {
		providerResource := fmt.Sprintf("projects/%s/locations/global/workloadIdentityPools/%s/providers/%s", spec.ProjectId, spec.PoolName, spec.PoolName)
		_, err := client.GetWorkloadIdentityProvider(ctx, providerResource)
		if err != nil {
			if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 404 && strings.Contains(gerr.Message, "Requested entity was not found") {
				provider := &iam.WorkloadIdentityPoolProvider{
					Name:        spec.PoolName,
					DisplayName: spec.PoolName,
					Description: poolDescription,
					State:       "ACTIVE",
					Disabled:    false,
					Oidc: &iam.Oidc{
						AllowedAudiences: []string{openShiftAudience},
						IssuerUri:        spec.IssuerUrl,
						JwksJson:         spec.Jwks,
					},
					AttributeMapping: map[string]string{
						// when token exchange happens, sub from oidc token shared by operator pod will be mapped to google.subject
						// field of google auth token. The field is used to allow fine-grained access to gcp service accounts.
						// The format is `system:serviceaccount:<service_account_namespace>:<service_account_name>`
						"google.subject": "assertion.sub",
					},
				}

				_, err := client.CreateWorkloadIdentityProvider(ctx, fmt.Sprintf("projects/%s/locations/global/workloadIdentityPools/%s", spec.ProjectId, spec.PoolName), spec.PoolName, provider)
				if err != nil {
					return errors.Wrapf(err, "failed to create workload identity provider %s", spec.PoolName)
				}
				log.Printf("workload identity provider created with name %s", spec.PoolName)
			} else {
				return errors.Wrapf(err, "failed to check if there is existing workload identity provider %s in pool %s", spec.PoolName, spec.PoolName)
			}
		} else {
			log.Printf("Workload identity provider %s already exists in pool %s", spec.PoolName, spec.PoolName)
		}

	}
	return nil
}

func createServiceAccounts(ctx context.Context, gcpClient gcp.GcpClient, wifOutput *models.WifConfigOutput, generateOnly bool) error {
	projectId := wifOutput.Spec.ProjectId
	fmtRoleResourceId := func(role models.Role) string {
		return fmt.Sprintf("roles/%s", role.Id)
	}
	if generateOnly {
		for _, serviceAccount := range wifOutput.Status.ServiceAccounts {
			serviceAccountID := serviceAccount.GetId()
			log.Printf("Would have created service account %s", serviceAccountID)
			log.Printf("Would have bound roles to %s", serviceAccountID)
			switch serviceAccount.AccessMethod {
			case "impersonate":
				log.Printf("Would have attached impersonator %s to %s", impersonatorServiceAccount, serviceAccountID)
			case "wif":
				log.Printf("Would have attached workload identity pool %s to %s", wifOutput.Status.WorkloadIdentityPoolData.IdentityProviderId, serviceAccountID)
			default:
				fmt.Printf("Warning: %s is not a supported access type\n", serviceAccount.AccessMethod)
			}
		}
		return nil
	}

	// Create service accounts
	for _, serviceAccount := range wifOutput.Status.ServiceAccounts {
		serviceAccountID := serviceAccount.GetId()
		serviceAccountName := wifOutput.Spec.DisplayName + "-" + serviceAccountID
		serviceAccountDesc := poolDescription + " for WIF config " + wifOutput.Spec.DisplayName

		fmt.Println("Creating service account", serviceAccountID)
		_, err := CreateServiceAccount(gcpClient, serviceAccountID, serviceAccountName, serviceAccountDesc, projectId, true)
		if err != nil {
			return errors.Wrap(err, "Failed to create IAM service account")
		}
		log.Printf("IAM service account %s created", serviceAccountID)
	}

	// Bind roles and grant access
	for _, serviceAccount := range wifOutput.Status.ServiceAccounts {
		serviceAccountID := serviceAccount.GetId()

		fmt.Printf("\t\tBinding roles to %s\n", serviceAccount.Id)
		for _, role := range serviceAccount.Roles {
			if !role.Predefined {
				fmt.Printf("Skipping role %q for service account %q as custom roles are not yet supported.", role.Id, serviceAccount.Id)
				continue
			}
			err := gcpClient.BindRole(serviceAccountID, projectId, fmtRoleResourceId(role))
			if err != nil {
				panic(err)
			}
		}
		fmt.Printf("\t\tRoles bound to %s\n", serviceAccount.Id)

		fmt.Printf("\t\tGranting access to %s...\n", serviceAccount.Id)
		switch serviceAccount.AccessMethod {
		case "impersonate":
			if err := gcpClient.AttachImpersonator(serviceAccount.Id, projectId, impersonatorServiceAccount); err != nil {
				panic(err)
			}
		case "wif":
			if err := gcpClient.AttachWorkloadIdentityPool(serviceAccount, wifOutput.Status.WorkloadIdentityPoolData.PoolId, projectId); err != nil {
				panic(err)
			}
		default:
			fmt.Printf("Warning: %s is not a supported access type\n", serviceAccount.AccessMethod)
		}
		fmt.Printf("\t\tAccess granted to %s\n", serviceAccount.Id)
	}

	return nil
}

func CreateServiceAccount(gcpClient gcp.GcpClient, svcAcctID, svcAcctName, svcAcctDescription, projectName string, allowExisting bool) (*adminpb.ServiceAccount, error) {
	request := &adminpb.CreateServiceAccountRequest{
		Name:      fmt.Sprintf("projects/%s", projectName),
		AccountId: svcAcctID,
		ServiceAccount: &adminpb.ServiceAccount{
			DisplayName: svcAcctName,
			Description: svcAcctDescription,
		},
	}
	svcAcct, err := gcpClient.CreateServiceAccount(context.TODO(), request)
	if err != nil {
		pApiError, ok := err.(*apierror.APIError)
		if ok {
			if pApiError.GRPCStatus().Code() == codes.AlreadyExists && allowExisting {
				return svcAcct, nil
			}
		}
	}
	return svcAcct, err
}

func generateServiceAccountID(serviceAccount models.ServiceAccount) string {
	serviceAccountID := "z-" + serviceAccount.Id
	if len(serviceAccountID) > 30 {
		serviceAccountID = serviceAccountID[:30]
	}
	return serviceAccountID
}

func mockWifConfig() *models.WifConfigOutput {
	return &models.WifConfigOutput{
		Metadata: &models.WifConfigMetadata{
			DisplayName:  "test01",
			Id:           "0001",
			Organization: &models.WifConfigMetadataOrganization{},
		},
		Spec: &models.WifConfigInput{
			DisplayName: "test01",
			ProjectId:   "sda-ccs-1",
		},
		Status: &models.WifConfigStatus{
			State:    "",
			Summary:  "",
			TimeData: models.WifTimeData{},
			WorkloadIdentityPoolData: models.WifWorkloadIdentityPoolData{
				IdentityProviderId: "oidc",
				IssuerUrl:          "https://fake-issuer.com",
				Jwks:               "{\"keys\":[{\"use\":\"sig\",\"kty\":\"RSA\",\"kid\":\"hZ8yVW_SaRg3zSLz3lZ5D65rHNageE7agstatYTVesA\",\"alg\":\"RS256\",\"n\":\"rKN70G7r_O2untSKxpO2uQQatadzk3d5_0hs_FQGCa_xx0pBRoBMUvdbS4A8quZEoJOk8jmwSGWKvlLk-Is_Rsk0lq3hfKuElYFq43KOHe5YZfmZV7lqCdDr_lIJS2gzjfe_8xTfOUpU_DC0Slrj7UvwpcmpFMtklgeN_LLwuRRD6nKQRx0n0ABBXs44D2s4THrSABvpuOQzBpx9Qx8ShMPdN5h_aePcQj9Ss-lOAqdUprGLU-O-Cm2STUCtHFLh8oB5dYZA2Ww-6RkX7LIj6cVbFwMMZAwLn4ObKE_r6s8yynfS6wvqOldk-pkC0wtQx7R8NoP3ff9RF3kzKrkegdXAkVewUYu6V2Vcl1Z07MmyYcUhcjxT9jOblWklHSYIgyi_n-p30dzL7avU8sSdGnhFrLh8p9d3A1o_g1JNnSTquIHIpyE5WKZyCTF-c2K6VOSRWz16RgI5pswW6-IquJ9g-1RrAsTtuInueYJJw4S32OKRcsanClBKtCg95g1ylZL8UfeQS9S5Q7VNxCUdW7pgtIZiLcPIP8-Ier0yJF5TjnnVwcWs0Rq4-zWV-JyjoqUh6zjR9tqgXdSl1IBl6MqiHGNlW9SF0kRmgtAzm_fwlvqCaEzMW-LEivTUBgwHAvGYftciHFalqbblUNYgJyAy2hZ4-17JJAII8Yk5mZs\",\"e\":\"AQAB\"}]}",
				PoolId:             "d7a2fb1099634da1a9d0a5e8d1c11c39",
				ProjectId:          "sda-ccs-1",
				ProjectNumber:      1008983090557,
			},
			ServiceAccounts: []models.ServiceAccount{
				{
					Kind:         "ServiceAccount",
					Id:           "osd-deployer",
					OsdRole:      "deployer",
					AccessMethod: "impersonate",
					Roles: []models.Role{
						{Kind: "Role", Id: "compute.admin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "dns.admin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.roleAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.securityAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.serviceAccountAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.ServiceAccountKeyAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.ServiceAccountUser", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "storage.admin", Predefined: true, Permissions: []string{}},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-support",
					OsdRole:      "support",
					AccessMethod: "impersonate",
					Roles: []models.Role{
						{Kind: "Role", Id: "compute.admin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "dns.admin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.roleAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.securityAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.serviceAccountAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.ServiceAccountKeyAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.ServiceAccountUser", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "storage.admin", Predefined: true, Permissions: []string{}},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-image-registry",
					OsdRole:      "operator-image-registry",
					AccessMethod: "wif",
					Roles: []models.Role{
						{Kind: "Role", Id: "resourcemanager.tagUser", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "storage.admin", Predefined: true, Permissions: []string{}},
					},
					CredentialRequest: models.CredentialRequest{
						SecretRef: models.SecretRef{
							Name:      "installer-cloud-credentials",
							Namespace: "openshift-image-registry",
						},
						ServiceAccountNames: []string{
							"cluster-image-registry-operator",
							"registry",
						},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-cluster-ingress",
					OsdRole:      "operator-cluster-ingress",
					AccessMethod: "wif",
					Roles: []models.Role{
						{Kind: "Role", Id: "dns.admin", Predefined: true, Permissions: []string{}},
					},
					CredentialRequest: models.CredentialRequest{
						SecretRef: models.SecretRef{
							Name:      "cloud-credentials",
							Namespace: "openshift-ingress-operator",
						},
						ServiceAccountNames: []string{
							"ingress-operator",
						},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-machine-api",
					OsdRole:      "operator-machine-api",
					AccessMethod: "wif",
					Roles: []models.Role{
						{Kind: "Role", Id: "compute.admin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.serviceAccountUser", Predefined: true, Permissions: []string{}},
					},
					CredentialRequest: models.CredentialRequest{
						SecretRef: models.SecretRef{
							Name:      "gcp-cloud-credentials",
							Namespace: "openshift-machine-api",
						},
						ServiceAccountNames: []string{
							"machine-api-controllers",
						},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-cloud-controller-manager",
					OsdRole:      "operator-cloud-controller-manager",
					AccessMethod: "wif",
					Roles: []models.Role{
						{Kind: "Role", Id: "compute.instanceAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "compute.loadBalancerAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.serviceAccountUser", Predefined: true, Permissions: []string{}},
					},
					CredentialRequest: models.CredentialRequest{
						SecretRef: models.SecretRef{
							Name:      "gcp-ccm-cloud-credentials",
							Namespace: "openshift-cloud-controller-manager",
						},
						ServiceAccountNames: []string{
							"cloud-controller-manager",
						},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-cluster-storage",
					OsdRole:      "operator-cluster-storage",
					AccessMethod: "wif",
					Roles: []models.Role{
						{Kind: "Role", Id: "compute.instanceAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "compute.storageAdmin", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.serviceAccountUser", Predefined: true, Permissions: []string{}},
					},
					CredentialRequest: models.CredentialRequest{
						SecretRef: models.SecretRef{
							Name:      "gcp-pd-cloud-credentials",
							Namespace: "openshift-cluster-csi-drivers",
						},
						ServiceAccountNames: []string{
							"gcp-pd-csi-driver-operator",
							"gcp-pd-csi-driver-controller-sa",
						},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-cloud-credential",
					OsdRole:      "operator-cloud-credential",
					AccessMethod: "wif",
					Roles: []models.Role{
						{Kind: "Role", Id: "iam.roleViewer", Predefined: true, Permissions: []string{}},
						{Kind: "Role", Id: "iam.securityReviewer", Predefined: true, Permissions: []string{}},
					},
					CredentialRequest: models.CredentialRequest{
						SecretRef: models.SecretRef{
							Name:      "cloud-credential-operator-gcp-ro-creds",
							Namespace: "openshift-cloud-credential-operator",
						},
						ServiceAccountNames: []string{
							"cloud-credential-operator",
						},
					},
				},
				{
					Kind:         "ServiceAccount",
					Id:           "osd-cncc",
					OsdRole:      "operator-cncc",
					AccessMethod: "wif",
					Roles: []models.Role{
						{Kind: "Role", Id: "compute.admin", Predefined: true, Permissions: []string{}},
					},
					CredentialRequest: models.CredentialRequest{
						SecretRef: models.SecretRef{
							Name:      "cloud-credentials",
							Namespace: "openshift-cloud-network-config-controller",
						},
						ServiceAccountNames: []string{
							"cloud-network-config-controller",
						},
					},
				},
			},
		},
	}
}
