package server

import (
	"strings"
	"time"

	gldap "github.com/go-ldap/ldap/v3"
	"github.com/h44z/wg-portal/internal/wireguard"

	"github.com/h44z/wg-portal/internal/ldap"
	"github.com/h44z/wg-portal/internal/users"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func (s *Server) SyncLdapWithUserDatabase() {
	logrus.Info("starting ldap user synchronization...")

	running := true
	for running {
		// Main work here
		logrus.Trace("syncing ldap users to database...")
		ldapUsers, err := ldap.FindAllObjects(&s.config.LDAP, ldap.Users)
		ldapGroups, errGroups := ldap.FindAllObjects(&s.config.LDAP, ldap.Groups)
		if err != nil && errGroups != nil {
			logrus.Errorf("failed to fetch users from ldap: %v", err)
			continue
		}
		logrus.Tracef("found %d users in ldap", len(ldapUsers))

		// Update existing LDAP users
		s.updateLdapUsers(ldapUsers, ldapGroups)

		// Disable missing LDAP users
		s.disableMissingLdapUsers(ldapUsers)

		// Select blocks until one of the cases happens
		select {
		case <-time.After(1 * time.Minute):
			// Sleep for 1 minute
		case <-s.ctx.Done():
			logrus.Trace("ldap-sync shutting down (context ended)...")
			running = false
			continue
		}
	}
	logrus.Info("ldap user synchronization stopped")
}

func (s Server) userIsInAdminGroup(ldapData *ldap.RawLdapData, ldapGroupData []ldap.RawLdapData, layer int) bool {
	if s.config.LDAP.EveryoneAdmin {
		return true
	}
	if s.config.LDAP.AdminLdapGroup_ == nil {
		return false
	}
	//fmt.Printf("%+v\n", ldapData.Attributes)
	var prefix string
	for i := 0; i < layer; i++ {
		prefix += "+"
	}
	logrus.Tracef("%s Group layer: %d\n", prefix, layer)
	for _, group := range ldapData.RawAttributes[s.config.LDAP.GroupMemberAttribute] {
		logrus.Tracef("%s%s\n", prefix, string(group))
		var dn, _ = gldap.ParseDN(string(group))
		if s.config.LDAP.AdminLdapGroup_.Equal(dn) {
			logrus.Tracef("%sFOUND: %s\n", prefix, string(group))
			return true
		}
		for _, group2 := range ldapGroupData {
			if group2.DN == string(group) {
				logrus.Tracef("%sChecking nested: %s\n", prefix, group2.DN)
				isAdmin := s.userIsInAdminGroup(&group2, ldapGroupData, layer+1)
				if isAdmin {
					return true
				}
			}
		}
	}
	return false
}

func (s Server) userChangedInLdap(user *users.User, ldapData *ldap.RawLdapData, ldapGroupData []ldap.RawLdapData) bool {
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

	if user.IsAdmin != s.userIsInAdminGroup(ldapData, ldapGroupData, 0) {
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
			peer.DeactivatedReason = wireguard.DeactivatedReasonLdapMissing
			if err := s.UpdatePeer(peer, now); err != nil {
				logrus.Errorf("failed to update deactivated peer %s: %v", peer.PublicKey, err)
			}
		}

		if err := s.users.DeleteUser(&activeUsers[i], true); err != nil {
			logrus.Errorf("failed to delete deactivated user %s in database: %v", activeUsers[i].Email, err)
		}
	}
}

func (s *Server) updateLdapUsers(ldapUsers []ldap.RawLdapData, ldapGroups []ldap.RawLdapData) {
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
				peer.DeactivatedReason = ""
				if err = s.UpdatePeer(peer, now); err != nil {
					logrus.Errorf("failed to update activated peer %s: %v", peer.PublicKey, err)
				}
			}
		}

		// Sync attributes from ldap
		if s.userChangedInLdap(user, &ldapUsers[i], ldapGroups) {
			logrus.Debugf("updating ldap user %s", user.Email)
			user.Firstname = ldapUsers[i].Attributes[s.config.LDAP.FirstNameAttribute]
			user.Lastname = ldapUsers[i].Attributes[s.config.LDAP.LastNameAttribute]
			user.Email = ldapUsers[i].Attributes[s.config.LDAP.EmailAttribute]
			user.Phone = ldapUsers[i].Attributes[s.config.LDAP.PhoneAttribute]
			user.IsAdmin = s.userIsInAdminGroup(&ldapUsers[i], ldapGroups, 0)
			user.Source = users.UserSourceLdap
			user.DeletedAt = gorm.DeletedAt{} // Not deleted

			if err = s.users.UpdateUser(user); err != nil {
				logrus.Errorf("failed to update ldap user %s in database: %v", user.Email, err)
				continue
			}
		}
	}
}
