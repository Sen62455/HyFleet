export type AdapterType = "native_hysteria2" | "standalone_sing_box" | "s_ui";

export type NodeStatus = "pending" | "online" | "stale" | "offline" | "degraded" | "disabled";

export interface Admin {
  id: string;
  username: string;
}

export interface Session {
  admin: Admin;
  csrf_token: string;
  expires_at: string;
}

export interface SetupStatus {
  setup_required: boolean;
  bootstrap_token_configured: boolean;
}

export interface NodeRecord {
  id: string;
  name: string;
  provider: string;
  region: string;
  adapter_type: AdapterType;
  enabled: boolean;
  status: NodeStatus;
  status_reason: string;
  desired_version: number;
  applied_version: number;
  agent_installation_id?: string;
  agent_version: string;
  protocol_version: number;
  os_name: string;
  os_version: string;
  architecture: string;
  core_name: string;
  core_version: string;
  core_running: boolean;
  uptime_seconds: number;
  cpu_percent: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  disk_used_bytes: number;
  disk_total_bytes: number;
  network_rx_bps: number;
  network_tx_bps: number;
  load_1: number;
  load_5: number;
  load_15: number;
  last_seen_at: string | null;
  last_applied_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface NodeInput {
  name: string;
  provider: string;
  region: string;
  adapter_type: AdapterType;
  enabled?: boolean;
}

export interface EnrollmentToken {
  node_id: string;
  enrollment_token: string;
  expires_at: string;
}

export interface APIErrorPayload {
  error?: {
    code?: string;
    message?: string;
    request_id?: string;
  };
}
