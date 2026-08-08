import { flushPromises, mount } from "@vue/test-utils";
import { NMessageProvider } from "naive-ui";
import { defineComponent } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../../api";
import type { NodeRecord, SUIState, UserRecord } from "../../types";
import SUIAdapterPanel from "./SUIAdapterPanel.vue";

const node = {
  id: "node-1",
  name: "DMIT",
  adapter_type: "s_ui",
  adapter_last_discovered_at: "2026-08-08T00:00:00Z",
} as NodeRecord;

const state: SUIState = {
  node_id: "node-1",
  adapter_status: "compatible",
  adapter_version: "v1.5.3",
  adapter_error_code: "",
  last_probed_at: "2026-08-08T00:00:00Z",
  last_discovered_at: "2026-08-08T00:00:00Z",
  target_inbound_ids: [7],
  inbounds: [
    {
      remote_id: 7,
      tag: "hy2-in",
      type: "hysteria2",
      listen: "::",
      listen_port: 443,
      observed_at: "2026-08-08T00:00:00Z",
    },
  ],
  clients: [
    {
      remote_id: 41,
      name: "existing-client",
      enabled: true,
      inbound_ids: [7],
      upload_bytes: 100,
      download_bytes: 200,
      expires_at: 0,
      online: true,
      observed_at: "2026-08-08T00:00:00Z",
      mapped_user_id: "user-1",
      mapped_username: "alice",
      management_mode: "read_only",
    },
  ],
};

const users = [
  {
    id: "user-1",
    username: "alice",
    display_name: "Alice",
    enabled: true,
    assignments: [
      {
        id: "assignment-1",
        node_id: "node-1",
        state: "pending",
        applied_version: 1,
        desired_version: 2,
        management_mode: "read_only",
      },
    ],
  },
] as UserRecord[];

afterEach(() => vi.restoreAllMocks());

describe("SUIAdapterPanel", () => {
  it("shows a read-only mapping and blocks adoption until the Agent applies it", async () => {
    vi.spyOn(api, "getSUIState").mockResolvedValue(state);
    vi.spyOn(api, "listUsers").mockResolvedValue(users);
    const host = defineComponent({
      components: { NMessageProvider, SUIAdapterPanel },
      setup: () => ({ node }),
      template: `
        <n-message-provider>
          <s-u-i-adapter-panel :node="node" />
        </n-message-provider>
      `,
    });

    const wrapper = mount(host, { attachTo: document.body });
    await flushPromises();

    expect(api.getSUIState).toHaveBeenCalledWith("node-1");
    expect(wrapper.text()).toContain("v1.5.3");
    expect(wrapper.text()).toContain("existing-client");
    expect(wrapper.text()).toContain("映射到 @alice");
    expect(wrapper.text()).not.toContain("password");
    const adoptButton = wrapper.findAll("button").find((button) => button.text().includes("接管"));
    expect(adoptButton?.attributes("disabled")).toBeDefined();

    wrapper.unmount();
  });

  it("keeps an applied read-only client non-adoptable until a target inbound is saved", async () => {
    vi.spyOn(api, "getSUIState").mockResolvedValue({ ...state, target_inbound_ids: [] });
    vi.spyOn(api, "listUsers").mockResolvedValue([
      {
        ...users[0],
        assignments: [
          {
            ...users[0].assignments[0],
            state: "applied",
            applied_version: 2,
            desired_version: 2,
          },
        ],
      },
    ]);
    const host = defineComponent({
      components: { NMessageProvider, SUIAdapterPanel },
      setup: () => ({ node }),
      template: `
        <n-message-provider>
          <s-u-i-adapter-panel :node="node" />
        </n-message-provider>
      `,
    });

    const wrapper = mount(host, { attachTo: document.body });
    await flushPromises();

    const adoptButton = wrapper.findAll("button").find((button) => button.text().includes("接管"));
    expect(adoptButton?.attributes("disabled")).toBeDefined();

    wrapper.unmount();
  });
});
