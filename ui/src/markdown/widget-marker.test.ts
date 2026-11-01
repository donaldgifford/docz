import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import { unified } from "unified";
import { describe, expect, it } from "vitest";

import { rehypeWidgetMarker } from "@/markdown/widget-marker";

import type { Element } from "hast";

function firstPre(markdown: string): Element {
  const processor = unified()
    .use(remarkParse)
    .use(remarkRehype)
    .use(rehypeWidgetMarker);
  const tree = processor.runSync(processor.parse(markdown));
  const pre = tree.children.find(
    (node): node is Element =>
      node.type === "element" && node.tagName === "pre",
  );
  if (pre === undefined) {
    throw new Error("no <pre>");
  }
  return pre;
}

describe("rehypeWidgetMarker", () => {
  it.each([
    ["docz-flow", "flow"],
    ["docz-stepper", "stepper"],
    ["docz-scorecard", "scorecard"],
  ])("marks a %s fence with its widget, YAML, and line", (fence, widget) => {
    const yaml = "title: T\nstages: []\n";
    const pre = firstPre(`Intro.\n\n\`\`\`${fence}\n${yaml}\`\`\`\n`);
    expect(pre.properties.dataWidget).toBe(widget);
    expect(pre.properties.dataWidgetSource).toBe(yaml);
    expect(pre.properties.dataWidgetLine).toBe("3");
    const code = pre.children[0] as Element;
    expect(code.properties.className).toEqual([]);
  });

  it("leaves other fences alone", () => {
    const pre = firstPre("```yaml\ntitle: T\n```\n");
    expect(pre.properties.dataWidget).toBeUndefined();
    expect((pre.children[0] as Element).properties.className).toEqual([
      "language-yaml",
    ]);
  });

  it("does not mark a fence named only like a widget", () => {
    const pre = firstPre("```docz-flowchart\ntitle: T\n```\n");
    expect(pre.properties.dataWidget).toBeUndefined();
  });
});
