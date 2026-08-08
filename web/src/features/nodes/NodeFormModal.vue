<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import {
  NButton,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NSelect,
  NSwitch,
  type FormInst,
  type FormRules,
} from "naive-ui";
import type { AdapterType, NodeInput, NodeRecord } from "../../types";

const props = defineProps<{
  show: boolean;
  node: NodeRecord | null;
  saving: boolean;
}>();

const emit = defineEmits<{
  "update:show": [show: boolean];
  submit: [input: Required<NodeInput>];
}>();

const formRef = ref<FormInst | null>(null);
const form = reactive<Required<NodeInput>>({
  name: "",
  provider: "",
  region: "",
  adapter_type: "native_hysteria2",
  public_host: "",
  public_port: 443,
  sni: "",
  tls_insecure: false,
  enabled: true,
});

const title = computed(() => (props.node ? "编辑节点" : "添加节点"));
const adapterLocked = computed(() => Boolean(props.node?.agent_installation_id));
const adapterOptions = [
  { label: "原生 Hysteria2", value: "native_hysteria2" },
  { label: "独立 sing-box", value: "standalone_sing_box" },
  { label: "S-UI", value: "s_ui" },
];
const rules: FormRules = {
  name: [
    { required: true, message: "请输入节点名称", trigger: "blur" },
    { min: 2, max: 64, message: "名称长度为 2 到 64 个字符", trigger: "blur" },
  ],
  adapter_type: [{ required: true, message: "请选择适配器", trigger: "change" }],
};

watch(
  () => [props.show, props.node] as const,
  ([show, node]) => {
    if (!show) return;
    form.name = node?.name ?? "";
    form.provider = node?.provider ?? "";
    form.region = node?.region ?? "";
    form.adapter_type = (node?.adapter_type ?? "native_hysteria2") as AdapterType;
    form.public_host = node?.public_host ?? "";
    form.public_port = node?.public_port ?? 443;
    form.sni = node?.sni ?? "";
    form.tls_insecure = node?.tls_insecure ?? false;
    form.enabled = node?.enabled ?? true;
    formRef.value?.restoreValidation();
  },
  { immediate: true },
);

async function submit() {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  emit("submit", {
    name: form.name.trim(),
    provider: form.provider.trim(),
    region: form.region.trim(),
    adapter_type: form.adapter_type,
    public_host: form.public_host.trim(),
    public_port: form.public_port,
    sni: form.sni.trim(),
    tls_insecure: form.tls_insecure,
    enabled: form.enabled,
  });
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="title"
    class="node-form-modal"
    :bordered="false"
    :mask-closable="!saving"
    :close-on-esc="!saving"
    @update:show="emit('update:show', $event)"
  >
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="top">
      <n-form-item label="名称" path="name">
        <n-input v-model:value="form.name" maxlength="64" placeholder="例如：LisaHost" autofocus />
      </n-form-item>
      <div class="form-grid">
        <n-form-item label="服务商" path="provider">
          <n-input v-model:value="form.provider" maxlength="64" placeholder="例如：Lisa" />
        </n-form-item>
        <n-form-item label="地区" path="region">
          <n-input v-model:value="form.region" maxlength="64" placeholder="例如：Los Angeles" />
        </n-form-item>
      </div>
      <n-form-item label="节点适配器" path="adapter_type">
        <n-select
          v-model:value="form.adapter_type"
          :options="adapterOptions"
          :disabled="adapterLocked"
        />
        <template v-if="adapterLocked" #feedback>Agent 注册后不可更换适配器。</template>
      </n-form-item>
      <div class="form-section-label">订阅端点</div>
      <div class="form-grid form-grid--endpoint">
        <n-form-item label="公网域名或 IP" path="public_host">
          <n-input v-model:value="form.public_host" maxlength="253" placeholder="hy2.example.com" />
        </n-form-item>
        <n-form-item label="UDP 端口" path="public_port">
          <n-input-number v-model:value="form.public_port" :min="1" :max="65535" :precision="0" />
        </n-form-item>
      </div>
      <n-form-item label="TLS SNI" path="sni">
        <n-input v-model:value="form.sni" maxlength="253" placeholder="留空时使用公网域名" />
      </n-form-item>
      <div class="switch-row switch-row--compact">
        <div><strong>跳过证书验证</strong></div>
        <n-switch v-model:value="form.tls_insecure" aria-label="跳过证书验证" />
      </div>
      <div class="switch-row">
        <div>
          <strong>启用节点</strong>
          <span>停用后仍保留节点与历史状态</span>
        </div>
        <n-switch v-model:value="form.enabled" aria-label="启用节点" />
      </div>
    </n-form>
    <template #footer>
      <div class="modal-actions">
        <n-button :disabled="saving" @click="emit('update:show', false)">取消</n-button>
        <n-button type="primary" :loading="saving" @click="submit">{{ node ? "保存" : "添加" }}</n-button>
      </div>
    </template>
  </n-modal>
</template>
