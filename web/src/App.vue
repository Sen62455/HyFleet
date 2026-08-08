<script setup lang="ts">
import { NConfigProvider, NDialogProvider, NGlobalStyle, NMessageProvider } from "naive-ui";
import AppController from "./AppController.vue";
import { themeOverrides } from "./theme";
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <n-global-style />
    <n-dialog-provider>
      <n-message-provider placement="top-right">
        <app-controller />
      </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>
