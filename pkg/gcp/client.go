package gcp

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/iam"
	iamadmin "cloud.google.com/go/iam/admin/apiv1"
	"cloud.google.com/go/iam/admin/apiv1/adminpb"
	"cloud.google.com/go/iam/apiv1/iampb"
	"cloud.google.com/go/storage"
	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	cloudresourcemanager "google.golang.org/api/cloudresourcemanager/v1"

	iamv1 "google.golang.org/api/iam/v1"
	secretmanager "google.golang.org/api/secretmanager/v1"
)

type GcpClient interface {
	AttachImpersonator(ctx context.Context, saId, projectId, impersonatorResourceId string) error
	AttachWorkloadIdentityPool(ctx context.Context, sa *cmv1.WifServiceAccount, poolId, projectId string) error
	AddRoleToServiceAccount(ctx context.Context, serviceAccountId string, roleId string, projectId string) error
	CreateRole(context.Context, *adminpb.CreateRoleRequest) (*adminpb.Role, error)
	CreateServiceAccount(ctx context.Context, request *adminpb.CreateServiceAccountRequest) (*adminpb.ServiceAccount, error)                               //nolint:lll
	CreateWorkloadIdentityPool(ctx context.Context, parent, poolID string, pool *iamv1.WorkloadIdentityPool) (*iamv1.Operation, error)                     //nolint:lll
	CreateWorkloadIdentityProvider(ctx context.Context, parent, providerID string, provider *iamv1.WorkloadIdentityPoolProvider) (*iamv1.Operation, error) //nolint:lll
	DeleteServiceAccount(ctx context.Context, saName string, project string, allowMissing bool) error
	DeleteWorkloadIdentityPool(ctx context.Context, resource string) (*iamv1.Operation, error) //nolint:lll
	DetachImpersonator(ctx context.Context, saId, projectId, impersonatorResourceId string) error
	DetachWorkloadIdentityPool(ctx context.Context, sa *cmv1.WifServiceAccount, poolId, projectId string) error
	DisableServiceAccount(ctx context.Context, serviceAccountId string, projectId string) error
	EnableServiceAccount(ctx context.Context, serviceAccountId string, projectId string) error
	GetProjectIamPolicy(ctx context.Context, projectName string, request *cloudresourcemanager.GetIamPolicyRequest) (*cloudresourcemanager.Policy, error) //nolint:lll
	GetRole(context.Context, *adminpb.GetRoleRequest) (*adminpb.Role, error)
	GetServiceAccount(ctx context.Context, request *adminpb.GetServiceAccountRequest) (*adminpb.ServiceAccount, error)
	GetServiceAccountv2(ctx context.Context, serviceAccountId string, projectId string) (ServiceAccount, error)
	GetWorkloadIdentityPool(ctx context.Context, resource string) (*iamv1.WorkloadIdentityPool, error)             //nolint:lll
	GetWorkloadIdentityProvider(ctx context.Context, resource string) (*iamv1.WorkloadIdentityPoolProvider, error) //nolint:lll
	ProjectNumberFromId(ctx context.Context, projectId string) (int64, error)
	RemoveRoleFromServiceAccount(ctx context.Context, serviceAccountId string, roleId string, projectId string) error
	SetProjectIamPolicy(ctx context.Context, svcAcctResource string, request *cloudresourcemanager.SetIamPolicyRequest) (*cloudresourcemanager.Policy, error) //nolint:lll
	UndeleteRole(context.Context, *adminpb.UndeleteRoleRequest) (*adminpb.Role, error)
	UndeleteServiceAccount(ctx context.Context, serviceAccountId string, projectId string) error
	UndeleteWorkloadIdentityPool(ctx context.Context, resource string, request *iamv1.UndeleteWorkloadIdentityPoolRequest) (*iamv1.Operation, error) //nolint:lll
	UpdateRole(context.Context, *adminpb.UpdateRoleRequest) (*adminpb.Role, error)
}

type gcpClient struct {
	ctx                  context.Context
	iamClient            *iamadmin.IamClient
	oldIamClient         *iamv1.Service
	cloudResourceManager *cloudresourcemanager.Service
	secretManager        *secretmanager.Service
	storageClient        *storage.Client
}

func NewGcpClient(ctx context.Context) (GcpClient, error) {
	iamClient, err := iamadmin.NewIamClient(ctx)
	if err != nil {
		return nil, err
	}
	cloudResourceManager, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		return nil, err
	}
	secretManager, err := secretmanager.NewService(ctx)
	if err != nil {
		return nil, err
	}
	// The new iam client doesn't support workload identity federation operations.
	oldIamClient, err := iamv1.NewService(ctx)
	if err != nil {
		return nil, err
	}

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, err
	}

	return &gcpClient{
		ctx:                  ctx,
		iamClient:            iamClient,
		cloudResourceManager: cloudResourceManager,
		secretManager:        secretManager,
		oldIamClient:         oldIamClient,
		storageClient:        storageClient,
	}, nil
}

