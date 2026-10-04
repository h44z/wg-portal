function b64urlToB64(input) {
  let b64 = input.replace(/-/g, '+').replace(/_/g, '/')
  while (b64.length % 4) {
    b64 += '='
  }
  return b64
}

function bytesToBase64(bytes) {
  let binary = ''
  for (let i = 0; i < bytes.byteLength; ++i) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}

export async function generateWireGuardKeypair() {
  const keyPair = await crypto.subtle.generateKey(
    { name: 'X25519', namedCurve: 'X25519' },
    true,
    ['deriveBits'],
  )

  const publicJwk = await crypto.subtle.exportKey('jwk', keyPair.publicKey)
  const privateJwk = await crypto.subtle.exportKey('jwk', keyPair.privateKey)

  return {
    publicKey: b64urlToB64(publicJwk.x),
    privateKey: b64urlToB64(privateJwk.d),
  }
}

export function generateWireGuardPresharedKey() {
  const key = new Uint8Array(32)
  crypto.getRandomValues(key)
  return bytesToBase64(key)
}

function optionValue(option) {
  if (!option || option.Value === undefined || option.Value === null) {
    return ''
  }
  return option.Value
}

function listValue(value) {
  if (Array.isArray(value)) {
    return value.filter(Boolean).join(', ')
  }
  return value || ''
}

export function buildWgQuickConfig(peer, privateKey) {
  const lines = [
    '[Interface]',
    `PrivateKey = ${privateKey}`,
  ]

  const addresses = listValue(peer.Addresses)
  if (addresses) {
    lines.push(`Address = ${addresses}`)
  }

  const dns = optionValue(peer.Dns)
  const dnsSearch = optionValue(peer.DnsSearch)
  const dnsValues = []
  if (Array.isArray(dns)) dnsValues.push(...dns)
  else if (dns) dnsValues.push(dns)
  if (Array.isArray(dnsSearch)) dnsValues.push(...dnsSearch)
  else if (dnsSearch) dnsValues.push(dnsSearch)
  if (dnsValues.length) {
    lines.push(`DNS = ${dnsValues.join(', ')}`)
  }

  const mtu = optionValue(peer.Mtu)
  if (mtu) lines.push(`MTU = ${mtu}`)

  const table = optionValue(peer.RoutingTable)
  if (table) lines.push(`Table = ${table}`)

  const fwMark = optionValue(peer.FirewallMark)
  if (fwMark) lines.push(`FwMark = ${fwMark}`)

  const hooks = [
    ['PreUp', optionValue(peer.PreUp)],
    ['PostUp', optionValue(peer.PostUp)],
    ['PreDown', optionValue(peer.PreDown)],
    ['PostDown', optionValue(peer.PostDown)],
  ]
  for (const [name, value] of hooks) {
    if (value) lines.push(`${name} = ${value}`)
  }

  lines.push('', '[Peer]')

  const endpointPublicKey = optionValue(peer.EndpointPublicKey)
  if (endpointPublicKey) lines.push(`PublicKey = ${endpointPublicKey}`)

  const endpoint = optionValue(peer.Endpoint)
  if (endpoint) lines.push(`Endpoint = ${endpoint}`)

  const allowedIPs = listValue(optionValue(peer.AllowedIPs))
  if (allowedIPs) lines.push(`AllowedIPs = ${allowedIPs}`)

  if (peer.PresharedKey) lines.push(`PresharedKey = ${peer.PresharedKey}`)

  const keepalive = optionValue(peer.PersistentKeepalive)
  if (keepalive && peer.Mode === 'client') {
    lines.push(`PersistentKeepalive = ${keepalive}`)
  }

  return `${lines.join('\n')}\n`
}

export function downloadWgQuickConfig(peer, privateKey) {
  const config = buildWgQuickConfig(peer, privateKey)
  const blob = new Blob([config], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const element = document.createElement('a')

  element.href = url
  element.download = peer.Filename || 'wireguard.conf'
  element.style.display = 'none'
  document.body.appendChild(element)
  element.click()
  document.body.removeChild(element)

  setTimeout(() => URL.revokeObjectURL(url), 0)
}
