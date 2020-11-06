package wireguard

var (
	ClientCfgTpl = `[Interface]
Address = {{ .Client.IPsStr }}
PrivateKey = {{ .Client.PrivateKey }}
{{ if ne (len .Server.DNS) 0 -}}
DNS = {{ .Server.DNSStr }}
{{- end }}
{{ if ne .Server.Mtu 0 -}}
MTU = {{.Server.Mtu}}
{{- end}}
[Peer]
PublicKey = {{ .Server.PublicKey }}
PresharedKey = {{ .Client.PresharedKey }}
AllowedIPs = {{ .Client.AllowedIPsStr }}
Endpoint = {{ .Server.Endpoint }}
{{ if and (ne .Server.PersistentKeepalive 0) (not .Client.IgnorePersistentKeepalive) -}}
PersistentKeepalive = {{.Server.PersistentKeepalive}}
{{- end}}
`
)
