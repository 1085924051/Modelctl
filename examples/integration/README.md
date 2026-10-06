# Capability integration example

This example uses the same files for Desktop, CLI, REST, and SDK integration.

```bash
modelctl capabilities publish --json refund-check.capability.json
modelctl capabilities schema refund-check
modelctl capabilities invoke refund-check --json refund-check.invoke.json
modelctl capabilities batch refund-check --json refund-check.batch.json --wait
```

The model variant must be downloaded first. After that, the first capability
call starts the matching runtime automatically. `metadata` is returned with the
invocation and stored with its run record, so the caller can associate results
with tickets, tenants, documents, or other business records.
