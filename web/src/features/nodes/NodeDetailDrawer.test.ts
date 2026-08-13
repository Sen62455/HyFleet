import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import type { NodeRecord } from "../../types";
import NodeDetailDrawer from "./NodeDetailDrawer.vue";

const realityNode = {
  id: "node-reality",
  name: "Reality LA",
  provider: "Lisa",
  region: "Los Angeles",
  adapter_type: "sing_box_vless_reality",
  public_host: "reality.example.com",
  public_port: 443,
  sni: "www.cloudflare.com",
  tls_insecure: false,
  tls_cert_fingerprint: "",
  tls_public_key_sha256: "",
  reality: {
    handshake_server: "www.cloudflare.com",
    handshake_port: 443,
    key_generation: 1,
    applied_key_generation: 1,
    public_key: "reality-public-key",
    short_id: "0123456789abcdef",
    material_applied_version: 3,
    material_reported_at: "2026-08-12T08:00:00Z",
  },
  enabled: true,
  status: "online",
  status_reason: "",
  desired_version: 3,
  applied_version: 3,
  agent_version: "v1.3.0-experimental",
  os_name: "Ubuntu",
  os_version: "24.04",
  architecture: "amd64",
  core_name: "sing-box",
  core_version: "1.13.18",
  core_running: true,
  usage_enabled: true,
  usage_available: true,
  traffic_upload_bytes: 1024,
  traffic_download_bytes: 2048,
  traffic_unattributed_bytes: 0,
  traffic_used_bytes: 3072,
  online_users: 1,
  online_connections: 2,
  online_unknown_users: 0,
  uptime_seconds: 3600,
  cpu_percent: 2,
  memory_used_bytes: 128 * 1024 ** 2,
  memory_total_bytes: 1024 * 1024 ** 2,
  disk_used_bytes: 1024 ** 3,
  disk_total_bytes: 10 * 1024 ** 3,
  network_rx_bps: 0,
  network_tx_bps: 0,
  load_1: 0.01,
  load_5: 0.02,
  load_15: 0.03,
  last_seen_at: "2026-08-12T08:00:00Z",
  last_applied_at: "2026-08-12T08:00:00Z",
} as NodeRecord;

describe("NodeDetailDrawer", () => {
  it("shows Reality endpoint identity and user-control telemetry", async () => {
    const wrapper = mount(NodeDetailDrawer, {
      attachTo: document.body,
      props: { show: true, node: realityNode },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("VLESS + Reality（sing-box）");
    expect(wrapper.text()).toContain("VLESS / TCP / Reality");
    expect(wrapper.text()).toContain("www.cloudflare.com:443");
    expect(wrapper.text()).toContain("已应用 · 第 1 代");
    expect(wrapper.text()).toContain("目标身份代际");
    expect(wrapper.text()).toContain("在线用户 / 活跃连接");
    expect(wrapper.text()).toContain("1 / 2");
    expect(wrapper.text()).toContain("当前周期有效用量");
    expect(wrapper.text()).not.toContain("暂不支持");
    expect(wrapper.text()).not.toContain("证书验证");
    wrapper.unmount();
  });
});