func (c *gcpClient) AttachImpersonator(ctx context.Context, saId, projectId string, impersonatorEmail string) error {
	saResourceId := fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com",
		projectId, saId, projectId)
	policy, err := c.iamClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: saResourceId,
	})
	if err != nil {
		return c.handleAttachImpersonatorError(err)
	}
	policy.Add(
		fmt.Sprintf("serviceAccount:%s", impersonatorEmail),
		iam.RoleName("roles/iam.serviceAccountTokenCreator"))
	_, err = c.iamClient.SetIamPolicy(ctx, &iamadmin.SetIamPolicyRequest{
		Resource: saResourceId,
		Policy:   policy,
	})
	if err != nil {
		return c.handleAttachImpersonatorError(err)
	}
	return nil
}

func (c *gcpClient) AttachWorkloadIdentityPool(
	ctx context.Context,
	sa *cmv1.WifServiceAccount,
	poolId string,
	projectId string,
) error {
	saResourceId := c.fmtSaResourceId(sa.ServiceAccountId(), projectId)

	projectNum, err := c.ProjectNumberFromId(ctx, projectId)
	if err != nil {
		return c.handleAttachWorkloadIdentityPoolError(err)
	}

	policy, err := c.iamClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: saResourceId,
	})
	if err != nil {
		return c.handleAttachWorkloadIdentityPoolError(err)
	}
	for _, openshiftServiceAccount := range sa.CredentialRequest().ServiceAccountNames() {
		policy.Add(
			//nolint:lll
			fmt.Sprintf(
				"principal://iam.googleapis.com/projects/%d/locations/global/workloadIdentityPools/%s/subject/system:serviceaccount:%s:%s",
				projectNum, poolId, sa.CredentialRequest().SecretRef().Namespace(), openshiftServiceAccount,
			),
			iam.RoleName("roles/iam.workloadIdentityUser"))
	}
	_, err = c.iamClient.SetIamPolicy(ctx, &iamadmin.SetIamPolicyRequest{
		Resource: saResourceId,
		Policy:   policy,
	})
	if err != nil {
		return c.handleAttachWorkloadIdentityPoolError(err)
	}
	return nil
}

func (c *gcpClient) AddRoleToServiceAccount(
	ctx context.Context,
	serviceAccountId string,
	roleId string,
	projectId string,
) error {
	policy, err := c.GetProjectIamPolicy(ctx, projectId, &cloudresourcemanager.GetIamPolicyRequest{})
	if err != nil {
		return err
	}
	for _, binding := range policy.Bindings {
		// The role name attached to the binding has a prefix based on whether it is a custom role.
		// A custom role will include the project it is define in within its path.
		// A predefined role will not.
		// The roleId passed in does not include this path, so the method looks for the basename that matches.
		roleParts := strings.Split(binding.Role, "/")
		roleBasename := roleParts[len(roleParts)-1]
		if roleBasename == roleId {
			// Iam policy syntax lists service account members with a type prefix.
			policyServiceAccountId := fmt.Sprintf(
				"serviceAccount:%s@%s.iam.gserviceaccount.com",
				serviceAccountId, projectId)
			binding.Members = append(binding.Members, policyServiceAccountId)
			_, err = c.SetProjectIamPolicy(ctx, projectId, &cloudresourcemanager.SetIamPolicyRequest{
				Policy: policy,
			})
			return err
		}
	}
	return nil
}

func (c *gcpClient) CreateRole(ctx context.Context, request *adminpb.CreateRoleRequest) (*adminpb.Role, error) {
	return c.iamClient.CreateRole(ctx, request)
}

func (c *gcpClient) CreateServiceAccount(
	ctx context.Context,
	request *adminpb.CreateServiceAccountRequest,
) (*adminpb.ServiceAccount, error) {
	svcAcct, err := c.iamClient.CreateServiceAccount(ctx, request)
	return svcAcct, err
}

//nolint:lll
func (c *gcpClient) CreateWorkloadIdentityPool(ctx context.Context, parent, poolID string, pool *iamv1.WorkloadIdentityPool) (*iamv1.Operation, error) {
	return c.oldIamClient.Projects.Locations.WorkloadIdentityPools.Create(parent, pool).WorkloadIdentityPoolId(poolID).Context(ctx).Do()
}

