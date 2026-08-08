import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import type { AlertRecord } from "../../types";
import AlertDrawer from "./AlertDrawer.vue";

const alert: AlertRecord = {
  id: "alert-1",
  node_id: "node-1",
  node_name: "LisaHost",
  type: "core_down",
  severity: "critical",
  status: "open",
  message: "core service is not running",
  occurrence_count: 1,
  first_seen_at: "2026-08-08T01:00:00Z",
  last_seen_at: "2026-08-08T01:01:00Z",
  acknowledged_at: null,
  resolved_at: null,
  created_at: "2026-08-08T01:00:00Z",
  updated_at: "2026-08-08T01:01:00Z",
};

describe("AlertDrawer", () => {
  it("emits acknowledgement and node navigation from an active alert", async () => {
    const wrapper = mount(AlertDrawer, {
      attachTo: document.body,
      props: { show: true, alerts: [alert], loading: false, working: "" },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();

    expect(wrapper.text()).toContain("核心停止");
    expect(wrapper.text()).toContain("LisaHost");
    expect(wrapper.text()).toContain("core service is not running");

    const nodeButton = wrapper.findAll("button").find((button) => button.text().trim() === "LisaHost");
    expect(nodeButton).toBeDefined();
    await nodeButton!.trigger("click");
    expect(wrapper.emitted("select-node")?.[0]).toEqual(["node-1"]);

    const acknowledgeButton = wrapper.find('button[aria-label="确认 LisaHost 告警"]');
    expect(acknowledgeButton.exists()).toBe(true);
    await acknowledgeButton.trigger("click");
    expect(wrapper.emitted("acknowledge")?.[0]).toEqual([alert]);
    wrapper.unmount();
  });
});
