import { Address4, Address6 } from "ip-address"

export function ipToBigInt(ip) {
  // Check if it's an IPv4 address
  if (ip.includes(".")) {
    const addr = new Address4(ip)
    return addr.bigInteger()
  }

  // Otherwise, assume it's an IPv6 address
  const addr = new Address6(ip)
  return addr.bigInteger()
}
