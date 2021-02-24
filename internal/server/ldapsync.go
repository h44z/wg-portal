package server

import (
	"time"

	"github.com/h44z/wg-portal/internal/ldap"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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

		for i := range ldapUsers {
			// prefilter
			if ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute] == "" ||
				ldapUsers[i].Attributes[s.config.LDAP.FirstNameAttribute] == "" ||
				ldapUsers[i].Attributes[s.config.LDAP.LastNameAttribute] == "" {
				continue
			}

			user, err := s.users.GetOrCreateUserUnscoped(ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute])
			if err != nil {
				logrus.Errorf("failed to get/create user %s in database: %v", ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute], err)
			}

			// check if user should be deactivated
			ldapDeactivated := false
			switch s.config.LDAP.Type {
			case ldap.TypeActiveDirectory:
				ldapDeactivated = ldap.IsActiveDirectoryUserDisabled(ldapUsers[i].Attributes[s.config.LDAP.DisabledAttribute])
			case ldap.TypeOpenLDAP:
				ldapDeactivated = ldap.IsOpenLdapUserDisabled(ldapUsers[i].Attributes[s.config.LDAP.DisabledAttribute])
			}

			// check if user has been disabled in ldap, update peers accordingly
			if ldapDeactivated != user.DeletedAt.Valid {
				if ldapDeactivated {
					// disable all peers for the given user
					for _, peer := range s.peers.GetPeersByMail(user.Email) {
						now := time.Now()
						peer.DeactivatedAt = &now
						if err = s.UpdatePeer(peer, now); err != nil {
							logrus.Errorf("failed to update deactivated peer %s: %v", peer.PublicKey, err)
						}
					}
				} else {
					// enable all peers for the given user
					for _, peer := range s.peers.GetPeersByMail(user.Email) {
						now := time.Now()
						peer.DeactivatedAt = nil
						if err = s.UpdatePeer(peer, now); err != nil {
							logrus.Errorf("failed to update activated peer %s: %v", peer.PublicKey, err)
						}
					}
				}
			}

			// Sync attributes from ldap
			if s.UserChangedInLdap(user, &ldapUsers[i]) {
				user.Firstname = ldapUsers[i].Attributes[s.config.LDAP.FirstNameAttribute]
				user.Lastname = ldapUsers[i].Attributes[s.config.LDAP.LastNameAttribute]
				user.Email = ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute]
				user.Phone = ldapUsers[i].Attributes[s.config.LDAP.PhoneAttribute]
				user.IsAdmin = false
				user.Source = users.UserSourceLdap
				user.DeletedAt = gorm.DeletedAt{} // Not deleted

				for _, group := range ldapUsers[i].RawAttributes[s.config.LDAP.GroupMemberAttribute] {
					if string(group) == s.config.LDAP.AdminLdapGroup {
						user.IsAdmin = true
						break
					}
				}

				if ldapDeactivated {
					if err = s.users.DeleteUser(user); err != nil {
						logrus.Errorf("failed to delete deactivated user %s in database: %v", user.Email, err)
						continue
					}
				} else {
					if err = s.users.UpdateUser(user); err != nil {
						logrus.Errorf("failed to update ldap user %s in database: %v", user.Email, err)
						continue
					}
				}
			}
		}
	}
	logrus.Info("ldap user synchronization stopped")
}

func (s Server) UserChangedInLdap(user *users.User, ldapData *ldap.RawLdapData) bool {
	if user.Firstname != ldapData.Attributes[s.config.LDAP.FirstNameAttribute] {
		return true
	}
	if user.Lastname != ldapData.Attributes[s.config.LDAP.LastNameAttribute] {
		return true
	}
	if user.Email != ldapData.Attributes[s.config.LDAP.EmailAttribute] {
		return true
	}
	if user.Phone != ldapData.Attributes[s.config.LDAP.PhoneAttribute] {
		return true
	}

	ldapDeactivated := false
	switch s.config.LDAP.Type {
	case ldap.TypeActiveDirectory:
		ldapDeactivated = ldap.IsActiveDirectoryUserDisabled(ldapData.Attributes[s.config.LDAP.DisabledAttribute])
	case ldap.TypeOpenLDAP:
		ldapDeactivated = ldap.IsOpenLdapUserDisabled(ldapData.Attributes[s.config.LDAP.DisabledAttribute])
	}
	if ldapDeactivated != user.DeletedAt.Valid {
		return true
	}

	ldapAdmin := false
	for _, group := range ldapData.RawAttributes[s.config.LDAP.GroupMemberAttribute] {
		if string(group) == s.config.LDAP.AdminLdapGroup {
			ldapAdmin = true
			break
		}
	}
	if user.IsAdmin != ldapAdmin {
		return true
	}

	return false
}
