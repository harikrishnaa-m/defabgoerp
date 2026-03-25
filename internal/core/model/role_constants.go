package model

const (
	RoleSuperAdmin      = "SuperAdmin"
	RoleStoreManager    = "StoreManager"
	RoleSalesPerson     = "SalesPerson"
	RoleAccountsManager = "AccountsManager"
)

var ValidRoles = map[string]bool{
	RoleSuperAdmin:      true,
	RoleStoreManager:    true,
	RoleSalesPerson:     true,
	RoleAccountsManager: true,
}
