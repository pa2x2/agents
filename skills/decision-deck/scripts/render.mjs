// Screenshots a deck at full resolution in a headless Chromium-family browser.
//
//   node render.mjs <deck URL> <out dir> [steps JSON]
//
// Without steps it shoots every slide, and on slides with variants every option of each variant,
// with the slide's other variants on their proposed option. A step is { name, hash?, eval? }: go to
// slide number <hash>, optionally run JS (its result is printed), and save <out dir>/<name>.png. For each
// shot it prints the scale the deck fitted the visuals to; then it prints any page errors and exits
// with 1 if there were some.
//
// Needs Node 22+ (global fetch and WebSocket). Looks for chromium, chrome, brave or edge; set
// DECK_BROWSER to the browser's executable if it isn't found. DECK_SIZE sets the viewport
// (default 1600x1000), DECK_SCALE the pixel ratio (default 1; 2 for sharp crops).
import { execFileSync, spawn } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const [url, outDir, stepsJson] = process.argv.slice(2);
if (!url || !outDir) {
  console.error("usage: node render.mjs <deck URL> <out dir> [steps JSON]");
  process.exit(2);
}
// Output piped into head or similar may close early; keep going so the browser still gets cleaned up.
process.stdout.on("error", () => {});
const [width, height] = (process.env.DECK_SIZE ?? "1600x1000").split("x").map(Number);
const scale = Number(process.env.DECK_SCALE ?? 1);

const onPath = (name) => {
  try { execFileSync("which", [name], { stdio: "ignore" }); return true; } catch { return false; }
};
const browserBin = [
  process.env.DECK_BROWSER, "chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "brave", "brave-browser",
  "microsoft-edge", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium",
  "/Applications/Brave Browser.app/Contents/MacOS/Brave Browser", "/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
].filter(Boolean).find((b) => (b.includes("/") ? existsSync(b) : onPath(b)));
if (!browserBin) {
  console.error("No Chromium-family browser found. Set DECK_BROWSER to its executable.");
  process.exit(2);
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const profile = mkdtempSync(join(tmpdir(), "deck-render-"));
// Port 0 lets the browser pick a free port; it writes the port to DevToolsActivePort in the profile.
// Detached, so the browser gets its own process group: some launchers are wrapper scripts whose
// browser process outlives them, and the whole group has to stop before the profile can go.
const browser = spawn(browserBin, ["--headless=new", "--remote-debugging-port=0", `--user-data-dir=${profile}`, "--no-first-run",
  "--no-default-browser-check", "--hide-scrollbars", "about:blank"], { stdio: "ignore", detached: true });
const groupAlive = () => { try { process.kill(-browser.pid, 0); return true; } catch { return false; } };
const cleanup = async () => {
  try { process.kill(-browser.pid, "SIGTERM"); } catch {}
  for (let i = 0; i < 50 && groupAlive(); i++) await sleep(100);
  if (groupAlive()) try { process.kill(-browser.pid, "SIGKILL"); } catch {}
  rmSync(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 });
};

let target;
for (let i = 0; i < 75 && !target; i++) {
  await sleep(200);
  const portFile = join(profile, "DevToolsActivePort");
  if (!existsSync(portFile)) continue;
  const port = readFileSync(portFile, "utf8").split("\n")[0];
  try { target = (await (await fetch(`http://127.0.0.1:${port}/json`)).json()).find((t) => t.type === "page"); } catch {}
}
if (!target) {
  await cleanup();
  console.error(`${browserBin} didn't start a debugging session within 15s. Set DECK_BROWSER to another browser.`);
  process.exit(2);
}

const ws = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((resolve) => (ws.onopen = resolve));
let id = 0;
const pending = new Map();
const errors = [];
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  if (msg.id) pending.get(msg.id)?.(msg);
  else if (msg.method === "Runtime.exceptionThrown") {
    const d = msg.params.exceptionDetails;
    errors.push(`${d.exception?.description ?? d.text} (line ${d.lineNumber + 1})`);
  } else if (msg.method === "Runtime.consoleAPICalled" && msg.params.type === "error") {
    errors.push(msg.params.args.map((a) => a.value ?? a.description).join(" "));
  } else if (msg.method === "Log.entryAdded" && msg.params.entry.level === "error") {
    errors.push(`${msg.params.entry.text}${msg.params.entry.url ? ` ${msg.params.entry.url}` : ""}`);
  }
};
const send = (method, params = {}) => new Promise((resolve) => { pending.set(++id, resolve); ws.send(JSON.stringify({ id, method, params })); });
const evaluate = async (expression) => {
  const res = await send("Runtime.evaluate", { expression, awaitPromise: true, returnByValue: true });
  return res.result?.exceptionDetails ? `error: ${res.result.exceptionDetails.exception?.description}` : res.result?.result?.value;
};

await send("Runtime.enable");
await send("Log.enable");
await send("Page.enable");
await send("Emulation.setDeviceMetricsOverride", { width, height, deviceScaleFactor: scale, mobile: false });
await send("Page.navigate", { url });
for (let i = 0; i < 50 && (await evaluate("document.readyState")) !== "complete"; i++) await sleep(100);
await evaluate("document.fonts.ready.then(() => true)");

// Default steps: each slide once, or once per variant option on slides with variants.
const steps = stepsJson ? JSON.parse(stepsJson) : (await evaluate(`(() => {
  const steps = [];
  document.querySelectorAll(".dk-slide").forEach((slide, i) => {
    const base = String(i + 1).padStart(2, "0") + "-" + slide.dataset.key;
    const groups = {};
    slide.querySelectorAll(".dk-variant input[type=radio]").forEach((r) => (groups[r.name] ??= []).push(r.value));
    if (!Object.keys(groups).length) steps.push({ name: base, hash: i + 1 });
    for (const [name, values] of Object.entries(groups)) for (const value of values) steps.push({
      name: base + "-" + name.slice(slide.dataset.key.length + 1) + "-" + value, hash: i + 1,
      // Reset the slide's other variants to their proposed option first, so each shot shows one change.
      eval: '(() => { const s = document.querySelectorAll(".dk-slide")[' + i + ']; ' +
        's.querySelectorAll("input[data-default]").forEach((r) => r.click()); ' +
        's.querySelector(\\'input[name="' + name + '"][value="' + value + '"]\\').click(); })()',
    });
  });
  return steps;
})()`)) ?? [];

mkdirSync(outDir, { recursive: true });
for (const step of steps) {
  if (step.hash !== undefined) await evaluate(`location.hash = "#${step.hash}"`);
  await sleep(150);
  const result = step.eval ? await evaluate(step.eval) : undefined;
  await sleep(250);
  const zoom = await evaluate(`(() => { const s = document.querySelector(".dk-slide.dk-on .dk-shots"); return s ? Math.round(parseFloat(s.style.zoom || 1) * 100) : null; })()`);
  const shot = await send("Page.captureScreenshot", { format: "png" });
  writeFileSync(join(outDir, `${step.name}.png`), Buffer.from(shot.result.data, "base64"));
  console.log(`${step.name}.png${zoom !== null ? `  visuals at ${zoom}%` : ""}${result !== undefined ? `  → ${JSON.stringify(result)}` : ""}`);
}

ws.close();
await cleanup();
if (errors.length) {
  console.log(`\nPAGE ERRORS:\n${[...new Set(errors)].join("\n")}`);
  process.exit(1);
}
