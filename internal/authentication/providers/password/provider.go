package password

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/h44z/wg-portal/internal/authentication"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Provider provide login with password method
type Provider struct {
	db *gorm.DB
}

func New(cfg *users.Config) (*Provider, error) {
	p := &Provider{}

	var err error
	p.db, err = users.GetDatabaseForConfig(cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to setup authentication database %s", cfg.Database)
	}

	return p, nil
}

// GetName return provider name
func (Provider) GetName() string {
	return string(users.UserSourceDatabase)
}

// GetType return provider type
func (Provider) GetType() authentication.AuthProviderType {
	return authentication.AuthProviderTypePassword
}

// GetPriority return provider priority
func (Provider) GetPriority() int {
	return 0 // DB password provider = highest prio
}

func (provider Provider) SetupRoutes(routes *gin.RouterGroup) {
	// nothing todo here
}

func (provider Provider) Login(ctx *authentication.AuthContext) (string, error) {
	username := strings.ToLower(ctx.Username)
	password := ctx.Password

	// Validate input
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		return "", errors.New("empty username or password")
	}

	// Authenticate agains the users database
	user := users.User{}
	provider.db.Where("email = ?", username).First(&user)

	if user.Email == "" {
		return "", errors.New("invalid username")
	}

	// Compare the stored hashed password, with the hashed version of the password that was received
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", errors.New("invalid password")
	}

	return user.Email, nil
}

func (provider Provider) Logout(context *authentication.AuthContext) error {
	return nil // nothing todo here
}

func (provider Provider) GetUserModel(ctx *authentication.AuthContext) (*authentication.User, error) {
	username := strings.ToLower(ctx.Username)

	// Validate input
	if strings.Trim(username, " ") == "" {
		return nil, errors.New("empty username")
	}

	// Fetch usermodel from users database
	user := users.User{}
	provider.db.Where("email = ?", username).First(&user)
	if user.Email != username {
		return nil, errors.New("invalid or disabled username")
	}

	return &authentication.User{
		Email:     user.Email,
		IsAdmin:   user.IsAdmin,
		Firstname: user.Firstname,
		Lastname:  user.Lastname,
		Phone:     user.Phone,
	}, nil
}

func (provider Provider) InitializeAdmin(email, password string) error {
	admin := users.User{}
	provider.db.Unscoped().Where("email = ?", email).FirstOrInit(&admin)

	// newly created admin
	if admin.Email != email {
		// For security reasons a random admin password will be generated if the default one is still in use!
		if password == "wgportal" {
			password = generateRandomPassword()

			fmt.Println("#############################################")
			fmt.Println("Administrator credentials:")
			fmt.Println("  Email:    ", email)
			fmt.Println("  Password: ", password)
			fmt.Println()
			fmt.Println("This information will only be displayed once!")
			fmt.Println("#############################################")
		}
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return errors.Wrap(err, "failed to hash admin password")
		}

		admin.Email = email
		admin.Password = string(hashedPassword)
		admin.Firstname = "WireGuard"
		admin.Lastname = "Administrator"
		admin.CreatedAt = time.Now()
		admin.UpdatedAt = time.Now()
		admin.IsAdmin = true
		admin.Source = users.UserSourceDatabase

		res := provider.db.Create(admin)
		if res.Error != nil {
			return errors.Wrapf(res.Error, "failed to create admin %s", admin.Email)
		}
	}

	// update/reactivate
	if !admin.IsAdmin || admin.DeletedAt.Valid {
		// For security reasons a random admin password will be generated if the default one is still in use!
		if password == "wgportal" {
			password = generateRandomPassword()

			fmt.Println("#############################################")
			fmt.Println("Administrator credentials:")
			fmt.Println("  Email:    ", email)
			fmt.Println("  Password: ", password)
			fmt.Println()
			fmt.Println("This information will only be displayed once!")
			fmt.Println("#############################################")
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return errors.Wrap(err, "failed to hash admin password")
		}

		admin.Password = string(hashedPassword)
		admin.IsAdmin = true
		admin.UpdatedAt = time.Now()

		res := provider.db.Save(admin)
		if res.Error != nil {
			return errors.Wrapf(res.Error, "failed to update admin %s", admin.Email)
		}
	}

	return nil
}

func generateRandomPassword() string {
	rand.Seed(time.Now().Unix())
	var randPassword strings.Builder
	charSet := "abcdedfghijklmnopqrstABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#$"
	for i := 0; i < 12; i++ {
		random := rand.Intn(len(charSet))
		randPassword.WriteString(string(charSet[random]))
	}
	return randPassword.String()
}
