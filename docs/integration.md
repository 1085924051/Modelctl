# Modelctl integration guide

Modelctl is the deployment and capability layer. Application code calls a
published capability; it does not read model files or build Laya `state` and
`questions` payloads.

## Choose the operating mode

| Scenario | Where Modelctl runs | How the application calls it |
| --- | --- | --- |
| Individual developer, local prototype | The developer's computer | `http://127.0.0.1:11435` with REST, Python, or JavaScript SDK |
| Desktop product with an on-device AI feature | The end user's computer | The product calls the local capability endpoint through an app-side HTTP client |
| Internal enterprise service | A GPU/CPU host inside the company network | HTTPS through the internal gateway, with a bearer token and a capability ID |
| Async document or ticket processing | An internal Modelctl host | Submit `/batch`, poll the task, then store each run result in the business database |

The business system owns users, permissions, queues, and domain data. Modelctl
owns model lifecycle, adapter processes, capability contracts, and inference
execution. This boundary lets a model or prompt change without rewriting every
business integration.

## Local developer mode

1. Open Modelctl Desktop.
2. Download and start a model variant.
3. Create a capability, for example `refund-check`.
4. Copy the generated endpoint or SDK example.
5. Call the local daemon from the application.

```http
POST http://127.0.0.1:11435/v1/capabilities/refund-check/invoke
Content-Type: application/json

{"input":{"text":"The customer was charged twice and wants a refund."},"metadata":{"ticket_id":"T-100"}}
```

The client receives a stable capability version, typed answers, model metadata,
and a replayable run id.

For a first integration, the shortest path is:

```bash
curl http://127.0.0.1:11435/v1/capabilities
curl -X POST http://127.0.0.1:11435/v1/capabilities/refund-check/invoke \
  -H 'Content-Type: application/json' \
  -d '{"input":{"text":"The customer was charged twice."}}'
```

Use `GET /v1/capabilities/{id}/schema` before generating a form, validating a
queue message, or writing a contract test. Use `?version=1.0.0` when a service
needs a pinned contract during a gradual rollout.

After publishing, `GET /v1/capabilities/{id}/integration?version=1.0.0` returns
the complete handoff package used by the Desktop Integration page: the fixed
endpoint, authentication policy, request and response schemas, expected error
codes, and copyable curl, Python, and JavaScript examples. The same package is
available from `modelctl capabilities integration <id> --version 1.0.0` and the
`client.integration(...)` method in both SDKs.

For gateway registration and CI contract checks, use
`GET /v1/capabilities/{id}/openapi?version=1.0.0` or
`modelctl capabilities openapi <id> --version 1.0.0` to export an OpenAPI 3.1
document for the invoke and batch endpoints.

Use `GET /v1/capabilities/{id}/status?version=1.0.0` as a deployment or CI
readiness check. It reports whether the bound model is installed and whether a
matching runtime is ready, with a machine-readable `next_action`.

The repository includes a complete copyable example in
[`examples/integration`](../examples/integration): publish the capability,
inspect its request/response schema, invoke one ticket, and submit a batch of
tickets with business metadata.

After the model variant has been downloaded, the first capability invocation
can start its matching runtime automatically. The business service does not
need to call the instance lifecycle API. If the model has not been downloaded,
the API returns `MODEL_NOT_INSTALLED` with the model and variant to prepare in
Desktop or through the CLI.

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

Use `MODELCTL_API_INVOKE_TOKEN` for application traffic when possible. It is a
least-privilege token limited to capability discovery, readiness checks,
invocation, and batch task polling. Keep `MODELCTL_API_TOKEN` for administrators
who publish or activate capabilities and manage models.

The desktop Settings page can apply a remote URL and token for administration.
Business services should inject their token from the company's secret manager:

```bash
export MODELCTL_API_TOKEN='read-from-secret-manager'
curl https://modelctl.internal.example/v1/capabilities/refund-check/invoke \
  -H "Authorization: Bearer $MODELCTL_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"input":{"text":"The customer was charged twice."}}'
```

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

Published versions are immutable. A second publish to the same `id` and
`version` returns `CAPABILITY_VERSION_EXISTS`; increment the version before
changing questions, model binding, or output expectations. Batch tasks record
and use the active capability version captured at task creation, even if an
administrator activates another version while the batch is running.

## Integration choices

- REST: browser, Python, Java, Go, and internal services.
- CLI: scripts, local automation, CI tasks, and capability contract checks (`modelctl capabilities schema ...`).
- MCP: AI agents and desktop developer tools. Published capabilities are
  discovered as typed `modelctl_capability_<id>` tools; the raw
  `laya_system_one` tool remains available for adapter-level debugging.

The desktop console is the administration and test surface for all three modes.

## Common application patterns

### Synchronous request/response

Use one capability invocation inside an API request, such as ticket routing,
refund intent detection, document triage, or a finance risk check. Store the
returned `run_id` with the business record so an operator can replay the exact
capability version and model instance from Desktop History.
The optional `metadata` object is stored with the run and returned in the
response, which lets a tenant, ticket, or document ID stay attached to the
inference record without being sent to the model as text.
Each answer also includes a normalized `value` field: a number for `noul`, the
selected label for `choice`, or the score payload for `score`. The original
type-specific field (`noul`, `choice`, or `score`) remains available for clients
that need the full model response.

### Batch processing

Use `/v1/capabilities/{id}/batch` for imports, nightly jobs, or queues. Each
item has the same input shape as a single call. Poll the task URL and persist
successful results by item index. Terminal task results include the normalized
`output`, optional metadata, and `run_id`; use the run ID when an operator needs
to replay or inspect the raw adapter response. A failed item does not cancel the
rest of the batch.

### Contract rollout

Publish `1.1.0` with `activate: false`, run it against a sample batch, then
activate it when the output contract is accepted. Existing callers continue to
use the active version until they opt into the new version.
