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
