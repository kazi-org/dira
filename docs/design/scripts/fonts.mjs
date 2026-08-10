// fonts.mjs — a font committed to this repository must be one this design
// actually uses, and one the binary actually carries.
//
//   node docs/design/scripts/fonts.mjs
//   node docs/design/scripts/fonts.mjs --probe-unwired    # the negative control
//   node docs/design/scripts/fonts.mjs --root <dir>       # check a staged tree
//
// WHY THIS GATE EXISTS
//
// dec-0016 decided to self-host TeX Gyre Pagella, the three woff2 subsets were
// committed under assets/fonts/, NOTICE and assets/fonts/README.md were written
// to satisfy the GUST Font Licence — and tokens.css was never touched. It kept
// declaring the Palatino system stack, which is the exact thing the decision
// exists to stop shipping. The entry sat `accepted` and unimplemented.
//
// Every one of the nine design gates measured the mockups, and the mockups used
// the system stack, so no gate could fail. That is this repository's recurring
// defect in its purest form: the check validated a declaration (the file is
// committed, the licence is recorded) rather than a result (something renders
// with it). `dira sniff` went unregistered for weeks the same way.
//
// So the rule here is a census in BOTH directions, because either direction
// alone is satisfiable by doing nothing:
//
//   committed  -> referenced   an unreferenced face is 20 KB of dead weight and
//                              a licence obligation carried for no reason
//   referenced -> resolvable   a src: url() that points at nothing renders the
//                              fallback while the CSS claims otherwise
//   declared   -> used         @font-face blocks that load a family nothing
//                              draws with are the same failure one level in
//   design     -> binary       a face in docs/ that is not byte-identical in
//                              internal/ui/assets/ is a face `dira ui` does not
//                              serve, which is where the audience actually is
//   committed  -> recorded     the licence files must name the files that ship
//
// It reads files and never opens a browser, so it costs ~50ms and can run in
// the fast lane and in CI, where the browser gates do not.

import { readFile, readdir, mkdtemp, cp, rm, writeFile } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { dirname, join, relative, resolve, sep } from 'node:path';

const HERE = dirname(new URL(import.meta.url).pathname);
const REPO = resolve(HERE, '../../..');

const ARGS = process.argv.slice(2);
const arg = (flag, fallback) => {
  const i = ARGS.indexOf(flag);
  return i >= 0 ? ARGS[i + 1] : fallback;
};
const PROBE = ARGS.includes('--probe-unwired');

// Paths, all relative to the root under examination. Relative so the negative
// control can point the identical code at a staged copy — editing the real
// files to test the checker is how a reference gets quietly rewritten to make a
// gate pass, and tokens-doc-sync.mjs already refuses to do it that way.
const TOKENS = 'docs/design/tokens.css';
const FONTS = 'assets/fonts';
const EMBED = 'internal/ui/assets/fonts';
const NOTICE = 'NOTICE';
const README = 'assets/fonts/README.md';

// The path prefix a src: url() must resolve to. It is load-bearing rather than
// cosmetic: `dira ui` serves the faces at /assets/fonts/<name> and tokens.css
// is served at /tokens.css, so a url() that resolves anywhere else is a 404 in
// the binary even though it opens fine from the working tree.
const SERVED_UNDER = 'assets/fonts';

const sha = (buf) => createHash('sha256').update(buf).digest('hex');

