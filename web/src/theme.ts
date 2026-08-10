import type { GlobalThemeOverrides } from "naive-ui";

const fontFamily = 'Inter, "SF Pro Text", "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif';

const componentOverrides: Omit<GlobalThemeOverrides, "common"> = {
  Button: {
    borderRadiusMedium: "5px",
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
    borderRadius: "5px",
    borderFocus: "1px solid #64748b",
    boxShadowFocus: "none",
  },
};

export const lightThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: "#334155",
    primaryColorHover: "#475569",
    primaryColorPressed: "#1e293b",
    primaryColorSuppl: "#334155",
    infoColor: "#52647a",
    successColor: "#5f7f78",
    warningColor: "#8a7453",
    errorColor: "#9a5e62",
    bodyColor: "#ffffff",
    cardColor: "#ffffff",
    modalColor: "#ffffff",
    popoverColor: "#ffffff",
    textColorBase: "#171717",
    textColor1: "#171717",
    textColor2: "#525252",
    textColor3: "#737373",
    borderColor: "#eaeaea",
    dividerColor: "#eaeaea",
    borderRadius: "6px",
    borderRadiusSmall: "4px",
    fontFamily,
  },
  ...componentOverrides,
};

export const darkThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: "#52647a",
    primaryColorHover: "#64748b",
    primaryColorPressed: "#3f4e62",
    primaryColorSuppl: "#52647a",
    infoColor: "#7b8da3",
    successColor: "#6f8d87",
    warningColor: "#9b8766",
    errorColor: "#aa7377",
    bodyColor: "#0a0a0a",
    cardColor: "#111111",
    modalColor: "#111111",
    popoverColor: "#171717",
    textColorBase: "#ededed",
    textColor1: "#ededed",
    textColor2: "#c7c7c7",
    textColor3: "#a1a1aa",
    borderColor: "#333333",
    dividerColor: "#2a2a2a",
    borderRadius: "6px",
    borderRadiusSmall: "4px",
    fontFamily,
  },
  ...componentOverrides,
  Button: {
    ...componentOverrides.Button,
    textColorPrimary: "#f5f5f5",
    textColorHoverPrimary: "#ffffff",
    textColorPressedPrimary: "#ffffff",
    textColorFocusPrimary: "#ffffff",
    textColorDisabledPrimary: "#a3a3a3",
  },
};

export const eyeThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: "#46574f",
    primaryColorHover: "#35463e",
    primaryColorPressed: "#293830",
    primaryColorSuppl: "#52645b",
    infoColor: "#60736a",
    successColor: "#668078",
    warningColor: "#897553",
    errorColor: "#956365",
    bodyColor: "#f3f6f1",
    cardColor: "#fbfcfa",
    modalColor: "#fbfcfa",
    popoverColor: "#fbfcfa",
    textColorBase: "#1c2420",
    textColor1: "#1c2420",
    textColor2: "#4d5a53",
    textColor3: "#718079",
    borderColor: "#dde4da",
    dividerColor: "#dde4da",
    borderRadius: "6px",
    borderRadiusSmall: "4px",
    fontFamily,
  },
  ...componentOverrides,
  Input: {
    ...componentOverrides.Input,
    borderFocus: "1px solid #52645b",
  },
};
