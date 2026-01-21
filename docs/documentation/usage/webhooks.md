
Webhooks allow WireGuard Portal to notify external services about events such as user creation, device changes, or configuration updates. This enables integration with other systems and automation workflows.

When webhooks are configured and a specified event occurs, WireGuard Portal sends an HTTP **POST** request to the configured webhook URL. 
The payload contains event-specific data in JSON format.

## Configuration

All available configuration options for webhooks can be found in the [configuration overview](../configuration/overview.md#webhook).

A basic webhook configuration looks like this:

```yaml
webhook:
  url: https://your-service.example.com/webhook
```

### Security

Webhooks can be secured by using a shared secret. This secret is included in the `Authorization` header of the webhook request, allowing your service to verify the authenticity of the request.
You can set the shared secret in the webhook configuration:

```yaml
webhook:
  url: https://your-service.example.com/webhook
  secret: "Basic dXNlcm5hbWU6cGFzc3dvcmQ="
```

You should also make sure that your webhook endpoint is secured with HTTPS to prevent eavesdropping and tampering.

## Available Events

WireGuard Portal supports various events that can trigger webhooks. The following events are available:

- `create`: Triggered when a new entity is created.
- `update`: Triggered when an existing entity is updated.
- `delete`: Triggered when an entity is deleted.
- `connect`: Triggered when a user connects to the VPN.
- `disconnect`: Triggered when a user disconnects from the VPN.

The following entity models are supported for webhook events:

- `user`: WireGuard Portal users support creation, update, or deletion events.
- `peer`: Peers support creation, update, or deletion events. Via the `peer_metric` entity, you can also receive connection status updates.
- `peer_metric`: Peer metrics support connection status updates, such as when a peer connects or disconnects.
- `interface`: WireGuard interfaces support creation, update, or deletion events.

## Payload Structure

All webhook events send a JSON payload containing relevant data. The structure of the payload depends on the event type and entity involved.
A common shell structure for webhook payloads is as follows:

```json
{
  "event": "create", // The event type, e.g. "create", "update", "delete", "connect", "disconnect"
  "entity": "user",  // The entity type, e.g. "user", "peer", "peer_metric", "interface"
  "identifier": "the-user-identifier", // Unique identifier of the entity, e.g. user ID or peer ID
  "payload": {
    // The payload of the event, e.g. a Peer model.
    // Detailed model descriptions are provided below.
  }
}
```

### Payload Models

All payload models are encoded as JSON objects. Fields with empty values might be omitted in the payload.

#### User Payload (entity: `user`)

| JSON Field     | Type          | Description                       |
|----------------|---------------|-----------------------------------|
| CreatedBy      | string        | Creator identifier                |
| UpdatedBy      | string        | Last updater identifier           |
| CreatedAt      | time.Time     | Time of creation                  |
| UpdatedAt      | time.Time     | Time of last update               |
| Identifier     | string        | Unique user identifier            |
| Email          | string        | User email                        |
| AuthSources    | []AuthSource  | Authentication sources            |
| IsAdmin        | bool          | Whether user has admin privileges |
| Firstname      | string        | User's first name (optional)      |
| Lastname       | string        | User's last name (optional)       |
| Phone          | string        | Contact phone number (optional)   |
| Department     | string        | User's department (optional)      |
| Notes          | string        | Additional notes (optional)       |
| Disabled       | *time.Time    | When user was disabled            |
| DisabledReason | string        | Reason for deactivation           |
| Locked         | *time.Time    | When user account was locked      |
| LockedReason   | string        | Reason for being locked           |

`AuthSource`:

| JSON Field   | Type          | Description                                         |
|--------------|---------------|-----------------------------------------------------|
| Source       | string        | The authentication source (e.g. LDAP, OAuth, or DB) |
| ProviderName | string        | The identifier of the authentication provider       |


#### Peer Payload (entity: `peer`)

| JSON Field           | Type       | Description                            |
|----------------------|------------|----------------------------------------|
| CreatedBy            | string     | Creator identifier                     |
| UpdatedBy            | string     | Last updater identifier                |
| CreatedAt            | time.Time  | Creation timestamp                     |
| UpdatedAt            | time.Time  | Last update timestamp                  |
| Endpoint             | string     | Peer endpoint address                  |
| EndpointPublicKey    | string     | Public key of peer endpoint            |
| AllowedIPsStr        | string     | Allowed IPs                            |
| ExtraAllowedIPsStr   | string     | Extra allowed IPs                      |
| PresharedKey         | string     | Pre-shared key for encryption          |
| PersistentKeepalive  | int        | Keepalive interval in seconds          |
| DisplayName          | string     | Display name of the peer               |
| Identifier           | string     | Unique identifier                      |
| UserIdentifier       | string     | Associated user ID (optional)          |
| InterfaceIdentifier  | string     | Interface this peer is attached to     |
| Disabled             | *time.Time | When the peer was disabled             |
| DisabledReason       | string     | Reason for being disabled              |
| ExpiresAt            | *time.Time | Expiration date                        |
| Notes                | string     | Notes for this peer                    |
| AutomaticallyCreated | bool       | Whether peer was auto-generated        |
| PrivateKey           | string     | Peer private key                       |
| PublicKey            | string     | Peer public key                        |
| InterfaceType        | string     | Type of the peer interface             |
| Addresses            | []string   | IP addresses                           |
| CheckAliveAddress    | string     | Address used for alive checks          |
| DnsStr               | string     | DNS servers                            |
| DnsSearchStr         | string     | DNS search domains                     |
| Mtu                  | int        | MTU (Maximum Transmission Unit)        |
| FirewallMark         | uint32     | Firewall mark (optional)               |
| RoutingTable         | string     | Custom routing table (optional)        |
| PreUp                | string     | Command before bringing up interface   |
| PostUp               | string     | Command after bringing up interface    |
| PreDown              | string     | Command before bringing down interface |
| PostDown             | string     | Command after bringing down interface  |


#### Interface Payload (entity: `interface`)

| JSON Field                 | Type       | Description                            |
|----------------------------|------------|----------------------------------------|
| CreatedBy                  | string     | Creator identifier                     |
| UpdatedBy                  | string     | Last updater identifier                |
| CreatedAt                  | time.Time  | Creation timestamp                     |
| UpdatedAt                  | time.Time  | Last update timestamp                  |
| Identifier                 | string     | Unique identifier                      |
| PrivateKey                 | string     | Private key for the interface          |
| PublicKey                  | string     | Public key for the interface           |
| ListenPort                 | int        | Listening port                         |
| Addresses                  | []string   | IP addresses                           |
| DnsStr                     | string     | DNS servers                            |
| DnsSearchStr               | string     | DNS search domains                     |
| Mtu                        | int        | MTU (Maximum Transmission Unit)        |
| FirewallMark               | uint32     | Firewall mark                          |
| RoutingTable               | string     | Custom routing table                   |
| PreUp                      | string     | Command before bringing up interface   |
| PostUp                     | string     | Command after bringing up interface    |
| PreDown                    | string     | Command before bringing down interface |
| PostDown                   | string     | Command after bringing down interface  |
| SaveConfig                 | bool       | Whether to save config to file         |
| DisplayName                | string     | Human-readable name                    |
| Type                       | string     | Type of interface                      |
| DriverType                 | string     | Driver used                            |
| Disabled                   | *time.Time | When the interface was disabled        |
| DisabledReason             | string     | Reason for being disabled              |
| PeerDefNetworkStr          | string     | Default peer network configuration     |
| PeerDefDnsStr              | string     | Default peer DNS servers               |
| PeerDefDnsSearchStr        | string     | Default peer DNS search domains        |
| PeerDefEndpoint            | string     | Default peer endpoint                  |
| PeerDefAllowedIPsStr       | string     | Default peer allowed IPs               |
| PeerDefMtu                 | int        | Default peer MTU                       |
| PeerDefPersistentKeepalive | int        | Default keepalive value                |
| PeerDefFirewallMark        | uint32     | Default firewall mark for peers        |
| PeerDefRoutingTable        | string     | Default routing table for peers        |
| PeerDefPreUp               | string     | Default peer pre-up command            |
| PeerDefPostUp              | string     | Default peer post-up command           |
| PeerDefPreDown             | string     | Default peer pre-down command          |
| PeerDefPostDown            | string     | Default peer post-down command         |


#### Peer Metrics Payload (entity: `peer_metric`)

| JSON Field | Type       | Description                |
|------------|------------|----------------------------|
| Status     | PeerStatus | Current status of the peer |
| Peer       | Peer       | Peer  data                 |

`PeerStatus` sub-structure:

| JSON Field       | Type       | Description                  |
|------------------|------------|------------------------------|
| UpdatedAt        | time.Time  | Time of last status update   |
| IsConnected      | bool       | Is peer currently connected  |
| IsPingable       | bool       | Can peer be pinged           |
| LastPing         | *time.Time | Time of last successful ping |
| BytesReceived    | uint64     | Bytes received from peer     |
| BytesTransmitted | uint64     | Bytes sent to peer           |
| Endpoint         | string     | Last known endpoint          |
| LastHandshake    | *time.Time | Last successful handshake    |
| LastSessionStart | *time.Time | Time the last session began  |


### Example Payloads

The following payload is an example of a webhook event when a peer connects to the VPN:

```json
{
  "event": "connect",
  "entity": "peer_metric",
  "identifier": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
  "payload": {
    "Status": {
      "UpdatedAt": "2025-06-27T22:20:08.734900034+02:00",
      "IsConnected": true,
      "IsPingable": false,
      "BytesReceived": 212,
      "BytesTransmitted": 2884,
      "Endpoint": "10.55.66.77:58756",
      "LastHandshake": "2025-06-27T22:19:46.580842776+02:00",
      "LastSessionStart": "2025-06-27T22:19:46.580842776+02:00"
    },
    "Peer": {
      "CreatedBy": "admin@wgportal.local",
      "UpdatedBy": "admin@wgportal.local",
      "CreatedAt": "2025-06-26T21:43:49.251839574+02:00",
      "UpdatedAt": "2025-06-27T22:18:39.67763985+02:00",
      "Endpoint": "10.55.66.1:51820",
      "EndpointPublicKey": "eiVibpi3C2PUPcx2kwA5s09OgHx7AEaKMd33k0LQ5mM=",
      "AllowedIPsStr": "10.11.12.0/24,fdfd:d3ad:c0de:1234::/64",
      "ExtraAllowedIPsStr": "",
      "PresharedKey": "p9DDeLUSLOdQcjS8ZsBAiqUzwDIUvTyzavRZFuzhvyE=",
      "PersistentKeepalive": 16,
      "DisplayName": "Peer Fb5TaziA",
      "Identifier": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
      "UserIdentifier": "admin@wgportal.local",
      "InterfaceIdentifier": "wgTesting",
      "AutomaticallyCreated": false,
      "PrivateKey": "QBFNBe+7J49ergH0ze2TGUJMFrL/2bOL50Z2cgluYW8=",
      "PublicKey": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
      "InterfaceType": "client",
      "Addresses": [
        "10.11.12.10/32",
        "fdfd:d3ad:c0de:1234::a/128"
      ],
      "CheckAliveAddress": "",
      "DnsStr": "",
      "DnsSearchStr": "",
      "Mtu": 1420
    }
  }
}
```

Here is another example of a webhook event when a peer is updated:

```json
{
  "event": "update",
  "entity": "peer",
  "identifier": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
  "payload": {
    "CreatedBy": "admin@wgportal.local",
    "UpdatedBy": "admin@wgportal.local",
    "CreatedAt": "2025-06-26T21:43:49.251839574+02:00",
    "UpdatedAt": "2025-06-27T22:18:39.67763985+02:00",
    "Endpoint": "10.55.66.1:51820",
    "EndpointPublicKey": "eiVibpi3C2PUPcx2kwA5s09OgHx7AEaKMd33k0LQ5mM=",
    "AllowedIPsStr": "10.11.12.0/24,fdfd:d3ad:c0de:1234::/64",
    "ExtraAllowedIPsStr": "",
    "PresharedKey": "p9DDeLUSLOdQcjS8ZsBAiqUzwDIUvTyzavRZFuzhvyE=",
    "PersistentKeepalive": 16,
    "DisplayName": "Peer Fb5TaziA",
    "Identifier": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
    "UserIdentifier": "admin@wgportal.local",
    "InterfaceIdentifier": "wgTesting",
    "AutomaticallyCreated": false,
    "PrivateKey": "QBFNBe+7J49ergH0ze2TGUJMFrL/2bOL50Z2cgluYW8=",
    "PublicKey": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
    "InterfaceType": "client",
    "Addresses": [
      "10.11.12.10/32",
      "fdfd:d3ad:c0de:1234::a/128"
    ],
    "CheckAliveAddress": "",
    "DnsStr": "",
    "DnsSearchStr": "",
    "Mtu": 1420
  }
}
```