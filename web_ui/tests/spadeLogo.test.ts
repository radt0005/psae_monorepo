import { describe, it, expect } from "vitest";
import { mount } from "@vue/test-utils";
import SpadeLogo from "~/components/brand/SpadeLogo.vue";

describe("SpadeLogo", () => {
  it("renders at the marketing-site default 28×34", () => {
    const w = mount(SpadeLogo);
    const svg = w.find("svg");
    expect(svg.attributes("width")).toBe("28");
    expect(svg.attributes("height")).toBe("34");
    // both path elements: spade body + base
    expect(w.findAll("path")).toHaveLength(2);
  });

  it("respects width and height props", () => {
    const w = mount(SpadeLogo, { props: { width: 80, height: 98 } });
    const svg = w.find("svg");
    expect(svg.attributes("width")).toBe("80");
    expect(svg.attributes("height")).toBe("98");
  });

  it("uses white fill when dark prop is set (footer variant)", () => {
    const w = mount(SpadeLogo, { props: { dark: true } });
    const paths = w.findAll("path");
    paths.forEach((p) => expect(p.attributes("fill")).toBe("#ffffff"));
  });

  it("uses black fill by default", () => {
    const w = mount(SpadeLogo);
    const paths = w.findAll("path");
    paths.forEach((p) => expect(p.attributes("fill")).toBe("#1a1a1a"));
  });

  it("exposes the title as aria-label for screen readers", () => {
    const w = mount(SpadeLogo, { props: { title: "Custom title" } });
    expect(w.find("svg").attributes("aria-label")).toBe("Custom title");
  });
});
