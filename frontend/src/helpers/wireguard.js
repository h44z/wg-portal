/**
 * Convert a Base64URL-encoded string to standard Base64 with padding.
 * @function b64urlToB64
 * @param {string} input - The Base64URL string.
 * @returns {string} The padded, standard Base64 string.
 */
function b64urlToB64(input) {
  let b64 = input.replace(/-/g, '+').replace(/_/g, '/');
  while (b64.length % 4) {
    b64 += '=';
  }
  return b64;
}

/**
 * Generate an X25519 keypair using the Web Crypto API and return Base64-encoded strings.
 * @async
 * @function generateKeypair
 * @returns {Promise<{ publicKey: string, privateKey: string }>} Resolves with an object containing
 *   - publicKey: the Base64-encoded public key
 *   - privateKey: the Base64-encoded private key
 */
export async function generateKeypair() {
  // 1. Generate an X25519 key pair
  const keyPair = await crypto.subtle.generateKey(
      { name: 'X25519', namedCurve: 'X25519' },
      true,                 // extractable
      ['deriveBits']        // allowed usage for ECDH
  );

  // 2. Export keys as JWK to access raw key material
  const pubJwk  = await crypto.subtle.exportKey('jwk', keyPair.publicKey);
  const privJwk = await crypto.subtle.exportKey('jwk', keyPair.privateKey);

  // 3. Convert Base64URL to standard Base64 with padding
  return {
    publicKey:  b64urlToB64(pubJwk.x),
    privateKey: b64urlToB64(privJwk.d)
  };
}

/**
 * Build a wg-quick configuration for a peer, mirroring internal/app/configfile/tpl_files/wg_peer.tpl.
 * @function buildWgQuickConfig
 * @param {object} peer - The peer as returned by the API.
 * @param {string} privateKey - The Base64-encoded private key that only exists in the browser.
 * @returns {string} The wg-quick configuration.
 */
function buildWgQuickConfig(peer, privateKey) {
  const lines = [
    '[Interface]',
    `PrivateKey = ${privateKey}`,
    `Address = ${peer.Addresses.join(', ')}`,
  ]

  if (peer.Dns.Value.length) {
    lines.push(`DNS = ${[...peer.Dns.Value, ...peer.DnsSearch.Value].join(', ')}`)
  }
  if (peer.Mtu.Value !== 0) lines.push(`MTU = ${peer.Mtu.Value}`)
  if (peer.RoutingTable.Value !== '') lines.push(`Table = ${peer.RoutingTable.Value}`)
  if (peer.FirewallMark.Value !== 0) lines.push(`FwMark = ${peer.FirewallMark.Value}`)
  if (peer.PreUp.Value) lines.push(`PreUp = ${peer.PreUp.Value}`)
  if (peer.PostUp.Value) lines.push(`PostUp = ${peer.PostUp.Value}`)
  if (peer.PreDown.Value) lines.push(`PreDown = ${peer.PreDown.Value}`)
  if (peer.PostDown.Value) lines.push(`PostDown = ${peer.PostDown.Value}`)

  lines.push(
    '',
    '[Peer]',
    `PublicKey = ${peer.EndpointPublicKey.Value}`,
    `Endpoint = ${peer.Endpoint.Value}`,
  )

  if (peer.AllowedIPs.Value.length) lines.push(`AllowedIPs = ${peer.AllowedIPs.Value.join(', ')}`)
  if (peer.PresharedKey) lines.push(`PresharedKey = ${peer.PresharedKey}`)
  if (peer.PersistentKeepalive.Value !== 0 && peer.Mode === 'client') {
    lines.push(`PersistentKeepalive = ${peer.PersistentKeepalive.Value}`)
  }

  return lines.join('\n') + '\n'
}

/**
 * Download a wg-quick configuration for a peer whose private key only exists in the browser.
 * @function downloadWgQuickConfig
 * @param {object} peer - The peer as returned by the API.
 * @param {string} privateKey - The Base64-encoded private key.
 */
export function downloadWgQuickConfig(peer, privateKey) {
  let element = document.createElement('a')
  element.setAttribute('href', 'data:application/octet-stream;charset=utf-8,' + encodeURIComponent(buildWgQuickConfig(peer, privateKey)))
  element.setAttribute('download', peer.Filename)

  element.style.display = 'none'
  document.body.appendChild(element)

  element.click()
  document.body.removeChild(element)
}
