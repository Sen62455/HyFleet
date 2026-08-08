import { flushPromises, mount } from "@vue/test-utils";
import { NDialogProvider, NMessageProvider } from "naive-ui";
import { defineComponent } from "vue";
import { afterEach, describe, expect, it, vi } from "vitest";
import { api } from "../../api";
import type { ConfigBackupRecord, NodeOperationRecord, NodeRecord } from "../../types";
import NodeOperationsPanel from "./NodeOperationsPanel.vue";

const node = {
  id: "node-1",
  name: "LisaHost",
  adapter_type: "native_hysteria2",
  core_name: "hysteria",
} as NodeRecord;

const failedOperation: NodeOperationRecord = {
  id: "operation-1",
  node_id: "node-1",
  node_name: "LisaHost",
  sequence: 4,
  type: "restart_core",
  status: "failed",
  attempt: 1,
  max_lines: 0,
  output: "service remained inactive",
  error_code: "core_restart_failed",
  error_message: "core restart failed",
  rolled_back: true,
  expires_at: "2026-08-08T01:15:00Z",
  started_at: "2026-08-08T01:00:00Z",
  completed_at: "2026-08-08T01:00:03Z",
  created_at: "2026-08-08T01:00:00Z",
  updated_at: "2026-08-08T01:00:03Z",
};

const backup: ConfigBackupRecord = {
  id: "backup-1",
  node_id: "node-1",
  node_name: "LisaHost",
  operation_id: "operation-1",
  local_path: "/var/lib/hyfleet-backups/restart-config.yaml.bak",
  sha256: "a".repeat(64),
  size_bytes: 2048,
  created_at: "2026-08-08T01:00:00Z",
};

afterEach(() => vi.restoreAllMocks());

describe("NodeOperationsPanel", () => {
  it("shows rollback evidence and queues a retry for a failed operation", async () => {
    vi.spyOn(api, "listNodeOperations").mockResolvedValue([failedOperation]);
    vi.spyOn(api, "listConfigBackups").mockResolvedValue([backup]);
    vi.spyOn(api, "retryNodeOperation").mockResolvedValue({
      ...failedOperation,
      id: "operation-2",
      sequence: 5,
      status: "queued",
      retry_of: failedOperation.id,
      attempt: 2,
      rolled_back: false,
    });

    const host = defineComponent({
      components: { NDialogProvider, NMessageProvider, NodeOperationsPanel },
      setup: () => ({ node }),
      template: `
        <n-dialog-provider>
          <n-message-provider>
            <node-operations-panel :node="node" />
          </n-message-provider>
        </n-dialog-provider>
      `,
    });
    const wrapper = mount(host, { attachTo: document.body });
    await flushPromises();

    expect(wrapper.text()).toContain("core_restart_failed");
    expect(wrapper.text()).toContain("已恢复最近可用配置");
    expect(wrapper.text()).toContain(backup.local_path);

    const retryButton = wrapper.findAll("button").find((button) => button.text().trim() === "重试");
    expect(retryButton).toBeDefined();
    await retryButton!.trigger("click");
    await flushPromises();

    expect(api.retryNodeOperation).toHaveBeenCalledWith("node-1", "operation-1");
    expect(api.listNodeOperations).toHaveBeenCalledTimes(2);
    wrapper.unmount();
  });
});
