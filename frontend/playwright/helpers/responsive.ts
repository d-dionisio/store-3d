import { expect, type Page, type TestInfo } from "@playwright/test";
import path from "node:path";
import { mkdir } from "node:fs/promises";

export type LayoutMeasurement = {
  viewportWidth: number;
  clientWidth: number;
  documentScrollWidth: number;
  horizontalOverflow: boolean;
  intentionalScrollableContainers: number;
  visibleControlsOutsideViewport: number;
  firstOutsideControlTags: string[];
};

export async function verifyResponsiveLayout(page: Page, testInfo: TestInfo): Promise<LayoutMeasurement> {
  const measurement = await page.evaluate(() => {
    const root = document.documentElement;
    const intentionallyScrollable = (element: Element) => {
      const style = getComputedStyle(element);
      return element.scrollWidth > element.clientWidth + 1 && ["auto", "scroll"].includes(style.overflowX);
    };
    const visible = (element: Element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return style.display !== "none" && style.visibility !== "hidden" && Number(style.opacity) > 0 && rect.width > 0 && rect.height > 0;
    };
    const controls = [...document.querySelectorAll("a,button,input,select,textarea,[role=button]")].filter(visible);
    const outside = controls.filter(element => {
      let parent = element.parentElement;
      while (parent && parent !== root) {
        if (intentionallyScrollable(parent)) return false;
        parent = parent.parentElement;
      }
      const rect = element.getBoundingClientRect();
      return rect.left < -1 || rect.right > window.innerWidth + 1;
    });
    const width = window.innerWidth;
    return {
      viewportWidth: width,
      clientWidth: root.clientWidth,
      documentScrollWidth: root.scrollWidth,
      horizontalOverflow: root.scrollWidth > width + 1,
      intentionalScrollableContainers: [...document.querySelectorAll("*")].filter(intentionallyScrollable).length,
      visibleControlsOutsideViewport: outside.length,
      firstOutsideControlTags: outside.slice(0, 5).map(element => element.tagName.toLowerCase()),
    };
  });

  testInfo.annotations.push({
    type: "layout",
    description: `viewport=${measurement.viewportWidth}; client=${measurement.clientWidth}; scroll=${measurement.documentScrollWidth}; overflow=${measurement.horizontalOverflow}; containers roláveis=${measurement.intentionalScrollableContainers}; controles fora=${measurement.visibleControlsOutsideViewport}; elementos=${measurement.firstOutsideControlTags.join(",")}`,
  });
  expect(measurement.horizontalOverflow, "A página não deve criar overflow horizontal global.").toBe(false);
  expect(measurement.visibleControlsOutsideViewport, "Controles visíveis devem permanecer acessíveis na viewport, exceto dentro de rolagem horizontal intencional.").toBe(0);
  return measurement;
}

export async function saveVisualScreenshot(page: Page, testInfo: TestInfo, routeId: string, viewportId: string): Promise<string> {
  const runId = process.env.PW_RUN_ID;
  if (!runId) throw new Error("PW_RUN_ID não foi inicializado pela configuração Playwright.");
  const output = path.resolve(process.cwd(), "test-results", "visual-homologation", "runs", runId, "screenshots", viewportId, `${routeId}.png`);
  await mkdir(path.dirname(output), { recursive: true });
  const personalFields = [
    page.locator('input[type="email"], input[type="tel"], input[name*="email" i], input[name*="phone" i], [data-sensitive="personal"]'),
    page.getByText(/[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}/i),
  ];
  await page.screenshot({ path: output, fullPage: true, animations: "disabled", mask: personalFields, maskColor: "#5b5364" });
  testInfo.annotations.push({ type: "screenshot", description: path.relative(process.cwd(), output).replaceAll("\\", "/") });
  return output;
}

export async function verifyKeyboardFocus(page: Page, testInfo: TestInfo): Promise<void> {
  let visibleFocus = false;
  let tabs = 0;
  for (; tabs < 8; tabs += 1) {
    await page.keyboard.press("Tab");
    visibleFocus = await page.evaluate(() => {
      const active = document.activeElement;
      if (!active || active === document.body || !active.matches(":focus-visible")) return false;
      if (!active.closest("main, nav")) return false;
      const bounds = active.getBoundingClientRect();
      return bounds.width > 0 && bounds.height > 0 && bounds.right > 0 && bounds.left < window.innerWidth;
    });
    if (visibleFocus) break;
  }
  testInfo.annotations.push({ type: "keyboard", description: `Foco visível em controle do painel após ${tabs + 1} Tab(s): ${visibleFocus}` });
  expect(visibleFocus, "Um controle do painel deve receber foco visível por teclado.").toBe(true);
}
