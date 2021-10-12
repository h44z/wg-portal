package persistence

import (
	"github.com/pkg/errors"
	"gorm.io/gorm/clause"
)

func (d *Database) GetAvailableInterfaces() ([]InterfaceIdentifier, error) {
	var interfaces []InterfaceConfig
	if err := d.db.Select("identifier").Find(&interfaces).Error; err != nil {
		return nil, errors.WithMessage(err, "unable to find interfaces")
	}

	interfaceIds := make([]InterfaceIdentifier, len(interfaces))
	for i := range interfaces {
		interfaceIds[i] = interfaces[i].Identifier
	}

	return interfaceIds, nil
}

func (d *Database) GetAllInterfaces(ids ...InterfaceIdentifier) (map[InterfaceConfig][]PeerConfig, error) {
	var interfaces []InterfaceConfig
	if err := d.db.Where("identifier IN ?", ids).Find(&interfaces).Error; err != nil {
		return nil, errors.WithMessage(err, "unable to find interfaces")
	}

	interfaceMap := make(map[InterfaceConfig][]PeerConfig, len(interfaces))
	for i := range interfaces {
		var peers []PeerConfig
		if err := d.db.Where("iface_identifier = ?", interfaces[i].Identifier).Find(&peers).Error; err != nil {
			return nil, errors.WithMessagef(err, "unable to find peers for %s", interfaces[i].Identifier)
		}
		interfaceMap[interfaces[i]] = peers
	}

	return interfaceMap, nil
}

func (d *Database) GetInterface(id InterfaceIdentifier) (InterfaceConfig, []PeerConfig, error) {
	var iface InterfaceConfig
	if err := d.db.First(&iface, id).Error; err != nil {
		return InterfaceConfig{}, nil, errors.WithMessage(err, "unable to find interface")
	}

	var peers []PeerConfig
	if err := d.db.Where("identifier = ?", id).Find(&peers).Error; err != nil {
		return InterfaceConfig{}, nil, errors.WithMessage(err, "unable to find peers")
	}

	return iface, peers, nil
}

func (d *Database) SaveInterface(cfg InterfaceConfig) error {
	d.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&cfg)
	return nil
}

func (d *Database) DeleteInterface(id InterfaceIdentifier) error {
	if err := d.db.Delete(&InterfaceConfig{}, id).Error; err != nil {
		return errors.WithMessage(err, "unable to delete interface")
	}

	return nil
}

func (d *Database) SavePeer(peer PeerConfig) error {
	if err := d.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&peer).Error; err != nil {
		return errors.WithMessage(err, "unable to save peer")
	}

	return nil
}

func (d *Database) DeletePeer(peerId PeerIdentifier) error {
	if err := d.db.Delete(&PeerConfig{}, peerId).Error; err != nil {
		return errors.WithMessage(err, "unable to delete peer")
	}
	return nil
}
