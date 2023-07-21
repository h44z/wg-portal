package model

type Error struct {
	Code    int    `json:"Code"`
	Message string `json:"Message"`
}

type Settings struct {
	MailLinkOnly              bool `json:"MailLinkOnly"`
	PersistentConfigSupported bool `json:"PersistentConfigSupported"`
	SelfProvisioning          bool `json:"SelfProvisioning"`
}
