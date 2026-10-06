# Modelctl SDK examples

These small clients call the stable capability API. They do not expose model
files or require callers to build the raw Laya `state/questions` request.

```python
from modelctl_client import Modelctl

client = Modelctl("http://127.0.0.1:11435")
result = client.invoke("refund-check", {"text": "The customer was charged twice."})
print(result["output"])
```

```js
import { Modelctl } from "./modelctl-client.mjs";

const client = new Modelctl("http://127.0.0.1:11435");
const result = await client.invoke("refund-check", { text: "The customer was charged twice." });
console.log(result.output);
```

For enterprise server mode, pass the API token to the constructor or set
`MODELCTL_API_TOKEN`.

Batch jobs return a task identifier. The SDKs include a polling helper so a
queue worker can wait for completion without implementing the task protocol:

```python
task = client.batch("ticket-routing", [{"input": {"text": "..."}}])
finished = client.wait_task(task["task_id"])
for item in finished["results"]:
    print(item["index"], item["run_id"])
```

```js
const task = await client.batch("ticket-routing", [{ input: { text: "..." } }]);
const finished = await client.waitTask(task.task_id);
```

Once a model variant has been downloaded, the first capability call can start
the matching runtime automatically. Services do not need to manage instance
ports or adapter processes.
