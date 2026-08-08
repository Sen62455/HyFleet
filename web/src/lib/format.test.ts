import { describe, expect, it } from "vitest";
import { formatBytes, formatDateTime, formatRate, percent, relativeTime } from "./format";

describe("format helpers", () => {
  it("formats byte and network values", () => {
    expect(formatBytes(1024 ** 3)).toBe("1.0 GiB");
    expect(formatBytes(0)).toBe("0 B");
    expect(formatRate(1_500_000)).toBe("1.5 Mbps");
  });

  it("clamps percentages", () => {
    expect(percent(1, 4)).toBe(25);
    expect(percent(5, 4)).toBe(100);
    expect(percent(1, 0)).toBe(0);
  });

  it("renders relative heartbeat time", () => {
    const now = Date.parse("2026-08-07T12:00:00Z");
    expect(relativeTime("2026-08-07T11:59:30Z", now)).toBe("30 秒前");
    expect(relativeTime(null, now)).toBe("尚未连接");
  });

  it("renders empty and invalid expiry values safely", () => {
    expect(formatDateTime(null)).toBe("永不过期");
    expect(formatDateTime(null, false)).toBe("-");
    expect(formatDateTime("not-a-date")).toBe("时间未知");
  });
});
