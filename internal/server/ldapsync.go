package server

import (
	"time"

	"github.com/h44z/wg-portal/internal/ldap"
	log "github.com/sirupsen/logrus"
)

// SyncLdapAttributesWithWireGuard starts to synchronize the "disabled" attribute from ldap.
// Users will be automatically disabled once they are disabled in ldap.
// This method is blocking.
func (s *Server) SyncLdapAttributesWithWireGuard() error {
	allUsers := s.users.GetAllUsers()
	for i := range allUsers {
		user := allUsers[i]
		if user.LdapUser == nil {
			continue // skip non ldap users
		}

		if user.DeactivatedAt != nil {
			continue // skip already disabled interfaces
		}

		if ldap.IsLdapUserDisabled(allUsers[i].LdapUser.Attributes["userAccountControl"]) {
			now := time.Now()
			user.DeactivatedAt = &now
			if err := s.UpdateUser(user, now); err != nil {
				log.Errorf("Failed to disable user %s: %v", user.Email, err)
			}
		}
	}
	return nil
}
