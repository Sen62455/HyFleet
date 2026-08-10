<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import {
  NButton,
  NCheckbox,
  NCheckboxGroup,
  NDatePicker,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NSpace,
  type FormInst,
  type FormRules,
} from "naive-ui";
import type { SubscriptionFormat, SubscriptionTokenInput } from "../../types";

const props = defineProps<{ show: boolean; saving: boolean }>();
const emit = defineEmits<{
  "update:show": [show: boolean];
  submit: [input: SubscriptionTokenInput];
}>();

const formRef = ref<FormInst | null>(null);
const form = reactive({
  name: "",
  allowed_formats: ["uri", "base64", "clash", "sing-box"] as SubscriptionFormat[],
  expires_at: null as number | null,
});
const rules: FormRules = {
  name: [
    { required: true, message: "请输入 Token 名称", trigger: "blur" },
    { max: 64, message: "名称不能超过 64 个字符", trigger: "blur" },
  ],
  allowed_formats: [{ type: "array", required: true, min: 1, message: "至少选择一种格式", trigger: "change" }],
};

watch(
  () => props.show,
  (show) => {
    if (!show) return;
    form.name = "";
    form.allowed_formats = ["uri", "base64", "clash", "sing-box"];
    form.expires_at = null;
    formRef.value?.restoreValidation();
  },
);

function dateDisabled(timestamp: number) {
  return timestamp < Date.now() - 60_000;
}

async function submit() {
  try {
    await formRef.value?.validate();
  } catch {
    return;
  }
  emit("submit", {
    name: form.name.trim(),
    allowed_formats: [...form.allowed_formats],
    expires_at: form.expires_at === null ? null : new Date(form.expires_at).toISOString(),
  });
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    title="创建订阅 Token"
    class="subscription-form-modal"
    :bordered="false"
    :mask-closable="!saving"
    :close-on-esc="!saving"
    @update:show="emit('update:show', $event)"
  >
    <n-form ref="formRef" :model="form" :rules="rules" label-placement="top">
      <n-form-item label="名称" path="name">
        <n-input v-model:value="form.name" maxlength="64" placeholder="例如：手机" autofocus />
      </n-form-item>
      <n-form-item label="输出格式" path="allowed_formats">
        <n-checkbox-group v-model:value="form.allowed_formats">
          <n-space>
            <n-checkbox value="uri" label="HY2 URI" />
            <n-checkbox value="base64" label="Base64" />
            <n-checkbox value="clash" label="Clash Meta" />
            <n-checkbox value="sing-box" label="sing-box" />
          </n-space>
        </n-checkbox-group>
      </n-form-item>
      <n-form-item label="到期时间" path="expires_at">
        <n-date-picker
          v-model:value="form.expires_at"
          type="datetime"
          clearable
          placeholder="不设置则永不过期"
          :is-date-disabled="dateDisabled"
          style="width: 100%"
        />
      </n-form-item>
    </n-form>
    <template #footer>
      <div class="modal-actions">
        <n-button :disabled="saving" @click="emit('update:show', false)">取消</n-button>
        <n-button type="primary" :loading="saving" @click="submit">创建</n-button>
      </div>
    </template>
  </n-modal>
</template>
