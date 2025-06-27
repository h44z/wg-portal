package mail

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
)

// region dependencies

type Mailer interface {
	// Send sends an email with the given subject and body to the given recipients.
	Send(ctx context.Context, subject, body string, to []string, options *domain.MailOptions) error
}

type ConfigFileManager interface {
	// GetInterfaceConfig returns the configuration for the given interface.
	GetInterfaceConfig(ctx context.Context, id domain.InterfaceIdentifier) (io.Reader, error)
	// GetPeerConfig returns the configuration for the given peer.
	GetPeerConfig(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
	// GetPeerConfigQrCode returns the QR code for the given peer.
	GetPeerConfigQrCode(ctx context.Context, id domain.PeerIdentifier) (io.Reader, error)
}

type UserDatabaseRepo interface {
	// GetUser returns the user with the given identifier.
	GetUser(ctx context.Context, id domain.UserIdentifier) (*domain.User, error)
}

type WireguardDatabaseRepo interface {
	// GetInterfaceAndPeers returns the interface and all peers for the given interface identifier.
	GetInterfaceAndPeers(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, []domain.Peer, error)
	// GetPeer returns the peer with the given identifier.
	GetPeer(ctx context.Context, id domain.PeerIdentifier) (*domain.Peer, error)
	// GetInterface returns the interface with the given identifier.
	GetInterface(ctx context.Context, id domain.InterfaceIdentifier) (*domain.Interface, error)
}

type TemplateRenderer interface {
	// GetConfigMail returns the text and html template for the mail with a link.
	GetConfigMail(user *domain.User, link string) (io.Reader, io.Reader, error)
	// GetConfigMailWithAttachment returns the text and html template for the mail with an attachment.
	GetConfigMailWithAttachment(user *domain.User, cfgName, qrName string) (
		io.Reader,
		io.Reader,
		error,
	)
}

// endregion dependencies

type Manager struct {
	cfg *config.Config

	tplHandler  TemplateRenderer
	mailer      Mailer
	configFiles ConfigFileManager
	users       UserDatabaseRepo
	wg          WireguardDatabaseRepo
}

// NewMailManager creates a new mail manager.
func NewMailManager(
	cfg *config.Config,
	mailer Mailer,
	configFiles ConfigFileManager,
	users UserDatabaseRepo,
	wg WireguardDatabaseRepo,
) (*Manager, error) {
	tplHandler, err := newTemplateHandler(cfg.Web.ExternalUrl, cfg.Web.SiteTitle)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template handler: %w", err)
	}

	m := &Manager{
		cfg:         cfg,
		tplHandler:  tplHandler,
		mailer:      mailer,
		configFiles: configFiles,
		users:       users,
		wg:          wg,
	}

	return m, nil
}

// SendPeerEmail sends an email to the user linked to the given peers.
func (m Manager) SendPeerEmail(ctx context.Context, linkOnly bool, peers ...domain.PeerIdentifier) error {
	for _, peerId := range peers {
		peer, err := m.wg.GetPeer(ctx, peerId)
		if err != nil {
			return fmt.Errorf("failed to fetch peer %s: %w", peerId, err)
		}

		if err := domain.ValidateUserAccessRights(ctx, peer.UserIdentifier); err != nil {
			return err
		}

		if peer.UserIdentifier == "" {
			slog.Debug("skipping peer email",
				"peer", peerId,
				"reason", "no user linked")
			continue
		}

		user, err := m.users.GetUser(ctx, peer.UserIdentifier)
		if err != nil {
			slog.Debug("skipping peer email",
				"peer", peerId,
				"reason", "unable to fetch user",
				"error", err)
			continue
		}

		if user.Email == "" {
			slog.Debug("skipping peer email",
				"peer", peerId,
				"reason", "user has no mail address")
			continue
		}

		err = m.sendPeerEmail(ctx, linkOnly, user, peer)
		if err != nil {
			return fmt.Errorf("failed to send peer email for %s: %w", peerId, err)
		}
	}

	return nil
}

func (m Manager) sendPeerEmail(ctx context.Context, linkOnly bool, user *domain.User, peer *domain.Peer) error {
	qrName := "WireGuardQRCode.png"
	configName := peer.GetConfigFileName()

	var (
		txtMail, htmlMail io.Reader
		err               error
		mailOptions       domain.MailOptions
	)
	if linkOnly {
		txtMail, htmlMail, err = m.tplHandler.GetConfigMail(user, "deep link TBD")
		if err != nil {
			return fmt.Errorf("failed to get mail body: %w", err)
		}

	} else {
		peerConfig, err := m.configFiles.GetPeerConfig(ctx, peer.Identifier)
		if err != nil {
			return fmt.Errorf("failed to fetch peer config for %s: %w", peer.Identifier, err)
		}

		peerConfigQr, err := m.configFiles.GetPeerConfigQrCode(ctx, peer.Identifier)
		if err != nil {
			return fmt.Errorf("failed to fetch peer config QR code for %s: %w", peer.Identifier, err)
		}

		txtMail, htmlMail, err = m.tplHandler.GetConfigMailWithAttachment(user, configName, qrName)
		if err != nil {
			return fmt.Errorf("failed to get full mail body: %w", err)
		}

		mailOptions.Attachments = append(mailOptions.Attachments, domain.MailAttachment{
			Name:        configName,
			ContentType: "text/plain",
			Data:        peerConfig,
			Embedded:    false,
		})
		mailOptions.Attachments = append(mailOptions.Attachments, domain.MailAttachment{
			Name:        qrName,
			ContentType: "image/png",
			Data:        peerConfigQr,
			Embedded:    true,
		})
	}

	txtMailStr, _ := io.ReadAll(txtMail)
	htmlMailStr, _ := io.ReadAll(htmlMail)
	mailOptions.HtmlBody = string(htmlMailStr)

	err = m.mailer.Send(ctx, "WireGuard VPN Configuration", string(txtMailStr), []string{user.Email}, &mailOptions)
	if err != nil {
		return fmt.Errorf("failed to send mail: %w", err)
	}

	return nil
}
