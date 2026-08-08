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
  public_host: string;
  public_port: number;
  sni: string;
  tls_insecure: boolean;
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
  usage_enabled: boolean;
  usage_available: boolean;
  usage_outbox_batches: number;
  usage_error_code: string;
  usage_sampled_at: string | null;
  traffic_upload_bytes: number;
  traffic_download_bytes: number;
  traffic_unattributed_bytes: number;
  traffic_last_report_at: string | null;
  online_users: number;
  online_connections: number;
  online_unknown_users: number;
  online_sampled_at: string | null;
  online_last_report_at: string | null;
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
  public_host: string;
  public_port: number;
  sni: string;
  tls_insecure: boolean;
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

export type UserStatus = "active" | "disabled" | "expired";
export type AssignmentState = "pending" | "applied" | "failed";
export type QuotaState = "unlimited" | "active" | "limited";

export interface UserAssignment {
  id: string;
  node_id: string;
  node_name: string;
  node_adapter: AdapterType;
  enabled: boolean;
  traffic_limit_bytes: number;
  traffic_upload_bytes: number;
  traffic_download_bytes: number;
  traffic_used_bytes: number;
  quota_state: QuotaState;
  last_traffic_at: string | null;
  online_connections: number;
  online_sampled_at: string | null;
  kick_generation: number;
  credential_fingerprint: string;
  desired_version: number;
  applied_version: number;
  state: AssignmentState;
  last_error_code: string;
  last_error_message: string;
  last_attempt_at: string | null;
  applied_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface UserRecord {
  id: string;
  username: string;
  display_name: string;
  notes: string;
  enabled: boolean;
  expires_at: string | null;
  status: UserStatus;
  traffic_limit_bytes: number;
  traffic_upload_bytes: number;
  traffic_download_bytes: number;
  traffic_used_bytes: number;
  quota_state: QuotaState;
  last_traffic_at: string | null;
  online_connections: number;
  online_nodes: number;
  assignments: UserAssignment[];
  created_at: string;
  updated_at: string;
}

export interface UserInput {
  username: string;
  display_name: string;
  notes: string;
  enabled: boolean;
  expires_at: string | null;
  traffic_limit_bytes: number;
  node_ids: string[];
}

export interface AssignmentInput {
  enabled?: boolean;
  traffic_limit_bytes?: number;
}

export interface UserCredential {
  node_id: string;
  node_name: string;
  credential: string;
  credential_fingerprint: string;
}

export interface CreateUserResponse {
  user: UserRecord;
  credentials: UserCredential[];
}

export interface AssignUserResponse {
  user: UserRecord;
  credential: UserCredential;
}

export type SubscriptionFormat = "uri" | "base64" | "clash" | "sing-box";
export type SubscriptionTokenStatus = "active" | "expired" | "revoked";

export interface SubscriptionTokenRecord {
  id: string;
  user_id: string;
  name: string;
  token_prefix: string;
  allowed_formats: SubscriptionFormat[];
  status: SubscriptionTokenStatus;
  expires_at: string | null;
  last_used_at: string | null;
  revoked_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface SubscriptionTokenInput {
  name: string;
  allowed_formats: SubscriptionFormat[];
  expires_at: string | null;
}

export interface SubscriptionURLs {
  uri?: string;
  base64?: string;
  clash?: string;
  sing_box?: string;
}

export interface IssuedSubscriptionToken {
  subscription: SubscriptionTokenRecord;
  token: string;
  urls: SubscriptionURLs;
}
