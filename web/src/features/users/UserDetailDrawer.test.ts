import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import type { UserAssignment, UserRecord } from "../../types";
import UserDetailDrawer from "./UserDetailDrawer.vue";

const assignment: UserAssignment = {
  id: "assignment-1",
  node_id: "node-dmit",
  node_name: "DMIT",
  node_adapter: "s_ui",
  enabled: true,
  traffic_limit_bytes: 5 * 1024 ** 3,
  traffic_upload_bytes: 1024,
  traffic_download_bytes: 2048,
  traffic_used_bytes: 3072,
  quota_state: "active",
  last_traffic_at: "2026-08-08T01:00:00Z",
  online_connections: 1,
  online_sampled_at: "2026-08-08T01:00:00Z",
  kick_generation: 0,
  credential_fingerprint: "sha256:managed",
  management_mode: "managed",
  remote_client_id: 41,
  subscription_eligible: true,
  subscription_reason: "eligible",
  desired_version: 4,
  applied_version: 4,
  state: "applied",
  last_error_code: "",
  last_error_message: "",
  last_attempt_at: "2026-08-08T01:00:00Z",
  applied_at: "2026-08-08T01:00:00Z",
  created_at: "2026-08-08T00:00:00Z",
  updated_at: "2026-08-08T01:00:00Z",
};

const user: UserRecord = {
  id: "user-1",
  username: "alice",
  display_name: "Alice",
  notes: "",
  enabled: true,
  expires_at: null,
  status: "active",
  traffic_limit_bytes: 0,
  traffic_upload_bytes: 1024,
  traffic_download_bytes: 2048,
  traffic_used_bytes: 3072,
  quota_state: "unlimited",
  last_traffic_at: "2026-08-08T01:00:00Z",
  online_connections: 1,
  online_nodes: 1,
  assignments: [assignment],
  created_at: "2026-08-08T00:00:00Z",
  updated_at: "2026-08-08T01:00:00Z",
};

describe("UserDetailDrawer", () => {
  it("shows quota controls and subscription eligibility after S-UI adoption", async () => {
    const wrapper = mount(UserDetailDrawer, {
      attachTo: document.body,
      props: {
        show: true,
        user,
        assignableNodes: [],
        subscriptionTokens: [],
        subscriptionLoading: false,
        working: "",
      },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("DMIT");
    expect(wrapper.text()).toContain("已纳入订阅");
    expect(wrapper.text()).not.toContain("只读导入");
    expect(wrapper.find('button[aria-label="保存 DMIT 流量额度"]').exists()).toBe(true);
    wrapper.unmount();
  });
});
