import type { GlobalThemeOverrides } from "naive-ui";

export const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: "#147d64",
    primaryColorHover: "#0f6c56",
    primaryColorPressed: "#0b5b49",
    primaryColorSuppl: "#147d64",
    infoColor: "#2563eb",
    successColor: "#16845b",
    warningColor: "#b54708",
    errorColor: "#b42318",
    borderRadius: "6px",
    borderRadiusSmall: "4px",
    fontFamily: 'Inter, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif',
  },
  Button: {
    borderRadiusMedium: "6px",
    heightMedium: "36px",
    fontWeight: "600",
  },
  Card: {
    borderRadius: "6px",
  },
  Dialog: {
    borderRadius: "6px",
  },
  Drawer: {
    borderRadius: "0px",
  },
  Input: {
    borderRadius: "6px",
  },
};
