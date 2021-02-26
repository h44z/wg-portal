package authentication

// User represents the data that can be retrieved from authentication backends.
type User struct {
	Email   string
	IsAdmin bool

	// optional fields
	Firstname string
	Lastname  string
	Phone     string
}
