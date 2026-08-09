import { flushPromises, mount } from "@vue/test-utils";
import { describe, expect, it } from "vitest";
import NodeFormModal from "./NodeFormModal.vue";

describe("NodeFormModal", () => {
  it("submits TLS certificate and public-key pins with the endpoint", async () => {
    const wrapper = mount(NodeFormModal, {
      attachTo: document.body,
      props: { show: true, node: null, saving: false },
      global: { stubs: { teleport: true } },
    });
    await flushPromises();

    await wrapper.find('input[placeholder="例如：LisaHost"]').setValue("Pinned node");
    await wrapper.find('input[placeholder="AA:BB:CC:..."]').setValue("AB:".repeat(31) + "AB");
    await wrapper.find('input[placeholder="Base64 SHA-256"]').setValue("QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI=");
    const addButton = wrapper.findAll("button").find((button) => button.text().trim() === "添加");
    expect(addButton).toBeDefined();
    await addButton!.trigger("click");
    await flushPromises();

    expect(wrapper.emitted("submit")?.[0]?.[0]).toMatchObject({
      name: "Pinned node",
      tls_cert_fingerprint: "AB:".repeat(31) + "AB",
      tls_public_key_sha256: "QkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkI=",
    });
    wrapper.unmount();
  });
});
