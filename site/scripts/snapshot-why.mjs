// W1-T3: the why pages. Builds the local dira binary, runs `dira ui` against
// THIS repo's own .dira/ ledger, and snapshots its real rendered output
// (never a reimplementation of it — see docs/plan/website.md's risk register:
// "a second renderer grows in the site and drifts from `dira ui`") to static
// HTML under site/public/why/. Astro copies public/ verbatim into dist/, so
// this produces the ledger index and every reachable public entry page.
//
// Swap point for when E6-L3's `dira render` ships (documented inline, per
// the task): replace fetchSnapshot()'s HTTP GET against a spawned `dira ui`
// with a call to `dira render <path>` (or equivalent), keep everything from
// transform() down unchanged. No page changes either way.
import { mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { spawn } from "node:child_process";
import { buildBinary, BIN_PATH, REPO_ROOT, SITE_ROOT } from "./lib/dira.mjs";

const PUBLIC_DIR = join(SITE_ROOT, "public");

async function waitForServer(url, timeoutMs = 10000) {
  const start = Date.now();
  for (;;) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      /* not listening yet */
    }
    if (Date.now() - start > timeoutMs) {
      throw new Error(`snapshot-why: dira ui did not answer ${url} within ${timeoutMs}ms`);
    }
    await new Promise((r) => setTimeout(r, 100));
  }
}

// Every public entry reachable from the rendered index is snapshotted below.
// Consume the CLI's own renderer so its privacy filtering and reasoning stay intact.
export function rewriteLinks(html) {
  return html
    .replace(/href="\/e\/([a-zA-Z0-9-]+)"/g, 'href="/why/$1/"')
    .replace(/href="\/"/g, 'href="/why/"');
}

// Adds one line of site-shell chrome into dira ui's own header bar — a link
// back to the marketing site and to the docs page — using tokens.css classes
// dira ui's own markup already defines (.crumb), so nothing new is styled.
export function injectSiteShell(html) {
  const shellNav =
    '<nav class="crumb" aria-label="Site"><a href="/">dira.sire.run</a> &middot; ' +
    '<a href="/guide/">guide</a> &middot; <a href="/docs/">commands</a></nav>\n</header>';
  return html.replace("</header>", shellNav);
}

export function transform(html) {
  return injectSiteShell(rewriteLinks(html));
}

async function main() {
  buildBinary();

  const child = spawn(BIN_PATH, ["ui", "-C", REPO_ROOT, "-addr", "127.0.0.1:0"], {
    stdio: ["ignore", "pipe", "pipe"],
  });

  let base = "";
  const urlPromise = new Promise((resolve, reject) => {
    let buf = "";
    const onData = (chunk) => {
      buf += chunk.toString();
      const m = buf.match(/http:\/\/127\.0\.0\.1:(\d+)/);
      if (m) {
        child.stdout.off("data", onData);
        resolve(`http://127.0.0.1:${m[1]}`);
      }
    };
    child.stdout.on("data", onData);
    child.once("error", reject);
    setTimeout(() => reject(new Error("snapshot-why: dira ui never printed its URL")), 10000);
  });

  try {
    base = await urlPromise;
    await waitForServer(base + "/");

    const read = async (path) => {
      const response = await fetch(base + path);
      if (!response.ok) throw new Error(`snapshot-why: ${path} returned ${response.status}`);
      return response.text();
    };
    const indexHtml = await read('/');
    // Remove old generated pages so removed or newly private entries cannot linger.
    rmSync(join(PUBLIC_DIR, 'why'), { recursive: true, force: true });
    mkdirSync(join(PUBLIC_DIR, 'why'), { recursive: true });
    writeFileSync(join(PUBLIC_DIR, 'why', 'index.html'), transform(indexHtml));
    const pending = new Set([...indexHtml.matchAll(/href="\/e\/([a-zA-Z0-9-]+)"/g)].map(m => m[1]));
    const visited = new Set();
    // Traverse rendered entry links only; never enumerate private source files.
    for (const id of pending) {
      if (visited.has(id)) continue;
      const html = await read('/e/' + id);
      visited.add(id);
      for (const match of html.matchAll(/href="\/e\/([a-zA-Z0-9-]+)"/g)) pending.add(match[1]);
      mkdirSync(join(PUBLIC_DIR, 'why', id), { recursive: true });
      writeFileSync(join(PUBLIC_DIR, 'why', id, 'index.html'), transform(html));
    }
    for (const asset of ['tokens.css', 'decision.css', 'index.css']) {
      writeFileSync(join(PUBLIC_DIR, asset), await read('/' + asset));
    }
    // Astro's route discovery does not see public/ snapshots. Publish their sitemap.
    const paths = ['/why/', ...[...visited].sort().map(id => `/why/${id}/`)];
    writeFileSync(join(PUBLIC_DIR, 'ledger-sitemap.xml'),
      '<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">' +
      paths.map(path => `<url><loc>https://dira.sire.run${path}</loc></url>`).join('') + '</urlset>');
    console.log(`snapshot-why: wrote index, ${visited.size} entry pages, stylesheets, and ledger sitemap`);
  } finally {
    child.kill("SIGTERM");
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main();
}
