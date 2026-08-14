// W2-T6: a plain static file server over site/dist for Playwright's webServer.
//
// NOT `astro preview`: astro preview (7.2.2) manages itself as a single
// system-wide background daemon — a second invocation on a different port
// just reports the first one's URL and exits immediately, which Playwright's
// webServer reads as "the process I spawned exited" and fails the whole run
// before a single test executes. Observed directly: `astro preview --port
// 4326` printed "Preview server already running at ...4325" and returned.
// This server is a plain foreground `http.Server`, one process per
// invocation, so it has none of that shared-daemon behaviour.
import { createServer } from "node:http";
import { readFile, stat } from "node:fs/promises";
import { extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";

const here = fileURLToPath(new URL(".", import.meta.url));
const DIST = join(here, "..", "dist");

const args = process.argv.slice(2);
const portFlag = args.indexOf("--port");
const PORT = portFlag >= 0 ? Number(args[portFlag + 1]) : 4325;
const hostFlag = args.indexOf("--host");
const HOST = hostFlag >= 0 ? args[hostFlag + 1] : "127.0.0.1";

const CONTENT_TYPES = {
  ".html": "text/html; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".json": "application/json; charset=utf-8",
  ".xml": "application/xml; charset=utf-8",
  ".woff2": "font/woff2",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".txt": "text/plain; charset=utf-8",
};

// Mirrors how GitHub Pages resolves a request: a path with no extension is
// tried as <path>/index.html, then as <path>.html, matching Astro's
// "directory" build.format (dist/docs/index.html for the /docs/ route).
async function resolveFile(urlPath) {
  const safePath = normalize(decodeURIComponent(urlPath)).replace(/^(\.\.[/\\])+/, "");
  const candidates = safePath.endsWith("/")
    ? [join(DIST, safePath, "index.html")]
    : [join(DIST, safePath), join(DIST, safePath, "index.html"), join(DIST, safePath + ".html")];
  for (const candidate of candidates) {
    try {
      const st = await stat(candidate);
      if (st.isFile()) return candidate;
    } catch {
      /* try the next candidate */
    }
  }
  return null;
}

const server = createServer(async (req, res) => {
  const urlPath = new URL(req.url, "http://localhost").pathname;
  const file = await resolveFile(urlPath);
  if (!file) {
    const notFound = await resolveFile("/404.html");
    res.writeHead(404, { "content-type": "text/html; charset=utf-8" });
    res.end(notFound ? await readFile(notFound) : "Not found");
    return;
  }
  const body = await readFile(file);
  res.writeHead(200, { "content-type": CONTENT_TYPES[extname(file)] ?? "application/octet-stream" });
  res.end(body);
});

server.listen(PORT, HOST, () => {
  console.log(`serve-dist: http://${HOST}:${PORT} (serving ${DIST})`);
});
