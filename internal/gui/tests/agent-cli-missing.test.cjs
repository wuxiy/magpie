// Run with Node's test runner and Playwright on the module path; see README.md.
// #843 (evilgaoshu): DeepSeek Harness uninstalled, its ~/.dsh left behind,
// was listed on the Agents page as if it were installed, and Install
// another agent didn't offer it. Now its row says its CLI isn't found (its
// settings are kept, the row stays), and Install another agent has it with
// its install command, marked the same. No backend: the API is faked here.
const assert = require("node:assert/strict");
const fs = require("node:fs/promises");
const path = require("node:path");
const { test } = require("node:test");
const { chromium, webkit } = require("playwright");

const assets = path.resolve(__dirname, "../assets");
const models = ["model-a", "model-b"].map((m) => ({ value: m, label: m }));
const agent = (id, name, more = {}) => ({
  id, name, path: "~/." + id + "/config.yaml", fields: [{ key: "model", label: "model", value: "model-a", options: models }], ...more,
});
const AGENTS = [agent("crush", "Crush"), agent("dsh", "DeepSeek Harness", { cliMissing: true })];
const CLIS = { crush: { version: "0.3.0", latest: "0.3.0", via: "brew", command: "brew upgrade crush" } };
const INSTALLS = [
  { id: "dsh", name: "DeepSeek Harness", icon: "deepseek-color", missing: true, commands: [{ via: "npm", command: "npm install -g @deepseek-ai/dsh" }] },
  { id: "opencode", name: "OpenCode", icon: "opencode", commands: [{ via: "npm", command: "npm install -g opencode-ai" }] },
];

function server(lang) {
  return async (route) => {
    const req = route.request(), url = new URL(req.url());
    if (url.pathname === "/boot.js") return route.fulfill({ contentType: "text/javascript", body: `window.bootPrefs = {lang:"${lang}",theme:"light",web:true};` });
    if (url.pathname === "/wails/runtime.js") return route.fulfill({ contentType: "text/javascript", body: "export const Window = {};" });
    if (url.pathname === "/api/state") return route.fulfill({ json: { agents: AGENTS, profiles: [], settings: { lang, theme: "light" } } });
    if (url.pathname === "/api/agents/cli" && req.method() === "GET") return route.fulfill({ json: { agents: CLIS, pending: false } });
    if (url.pathname === "/api/agents/install") return route.fulfill({ json: INSTALLS });
    if (url.pathname === "/api/usage/quotas") return route.fulfill({ json: [] });
    if (url.pathname === "/api/groups") return route.fulfill({ json: { groups: [] } });
    if (url.pathname === "/api/providers") return route.fulfill({ json: { providers: [], gateway: { running: true } } });
    if (url.pathname.startsWith("/api/")) return route.fulfill({ json: {} });
    const file = path.join(assets, url.pathname === "/" ? "index.html" : url.pathname);
    const contentType = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".svg": "image/svg+xml", ".png": "image/png" }[path.extname(file)];
    await route.fulfill({ body: await fs.readFile(file), contentType });
  };
}

const row = (id) => `.row.agent[data-id="${id}"]`;

for (const engine of (process.env.BROWSER ? [process.env.BROWSER] : ["chromium", "webkit"])) {
  test(engine + ": an agent whose CLI is gone says so, and is offered again", async (t) => {
    assert(["chromium", "webkit"].includes(engine), "BROWSER must be chromium or webkit");
    const browser = await (engine === "webkit" ? webkit.launch() : chromium.launch({ channel: "chromium" }));
    const errors = [];
    t.after(async () => {
      if (errors.length) console.log(errors);
      await browser.close();
    });
    for (const [lang, said, also] of [["en", "CLI not found", "Install another agent (2)"], ["zh", "未找到 CLI", "安装其他 Agent（2）"]]) {
      await t.test(lang, async () => {
        const page = await (await browser.newContext({ viewport: { width: 980, height: 700 } })).newPage();
        page.setDefaultTimeout(5000);
        page.on("pageerror", (e) => errors.push(e.message));
        await page.route("**/*", server(lang));
        await page.goto("http://magpie.test/");
        await page.locator(`${row("crush")} .ag-ver`).waitFor();
        // the row stays, its settings kept, and says its CLI isn't found
        const tag = page.locator(`${row("dsh")} .ag-missing`);
        assert.equal(await tag.textContent(), said);
        assert.match(await tag.getAttribute("title"), /~\/\.dsh\/config\.yaml/);
        assert.equal(await page.locator(`${row("dsh")} .ag-ver`).count(), 0);
        assert.equal(await page.locator(`${row("crush")} .ag-missing`).count(), 0, "an installed one says nothing of it");
        // on its name's line, not pushing the row taller than crush's
        const h = (id) => page.locator(row(id)).evaluate((r) => r.getBoundingClientRect().height);
        assert(Math.abs(await h("dsh") - await h("crush")) < 0.5, "the row grew");
        // Install another agent offers it, though it is listed above
        const box = page.locator("#agentsInstall");
        assert.equal(await box.locator(".ag-install-head").textContent(), also);
        await box.locator(".ag-install-head").click();
        assert.deepEqual(await box.locator(".ag-install-row").evaluateAll((rs) => rs.map((r) => r.dataset.id)), ["dsh", "opencode"]);
        const dsh = box.locator('.ag-install-row[data-id="dsh"]');
        assert.equal(await dsh.locator("b").textContent(), "DeepSeek Harness");
        assert.equal(await dsh.locator(".ag-install-missing").textContent(), said);
        assert.equal(await dsh.locator("code").textContent(), "npm install -g @deepseek-ai/dsh");
        assert.equal(await box.locator('.ag-install-row[data-id="opencode"] .ag-install-missing').count(), 0);
        // the name isn't cut by the note under it
        assert(await dsh.locator("b").evaluate((b) => b.scrollWidth <= b.clientWidth + 1), "name cut");
        await page.screenshot({ path: path.join(process.env.SHOTS || "/tmp", `cli-missing-${engine}-${lang}.png`), fullPage: true }).catch(() => {});
        await page.context().close();
      });
    }
    assert.deepEqual(errors, []);
  });
}
