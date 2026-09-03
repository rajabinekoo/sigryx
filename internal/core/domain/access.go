package domain

import "time"

type Permission string

const (
	PermissionVaultStatusRead     Permission = "vault.status.read"
	PermissionVaultInitialize     Permission = "vault.initialize"
	PermissionVaultUnseal         Permission = "vault.unseal"
	PermissionVaultSeal           Permission = "vault.seal"
	PermissionKeyRootRead         Permission = "keyroot.read"
	PermissionKeyRootCreate       Permission = "keyroot.create"
	PermissionWalletCreate        Permission = "wallet.create"
	PermissionSignTransaction     Permission = "sign.transaction"
	PermissionSignTypedData       Permission = "sign.typed_data"
	PermissionSignGeneric         Permission = "sign.generic"
	PermissionVerifyTransaction   Permission = "verify.transaction"
	PermissionVerifyTypedData     Permission = "verify.typed_data"
	PermissionVerifyGeneric       Permission = "verify.generic"
	PermissionAccessUsersManage   Permission = "access.users.manage"
	PermissionAccessRolesManage   Permission = "access.roles.manage"
	PermissionAccessServiceManage Permission = "access.service_accounts.manage"
)

type PermissionDefinition struct {
	Permission Permission
	Category   string
	Label      string
}

var PermissionDefinitions = []PermissionDefinition{
	{PermissionVaultStatusRead, "vault", "Read vault status"},
	{PermissionVaultInitialize, "vault", "Initialize vault"},
	{PermissionVaultUnseal, "vault", "Unseal vault"},
	{PermissionVaultSeal, "vault", "Seal vault"},
	{PermissionKeyRootRead, "key-root", "List key roots"},
	{PermissionKeyRootCreate, "key-root", "Create key root"},
	{PermissionWalletCreate, "wallet", "Create or resolve wallet"},
	{PermissionSignTransaction, "signing", "Sign Ethereum transaction"},
	{PermissionSignTypedData, "signing", "Sign EIP-712 typed data"},
	{PermissionSignGeneric, "signing", "Sign generic data"},
	{PermissionVerifyTransaction, "verification", "Verify Ethereum transaction"},
	{PermissionVerifyTypedData, "verification", "Verify EIP-712 typed data"},
	{PermissionVerifyGeneric, "verification", "Verify generic data"},
	{PermissionAccessUsersManage, "access", "Manage users"},
	{PermissionAccessRolesManage, "access", "Manage roles"},
	{PermissionAccessServiceManage, "access", "Manage service accounts"},
}

type Role struct {
	ID          string
	Name        string
	Permissions []Permission
}

type User struct {
	ID           string
	Username     string
	PasswordHash string
	IsRootAdmin  bool
	Active       bool
	RoleID       string
	AllowedCIDRs []string
}

type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash []byte
	ExpiresAt        time.Time
	RevokedAt        *time.Time
}

type ServiceAccount struct {
	ID               string
	Name             string
	ClientID         string
	ClientSecretHash []byte
	Active           bool
	RoleID           string
	AllowedCIDRs     []string
}

type PrincipalKind string

const (
	PrincipalUser    PrincipalKind = "USER"
	PrincipalService PrincipalKind = "SERVICE"
)

type Principal struct {
	ID          string
	Kind        PrincipalKind
	SessionID   string
	RootAdmin   bool
	Permissions []Permission
}
