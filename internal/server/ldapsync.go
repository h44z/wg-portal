package server

import (
	"strings"
	"time"

	"github.com/h44z/wg-portal/internal/ldap"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	gldap "github.com/go-ldap/ldap/v3"
)

func (s *Server) SyncLdapWithUserDatabase() {
	logrus.Info("starting ldap user synchronization...")
	running := true
	for running {
		// Select blocks until one of the cases happens
		select {
		case <-time.After(1 * time.Minute):
			// Sleep for 1 minute
		case <-s.ctx.Done():
			logrus.Trace("ldap-sync shutting down (context ended)...")
			running = false
			continue
		}

		// Main work here
		logrus.Trace("syncing ldap users to database...")
		ldapUsers, err := ldap.FindAllUsers(&s.config.LDAP)
		if err != nil {
			logrus.Errorf("failed to fetch users from ldap: %v", err)
			continue
		}
		logrus.Tracef("found %d users in ldap", len(ldapUsers))

		// Update existing LDAP users
		s.updateLdapUsers(ldapUsers)

		// Disable missing LDAP users
		s.disableMissingLdapUsers(ldapUsers)
	}
	logrus.Info("ldap user synchronization stopped")
}

func (s Server) userIsInAdminGroup(ldapData *ldap.RawLdapData) bool {
	if s.config.LDAP.AdminLdapGroup_ == nil {
		return false
	}
	for _, group := range ldapData.RawAttributes[s.config.LDAP.GroupMemberAttribute] {
		var dn, _ = gldap.ParseDN(string(group))
		if s.config.LDAP.AdminLdapGroup_.Equal(dn) {
			return true
		}
	}
	return false
}

func (s Server) userChangedInLdap(user *users.User, ldapData *ldap.RawLdapData) bool {
	if user.Firstname != ldapData.Attributes[s.config.LDAP.FirstNameAttribute] {
		return true
	}
	if user.Lastname != ldapData.Attributes[s.config.LDAP.LastNameAttribute] {
		return true
	}
	if user.Email != strings.ToLower(ldapData.Attributes[s.config.LDAP.EmailAttribute]) {
		return true
	}
	if user.Phone != ldapData.Attributes[s.config.LDAP.PhoneAttribute] {
		return true
	}
	if user.Source != users.UserSourceLdap {
		return true
	}

	if user.DeletedAt.Valid {
		return true
	}

	if user.IsAdmin != s.userIsInAdminGroup(ldapData) {
		return true
	}

	return false
}

func (s *Server) disableMissingLdapUsers(ldapUsers []ldap.RawLdapData) {
	// Disable missing LDAP users
	activeUsers := s.users.GetUsers()
	for i := range activeUsers {
		if activeUsers[i].Source != users.UserSourceLdap {
			continue
		}

		existsInLDAP := false
		for j := range ldapUsers {
			if activeUsers[i].Email == strings.ToLower(ldapUsers[j].Attributes[s.config.LDAP.EmailAttribute]) {
				existsInLDAP = true
				break
			}
		}

		if existsInLDAP {
			continue
		}

		// disable all peers for the given user
		for _, peer := range s.peers.GetPeersByMail(activeUsers[i].Email) {
			now := time.Now()
			peer.DeactivatedAt = &now
			if err := s.UpdatePeer(peer, now); err != nil {
				logrus.Errorf("failed to update deactivated peer %s: %v", peer.PublicKey, err)
			}
		}

		if err := s.users.DeleteUser(&activeUsers[i], true); err != nil {
			logrus.Errorf("failed to delete deactivated user %s in database: %v", activeUsers[i].Email, err)
		}
	}
}

func (s *Server) updateLdapUsers(ldapUsers []ldap.RawLdapData) {
	for i := range ldapUsers {
		if ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute] == "" {
			logrus.Tracef("skipping sync of %s, empty email attribute", ldapUsers[i].DN)
			continue
		}

		user, err := s.users.GetOrCreateUserUnscoped(ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute])
		if err != nil {
			logrus.Errorf("failed to get/create user %s in database: %v", ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute], err)
		}

		// re-enable LDAP user if the user was disabled
		if user.DeletedAt.Valid {
			// enable all peers for the given user
			for _, peer := range s.peers.GetPeersByMail(user.Email) {
				now := time.Now()
				peer.DeactivatedAt = nil
				if err = s.UpdatePeer(peer, now); err != nil {
					logrus.Errorf("failed to update activated peer %s: %v", peer.PublicKey, err)
				}
			}
		}

		// Sync attributes from ldap
		if s.userChangedInLdap(user, &ldapUsers[i]) {
			logrus.Debugf("updating ldap user %s", user.Email)
			user.Firstname = ldapUsers[i].Attributes[s.config.LDAP.FirstNameAttribute]
			user.Lastname = ldapUsers[i].Attributes[s.config.LDAP.LastNameAttribute]
			user.Email = ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute]
			user.Phone = ldapUsers[i].Attributes[s.config.LDAP.PhoneAttribute]
			user.IsAdmin = s.userIsInAdminGroup(&ldapUsers[i])
			user.Source = users.UserSourceLdap
			user.DeletedAt = gorm.DeletedAt{} // Not deleted

			if err = s.users.UpdateUser(user); err != nil {
				logrus.Errorf("failed to update ldap user %s in database: %v", user.Email, err)
				continue
			}
		}
	}
}
