package mail

import (
	"context"
	"fmt"
	"github.com/h44z/wg-portal/internal/config"
	"github.com/h44z/wg-portal/internal/domain"
	"github.com/sirupsen/logrus"
	"io"
)

type Manager struct {
	cfg        *config.Config
	tplHandler *TemplateHandler

	mailer      Mailer
	configFiles ConfigFileManager
	users       UserDatabaseRepo
	wg          WireguardDatabaseRepo
}

func NewMailManager(cfg *config.Config, mailer Mailer, configFiles ConfigFileManager, users UserDatabaseRepo, wg WireguardDatabaseRepo) (*Manager, error) {
	tplHandler, err := newTemplateHandler(cfg.Web.ExternalUrl)
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
			logrus.Debugf("skipping peer email for %s, no user linked", peerId)
			continue
		}

		user, err := m.users.GetUser(ctx, peer.UserIdentifier)
		if err != nil {
			logrus.Debugf("skipping peer email for %s, unable to fetch user: %v", peerId, err)
			continue
		}

		if user.Email == "" {
			logrus.Debugf("skipping peer email for %s, user has no mail address", peerId)
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
