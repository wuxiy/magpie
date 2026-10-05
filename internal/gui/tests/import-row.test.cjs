// An app magpie is added to by a link of its own (Cindy) is a row like the
// others on the Agents page (the owner: its wide button, with no line under
// the name, sat apart from every other): a line under its name says whether
// magpie is added, and its button stands where the others' model picker
// does, as wide and lined up with it.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");

// connected when Cindy has magpie, so neither is folded under Show more
const crush = (wired) => ({
  id: "crush", name: "Crush", path: "/test/crush.json", icon: "crush", wired,
  fields: [{ key: "model", label: "model", value: wired ? "magpie/deepseek-flash" : "", options: [{ value: "magpie/deepseek-flash", label: "DeepSeek Flash", ref: "deepseek/deepseek-flash" }] }],
});
const cindy = (added) => ({ id: "cindy", name: "Cindy", path: "", icon: "cindy", import: "cindy://import?x=1", added, fields: [] });

async function serve(route, lang, added) {
  const url = new URL(route.request().url());
  const json = (data) => route.fulfill({ json: data });
  if (url.pathname === "/boot.js") {
    return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:false,textSize:100};` });
  }
  if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
  if (url.pathname === "/api/state") return json({ agents: [crush(added), cindy(added)], profiles: [], settings: { lang, theme: "light", textSize: 100 } });
  if (url.pathname === "/api/providers") return json({ providers: [], gateway: { running: true, window: true } });
  if (url.pathname === "/api/plugins") return json({ plugins: [] });
  if (url.pathname.startsWith("/api/")) return json({});
  const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
  const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
  await route.fulfill({ body: await fs.readFile(file), contentType });
}

const SAID = {
  en: [/^Not added · Cindy asks to add magpie/, /^Added · magpie is a provider in Cindy/],
  zh: [/^未添加 · /, /^已添加 · /],
};

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh"]) {
    for (const added of [false, true]) {
      test(`${engine} ${lang} ${added ? "added" : "not added"}: Cindy's row is laid out as the others`, async (t) => {
        const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
        const page = await browser.newPage({ viewport: { width: 960, height: 600 } });
        t.after(async () => {
          if (process.env.ARTIFACT_DIR) {
            await fs.mkdir(process.env.ARTIFACT_DIR, { recursive: true });
            await page.screenshot({ path: path.join(process.env.ARTIFACT_DIR, `${engine}-${lang}-${added}-import-row.png`) });
          }
          await browser.close();
        });
        page.setDefaultTimeout(5000);
        await page.route("**/*", (route) => serve(route, lang, added));
        await page.goto("http://magpie.test/");
        await page.waitForLoadState("networkidle");

        const row = page.locator('.row.agent[data-id="cindy"]');
        await row.locator(".ag-st").waitFor();
        assert.match(await row.locator(".ag-st-t").innerText(), SAID[lang][added ? 1 : 0]);
        assert.equal(await row.locator(".ag-st").evaluate((e) => e.classList.contains("on")), added);
        for (const width of [960, 700]) {
          await page.setViewportSize({ width, height: 600 });
          const box = (sel) => page.locator(sel).evaluate((e) => { const r = e.getBoundingClientRect(); return { x: r.x, right: r.right, width: r.width, y: r.y, bottom: r.bottom }; });
          const pick = await box('.row.agent[data-id="crush"] > .ag-start');
          const link = await box('.row.agent[data-id="cindy"] > .ag-start');
          assert(Math.abs(pick.right - link.right) < 1 && Math.abs(pick.width - link.width) < 1, `Cindy's button lines up with the model picker at ${width}px: ${JSON.stringify({ pick, link })}`);
          const who = await box('.row.agent[data-id="cindy"] > .who');
          assert(link.x >= who.right, `the button stays clear of the name at ${width}px`);
        }
        // a click opens the app's link, the row's one control
        const opened = page.waitForRequest((r) => r.url().includes("/api/open"));
        await row.locator(".ag-start").click();
        const req = await opened;
        assert.equal(JSON.parse(req.postData() || "{}").url, "cindy://import?x=1");
      });
    }
  }
}
