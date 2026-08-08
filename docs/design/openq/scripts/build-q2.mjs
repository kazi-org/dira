// build-q2.mjs — emits the three WITHHELD-STATE treatments from ONE page.
//
//   node docs/design/openq/scripts/build-q2.mjs
//
// dec-0011 settled the model: resolution reports oriented / withheld / orphan,
// and only orphan is drift. dec-0012 settled the architecture. The visual has
// never been designed, and the hard constraint is that withheld must read as
// NEITHER an error NOR a warning — red is reserved for drift and contradiction
// (Law 1), so withheld cannot borrow it, and it cannot borrow warning grammar
// either (no triangle, no exclamation, no amber-as-caution).
//
// Every option renders the identical page and shows all three states adjacent,
// with the real red drift card on the same screen. The test is not "does
// withheld look calm alone" — it is "does withheld read as a different KIND of
// thing from the orphan two columns away".

import { writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const OUT = resolve(import.meta.dirname, '..');

// ---- the three treatments ---------------------------------------------------
const T = [
  {
    letter: 'A',
    file: 'q2-a-plain.html',
    label: 'plain speech &mdash; no glyph, no hue, no material; the state is a sentence',
    css: `
/* OPTION A — PLAIN SPEECH. The withheld state spends nothing: no hue, no icon,
   no surface change. Where the parent's title would be, the page says what is
   true in words, set in the same neutral ink a refused alternative uses. The
   claim under test: a state that borrows no alarm grammar cannot be misread as
   an alarm. The exposure: it also borrows no presence, and "nothing here plus
   grey text" is what a failed fetch looks like too. */
.wh-mark { color: var(--ink-low); }
.wh-title {
  font-family: var(--serif); font-size: var(--t-small); font-style: italic;
  color: var(--ink-low); max-width: var(--m-prose);
}
.st-withheld .stmark, .st-withheld .stname { color: var(--ink-low); }
.chain .id-w { color: var(--ink-mid); }
.chain .wh { color: var(--ink-low); font-style: italic; }
.whrail em { font-style: normal; color: var(--ink-low); }
.arising .whref { color: var(--ink-mid); }`,
    chainRow: `<div class="row"><span class="id-w">sire:int-0002</span><span class="wh" data-contrast>declared private &mdash; not readable here</span><span class="lo">withheld</span></div>`,
    stackRow: `<div class="row"><span class="k" style="color:var(--ink-mid)">sire:int-0002</span><span class="v">private &mdash; not readable here</span></div>`,
    mark: '&mdash;',
    slot: `<div class="stslot"><span class="wh-title" data-contrast>declared private &mdash; not readable here</span></div>`,
    rail: `<span class="whrail">sire:int-0002 <em data-contrast>withheld</em></span>`,
    arising: `<span class="whref" data-contrast>sire:int-0002</span> &mdash; a parent in a ledger this repo declares but cannot show you.`,
  },

  {
    letter: 'B',
    file: 'q2-b-declared.html',
    label: 'declared state &mdash; a first-class chip in the instrument hue',
    css: `
/* OPTION B — A DECLARED STATE. Withheld is promoted to a first-class resolution
   state with its own chip, in the instrument hue, using exactly the grammar
   tokens.css already gives .chip-staged. The claim under test: something that
   looks DESIGNED cannot look broken — a state with a name, a mark and a chip
   reads as a decision someone made. The exposure: --bearing has one documented
   meaning (the instrument: focus, brand, links, active intents) and this adds a
   second, so either that meaning widens or the hue budget grows to four. */
.wh-mark { color: var(--bearing); }
.wh-chip {
  display: inline-flex; align-items: center; gap: 5px;
  font-family: var(--mono); font-size: var(--t-chip); letter-spacing: .03em;
  padding: 2px 7px; border-radius: var(--r-chip); white-space: nowrap;
  color: var(--bearing);
  /* 7%, NOT the 13% .chip-staged uses. Measured as rendered, --bearing on a 13%
     --bearing tint over --panel is 4.11:1 in light — under the 4.5 floor. The
     token matrix cannot see this: it checks --bearing on --panel (5.12:1), and
     every chip in the system actually sits on a tinted version of that surface.
     7% measures 4.74:1 in both schemes. See the report — the same construction
     is currently below the floor in tokens.css itself. */
  background: color-mix(in oklab, var(--bearing) 7%, transparent);
  border: 1px solid color-mix(in oklab, var(--bearing) 30%, transparent);
}
.wh-title { font-family: var(--serif); font-size: var(--t-small); color: var(--ink-mid); max-width: var(--m-prose); }
.st-withheld { border-color: color-mix(in oklab, var(--bearing) 38%, var(--rule)); }
.st-withheld .stmark, .st-withheld .stname { color: var(--bearing); }
.chain .id-w { color: var(--bearing); }
.chain .wh { color: var(--ink-mid); }
.chain .wh-tag { color: var(--bearing); }
.chain .row > .wh-tag:last-child { margin-left: auto; padding-left: var(--s4); }
.whrail em { font-style: normal; color: var(--bearing); }
.arising .whref { color: var(--bearing); }`,
    chainRow: `<div class="row"><span class="id-w" data-contrast>sire:int-0002</span><span class="wh">private ledger &mdash; ref published, body not</span><span class="wh-tag" data-contrast>&#8857; withheld</span></div>`,
    stackRow: `<div class="row"><span class="k">sire:int-0002</span><span class="v">private ledger &middot; &#8857; withheld</span></div>`,
    mark: '&#8857;',
    slot: `<div class="stslot"><span class="wh-chip" data-contrast>&#8857; withheld</span></div>
        <div class="wh-title" data-contrast>a parent this repo names and does not publish</div>`,
    rail: `<span class="whrail">sire:int-0002 <em data-contrast>&#8857;</em></span>`,
    arising: `<span class="whref" data-contrast>sire:int-0002</span> &mdash; a parent in a ledger this repo declares but cannot show you.`,
  },

  {
    letter: 'C',
    file: 'q2-c-sealed.html',
    label: 'sealed material &mdash; the absence is drawn, not coloured',
    css: `
/* OPTION C — SEALED MATERIAL. No hue is spent at all. Where the parent's title
   would be, the page draws a sealed slab: a hatch built from --rule-soft over
   --sunk, the same weight as a hairline. The claim under test: a woven surface
   is unambiguously deliberate — no error state in any interface is hatched —
   so the meaning "covered on purpose" arrives before any word is read. The
   exposure: it introduces a texture primitive the system does not otherwise
   have, and it is the one place where a graphic stands in for content, which a
   reviewer will read against Law 3 even though the content does not exist. */
.wh-mark { color: var(--ink-low); }
.wh-seal {
  display: block; height: 16px; border-radius: 2px;
  border: 1px solid var(--rule);
  background: repeating-linear-gradient(135deg,
    var(--rule-soft) 0 3px, var(--sunk) 3px 7px);
}
.stslot .wh-seal { width: 100%; }
.wh-title { font-family: var(--serif); font-size: var(--t-small); color: var(--ink-low); max-width: var(--m-prose); }
.st-withheld .stmark, .st-withheld .stname { color: var(--ink-low); }
.chain .id-w { color: var(--ink-mid); }
.chain .wh-seal-inline {
  display: inline-block; width: 22ch; height: 9px; vertical-align: middle;
  border-radius: 2px; border: 1px solid var(--rule);
  background: repeating-linear-gradient(135deg,
    var(--rule-soft) 0 3px, var(--sunk) 3px 7px);
}
.whrail em { font-style: normal; color: var(--ink-low); }
.arising .whref { color: var(--ink-mid); }`,
    chainRow: `<div class="row"><span class="id-w">sire:int-0002</span><span class="dim"><span class="wh-seal-inline" aria-hidden="true"></span></span><span class="lo">withheld</span></div>`,
    stackRow: `<div class="row"><span class="k" style="color:var(--ink-mid)">sire:int-0002</span><span class="v"><span class="wh-seal-inline" aria-hidden="true"></span> withheld</span></div>`,
    mark: '&#9640;',
    slot: `<div class="stslot"><span class="wh-seal" aria-hidden="true"></span></div>
        <div class="wh-title" data-contrast>sealed &mdash; declared private, not published here</div>`,
    rail: `<span class="whrail">sire:int-0002 <em data-contrast>sealed</em></span>`,
    arising: `<span class="whref" data-contrast>sire:int-0002</span> &mdash; a parent in a ledger this repo declares but cannot show you.`,
  },
];

// ---- the page, identical for all three --------------------------------------
const page = t => `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#0f151c" media="(prefers-color-scheme: dark)">
<meta name="theme-color" content="#f7f4ed" media="(prefers-color-scheme: light)">
<title>Option ${t.letter} — the withheld state · dira open-question study</title>
<link rel="stylesheet" href="../tokens.css">
<link rel="stylesheet" href="common.css">
<link rel="stylesheet" href="q2-common.css">
<style>${t.css}
</style>
</head>
<body>

<div class="study">OPEN QUESTION 2 &middot; the withheld state &middot; option <b>${t.letter}</b> &middot; ${t.label}</div>
<header class="topbar">
  <a class="wordmark" href="#">di<b>ra</b></a>
  <nav class="crumb" aria-label="Breadcrumb"><a href="#">kazi-org/dira</a> &middot; <a href="#">ledger</a> &middot; dec-0011</nav>
  <span class="spacer"></span>
  <span class="crumb">decision memory for AI coding agents</span>
</header>

<main class="wrap">

  <div class="invoke"><span class="p">$</span><span class="c">dira why</span><span class="a">tiers</span></div>

  <section class="chain" aria-label="Why-chain">
    ${t.chainRow}
    <div class="node">
      <div class="row"><span class="id">dec-0011</span><span class="txt">Cross-boundary parents publish opaquely</span><span class="lo">accepted 30 Jul</span></div>
      <div class="node">
        <div class="row"><span class="ok">&#10003; opaque refs</span><span class="dim">cst-0003 rule 3, applied consistently</span></div>
        <div class="row"><span class="no">&#10007; strip on publish</span><span class="dim">makes every adopting ledger read as orphan work</span></div>
        <div class="row"><span class="no">&#10007; redacted stubs</span><span class="dim">a private-to-public write path, judged per write</span></div>
        <div class="row"><span class="id-q">cst-0003</span><span class="dim">cite the ref, never the text</span><span class="lo">informs</span></div>
      </div>
    </div>
  </section>

  <section class="chain-stack" aria-label="Why-chain">
    ${t.stackRow}
    <div class="row sub"><span class="k">dec-0011</span><span class="v">Cross-boundary parents publish opaquely</span></div>
    <div class="row sub2"><span class="m ok">&#10003;</span><span class="v">opaque refs &mdash; cst-0003 applied consistently</span></div>
    <div class="row sub2"><span class="m no">&#10007;</span><span class="v">strip on publish &mdash; everything reads orphan</span></div>
    <div class="row sub2"><span class="m no">&#10007;</span><span class="v">redacted stubs &mdash; judged on every write</span></div>
  </section>

  <div class="body">
    <article>
      <div class="tagrow">
        <span class="chip chip-id">dec-0011</span>
        <span class="chip chip-accepted">&#9670; accepted</span>
        <span class="chip chip-date">30 Jul 2026</span>
      </div>
      <p class="arising">Arising from ${t.arising}</p>
      <h1 class="ruling">The ref publishes. The reasoning behind it does not.</h1>
      <p class="because">
        This entry derives from a parent in a private ledger, and it ships that edge verbatim
        rather than hiding it. You cannot open it, and that is not a fault in the record &mdash;
        <em>it is the record being honest about a boundary it will not cross.</em>
      </p>

      <section class="states">
        <h2 class="seclabel">How this entry resolves</h2>
        <div class="stgrid">

          <div class="stcell st-oriented">
            <div class="sthead"><span class="stmark">&#10003;</span><span class="stname">oriented</span></div>
            <div class="stref"><span class="k">informs</span> cst-0003</div>
            <div class="stslot"><span class="sttitle"><a href="#">Cite the ref, never the text</a></span></div>
            <div class="stnote">Edge present, target resolved and readable.</div>
          </div>

          <div class="stcell st-withheld">
            <div class="sthead"><span class="stmark wh-mark">${t.mark}</span><span class="stname">withheld</span></div>
            <div class="stref"><span class="k">derives_from</span> sire:int-0002</div>
            ${t.slot}
            <div class="stnote">Edge present, parent namespace declared private. Not drift.</div>
          </div>

          <div class="stcell st-orphan">
            <div class="sthead"><span class="stmark">&#9888;</span><span class="stname">orphan</span></div>
            <div class="stref"><span class="k">derives_from</span> &mdash; none &mdash;</div>
            <div class="stslot"><span class="sttitle">no parent recorded at all</span></div>
            <div class="stnote">This one is drift, and the only one of the three that is.</div>
          </div>

        </div>
      </section>

      <section class="args">
        <h2 class="seclabel">Alternatives on record</h2>

        <div class="arg upheld">
          <div class="opt"><span class="mark">&#10003;</span><span class="name">Publish the ref opaquely, resolve to three states</span><span class="tag">upheld</span></div>
          <p class="grounds">The only candidate that does not require overruling an accepted constraint. <code>cst-0003</code> rule 3 already settles the adjacent case &mdash; cite the ref, never the text &mdash; and a namespaced <code>derives_from</code> in a public entry is that rule applied consistently.</p>
        </div>

        <div class="arg">
          <div class="opt"><span class="mark">&#10007;</span><span class="name">Publish opaquely, but keep drift two-state</span><span class="tag">refused</span></div>
          <p class="grounds">Identical publishing behaviour, and it collapses &ldquo;parent exists and is not readable here&rdquo; into &ldquo;no parent&rdquo;. That distinction is already in the data; discarding it destroys the orphan signal on exactly the surface where a stranger judges the tool.</p>
        </div>

        <div class="arg">
          <div class="opt"><span class="mark">&#10007;</span><span class="name">Strip upward edges on publish</span><span class="tag">refused</span></div>
          <p class="grounds">Looks safest and is worst: the public artifact would deny an edge that exists, so every adopting repo&rsquo;s ledger reads as unexplained work. It also presumes a publish step, and <code>dec-0002</code> is built on there not being one.</p>
          <p class="revisit"><b>revisit if</b>a ledger must be published somewhere the namespace itself is sensitive.</p>
        </div>
      </section>
    </article>

    <aside class="rail">
      <div class="card">
        <h3>What this is</h3>
        <p style="font-size:var(--t-ui);color:var(--ink-mid);margin-bottom:var(--s3)">
          <b style="color:var(--ink)">Nobody typed this page.</b> A coding agent recorded
          it while the work happened, and reads it back at the start of the next session.</p>
        <p style="font-family:var(--mono);font-size: var(--t-chip);color:var(--ink-low)">
          <b style="color:var(--bearing);font-weight:400">int-</b> an intent &middot;
          <b style="color:var(--bearing);font-weight:400">dec-</b> a decision &middot;
          <b style="color:var(--bearing);font-weight:400">sire:</b> another ledger</p>
      </div>
      <div class="card">
        <h3>Edges</h3>
        <div class="kv"><span class="k">derives_from</span>${t.rail}</div>
        <div class="kv"><span class="k">informs</span><a href="#">cst-0003</a></div>
        <div class="kv"><span class="k">supersedes</span><a href="#">qst-0001</a></div>
      </div>
      <div class="drift">
        <h3>&#9888; dira flagged this</h3>
        <p><b>int-0004</b> is active work with no recorded purpose. dira raises it
          unprompted &mdash; nobody asked it to check.</p>
      </div>
    </aside>
  </div>

  <div style="margin-top:var(--s7);padding-top:var(--s4);border-top:1px solid var(--rule);
              display:flex;gap:var(--s3);align-items:baseline;flex-wrap:wrap">
    <p style="font-family:var(--serif);font-size: var(--t-body);color:var(--ink-mid);flex:1;min-width:240px;margin:0">
      Every decision in this repo is recorded like this one.</p>
    <a href="#" style="font-size:var(--t-ui);color:var(--bearing);text-decoration:none;border-bottom:1px solid currentColor">Keep your own &rarr;</a>
  </div>

  <footer class="made">
    <span>11 of 22 entries &middot; <a href="#">see the whole graph</a></span>
    <span class="sep">&middot;</span>
    <span>kept with <a href="#">dira</a>, written by an agent, read by one</span>
  </footer>
</main>
</body>
</html>
`;

for (const t of T) await writeFile(resolve(OUT, t.file), page(t));
console.log(`wrote ${T.map(t => t.file).join(' / ')}`);
