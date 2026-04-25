import { describe, it, expect } from "vitest";
import config from "~/tailwind.config";

/**
 * Pin the design tokens so we know if someone accidentally drifts away from
 * the marketing-site palette / typography.
 */
describe("tailwind design tokens", () => {
  const colors: any = (config.theme?.extend?.colors as any) ?? {};
  const fonts: any = (config.theme?.extend?.fontFamily as any) ?? {};

  it("preserves the spade brand palette", () => {
    expect(colors.spade.black).toBe("#1a1a1a");
    expect(colors.spade.white).toBe("#ffffff");
    expect(colors.spade.red.DEFAULT).toBe("#c0392b");
    expect(colors.spade.red.light).toBe("#e74c3c");
    expect(colors.spade.red.dark).toBe("#96281b");
    expect(colors.spade.gray.DEFAULT).toBe("#888888");
  });

  it("preserves admonition palette inherited from docs site", () => {
    expect(colors.spade.blue.DEFAULT).toBe("#2980b9");
    expect(colors.spade.green.DEFAULT).toBe("#27ae60");
    expect(colors.spade.yellow.DEFAULT).toBe("#f39c12");
  });

  it("maps Nuxt UI primary palette to the spade red", () => {
    expect(colors.primary["600"]).toBe("#c0392b");
    expect(colors.primary["500"]).toBe("#e74c3c");
    expect(colors.primary["800"]).toBe("#96281b");
  });

  it("uses the marketing-site fonts in priority order", () => {
    expect(fonts.heading[0]).toBe("Playfair Display");
    expect(fonts.body[0]).toBe("Inter");
    expect(fonts.mono[0]).toBe("JetBrains Mono");
  });
});