//nolint:lll
func (c *gcpClient) CreateWorkloadIdentityProvider(ctx context.Context, parent, providerID string, provider *iamv1.WorkloadIdentityPoolProvider) (*iamv1.Operation, error) {
	return c.oldIamClient.Projects.Locations.WorkloadIdentityPools.Providers.Create(parent, provider).WorkloadIdentityPoolProviderId(providerID).Context(ctx).Do()
}

func (c *gcpClient) DeleteServiceAccount(ctx context.Context, saName string, project string, allowMissing bool) error {
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", project, saName, project)
	err := c.iamClient.DeleteServiceAccount(ctx, &adminpb.DeleteServiceAccountRequest{
		Name: name,
	})
	if err != nil {
		return c.handleDeleteServiceAccountError(err, allowMissing)
	}
	return nil
}

//nolint:lll
func (c *gcpClient) DeleteWorkloadIdentityPool(ctx context.Context, resource string) (*iamv1.Operation, error) {
	return c.oldIamClient.Projects.Locations.WorkloadIdentityPools.Delete(resource).Context(ctx).Do()
}

func (c *gcpClient) DetachImpersonator(
	ctx context.Context,
	saId string,
	projectId string,
	impersonatorEmail string,
) error {
	saResourceId := fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com",
		projectId, saId, projectId)
	policy, err := c.iamClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: saResourceId,
	})
	if err != nil {
		return c.handleDetachImpersonatorError(err)
	}
	policy.Remove(
		fmt.Sprintf("serviceAccount:%s", impersonatorEmail),
		iam.RoleName("roles/iam.serviceAccountTokenCreator"))
	_, err = c.iamClient.SetIamPolicy(ctx, &iamadmin.SetIamPolicyRequest{
		Resource: saResourceId,
		Policy:   policy,
	})
	if err != nil {
		return c.handleDetachImpersonatorError(err)
	}
	return nil
}

func (c *gcpClient) DetachWorkloadIdentityPool(
	ctx context.Context,
	sa *cmv1.WifServiceAccount,
	poolId string,
	projectId string,
) error {
	saResourceId := c.fmtSaResourceId(sa.ServiceAccountId(), projectId)

	projectNum, err := c.ProjectNumberFromId(ctx, projectId)
	if err != nil {
		return c.handleDetachWorkloadIdentityPoolError(err)
	}

	policy, err := c.iamClient.GetIamPolicy(ctx, &iampb.GetIamPolicyRequest{
		Resource: saResourceId,
	})
	if err != nil {
		return c.handleAttachWorkloadIdentityPoolError(err)
	}
	for _, openshiftServiceAccount := range sa.CredentialRequest().ServiceAccountNames() {
		policy.Remove(
			//nolint:lll
			fmt.Sprintf(
				"principal://iam.googleapis.com/projects/%d/locations/global/workloadIdentityPools/%s/subject/system:serviceaccount:%s:%s",
				projectNum, poolId, sa.CredentialRequest().SecretRef().Namespace(), openshiftServiceAccount,
			),
			iam.RoleName("roles/iam.workloadIdentityUser"))
	}
	_, err = c.iamClient.SetIamPolicy(ctx, &iamadmin.SetIamPolicyRequest{
		Resource: saResourceId,
		Policy:   policy,
	})
	if err != nil {
		return c.handleDetachWorkloadIdentityPoolError(err)
	}
	return nil
}

func (c *gcpClient) DisableServiceAccount(
	ctx context.Context,
	serviceAccountId string,
	projectId string,
) error {
	_, err := c.oldIamClient.Projects.ServiceAccounts.Disable(
		c.fmtSaResourceId(serviceAccountId, projectId),
		&iamv1.DisableServiceAccountRequest{},
	).Do()
	if err != nil {
		return c.handleDisableServiceAccountError(err)
	}
	return nil
}

func (c *gcpClient) EnableServiceAccount(
	ctx context.Context,
	serviceAccountId string,
	projectId string,
) error {
	_, err := c.oldIamClient.Projects.ServiceAccounts.Enable(
		c.fmtSaResourceId(serviceAccountId, projectId),
		&iamv1.EnableServiceAccountRequest{},
	).Do()
	if err != nil {
		return c.handleEnableServiceAccountError(err)
	}
	return nil
}

//nolint:lll
func (c *gcpClient) GetProjectIamPolicy(
	ctx context.Context,
	projectName string,
	request *cloudresourcemanager.GetIamPolicyRequest,
) (*cloudresourcemanager.Policy, error) {
	return c.cloudResourceManager.Projects.GetIamPolicy(projectName, request).Context(context.Background()).Do()
}

func (c *gcpClient) GetRole(ctx context.Context, request *adminpb.GetRoleRequest) (*adminpb.Role, error) {
	return c.iamClient.GetRole(ctx, request)
}

