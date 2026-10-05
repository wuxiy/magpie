// Run with Node's test runner and Playwright on the module path; see README.md.
// A setting nobody has picked is the agent's own logo on the row, for want of
// room for its name. In the window's opened row those are laid side by side,
// and Aside's four task roles and its picture model all read "default" under
// the same logo: five pills saying nothing about which setting is which. The
// tray panel already names them as its rows open, so the window does the same,
// and none of the names is cut short (Aside's standard, Claude Code's
// thinking, in a 44px column, read stand...). In English and Chinese, in both
// places. No backend: the API is faked here.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const ref = (value) => ({ value, label: value, ref: value, note: "via magpie" });
const options = [ref("magpie/minimax-cn/MiniMax-M3"), ref("magpie/relay/glm-4.6")];
const roles = ["fast", "standard", "deep", "visual"];
// as magpie reads an Aside: the model it talks and its level beside it, the
// model it draws with, and the four tasks it picks a model by hand
const state = {
  agents: [{
    id: "aside", name: "Aside", icon: "aside", path: "/fixture/aside", wired: true,
    fields: [
      { key: "model", label: "model", value: "magpie/minimax-cn/MiniMax-M3", options },
      { key: "effort", label: "thinking", value: "high", options: ["low", "medium", "high"].map((value) => ({ value })) },
      { key: "image", label: "image", value: "magpie/agnes-image-2.1-flash", options },
      ...roles.map((key) => ({ key, label: key, value: "", options })),
    ],
  }],
  profiles: [],
};

function server(lang) {
  return async (route) => {
    const req = route.request(), url = new URL(req.url());
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return route.fulfill({ json: { ...state, settings: { lang, theme: "light" } } });
    if (url.pathname === "/api/usage/quotas") return route.fulfill({ json: [] });
    if (url.pathname === "/api/groups") return route.fulfill({ json: { groups: [] } });
    if (url.pathname === "/api/plugins") return route.fulfill({ json: { plugins: [] } });
    if (url.pathname === "/api/providers") return route.fulfill({ json: { providers: [], gateway: { running: true, window: true } } });
    if (url.pathname.startsWith("/api/")) return route.fulfill({ json: {} });
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const row = '.row.agent[data-id="aside"]';
const words = {
  en: { thinking: "thinking", image: "image", fast: "fast", standard: "standard", deep: "deep", visual: "visual", high: "high", unset: "default" },
  zh: { thinking: "思考", image: "图片", fast: "快速", standard: "标准", deep: "深度", visual: "视觉", high: "高", unset: "默认" },
};
// the pills of the "New sessions" line, each as [setting, name, value]
const pills = (scope) => [...document.querySelectorAll(scope + " .field")].map((b) => [b.dataset.key, b.querySelector(":scope > .k")?.textContent ?? null, b.querySelector(":scope > .v")?.textContent ?? null]);
// a name cut short by its column reads as an ellipsis, not a word
const cut = (scope) => [...document.querySelectorAll(scope + " .k")].filter((k) => k.scrollWidth > k.clientWidth + 1).map((k) => k.textContent);

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh"]) {
    const w = words[lang];
    test(`${engine} ${lang}: the window's opened row names every setting`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await (await browser.newContext({ viewport: { width: 1240, height: 900 }, reducedMotion: "reduce" })).newPage();
      page.setDefaultTimeout(5000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.route("**/*", server(lang));
      await page.goto("http://magpie.test/");
      await page.locator(row).waitFor();
      await page.locator(`${row} .ag-link`).click();
      await page.locator(`${row} .ag-exp`).waitFor();
      // the model it starts on is on the row itself, so what is left here is
      // its level and the settings that follow the model
      const got = await page.evaluate(pills, `${row} .ag-exp .ag-vl`);
      assert.deepEqual(got, [
        [ "effort", w.thinking, w.high ],
        [ "image", w.image, "magpie/agnes-image-2.1-flash" ],
        [ "fast", w.fast, w.unset ],
        [ "standard", w.standard, w.unset ],
        [ "deep", w.deep, w.unset ],
        [ "visual", w.visual, w.unset ],
      ]);
      assert.deepEqual(await page.evaluate(cut, `${row} .ag-exp`), [], "a name cut short");
      assert.deepEqual(errors, []);
      await page.close();
    });

    test(`${engine} ${lang}: the panel's opened row names every setting whole`, async (t) => {
      const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
      t.after(() => browser.close());
      const page = await (await browser.newContext({ viewport: { width: 440, height: 720 }, reducedMotion: "reduce" })).newPage();
      page.setDefaultTimeout(5000);
      const errors = [];
      page.on("pageerror", (e) => errors.push(e.message));
      await page.route("**/*", server(lang));
      await page.goto("http://magpie.test/?mode=panel");
      await page.locator(row).waitFor();
      await page.locator(`${row} .ag-sum`).click();
      await page.locator(`${row} .ag-open .field`).first().waitFor();
      const got = await page.evaluate(pills, `${row} .ag-open`);
      for (const [, name] of got) assert.notEqual(name, null, "a setting with no name of its own");
      assert.deepEqual(await page.evaluate(cut, `${row} .ag-open`), [], "a name cut short");
      // standard is the longest of Aside's, and it is the one that was cut
      assert.ok(got.some(([key, name]) => key === "standard" && name === w.standard), JSON.stringify(got));
      assert.deepEqual(errors, []);
      await page.close();
    });
  }
}
