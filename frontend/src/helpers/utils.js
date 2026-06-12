import { Address4, Address6 } from "ip-address"

export function ipToBigInt(ip) {
  // Check if it's an IPv4 address
  if (ip.includes(".")) {
    const addr = new Address4(ip)
    return addr.bigInt()
  }

  // Otherwise, assume it's an IPv6 address
  const addr = new Address6(ip)
  return addr.bigInt()
}

export function humanFileSize(size) {
  const sizes = ["B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB"]
  if (size === 0) return "0B"
  const i = parseInt(Math.floor(Math.log(size) / Math.log(1024)))
  return Math.round(size / Math.pow(1024, i), 2) + sizes[i]
}

// Formats an ISO date or datetime string as "DD.MM.YYYY HH:MM UTC".
export function formatDateTime(value) {
  if (!value) return String(value ?? '')
  try {
    const d = new Date(value)
    if (isNaN(d.getTime())) return String(value)
    const pad = (n) => String(n).padStart(2, '0')
    const day   = pad(d.getUTCDate())
    const month = pad(d.getUTCMonth() + 1)
    const year  = d.getUTCFullYear()
    const hours = pad(d.getUTCHours())
    const mins  = pad(d.getUTCMinutes())
    return `${day}.${month}.${year} ${hours}:${mins} UTC`
  } catch {
    return String(value)
  }
}
