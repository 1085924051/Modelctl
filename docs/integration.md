# Modelctl integration guide

Modelctl is the deployment and capability layer. Application code calls a
published capability; it does not read model files or build Laya `state` and
`questions` payloads.

## Local developer mode

1. Open Modelctl Desktop.
2. Download and start a model variant.
3. Create a capability, for example `refund-check`.
4. Copy the generated endpoint or SDK example.
5. Call the local daemon from the application.

```http
POST http://127.0.0.1:11435/v1/capabilities/refund-check/invoke
Content-Type: application/json

{"input":{"text":"The customer was charged twice and wants a refund."}}
```

The client receives a stable capability version, typed answers, model metadata,
and a replayable run id.

## Enterprise server mode

Run Modelctl on an internal inference host. Remote binding is opt-in and
requires an API token:

```bash
MODELCTL_HOST=0.0.0.0 \
MODELCTL_API_TOKEN=replace-me \
node bin/modelctl.js daemon
```

Put the service behind the enterprise TLS gateway. Business applications send
the token in an `Authorization: Bearer` header. Keep adapter ports and the model
store private to the Modelctl host.

## Capability contract

A capability pins:

- a stable id and semantic version;
- the input field contract;
- the human-facing question definition;
- the model id, revision, variant, and runtime profile;
- the output and replay metadata.

Publishing a new output shape should create a new capability version. This lets
an application upgrade deliberately and keeps model upgrades from silently
breaking production integrations.

## Integration choices

- REST: browser, Python, Java, Go, and internal services.
- CLI: scripts, local automation, and CI tasks.
- MCP: AI agents and desktop developer tools.

The desktop console is the administration and test surface for all three modes.
