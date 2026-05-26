# Migration: TTLLocked Feature (No Manual Action Required)

## Overview

The `TTLLocked` feature adds control over whether peer expiration dates should be updated by activity tracking or locked to an explicit value.

**IMPORTANT:** For existing peers, NO migration is needed. All old peers will have `ttl_locked = false` (default), which means:
- They continue to work exactly as before
- Activity tracking will continue to update their `ExpiresAt` dates
- The `default_user_ttl` behavior is preserved

## How TTLLocked Works

When `ttl_locked = false` (default):
- If peer has `ExpiresAt`: Activity tracking updates it (old behavior preserved)
- If peer has no `ExpiresAt`: Peer never expires

When `ttl_locked = true`:
- Activity tracking is blocked from updating `ExpiresAt`
- The explicit expiration date is respected exactly
- This is only set when user explicitly provides `ExpiresAt` via API

## Database Schema Change

GORM automatically adds the `ttl_locked` column on startup:
```sql
ALTER TABLE peers ADD COLUMN ttl_locked BOOLEAN DEFAULT false;
```

All existing records get `ttl_locked = false`, which preserves existing behavior.

## API Behavior (New)

### 1. Default TTL (activity-based):
```json
POST /provisioning/new-peer
{
  "InterfaceIdentifier": "wg0",
  "DisplayName": "My Peer"
}
```
Result: `ExpiresAt = now + DefaultUserTTL`, `TTLLocked = false` ✅ Updates on activity

### 2. Explicit Date (locked):
```json
POST /provisioning/new-peer
{
  "InterfaceIdentifier": "wg0",
  "DisplayName": "My Peer",
  "ExpiresAt": "2025-12-31T23:59:59Z"
}
```
Result: `ExpiresAt = 2025-12-31...`, `TTLLocked = true` 🔒 Never updated by activity

### 3. No Expiration (permanent):
```json
POST /provisioning/new-peer
{
  "InterfaceIdentifier": "wg0",
  "DisplayName": "My Peer",
  "DoNotExpire": true
}
```
Result: `ExpiresAt = null`, `TTLLocked = true` ♾️ Never expires (TTL locked to prevent updates on activity)

## Behavior Comparison

| Scenario | ExpiresAt | TTLLocked | Activity Updates | Result |
|----------|-----------|-----------|------------------|--------|
| **Old peer (after upgrade)** | `2025-01-15` | `false` | ✅ Yes | Works exactly as before |
| **New peer, no date** | `null` | `false` | - | Uses default_user_ttl on first activity |
| **New peer, explicit date** | `2025-12-31` | `true` | ❌ No | Locked to this date |
| **New peer, DoNotExpire** | `null` | `true` | ❌ No | Never expires (TTL locked to prevent updates) |

## Conclusion

✅ **Zero manual migration needed** - All existing peers continue to work unchanged.
✅ **Backward compatible** - Old behavior is preserved (default_user_ttl still works).
✅ **New feature opt-in** - Users must explicitly pass `ExpiresAt` to lock a date.
