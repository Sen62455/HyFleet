import { mount } from "@vue/test-utils";
import { NMessageProvider } from "naive-ui";
import { defineComponent, nextTick } from "vue";
import { describe, expect, it, vi } from "vitest";
import SubscriptionTokenDialog from "./SubscriptionTokenDialog.vue";
import type { IssuedSubscriptionToken } from "../../types";

const issued: IssuedSubscriptionToken = {
  subscription: {
    id: "token-1",
    user_id: "user-1",
    name: "QA Token",
    token_prefix: "hys_example_",
    allowed_formats: ["uri", "base64", "clash", "sing-box"],
    status: "active",
    expires_at: null,
    last_used_at: null,
    revoked_at: null,
    created_at: "2026-08-08T00:00:00Z",
    updated_at: "2026-08-08T00:00:00Z",
  },
  token: "hys_example_secret",
  urls: {
    uri: "https://panel.example/sub/hys_example_secret/uri",
    base64: "https://panel.example/sub/hys_example_secret/base64",
    clash: "https://panel.example/sub/hys_example_secret/clash",
    sing_box: "https://panel.example/sub/hys_example_secret/sing-box",
  },
};

describe("SubscriptionTokenDialog", () => {
  it("does not invoke clipboard actions while rendering copy icons", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const host = defineComponent({
      components: { NMessageProvider, SubscriptionTokenDialog },
      setup: () => ({ issued }),
      template: `
        <n-message-provider>
          <subscription-token-dialog :show="true" :issued="issued" />
        </n-message-provider>
      `,
    });

    const wrapper = mount(host, { attachTo: document.body });
    await nextTick();

    expect(writeText).not.toHaveBeenCalled();
    wrapper.unmount();
  });
});