// ---- parse the @font-face blocks --------------------------------------------
// Deliberately a small hand parser and not a CSS library: the repo has no
// package.json and cst-0004 says a cold clone must be able to run its own
// gates. Comments are stripped first so a commented-out @font-face cannot be
// counted as a reference — which would make the gate pass on prose.
function faces(css) {
  const bare = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const out = [];
  for (const m of bare.matchAll(/@font-face\s*\{([^}]*)\}/g)) {
    const body = m[1];
    const family = body.match(/font-family\s*:\s*("([^"]*)"|'([^']*)'|[^;]+)/);
    const urls = [...body.matchAll(/url\(\s*(?:"([^"]*)"|'([^']*)'|([^)\s]+))\s*\)/g)]
      .map(u => u[1] ?? u[2] ?? u[3]);
    out.push({
      family: family ? (family[2] ?? family[3] ?? family[1]).trim() : null,
      weight: (body.match(/font-weight\s*:\s*([^;]+)/)?.[1] ?? '400').trim(),
      style: (body.match(/font-style\s*:\s*([^;]+)/)?.[1] ?? 'normal').trim(),
      display: body.match(/font-display\s*:\s*([^;]+)/)?.[1]?.trim() ?? null,
      urls,
    });
  }
  return { faces: out, bare };
}

// ---- the check ---------------------------------------------------------------
async function check(root) {
  const fail = [];
  const note = [];

  const tokensPath = join(root, TOKENS);
  if (!existsSync(tokensPath)) return { fail: [`${TOKENS} does not exist under ${root}`], note };
  const css = await readFile(tokensPath, 'utf8');
  const { faces: declared, bare } = faces(css);

  // ---- committed ---------------------------------------------------------
  const fontsDir = join(root, FONTS);
  let committed = [];
  try {
    committed = (await readdir(fontsDir)).filter(f => f.endsWith('.woff2')).sort();
  } catch {
    fail.push(`${FONTS}/ does not exist; there is nothing to check and the gate is not measuring anything`);
    return { fail, note };
  }
  if (!committed.length) {
    fail.push(`${FONTS}/ holds no .woff2 files; the gate is not measuring anything`);
    return { fail, note };
  }
  note.push(`${committed.length} committed face(s) in ${FONTS}/`);
  note.push(`${declared.length} @font-face block(s) in ${TOKENS}`);

  // ---- referenced -> resolvable, and served where the binary serves ------
  const referenced = new Map();   // basename -> the url that named it
  for (const f of declared) {
    if (!f.family) fail.push(`an @font-face block in ${TOKENS} declares no font-family`);
    if (!f.urls.length) fail.push(`the @font-face for ${f.family} has no src: url()`);
    for (const u of f.urls) {
      if (/^(https?:)?\/\//i.test(u) || /^data:/i.test(u)) {
        fail.push(`${TOKENS} loads ${u} — a font from a URL, which cst-0004 and int-0002 forbid ` +
          `outright. The faces are embedded in the binary; nothing here fetches.`);
        continue;
      }
      const abs = resolve(dirname(tokensPath), u);
      const rel = relative(root, abs).split(sep).join('/');
      if (!existsSync(abs)) {
        fail.push(`${TOKENS} references ${u}, which resolves to ${rel} — and no such file exists. ` +
          `A src that points at nothing renders the fallback while the stylesheet claims otherwise.`);
        continue;
      }
      if (!rel.startsWith(SERVED_UNDER + '/')) {
        fail.push(`${TOKENS} references ${u}, which resolves to ${rel}, outside ${SERVED_UNDER}/. ` +
          `\`dira ui\` serves the faces at /${SERVED_UNDER}/<name> and serves this stylesheet at ` +
          `/tokens.css, so a url() resolving anywhere else opens from the working tree and 404s in ` +
          `the binary — which is a failure only a reader on the served page would ever see.`);
        continue;
      }
      referenced.set(rel.slice(SERVED_UNDER.length + 1), u);
    }
  }

  // ---- committed -> referenced -------------------------------------------
  // THE CLAUSE THIS GATE WAS ADDED FOR.
  for (const f of committed) {
    if (!referenced.has(f)) {
      fail.push(`${FONTS}/${f} is committed but nothing in ${TOKENS} references it.\n` +
        `      A font in the tree that no stylesheet names is a decision that was recorded and\n` +
        `      never wired — dec-0016 sat accepted in exactly this state, with the woff2 files\n` +
        `      committed and --serif still declaring the Palatino system stack. Either add an\n` +
        `      @font-face for it, or delete the file and the licence obligation it carries.`);
    }
  }

  // ---- declared -> used ---------------------------------------------------
  // Loading a family nothing draws with is the same defect one level in: the
  // requests succeed, the gate sees no failure, and the page renders in the
  // fallback anyway.
  const families = [...new Set(declared.map(f => f.family).filter(Boolean))];
  const serif = bare.match(/--serif\s*:\s*([^;]+);/)?.[1]?.trim();
  if (!serif) {
    fail.push(`${TOKENS} declares no --serif; the token the embedded face exists to fill is gone`);
  } else {
    for (const fam of families) {
      if (!serif.includes(fam)) {
        fail.push(`${TOKENS} loads the family "${fam}" and then never uses it: --serif is ${serif}. ` +
          `Three faces would be fetched and nothing would be drawn with them.`);
      }
    }
    const first = serif.split(',')[0].trim().replace(/^["']|["']$/g, '');
    if (families.length && first !== families[0]) {
      fail.push(`--serif leads with "${first}", not with the embedded face "${families[0]}".\n` +
        `      A self-hosted face behind a system font is a self-hosted face that never renders on\n` +
        `      the machine that has the system font — which is the machine this design was tuned on,\n` +
        `      and therefore the machine every gate runs on. The fallback chain belongs BEHIND it.`);
    }
    // The fallback chain must survive. dec-0016 requires it explicitly: a build
    // that somehow ships without the woff2 still has to render.
    if (serif.split(',').length < 3 || !/serif\s*$/.test(serif)) {
      fail.push(`--serif is ${serif} — dec-0016 keeps the old stack behind the embedded face so a ` +
        `build that ships without the font still renders, ending in the generic \`serif\`.`);
    }
  }

  // font-display: swap would paint the fallback first and reflow when the real
  // face arrives. That reflow is a layout shift render.mjs fails, and the frame
  // it paints is the wrong-metrics frame this decision exists to eliminate.
  for (const f of declared) {
    if (f.display && f.display !== 'block' && f.display !== 'optional') {
      fail.push(`the @font-face for ${f.family} ${f.weight}/${f.style} uses font-display: ${f.display}. ` +
        `From an in-binary blob over loopback there is nothing to swap for; a swap only ever paints ` +
        `the fallback metrics first and then reflows.`);
    }
  }

  // ---- design -> binary ---------------------------------------------------
  for (const f of committed) {
    const embedded = join(root, EMBED, f);
    if (!existsSync(embedded)) {
      fail.push(`${EMBED}/${f} is missing. go:embed cannot reach outside its package directory, so ` +
        `the binary needs its own copy; without it \`dira ui\` serves a 404 for a face tokens.css asks ` +
        `for, and every reader of the served pages gets the fallback.`);
      continue;
    }
    const a = sha(await readFile(join(fontsDir, f)));
    const b = sha(await readFile(embedded));
    if (a !== b) {
      fail.push(`${EMBED}/${f} differs from ${FONTS}/${f}. The licence text in NOTICE describes the ` +
        `root copy; a binary serving different bytes is serving something NOTICE does not describe.`);
    }
  }

  // ---- committed -> recorded ---------------------------------------------
  // The GUST Font Licence obligation is per-file. A face added without a row in
  // the README is a face shipped outside the notice that covers it.
  for (const path of [NOTICE, README]) {
    const p = join(root, path);
    if (!existsSync(p)) { fail.push(`${path} does not exist; the licence record is gone`); continue; }
    // Whitespace-collapsed: NOTICE wraps "GUST Font / License" across two
    // lines, and a literal test could never match a file that is in fact
    // correct. Same normalization check-coherence.mjs uses, for the same reason
    // — and caught here the same way, by a baseline run that came back red.
    const text = (await readFile(p, 'utf8')).replace(/\s+/g, ' ');
    if (!/GUST Font Licen[cs]e/i.test(text)) {
      fail.push(`${path} does not name the GUST Font Licence, which is the licence these faces ship under`);
    }
    if (!/subset/i.test(text)) {
      fail.push(`${path} does not state that the faces are SUBSETS — LPPL 6b requires prominent ` +
        `notice of the nature of the change, and subsetting is the change`);
    }
  }
  const readmePath = join(root, README);
  if (existsSync(readmePath)) {
    const text = await readFile(readmePath, 'utf8');
    for (const f of committed) {
      if (!text.includes(f)) {
        fail.push(`${README} has no row for ${f}; the licence record does not name a file that ships`);
      }
    }
  }

  return { fail, note };
}

// ---- the negative control ----------------------------------------------------
// Four staged trees, each a copy of the real one with exactly one thing wrong,
// each a defect that has either already happened here or is one edit away.
// The first of them is not a hypothetical at all — it is the tree as dec-0016
// actually left it, which every one of the nine existing gates passed.
// A control that only proves the checker can fail SOMEHOW is worth very little;
// each scenario asserts on the substring identifying the specific finding, so a
// checker that failed for an unrelated reason is not mistaken for one that
// works.
async function stage(work, n, mutate) {
  const root = join(work, `probe-${n}`);
  for (const p of [TOKENS, FONTS, EMBED, NOTICE, README]) {
    await cp(join(REPO, p), join(root, p), { recursive: true });
  }
  await mutate(root);
  return root;
}

async function control() {
  const work = await mkdtemp(join(tmpdir(), 'dira-fonts-probe-'));
  const scenarios = [
    {
      // THE HISTORICAL ONE. Not a hypothetical: this is the tree as dec-0016
      // left it — three woff2 files committed, NOTICE and the README written,
      // the licence obligation accepted, and tokens.css never touched. All nine
      // design gates passed on it. This scenario is the regression test for the
      // state that made this gate necessary, and it must stay red.
      id: 'dec-0016-as-it-actually-was',
      is: 'the faces are committed and licensed, and tokens.css still declares the Palatino system stack',
      wants: 'is committed but nothing in',
      mutate: async (root) => {
        const p = join(root, TOKENS);
        const css = await readFile(p, 'utf8');
        await writeFile(p, css
          .replace(/@font-face\s*\{[^}]*\}\s*/g, '')
          .replace(/--serif:[^;]+;/,
            '--serif: "Palatino", "Palatino Linotype", "Book Antiqua", Georgia, serif;'));
      },
    },
    {
      id: 'committed-but-unreferenced',
      is: 'a fourth woff2 lands in assets/fonts/ and no stylesheet ever names it — dec-0016\'s exact state',
      wants: 'is committed but nothing in',
      mutate: async (root) => {
        const ghost = join(root, FONTS, 'pagella-smallcaps.core.woff2');
        await cp(join(REPO, FONTS, 'pagella-regular.core.woff2'), ghost);
        await cp(ghost, join(root, EMBED, 'pagella-smallcaps.core.woff2'));
        const readme = join(root, README);
        await writeFile(readme, (await readFile(readme, 'utf8')) + '\n| `pagella-smallcaps.core.woff2` | 400 |\n');
      },
    },
    {
      id: 'declared-but-unused',
      is: '--serif reverts to the Palatino system stack while the @font-face blocks stay',
      wants: 'and then never uses it',
      mutate: async (root) => {
        const p = join(root, TOKENS);
        const css = await readFile(p, 'utf8');
        await writeFile(p, css.replace(/--serif:[^;]+;/,
          '--serif: "Palatino", "Palatino Linotype", "Book Antiqua", Georgia, serif;'));
      },
    },
    {
      id: 'reference-resolves-to-nothing',
      is: 'a src: url() survives a file rename and now points at nothing',
      wants: 'and no such file exists',
      mutate: async (root) => {
        const p = join(root, TOKENS);
        const css = await readFile(p, 'utf8');
        await writeFile(p, css.replace('pagella-italic.core.woff2', 'pagella-oblique.core.woff2'));
      },
    },
    {
      id: 'in-the-design-not-in-the-binary',
      is: 'the embedded copy is deleted, so the mockups render and `dira ui` 404s',
      wants: 'is missing. go:embed cannot reach',
      mutate: async (root) => { await rm(join(root, EMBED, 'pagella-bold.core.woff2')); },
    },
  ];

  console.log(`PROBE — ${scenarios.length} staged trees, each one edit away from the real one. ` +
    `Nothing on disk is touched.\n`);
  let blind = 0;
  for (const [i, s] of scenarios.entries()) {
    const root = await stage(work, i, s.mutate);
    const { fail } = await check(root);
    const hit = fail.find(f => f.includes(s.wants));
    if (hit) {
      console.log(`  CAUGHT  ${s.id}`);
      console.log(`          ${s.is}`);
      console.log(`          -> ${hit.split('\n')[0]}`);
    } else {
      blind++;
      console.log(`  BLIND   ${s.id}`);
      console.log(`          ${s.is}`);
      console.log(`          -> the checker reported ${fail.length} finding(s), none of them this one.`);
      for (const f of fail) console.log(`             ${f.split('\n')[0]}`);
    }
  }
  await rm(work, { recursive: true, force: true });

  if (blind) {
    console.log(`\nPROBE BROKEN — ${blind} of ${scenarios.length} staged defects went unnoticed. ` +
      `A gate that cannot fail is indistinguishable from one that always prints ok.`);
    process.exit(3);
  }
  console.log(`\nPROBE OK — all ${scenarios.length} staged defects were caught, each by name.`);
  process.exit(1);
}

// ---- run ---------------------------------------------------------------------
if (PROBE) {
  await control();
} else {
  const root = resolve(arg('--root', REPO));
  const { fail, note } = await check(root);
  for (const n of note) console.log(`  ${n}`);
  console.log(`\n${fail.length} failures`);
  if (fail.length) {
    console.log('\nFONT WIRING FAIL:');
    for (const f of fail) console.log(`  - ${f}`);
    process.exit(1);
  }
  console.log('FONT WIRING PASS — every committed face is referenced, resolves under the path the ' +
    'binary serves, is drawn with, is byte-identical in the binary, and is named by the licence record.');
}
