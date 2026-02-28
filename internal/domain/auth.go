package domain

type LoginProvider string

type LoginProviderInfo struct {
	Identifier  string
	Name        string
	ProviderUrl string
	CallbackUrl string
}

type AuthenticatorUserInfo struct {
	Identifier         UserIdentifier
	Email              string
	Firstname          string
	Lastname           string
	Phone              string
	Department         string
	IsAdmin            bool
	AdminInfoAvailable bool // true if the IsAdmin flag is valid
}
