# Integration center

Modelctl is integrated into a business system through a published capability. A
customer application depends on a capability ID, a version, and its request /
response schema. It does not depend on the model file or the raw Laya/Jev
`state/questions` protocol.

After publishing a capability, open **Integration** in Desktop or run:

```bash
modelctl capabilities integration refund-check --version 1.0.0
```

The response contains the fixed invocation endpoint, authentication policy,
request and response schemas, expected errors, and copyable curl, Python, and
JavaScript examples. The same contract is available to SDK users:

```python
contract = client.integration("refund-check", "1.0.0")
print(contract["endpoint"])
```

For API gateway registration or CI contract checks, fetch the generated OpenAPI
document:

```bash
modelctl capabilities openapi refund-check --version 1.0.0 > refund-check.openapi.json
```

It describes the versioned synchronous and batch endpoints and adds the bearer
security scheme when the daemon is running in enterprise mode.

Before switching a business service to production, check deployment readiness:

```bash
modelctl capabilities status refund-check --version 1.0.0
```

The result distinguishes a missing model (`download_model`), an installed model
whose runtime has not started yet, and a ready capability. This check does not
run a business request or mutate runtime state.

For local use, a loopback daemon can be called without a token. For an
enterprise daemon bound to a non-loopback address, set `MODELCTL_API_TOKEN` and
keep it in the business service's secret manager. The Desktop app never copies
the token into examples.

For production business services, configure `MODELCTL_API_INVOKE_TOKEN` instead
of sharing the administrator token. This token can inspect published contracts,
check readiness, invoke capabilities, and poll batch tasks, but cannot publish
or activate capabilities, manage models, or stop runtimes.

Production services should pin a capability version and persist `run_id`,
`capability_version`, and their own `metadata` (for example a ticket or order
ID). When a new contract is ready, publish it as a new semantic version, test
it, and activate it explicitly.
