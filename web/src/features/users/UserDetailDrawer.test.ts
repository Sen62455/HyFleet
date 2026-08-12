import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import type { SubscriptionTokenRecord, UserAssignment, UserRecord } from "../../types";
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

const activeToken: SubscriptionTokenRecord = {
  id: "token-active",
  user_id: user.id,
  name: "当前设备",
  token_prefix: "hys_active_",
  status: "active",
  allowed_formats: ["clash", "sing-box"],
  expires_at: null,
  revoked_at: null,
  last_used_at: "2026-08-08T01:00:00Z",
  created_at: "2026-08-08T00:00:00Z",
  updated_at: "2026-08-08T01:00:00Z",
};

const revokedToken: SubscriptionTokenRecord = {
  ...activeToken,
  id: "token-revoked",
  name: "已撤销设备",
  token_prefix: "hys_revoked_",
  status: "revoked",
  revoked_at: "2026-08-08T02:00:00Z",
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

  it("keeps revoked subscription tokens out of the current list until history is expanded", async () => {
    const wrapper = mount(UserDetailDrawer, {
      attachTo: document.body,
      props: {
        show: true,
        user,
        assignableNodes: [],
        subscriptionTokens: [activeToken, revokedToken],
        subscriptionLoading: false,
        working: "",
      },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("当前设备");
    expect(wrapper.text()).not.toContain("已撤销设备");
    expect(wrapper.text()).toContain("历史 Token 1");
    expect(wrapper.text()).toContain("1 个有效 Token");

    const historyButton = wrapper.find('button[aria-label="展开历史 Token"]');
    expect(historyButton.exists()).toBe(true);
    await historyButton.trigger("click");
    await flushPromises();

    expect(wrapper.text()).toContain("已撤销设备");
    expect(wrapper.text()).toContain("已撤销");

    await wrapper.setProps({ subscriptionTokens: [revokedToken] });
    await wrapper.find('button[aria-label="收起历史 Token"]').trigger("click");
    await flushPromises();

    expect(wrapper.text()).not.toContain("已撤销设备");
    expect(wrapper.text()).toContain("暂无有效订阅 Token");
    expect(wrapper.text()).toContain("0 个有效 Token");
    wrapper.unmount();
  });

  it("marks Reality traffic, online state, quotas, and kicking as unsupported", async () => {
    const realityAssignment: UserAssignment = {
      ...assignment,
      id: "assignment-reality",
      node_id: "node-reality",
      node_name: "Reality LA",
      node_adapter: "sing_box_vless_reality",
      traffic_limit_bytes: 0,
      traffic_upload_bytes: 0,
      traffic_download_bytes: 0,
      traffic_used_bytes: 0,
      online_connections: 0,
    };
    const realityUser: UserRecord = {
      ...user,
      online_connections: 0,
      online_nodes: 0,
      traffic_upload_bytes: 0,
      traffic_download_bytes: 0,
      traffic_used_bytes: 0,
      assignments: [realityAssignment],
    };
    const wrapper = mount(UserDetailDrawer, {
      attachTo: document.body,
      props: {
        show: true,
        user: realityUser,
        assignableNodes: [],
        subscriptionTokens: [],
        subscriptionLoading: false,
        working: "",
      },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("VLESS + Reality（sing-box）");
    expect(wrapper.text()).toContain("按用户流量、在线状态和踢下线暂不支持");
    expect(wrapper.text()).toContain("流量额度不会在此节点执行");
    expect(wrapper.find('button[aria-label="保存 Reality LA 流量额度"]').exists()).toBe(false);
    expect(wrapper.find('button[aria-label="将用户从 Reality LA 踢下线"]').exists()).toBe(false);
    wrapper.unmount();
  });

  it("explains why a Reality assignment is withheld from subscriptions", async () => {
    const realityAssignment: UserAssignment = {
      ...assignment,
      node_adapter: "sing_box_vless_reality",
      subscription_eligible: false,
      subscription_reason: "adapter_not_compatible",
    };
    const wrapper = mount(UserDetailDrawer, {
      attachTo: document.body,
      props: {
        show: true,
        user: { ...user, assignments: [realityAssignment] },
        assignableNodes: [],
        subscriptionTokens: [],
        subscriptionLoading: false,
        working: "",
      },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("Reality 适配器未通过兼容性检查");
    wrapper.unmount();
  });
});
