package api

// Permission identifies a back-office capability. The role→permission matrix
// below is the single source of truth for authorization: handlers gate on
// permissions (requirePerm), never on the raw role string.
type Permission string

const (
	permBackoffice      Permission = "backoffice.access"   // reach the back-office at all
	permReportsView     Permission = "reports.view"        // view analytics/reports
	permUsersManage     Permission = "users.manage"        // create staff, change roles, enable/disable
	permMerchantsVerify Permission = "merchants.verify"    // approve/reject merchants
	permMerchantsFee    Permission = "merchants.commission" // adjust merchant commission
)

// Valid roles stored in users.role. 'user' is the default end customer;
// 'merchant' is a customer who operates merchants; the rest are back-office.
const (
	roleUser     = "user"
	roleMerchant = "merchant"
	roleSupport  = "support"
	roleAnalyst  = "analyst"
	roleAdmin    = "admin"
)

// rolePerms is the authorization matrix. A role not present here (or unknown)
// has no permissions.
var rolePerms = map[string]map[Permission]bool{
	roleAdmin: {
		permBackoffice:      true,
		permReportsView:     true,
		permUsersManage:     true,
		permMerchantsVerify: true,
		permMerchantsFee:    true,
	},
	roleSupport: {
		permBackoffice:      true,
		permReportsView:     true,
		permMerchantsVerify: true,
	},
	roleAnalyst: {
		permBackoffice:  true,
		permReportsView: true,
	},
	roleMerchant: {},
	roleUser:     {},
}

func roleHas(role string, p Permission) bool {
	return rolePerms[role][p]
}

// isValidRole reports whether a role string is one a back-office can assign.
func isValidRole(role string) bool {
	_, ok := rolePerms[role]
	return ok
}

// assignableRoles is the list a user-manager can hand out (catalog for the UI).
func assignableRoles() []string {
	return []string{roleUser, roleMerchant, roleSupport, roleAnalyst, roleAdmin}
}

// roleCapabilities is what the client may use to decide what to render. The
// server still enforces every endpoint on its own; this is UX, not access
// control — a tampered client still can't call an endpoint it lacks the
// permission for.
func roleCapabilities(role string) map[string]bool {
	return map[string]bool{
		"backoffice":      roleHas(role, permBackoffice),
		"reports":         roleHas(role, permReportsView),
		"manageUsers":     roleHas(role, permUsersManage),
		"verifyMerchants": roleHas(role, permMerchantsVerify),
		"setCommission":   roleHas(role, permMerchantsFee),
	}
}
