package gcp

import (
	"fmt"

	"github.com/googleapis/gax-go/v2/apierror"
	googleapi "google.golang.org/api/googleapi"
	"google.golang.org/grpc/codes"
)

func (c *gcpClient) handleAttachImpersonatorError(err error) error {
	pApiError, ok := err.(*apierror.APIError)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	return fmt.Errorf(pApiError.Details().String())
}

func (c *gcpClient) handleAttachWorkloadIdentityPoolError(err error) error {
	pApiError, ok := err.(*apierror.APIError)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	fmt.Println(pApiError.Error())
	return fmt.Errorf(pApiError.Error())
}

func (c *gcpClient) handleDeleteServiceAccountError(err error, allowMissing bool) error {
	pApiError, ok := err.(*apierror.APIError)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	if pApiError.GRPCStatus().Code() == codes.NotFound && allowMissing {
		return nil
	}
	return fmt.Errorf(pApiError.Details().String())
}

func (c *gcpClient) handleDisableServiceAccountError(err error) error {
	gError, ok := err.(*googleapi.Error)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	return fmt.Errorf(gError.Error())
}

func (c *gcpClient) handleDetachImpersonatorError(err error) error {
	pApiError, ok := err.(*apierror.APIError)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	return fmt.Errorf(pApiError.Details().String())
}

func (c *gcpClient) handleDetachWorkloadIdentityPoolError(err error) error {
	pApiError, ok := err.(*apierror.APIError)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	fmt.Println(pApiError.Error())
	return fmt.Errorf(pApiError.Error())
}

func (c *gcpClient) handleGetServiceAccountError(err error) error {
	pApiError, ok := err.(*apierror.APIError)
	if !ok {
		return fmt.Errorf("Unexpected error: %v", err)
	}
	switch pApiError.GRPCStatus().Code() {
	case codes.NotFound:
		return fmt.Errorf("service account not found")
	default:
		return fmt.Errorf(pApiError.Error())
	}
}

func (c *gcpClient) handleEnableServiceAccountError(err error) error {
	gError, ok := err.(*googleapi.Error)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	return fmt.Errorf(gError.Error())
}

func (c *gcpClient) handleUndeleteServiceAccountError(err error) error {
	gError, ok := err.(*googleapi.Error)
	if !ok {
		return fmt.Errorf("Unexpected error")
	}
	return fmt.Errorf(gError.Error())
}
