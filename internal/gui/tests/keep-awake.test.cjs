// Run with Node's test runner and Playwright on the module path; see README.md.
// xiao_wang24004 on X: 能不能在里面加一个保持电脑任务常亮的选项呢？电脑在做任务
// 的过程中经常自动关闭了. Settings → General has a "Keep awake while agents
// work" row, off by default: On and Off are saved as keepAwake true and
// false, the click moving nothing; another setting saved keeps it; a
// browser's page (magpie web, the gateway's own computer) shows it too. In
// English and Chinese, with the API faked.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const words = {
  en: { name: "Keep awake while agents work", sub: "Keeps this computer from going to sleep by itself while agents work through magpie and for ten minutes after; the display may still turn off", on: "On", off: "Off" },
  zh: { name: "防止睡眠（Agent 工作时）", sub: "Agent 通过 magpie 工作时及之后十分钟内，阻止这台电脑自动睡眠；屏幕仍可能关闭", on: "开启", off: "关闭" },
};

function serve(lang, web, posted, st) {
  const settings = () => ({ lang, theme: "light", searchVendors: [], searchAPIs: [], ...st });
  return async (r) => {
    const url = new URL(r.request().url());
    const json = (data, status = 200) => r.fulfill({ status, json: data });
    if (url.pathname === "/boot.js") return r.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:${web}};` });
    if (url.pathname === "/wails/runtime.js") return r.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return json({ agents: [], profiles: [], settings: settings() });
    if (url.pathname === "/api/settings") {
      if (r.request().method() === "POST") {
        const b = JSON.parse(r.request().postData());
        posted.push(b);
        Object.assign(st, b);
      }
      return json(settings());
    }
    if (url.pathname === "/api/providers") return json({ providers: [], presets: [], excluded: [], gateway: { running: true, window: true, url: "http://127.0.0.1:3999" } });
    if (url.pathname === "/api/groups") return json({ models: [], groups: [], pools: [] });
    if (url.pathname === "/api/plugins") return json({ plugins: [] });
    if (url.pathname === "/api/usage/quotas") return json([]);
    if (url.pathname.startsWith("/api/")) return json({});
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await r.fulfill({ body: await fs.readFile(file), contentType });
  };
}

async function open(engine, lang, web, posted, st, t) {
  const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
  t.after(() => browser.close());
  const page = await (await browser.newContext({ viewport: { width: 900, height: 700 } })).newPage();
  page.setDefaultTimeout(5000);
  const errors = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.route("**/*", serve(lang, web, posted, st));
  await page.goto("http://magpie.test/?view=settings&tab=general");
  await page.locator("#setPage-general .row.pref").first().waitFor();
  return { page, errors };
}

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  for (const lang of ["en", "zh"]) {
    const w = words[lang];
    test(`${engine} ${lang}: keep awake is turned on and off in Settings → General`, async (t) => {
      const posted = [], st = {};
      const { page, errors } = await open(engine, lang, false, posted, st, t);
      const row = page.locator("#keepAwakeRow");
      await row.waitFor();
      // the row is near the page's end: scrolled by the reader, out from under the footer
      await page.mouse.move(450, 350);
      const end = () => page.evaluate(() => { const m = document.querySelector("#view-settings"); return m.scrollTop + m.clientHeight >= m.scrollHeight - 1; });
      for (let i = 0; i < 20 && !(await end()); i++) {
        await page.mouse.wheel(0, 400);
        await page.waitForTimeout(150);
      }
      assert.equal(await row.locator(".name").innerText(), w.name);
      assert.equal(await row.locator(".sub").innerText(), w.sub);
      const opt = (name) => row.locator("#keepAwakeSegs .opt", { hasText: name });
      assert.equal(await row.locator("#keepAwakeSegs .opt.on").innerText(), w.off, "off by default");
      const where = () => page.evaluate(() => [scrollX, scrollY, document.scrollingElement.scrollTop, ...[...document.querySelectorAll("*")].filter((e) => e.scrollTop).map((e) => e.scrollTop)]);
      const before = await where();
      const click = async (loc) => {
        let b = await loc.boundingBox();
        // the row is drawn again as the save comes back
        for (let i = 0; i < 20 && !b; i++) { await page.waitForTimeout(50); b = await loc.boundingBox(); }
        await page.mouse.click(b.x + b.width / 2, b.y + b.height / 2);
      };
      await click(opt(w.on));
      for (let i = 0; i < 50 && !posted.length; i++) await page.waitForTimeout(50);
      assert.equal(posted.at(-1)?.keepAwake, true, `posted ${JSON.stringify(posted)}`);
      assert.equal(st.keepAwake, true);
      assert.equal(await row.locator("#keepAwakeSegs .opt.on").innerText(), w.on);
      // another setting saved keeps it on
      const n = posted.length;
      await click(page.locator("#dockSegs .opt").first()).catch(() => {});
      for (let i = 0; i < 20 && posted.length === n; i++) await page.waitForTimeout(50);
      if (posted.length > n) assert.equal(posted.at(-1).keepAwake, true, "another setting's save turned it off");
      await click(opt(w.off));
      for (let i = 0; i < 50 && posted.at(-1)?.keepAwake !== false; i++) await page.waitForTimeout(50);
      assert.equal(posted.at(-1).keepAwake, false);
      assert.deepEqual(await where(), before, "the clicks moved the page");
      // the row has no coloured stripe down its left
      const left = await row.evaluate((e) => getComputedStyle(e).borderLeftWidth);
      assert(parseFloat(left) <= 1, `left border ${left}`);
      assert.deepEqual(errors, []);
    });

    test(`${engine} ${lang}: a browser's page has keep awake too`, async (t) => {
      const { page, errors } = await open(engine, lang, true, [], { keepAwake: true }, t);
      assert.equal(await page.locator("#keepAwakeRow").isVisible(), true);
      assert.equal(await page.locator("#keepAwakeSegs .opt.on").innerText(), words[lang].on);
      assert.deepEqual(errors, []);
    });
  }
}
