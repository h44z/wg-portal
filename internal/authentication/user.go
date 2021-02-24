package authentication

type User struct {
	Email   string
	IsAdmin bool

	// optional fields
	Firstname string
	Lastname  string
	Phone     string
}
