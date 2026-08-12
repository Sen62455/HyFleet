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
  it("shows Reality endpoint identity and explicit MVP capability limits", async () => {
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
    expect(wrapper.text()).toContain("按用户流量");
    expect(wrapper.text()).toContain("在线状态");
    expect(wrapper.text()).toContain("踢下线");
    expect(wrapper.text()).toContain("额度执行");
    expect(wrapper.text()).not.toContain("证书验证");
    wrapper.unmount();
  });
});
