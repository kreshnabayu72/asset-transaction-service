package provider

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewProvidersBindsEachOwnerToItsBaseURL(t *testing.T) {
	p := NewProviders(Endpoints{
		MasterAssetRegistry:  "http://mar",
		EnterpriseManagement: "http://ems",
		IAM:                  "http://iam",
	}, testToken)

	require.Equal(t, "http://ems", p.Plans.ems.client.baseURL)
	require.Equal(t, "http://iam", p.Plans.iam.client.baseURL)
	require.Equal(t, "http://mar", p.Categories.mar.client.baseURL)
	require.Equal(t, "http://mar", p.Assets.mar.client.baseURL)
	require.Equal(t, "http://ems", p.Assets.ems.client.baseURL)
	require.Equal(t, "http://mar", p.OverrideTargets.mar.client.baseURL)
	require.Equal(t, "http://ems", p.OverrideTargets.ems.client.baseURL)
	require.Equal(t, "http://mar", p.GroupScope.mar.client.baseURL)
	require.Equal(t, "http://ems", p.GroupScope.ems.client.baseURL)
	require.Equal(t, "http://iam", p.DenyContacts.iam.client.baseURL)
	require.Equal(t, testToken, p.Plans.ems.client.serviceToken)
}
