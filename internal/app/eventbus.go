package app

// region misc-events

const TopicAuthLogin = "auth:login"
const TopicRouteUpdate = "route:update"
const TopicRouteRemove = "route:remove"

// endregion misc-events

// region user-events

const TopicUserCreated = "user:created"
const TopicUserDeleted = "user:deleted"
const TopicUserUpdated = "user:updated"
const TopicUserApiEnabled = "user:api:enabled"
const TopicUserApiDisabled = "user:api:disabled"
const TopicUserRegistered = "user:registered"
const TopicUserDisabled = "user:disabled"
const TopicUserEnabled = "user:enabled"

// endregion user-events

// region interface-events

const TopicInterfaceCreated = "interface:created"
const TopicInterfaceUpdated = "interface:updated"
const TopicInterfaceDeleted = "interface:deleted"
const TopicInterfaceStatsUpdated = "interface:stats:updated"

// endregion interface-events

// region peer-events

const TopicPeerCreated = "peer:created"
const TopicPeerDeleted = "peer:deleted"
const TopicPeerUpdated = "peer:updated"
const TopicPeerInterfaceUpdated = "peer:interface:updated"
const TopicPeerIdentifierUpdated = "peer:identifier:updated"
const TopicPeerStateChanged = "peer:state:changed"
const TopicPeerStatsUpdated = "peer:stats:updated"

// endregion peer-events

// region audit-events

const TopicAuditLoginSuccess = "audit:login:success"
const TopicAuditLoginFailed = "audit:login:failed"

const TopicAuditInterfaceChanged = "audit:interface:changed"
const TopicAuditPeerChanged = "audit:peer:changed"

// endregion audit-events