func (c *gcpClient) GetServiceAccount(
	ctx context.Context,
	request *adminpb.GetServiceAccountRequest,
) (*adminpb.ServiceAccount, error) {
	return c.iamClient.GetServiceAccount(ctx, request)
}

func (c *gcpClient) GetServiceAccountv2(
	ctx context.Context,
	serviceAccountId string,
	projectId string,
) (ServiceAccount, error) {
	sa, err := c.iamClient.GetServiceAccount(
		ctx,
		&adminpb.GetServiceAccountRequest{
			Name: c.fmtSaResourceId(serviceAccountId, projectId),
		},
	)
	if err != nil {
		return nil, c.handleGetServiceAccountError(err)
	}
	return NewServiceAccount(sa)
}

//nolint:lll
func (c *gcpClient) GetWorkloadIdentityPool(ctx context.Context, resource string) (*iamv1.WorkloadIdentityPool, error) {
	return c.oldIamClient.Projects.Locations.WorkloadIdentityPools.Get(resource).Context(ctx).Do()
}

//nolint:lll
func (c *gcpClient) GetWorkloadIdentityProvider(ctx context.Context, resource string) (*iamv1.WorkloadIdentityPoolProvider, error) {
	return c.oldIamClient.Projects.Locations.WorkloadIdentityPools.Providers.Get(resource).Context(ctx).Do()
}

func (c *gcpClient) ProjectNumberFromId(ctx context.Context, projectId string) (int64, error) {
	project, err := c.cloudResourceManager.Projects.Get(projectId).Do()
	if err != nil {
		return 0, err
	}
	return project.ProjectNumber, nil
}

func (c *gcpClient) RemoveRoleFromServiceAccount(
	ctx context.Context,
	serviceAccountId string,
	roleId string,
	projectId string,
) error {
	policy, err := c.GetProjectIamPolicy(ctx, projectId, &cloudresourcemanager.GetIamPolicyRequest{})
	if err != nil {
		return err
	}
	for i, binding := range policy.Bindings {
		// The role name attached to the binding has a prefix based on whether it is a custom role.
		// A custom role will include the project it is define in within its path.
		// A predefined role will not.
		// The roleId passed in does not include this path, so the method looks for the basename that matches.
		roleParts := strings.Split(binding.Role, "/")
		roleBasename := roleParts[len(roleParts)-1]
		if roleBasename == roleId {
			newMembers := []string{}
			for _, member := range binding.Members {
				// Iam policy syntax lists service account members with a type prefix.
				policyServiceAccountId := fmt.Sprintf(
					"serviceAccount:%s@%s.iam.gserviceaccount.com",
					serviceAccountId, projectId)

				if member != policyServiceAccountId {
					newMembers = append(newMembers, member)
				}
			}
			policy.Bindings[i].Members = newMembers
			_, err = c.SetProjectIamPolicy(ctx, projectId, &cloudresourcemanager.SetIamPolicyRequest{
				Policy: policy,
			})
			return err
		}
	}
	return nil
}

//nolint:lll
func (c *gcpClient) SetProjectIamPolicy(ctx context.Context, svcAcctResource string, request *cloudresourcemanager.SetIamPolicyRequest) (*cloudresourcemanager.Policy, error) {
	return c.cloudResourceManager.Projects.SetIamPolicy(svcAcctResource, request).Context(ctx).Do()
}

func (c *gcpClient) UndeleteRole(ctx context.Context, request *adminpb.UndeleteRoleRequest) (*adminpb.Role, error) {
	return c.iamClient.UndeleteRole(ctx, request)
}

// Note: The serviceAccountId value must be the service account's unique ID,
// not the email, for undelete to work.
func (c *gcpClient) UndeleteServiceAccount(
	ctx context.Context,
	serviceAccountId string,
	projectId string,
) error {
	_, err := c.oldIamClient.Projects.ServiceAccounts.Undelete(
		fmt.Sprintf("projects/%s/serviceAccounts/%s", projectId, serviceAccountId),
		&iamv1.UndeleteServiceAccountRequest{},
	).Do()
	if err != nil {
		return c.handleUndeleteServiceAccountError(err)
	}
	return nil
}

//nolint:lll
func (c *gcpClient) UndeleteWorkloadIdentityPool(ctx context.Context, resource string, request *iamv1.UndeleteWorkloadIdentityPoolRequest) (*iamv1.Operation, error) {
	return c.oldIamClient.Projects.Locations.WorkloadIdentityPools.Undelete(resource, request).Context(ctx).Do()
}

func (c *gcpClient) UpdateRole(ctx context.Context, request *adminpb.UpdateRoleRequest) (*adminpb.Role, error) {
	return c.iamClient.UpdateRole(ctx, request)
}
