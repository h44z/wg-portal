package persistence

func (d *Database) Migrate() error {
	d.db.AutoMigrate(&InterfaceConfig{}, &User{})
	d.db.AutoMigrate(&PeerConfig{})
	return nil
}
