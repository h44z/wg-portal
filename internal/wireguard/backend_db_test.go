package wireguard

import (
	"database/sql"
	"database/sql/driver"
	"reflect"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type AnyTime struct{}

// Match satisfies sqlmock.Argument interface
func (a AnyTime) Match(v driver.Value) bool {
	_, ok := v.(time.Time)
	return ok
}

func getMockedGorm() (*gorm.DB, sqlmock.Sqlmock, error) {
	// Default mock with regex matching (https://tienbm90.medium.com/unit-test-for-gorm-application-with-go-sqlmock-ecb5c369e570)
	db, mock, err := sqlmock.New()
	if err != nil {
		return nil, nil, err
	}

	gdb, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      db,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	}) // open gorm db
	if err != nil {
		return nil, nil, err
	}
	return gdb, mock, nil
}

func TestDatabaseBackend_DeleteInterface(t *testing.T) {
	db, mock, err := getMockedGorm()
	require.NoError(t, err)
	backend := &DatabaseBackend{db: db}

	type args struct {
		iface InterfaceConfig
		peers []PeerConfig
	}
	tests := []struct {
		name    string
		mock    func()
		args    args
		wantErr bool
	}{
		{
			name: "Success",
			mock: func() {
				mock.ExpectExec("DELETE FROM `peer` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM `peer_defaults` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM `interface` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			args: args{
				iface: InterfaceConfig{DeviceName: "wg0"},
				peers: nil,
			},
			wantErr: false,
		},
		{
			name: "Peer Delete Failure",
			mock: func() {
				mock.ExpectExec("DELETE FROM `peer` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnError(errors.New("peererr"))
			},
			args: args{
				iface: InterfaceConfig{DeviceName: "wg0"},
				peers: nil,
			},
			wantErr: true,
		},
		{
			name: "Peer Defaults Delete Failure",
			mock: func() {
				mock.ExpectExec("DELETE FROM `peer` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM `peer_defaults` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnError(errors.New("defaultserr"))
			},
			args: args{
				iface: InterfaceConfig{DeviceName: "wg0"},
				peers: nil,
			},
			wantErr: true,
		},
		{
			name: "Interface Delete Failure",
			mock: func() {
				mock.ExpectExec("DELETE FROM `peer` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM `peer_defaults` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("DELETE FROM `interface` WHERE device_name = \\?").
					WithArgs("wg0").WillReturnError(errors.New("ifaceerr"))
			},
			args: args{
				iface: InterfaceConfig{DeviceName: "wg0"},
				peers: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			if err := backend.DeleteInterface(tt.args.iface, tt.args.peers); (err != nil) != tt.wantErr {
				t.Errorf("DeleteInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDatabaseBackend_DeletePeer(t *testing.T) {
	db, mock, err := getMockedGorm()
	require.NoError(t, err)
	backend := &DatabaseBackend{db: db}

	type args struct {
		peer  PeerConfig
		iface InterfaceConfig
	}
	tests := []struct {
		name    string
		mock    func()
		args    args
		wantErr bool
	}{
		{
			name: "Success",
			mock: func() {
				mock.ExpectExec("DELETE FROM `peer` WHERE device_name = \\? AND uid = \\?").
					WithArgs("wg0", "peer0").WillReturnResult(sqlmock.NewResult(1, 1))
			},
			args: args{
				peer:  PeerConfig{Uid: "peer0"},
				iface: InterfaceConfig{DeviceName: "wg0"},
			},
			wantErr: false,
		},
		{
			name: "Peer Delete Failure",
			mock: func() {
				mock.ExpectExec("DELETE FROM `peer` WHERE device_name = \\? AND uid = \\?").
					WithArgs("wg0", "peer0").WillReturnError(errors.New("peererr"))
			},
			args: args{
				peer:  PeerConfig{Uid: "peer0"},
				iface: InterfaceConfig{DeviceName: "wg0"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			if err := backend.DeletePeer(tt.args.peer, tt.args.iface); (err != nil) != tt.wantErr {
				t.Errorf("DeletePeer() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDatabaseBackend_Load(t *testing.T) {
	type fields struct {
		db *gorm.DB
	}
	type args struct {
		identifier DeviceIdentifier
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    InterfaceConfig
		want1   []PeerConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DatabaseBackend{
				db: tt.fields.db,
			}
			got, got1, err := d.Load(tt.args.identifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Load() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("Load() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func TestDatabaseBackend_LoadAll(t *testing.T) {
	type fields struct {
		db *gorm.DB
	}
	type args struct {
		ignored []DeviceIdentifier
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[InterfaceConfig][]PeerConfig
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DatabaseBackend{
				db: tt.fields.db,
			}
			got, err := d.LoadAll(tt.args.ignored...)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadAll() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDatabaseBackend_SaveInterface(t *testing.T) {
	db, mock, err := getMockedGorm()
	require.NoError(t, err)
	backend := &DatabaseBackend{db: db}

	type args struct {
		cfg   InterfaceConfig
		peers []PeerConfig
	}
	tests := []struct {
		name    string
		mock    func()
		args    args
		wantErr bool
	}{
		{
			name: "Success Create",
			mock: func() {
				mock.ExpectExec("INSERT INTO `interface` .*").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("INSERT INTO `peer_defaults` .*").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			args: args{
				cfg:   InterfaceConfig{DeviceName: "wg0"},
				peers: nil,
			},
			wantErr: false,
		},
		{
			name: "Error Interface",
			mock: func() {
				mock.ExpectExec("INSERT INTO `interface` .*").
					WillReturnError(errors.New("ifaceerr"))
			},
			args: args{
				cfg:   InterfaceConfig{DeviceName: "wg0"},
				peers: nil,
			},
			wantErr: true,
		},
		{
			name: "Error Peer Defaults",
			mock: func() {
				mock.ExpectExec("INSERT INTO `interface` .*").
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectExec("INSERT INTO `peer_defaults` .*").
					WillReturnError(errors.New("ifaceerr"))
			},
			args: args{
				cfg:   InterfaceConfig{DeviceName: "wg0"},
				peers: nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			if err := backend.SaveInterface(tt.args.cfg, tt.args.peers); (err != nil) != tt.wantErr {
				t.Errorf("SaveInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestDatabaseBackend_SavePeer(t *testing.T) {
	db, mock, err := getMockedGorm()
	require.NoError(t, err)
	backend := &DatabaseBackend{db: db}

	type args struct {
		peer  PeerConfig
		iface InterfaceConfig
	}
	tests := []struct {
		name    string
		mock    func()
		args    args
		wantErr bool
	}{
		{
			name: "Success Create",
			mock: func() {
				mock.ExpectExec("INSERT INTO `peer` .*").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			args: args{
				peer:  PeerConfig{Uid: "peer0"},
				iface: InterfaceConfig{DeviceName: "wg0"},
			},
			wantErr: false,
		},
		{
			name: "Error",
			mock: func() {
				mock.ExpectExec("INSERT INTO `peer` .*").
					WillReturnError(errors.New("peererr"))
			},
			args: args{
				peer:  PeerConfig{Uid: "peer0"},
				iface: InterfaceConfig{DeviceName: "wg0"},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mock()
			if err := backend.SavePeer(tt.args.peer, tt.args.iface); (err != nil) != tt.wantErr {
				t.Errorf("SavePeer() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestNewDatabaseBackend(t *testing.T) {
	db, mock, err := getMockedGorm()
	require.NoError(t, err)

	// Success
	mock.ExpectExec("CREATE TABLE `interface` .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("CREATE TABLE `peer_defaults` .*").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("CREATE TABLE `peer` .*").WillReturnResult(sqlmock.NewResult(1, 1))
	backend, err := NewDatabaseBackend(db)
	assert.NoError(t, err)
	assert.NotNil(t, backend)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Migration failure
	mock.ExpectExec("CREATE TABLE `interface` .*").WillReturnError(errors.New("migerr"))
	backend, err = NewDatabaseBackend(db)
	assert.Error(t, err)
	assert.Nil(t, backend)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func Test_convertInterface(t *testing.T) {
	config, peerDefaultConfig := convertInterface(InterfaceConfig{})
	assert.Equal(t, dbInterfaceConfig{}, config)
	assert.Equal(t, dbDefaultPeerConfig{}, peerDefaultConfig)

	now := time.Now()
	config, peerDefaultConfig = convertInterface(InterfaceConfig{DisabledAt: &now})
	assert.Equal(t, dbInterfaceConfig{DisabledAt: sql.NullTime{Time: now, Valid: true}}, config)
	assert.Equal(t, dbDefaultPeerConfig{}, peerDefaultConfig)
}

func Test_convertPeer(t *testing.T) {
	peer := convertPeer(PeerConfig{}, "wg0")
	assert.Equal(t, dbPeerConfig{DeviceName: "wg0"}, peer)

	now := time.Now()
	peer = convertPeer(PeerConfig{DisabledAt: &now}, "wg0")
	assert.Equal(t, dbPeerConfig{DeviceName: "wg0", DisabledAt: sql.NullTime{Time: now, Valid: true}}, peer)
}

func Test_dbDefaultPeerConfig_TableName(t *testing.T) {
	assert.Equal(t, "peer_defaults", dbDefaultPeerConfig{}.TableName())
}

func Test_dbInterfaceConfig_TableName(t *testing.T) {
	assert.Equal(t, "interface", dbInterfaceConfig{}.TableName())
}

func Test_dbPeerConfig_TableName(t *testing.T) {
	assert.Equal(t, "peer", dbPeerConfig{}.TableName())
}
