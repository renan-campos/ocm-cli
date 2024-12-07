// This file defines a package representation of Google's ServiceAccount type
package gcp

import (
	"fmt"

	"cloud.google.com/go/iam/admin/apiv1/adminpb"
)

type ServiceAccount interface {
	Disabled() bool
	Name() string
	UniqueId() string
}

type serviceAccount struct {
	sa *adminpb.ServiceAccount
}

func NewServiceAccount(sa *adminpb.ServiceAccount) (ServiceAccount, error) {
	if sa == nil {
		return nil, fmt.Errorf("nil value passed for service account")
	}
	return &serviceAccount{
		sa: sa,
	}, nil
}

func (s *serviceAccount) Name() string {
	return s.sa.Name
}

func (s *serviceAccount) Disabled() bool {
	return s.sa.Disabled
}

func (s *serviceAccount) UniqueId() string {
	return s.sa.UniqueId
}
