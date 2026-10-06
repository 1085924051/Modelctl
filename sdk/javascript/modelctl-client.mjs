export class ModelctlError extends Error {}

export class Modelctl {
  constructor(baseUrl = "http://127.0.0.1:11435", token = process.env.MODELCTL_API_TOKEN) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.token = token;
  }

  async invoke(capabilityId, input, metadata) {
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/invoke`, {
      method: "POST",
      body: JSON.stringify({ input, ...(metadata ? { metadata } : {}) }),
    });
  }

  async capabilities() {
    return (await this.request("/v1/capabilities")).items || [];
  }

  async runs() {
    return (await this.request("/v1/runs")).items || [];
  }

  async batch(capabilityId, items) {
    return this.request(`/v1/capabilities/${encodeURIComponent(capabilityId)}/batch`, {
      method: "POST",
      body: JSON.stringify({ items }),
    });
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
