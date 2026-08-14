// W1-T3: the why pages. Builds the local dira binary, runs `dira ui` against
// THIS repo's own .dira/ ledger, and snapshots its real rendered output
// (never a reimplementation of it — see docs/plan/website.md's risk register:
// "a second renderer grows in the site and drifts from `dira ui`") to static
// HTML under site/public/why/. Astro copies public/ verbatim into dist/, so
// this alone produces dist/why/dec-0001/index.html and dist/why/index.html.
//
// Swap point for when E6-L3's `dira render` ships (documented inline, per
// the task): replace fetchSnapshot()'s HTTP GET against a spawned `dira ui`
// with a call to `dira render <path>` (or equivalent), keep everything from
// transform() down unchanged. No page changes either way.
import { mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { spawn } from "node:child_process";
import { buildBinary, BIN_PATH, REPO_ROOT, SITE_ROOT } from "./lib/dira.mjs";

const PUBLIC_DIR = join(SITE_ROOT, "public");
const REPO = "https://github.com/kazi-org/dira";

// The one decision page this lane snapshots (docs/plan/website.md W1-T3).
// Every OTHER /e/<id> link the fetched pages contain gets rewritten to its
// source file on GitHub instead of a local page that does not exist — see
// rewriteLinks() — so the site never ships a link to a page it never built.
const SNAPSHOT_ID = "dec-0001";

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

// Rewrites dira ui's own root-relative links so they resolve correctly once
// this HTML is mounted under /why/ on the site instead of at true root:
//   href="/"          -> href="/why/"              (dira ui's own home)
//   href="/e/dec-0001" -> href="/why/dec-0001/"     (the one page we snapshot)
//   href="/e/<id>"     -> the entry's source on GitHub (every other id — no
//                          local page exists for it, so linking locally would
//                          be the broken-link defect W2-T6 gates on)
//   href="/tokens.css", "/decision.css", "/index.css", "/assets/fonts/..."
//                      -> left alone; the site serves the identical files at
//                         the identical root-absolute paths (see fontsAndCss
//                         below and site/public/assets/fonts/, copied in T1).
export function rewriteLinks(html) {
  return html
    .replace(/href="\/e\/([a-zA-Z0-9-]+)"/g, (m, id) =>
      id === SNAPSHOT_ID ? `href="/why/${id}/"` : `href="${REPO}/blob/main/.dira/entries/${id}.md" rel="noopener"`,
    )
    .replace(/href="\/"/g, 'href="/why/"');
}

// Adds one line of site-shell chrome into dira ui's own header bar — a link
// back to the marketing site and to the docs page — using tokens.css classes
// dira ui's own markup already defines (.crumb), so nothing new is styled.
export function injectSiteShell(html) {
  const shellNav =
    '<nav class="crumb" aria-label="Site"><a href="/">dira.sire.run</a> &middot; ' +
    '<a href="/docs/">docs</a></nav>\n</header>';
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

    const [indexHtml, decisionHtml, tokensCss, decisionCss, indexCss] = await Promise.all([
      fetch(base + "/").then((r) => r.text()),
      fetch(base + "/e/" + SNAPSHOT_ID).then((r) => r.text()),
      fetch(base + "/tokens.css").then((r) => r.text()),
      fetch(base + "/decision.css").then((r) => r.text()),
      fetch(base + "/index.css").then((r) => r.text()),
    ]);

    mkdirSync(join(PUBLIC_DIR, "why", SNAPSHOT_ID), { recursive: true });
    writeFileSync(join(PUBLIC_DIR, "why", "index.html"), transform(indexHtml));
    writeFileSync(join(PUBLIC_DIR, "why", SNAPSHOT_ID, "index.html"), transform(decisionHtml));
    writeFileSync(join(PUBLIC_DIR, "tokens.css"), tokensCss);
    writeFileSync(join(PUBLIC_DIR, "decision.css"), decisionCss);
    writeFileSync(join(PUBLIC_DIR, "index.css"), indexCss);

    console.log(`snapshot-why: wrote why/index.html, why/${SNAPSHOT_ID}/index.html, and 3 stylesheets`);
  } finally {
    child.kill("SIGTERM");
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  main();
}
