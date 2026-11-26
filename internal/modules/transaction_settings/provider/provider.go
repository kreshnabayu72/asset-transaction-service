// Package provider implements the outbound adapters of the transaction settings module: the internal
// reads of master-asset-registry, enterprise-management and iam-service behind the service seams, the
// settings plan, the analytics-reporting change-log dispatcher and the export workbook renderer.
package provider

// Endpoints are the internal base URLs of the owner services the providers call.
type Endpoints struct {
	MasterAssetRegistry  string
	EnterpriseManagement string
	IAM                  string
}

// Providers is the set of owner-service providers wired into the transaction settings service.
type Providers struct {
	Plans           PlanProvider
	Categories      CategoryDirectory
	Assets          AssetDirectory
	OverrideTargets OverrideTargetDirectory
	GroupScope      GroupScopeDirectory
	DenyContacts    DenyContactResolver
	Entities        EntityProvider
}

// NewProviders builds every owner-service provider on the internal clients of the three base URLs.
func NewProviders(endpoints Endpoints, serviceToken string) Providers {
	mar := marClient{client: newInternalClient("master-asset-registry", endpoints.MasterAssetRegistry, serviceToken)}
	ems := emsClient{client: newInternalClient("enterprise-management", endpoints.EnterpriseManagement, serviceToken)}
	iam := iamClient{client: newInternalClient("iam-service", endpoints.IAM, serviceToken)}
	return Providers{
		Plans:           PlanProvider{ems: ems, iam: iam},
		Categories:      CategoryDirectory{mar: mar},
		Assets:          AssetDirectory{mar: mar, iam: iam},
		OverrideTargets: OverrideTargetDirectory{mar: mar, iam: iam},
		GroupScope:      GroupScopeDirectory{mar: mar, iam: iam},
		DenyContacts:    DenyContactResolver{iam: iam},
		Entities:        EntityProvider{ems: ems},
	}
}
