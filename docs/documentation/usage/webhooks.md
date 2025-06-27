
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

The following entity types can trigger webhooks:

- `user`: When a WireGuard Portal user is created, updated, or deleted.
- `peer`: When a peer is created, updated, or deleted. This entity can also trigger `connect` and `disconnect` events.
- `interface`: When a device is created, updated, or deleted.

## Payload Structure

All webhook events send a JSON payload containing relevant data. The structure of the payload depends on the event type and entity involved.
A common shell structure for webhook payloads is as follows:

```json
{
  "event": "create",
  "entity": "user",
  "identifier": "the-user-identifier",
  "payload": {
    // The payload of the event, e.g. peer data.
    // Check the API documentation for the exact structure.
  }
}
```


### Example Payload

The following payload is an example of a webhook event when a peer connects to the VPN:

```json
{
  "event": "connect",
  "entity": "peer",
  "identifier": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
  "payload": {
    "PeerId": "Fb5TaziAs1WrPBjC/MFbWsIelVXvi0hDKZ3YQM9wmU8=",
    "IsConnected": true,
    "IsPingable": false,
    "LastPing": null,
    "BytesReceived": 1860,
    "BytesTransmitted": 10824,
    "LastHandshake": "2025-06-26T23:04:33.325216659+02:00",
    "Endpoint": "10.55.66.77:33874",
    "LastSessionStart": "2025-06-26T22:50:40.10221606+02:00"
  }
}
```