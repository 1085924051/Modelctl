export class ModelctlError extends Error {}

export class Modelctl {
  constructor(baseUrl = "http://127.0.0.1:11435", token = process.env.MODELCTL_API_INVOKE_TOKEN || process.env.MODELCTL_API_TOKEN) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.token = token;
  }

  async invoke(capabilityId, input, metadata, version) {
    const suffix = version ? `?version=${encodeURIComponent(version)}` : "";
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/invoke${suffix}`, {
      method: "POST",
      body: JSON.stringify({ input, ...(metadata ? { metadata } : {}) }),
    });
  }

  async capabilities() {
    return (await this.request("/v1/capabilities")).items || [];
  }

  async schema(capabilityId, version) {
    const suffix = version ? `?version=${encodeURIComponent(version)}` : "";
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/schema${suffix}`);
  }

  async integration(capabilityId, version) {
    const suffix = version ? `?version=${encodeURIComponent(version)}` : "";
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/integration${suffix}`);
  }

  async openapi(capabilityId, version) {
    const suffix = version ? `?version=${encodeURIComponent(version)}` : "";
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/openapi${suffix}`);
  }

  async status(capabilityId, version) {
    const suffix = version ? `?version=${encodeURIComponent(version)}` : "";
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/status${suffix}`);
  }

  async runs() {
    return (await this.request("/v1/runs")).items || [];
  }

  async batch(capabilityId, items, version) {
    const suffix = version ? `?version=${encodeURIComponent(version)}` : "";
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/batch${suffix}`, {
      method: "POST",
      body: JSON.stringify({ items }),
    });
  }

  async task(taskId) {
    return this.request(`/v1/tasks/${encodeURIComponent(taskId)}`);
  }

  async waitTask(taskId, { pollMs = 500, timeoutMs = 3600000 } = {}) {
    const deadline = Date.now() + timeoutMs;
    while (true) {
      const result = await this.task(taskId);
      if (["succeeded", "failed", "cancelled"].includes(result.status)) return result;
      if (Date.now() >= deadline) throw new ModelctlError(`task ${taskId} did not finish before timeout`);
      await new Promise((resolve) => setTimeout(resolve, pollMs));
    }
  }

  async request(path, options = {}) {
    const headers = { accept: "application/json", ...(options.body ? { "content-type": "application/json" } : {}) };
    if (this.token) headers.authorization = `Bearer ${this.token}`;
    const response = await fetch(this.baseUrl + path, { ...options, headers });
    const body = await response.json().catch(() => ({}));
    if (!response.ok) throw new ModelctlError(body.error?.message || `Modelctl returned HTTP ${response.status}`);
    return body;
  }
}
