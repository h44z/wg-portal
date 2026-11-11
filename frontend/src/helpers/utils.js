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

// Returns the UTC calendar date ("YYYY-MM-DD") of an RFC3339 timestamp, as expected by <input type="date">.
// Empty or invalid values result in an empty string.
export function rfc3339ToDateInput(value) {
  if (!value) return ''
  const d = new Date(value)
  if (isNaN(d.getTime())) return ''
  return d.toISOString().substring(0, 10)
}

// Converts a "YYYY-MM-DD" value of <input type="date"> to an RFC3339 timestamp (midnight UTC).
// An empty value results in null (no expiry).
export function dateInputToRfc3339(value) {
  if (!value) return null
  return `${value}T00:00:00Z`
}

// Creates the getter/setter pair used to bind an RFC3339 field to <input type="date">.
// If the selected date equals the date of the current value, the value is kept unchanged so that
// the time-of-day of an existing expiry date (e.g. set by the peer rotation interval) is preserved.
export function rfc3339DateInputModel(getValue, setValue) {
  return {
    get: () => rfc3339ToDateInput(getValue()),
    set: (dateValue) => {
      if (dateValue === rfc3339ToDateInput(getValue())) return
      setValue(dateInputToRfc3339(dateValue))
    },
  }
}
