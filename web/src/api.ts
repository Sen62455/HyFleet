import type { EnrollmentToken, NodeInput, NodeRecord, Session, SetupStatus } from "./types";

export class APIError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId: string;

  constructor(status: number, code: string, message: string, requestId = "") {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

let csrfToken = "";

function setSession(session: Session | null) {
  csrfToken = session?.csrf_token ?? "";
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body) {
    headers.set("Content-Type", "application/json");
  }
  if (!(["GET", "HEAD", "OPTIONS"].includes(method)) && csrfToken) {
    headers.set("X-CSRF-Token", csrfToken);
  }
  const response = await fetch(path, {
    ...init,
    credentials: "same-origin",
    headers,
  });
  if (!response.ok) {
    let code = "request_failed";
    let message = `请求失败（HTTP ${response.status}）`;
    let requestId = response.headers.get("X-Request-ID") ?? "";
    try {
      const payload = (await response.json()) as {
        error?: { code?: string; message?: string; request_id?: string };
      };
      code = payload.error?.code ?? code;
      message = payload.error?.message ?? message;
      requestId = payload.error?.request_id ?? requestId;
    } catch {
      // Preserve the generic HTTP error when the response is not JSON.
    }
    throw new APIError(response.status, code, message, requestId);
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export const api = {
  setupStatus: () => request<SetupStatus>("/api/v1/setup/status"),

  async bootstrap(input: { bootstrap_token: string; username: string; password: string }) {
    const session = await request<Session>("/api/v1/setup/bootstrap", {
      method: "POST",
      body: JSON.stringify(input),
    });
    setSession(session);
    return session;
  },

  async login(input: { username: string; password: string }) {
    const session = await request<Session>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify(input),
    });
    setSession(session);
    return session;
  },

  async session() {
    const session = await request<Session>("/api/v1/auth/session");
    setSession(session);
    return session;
  },

  async logout() {
    await request<void>("/api/v1/auth/logout", { method: "POST", body: "{}" });
    setSession(null);
  },

  async listNodes() {
    const result = await request<{ nodes: NodeRecord[] }>("/api/v1/nodes");
    return result.nodes;
  },

  createNode: (input: NodeInput) =>
    request<NodeRecord>("/api/v1/nodes", { method: "POST", body: JSON.stringify(input) }),

  updateNode: (id: string, input: Required<NodeInput>) =>
    request<NodeRecord>(`/api/v1/nodes/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(input),
    }),

  archiveNode: (id: string) =>
    request<void>(`/api/v1/nodes/${encodeURIComponent(id)}`, { method: "DELETE", body: "{}" }),

  createEnrollmentToken: (id: string) =>
    request<EnrollmentToken>(`/api/v1/nodes/${encodeURIComponent(id)}/enrollment-token`, {
      method: "POST",
      body: "{}",
    }),
};
