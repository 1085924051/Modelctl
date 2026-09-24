# Laya System One Skill

This skill requires a running local `modelctl` daemon and the
`system_one` capability. It does not install models or start processes.

## Invocation

Call the MCP tool `laya_system_one` with:

```json
{
  "state": "input text or a structured state",
  "questions": {
    "question_id": {
      "type": "noul | choice | score",
      "instructions": "decision question"
    }
  }
}
```

Use `instance_id` only when a specific ready instance has been selected. The
tool returns Laya's structured answers, usage, and routing fields unchanged.

## Permissions

- Network: local HTTP requests to `127.0.0.1:11435` only.
- Filesystem: none.
- Model installation: not permitted.
