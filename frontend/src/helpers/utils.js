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

export function humanFileSize(size) {
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"]
  if (size === 0) return "0B"
  const i = parseInt(Math.floor(Math.log(size) / Math.log(1024)))
  return Math.round(size / Math.pow(1024, i), 2) + sizes[i]
}
