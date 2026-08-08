// build-q1.mjs — emits the three LONG-CONTENT treatments from ONE dataset.
//
//   node docs/design/openq/scripts/build-q1.mjs
//
// Open question 2 in DESIGN.md: the screens have never been tested against a
// 400-word why_not or a 20-alternative decision. This builds a decision that has
// both, and renders it three ways. The content is generated rather than typed
// three times so that the ONLY difference between q1-a, q1-b and q1-c is the
// disclosure treatment. If the prose differed, the comparison would be measuring
// the prose.
//
// The subject is a real decision dira has to make and a genuinely 20-wide one:
// what a ledger entry physically IS on disk.

import { writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';

const OUT = resolve(import.meta.dirname, '..');

// ---- the dataset ------------------------------------------------------------
// one:  the scannable line — what treatment (c) shows before you expand
// full: the grounds, as paragraphs. Alternative 2 is the 400-word case.
const ALTS = [
  { upheld: true,
    name: 'Markdown files with YAML front-matter, one per entry',
    one: 'readable with cat, reviewable in the pull request, outlives the binary',
    full: ['Every entry is a file a human can read with no tool installed, and every change is a diff a reviewer already sees in the pull request that carries the work. The front-matter holds the structure — id, kind, state, edges — so one file is both the prose and the data without being two artifacts that can disagree.'] },

  { name: 'SQLite as the source of truth',
    one: 'the right database and the wrong record — a store you must open a tool to read',
    full: [
      'SQLite is the correct answer to almost every question this decision asks except the one that matters. It gives a real query planner, transactions, an index on the edge table, and a single file that copies trivially — and dira will almost certainly end up with a SQLite index derived from the entries, because resolving a chain by walking the filesystem stops being acceptable somewhere around a few thousand entries. None of that is in dispute. What is in dispute is whether that file is the record or a cache of the record, and the whole product rests on the answer.',
      'The ledger&rsquo;s claim is that it outlives the tool. A decision recorded in 2026 is worth reading in 2031, and by then the binary that wrote it may not compile, the schema may have moved twice, and the person reading it may have no interest in installing anything. A directory of markdown files survives all three: <code>cat</code> reads it, <code>grep</code> searches it, the forge renders it, and a reviewer who has never heard of dira can still follow the reasoning. A database file survives only as long as someone is willing to open a database to read a paragraph of prose. That is a low bar today and an unknown bar in five years — and it is a bar that exists at all only because we chose to put one there.',
      'Second, and more immediately: <code>git log -p</code> is a review surface, not an implementation detail. When an agent records a decision mid-session, the diff a human reads at review time is the entry itself — the title, the because, the refusals in full sentences. With a binary store that diff is a wall of unreadable bytes, so review moves to a tool that must be built and kept working, and the review that actually happens is the one that costs nothing. Merge behaviour follows the same logic: two agents adding entries on two branches produce a conflict a human can resolve in an editor, or one they cannot resolve at all.',
      'Third, the failure mode is asymmetric. Choosing files and later needing speed produces a derived index — additive, discardable, rebuildable from the files at any moment. Choosing the database and later needing durability or reviewability produces an export path, a second copy, and a drift problem between them. One of those mistakes is cheap to correct and the other is not.',
    ],
    revisit: 'the derived index stops being derivable in acceptable time — that is a performance problem with a performance fix, not grounds to move the record.' },

  { name: 'A single YAML file for the whole ledger',
    one: 'every write touches one file, so every concurrent agent conflicts',
    full: ['One file is trivially loadable and impossible to write concurrently. Two agents recording two unrelated decisions in the same session collide on adjacent lines, and the collision is in a structure git cannot merge meaningfully. One file per entry makes the common case — two additions — a clean merge with no resolution at all.'] },

  { name: 'JSON, one file per entry',
    one: 'solves the parsing and loses the prose',
    full: ['The structural half is served better than YAML serves it. But the payload here is paragraphs of reasoning, and reasoning stored as escaped strings is unreadable in a diff, unwritable by hand, and hostile to the exact surface the record depends on being legible.'] },

  { name: 'An append-only JSONL event log',
    one: 'correct for events, wrong for a record that gets revised',
    full: ['Entries are revised — a question becomes answered, a decision gets superseded — and an append-only log answers that with replay. It buys an audit trail that git already provides, at the cost of making the current state of any entry something you compute rather than something you read.'] },

  { name: 'TOML front-matter instead of YAML',
    one: 'the better specification, the worse ecosystem on the surface that renders it',
    full: ['TOML is more predictable and YAML&rsquo;s ambiguities are real, so this is close. YAML front-matter is what every forge, static site generator and note-taking tool already parses, and rendering wherever the record already lives is worth more here than a cleaner grammar.'],
    revisit: 'a YAML ambiguity actually corrupts an entry in practice rather than in principle.' },

  { name: 'Git notes attached to the commits that caused the work',
    one: 'the most native option, invisible on every surface a human uses',
    full: ['Notes attach the record to the commit that produced it, which is the correct relationship and the reason this keeps coming back. They are also not fetched by default, not shown in pull requests, not rendered by any forge, and routinely dropped by tooling that does not know they exist.'] },

  { name: 'Git trailers in commit messages',
    one: 'zero new files, and the record can never be revised',
    full: ['A trailer is written once into an immutable object. Superseding a decision would mean rewriting history, so the single operation the ledger exists to support is the one operation this makes impossible.'] },

  { name: 'GitHub Issues as the store',
    one: 'an excellent writing surface owned by someone else',
    full: ['Issues bring search, threading, cross-references and notifications for free. They also live on a host the project does not own, cannot be read with the network unplugged, do not clone, and leave the record behind if the repository ever moves. cst-0004 disqualifies this on its own.'] },

  { name: 'GitHub Discussions',
    one: 'the same trade as Issues with a thinner API',
    full: ['Every objection to Issues applies, and the API is weaker for programmatic writes. It threads better for humans, which is not what an agent-written record optimises for.'] },

  { name: 'Protocol buffers with a generated reader',
    one: 'a wire format for a record whose readers are people',
    full: ['Schema evolution is genuinely better handled here than in any text format. The ledger is not on a wire — it is on disk, being read by humans and diffed by review tools, and both of those needs invert the trade.'] },

  { name: 'CBOR',
    one: 'binary compactness for a corpus measured in kilobytes',
    full: ['The size win is real and irrelevant: a large ledger is a few megabytes of prose. Paying unreadability for compactness at this scale is paying for something nobody asked for.'] },

  { name: 'XML with an authoring schema',
    one: 'validation that works, authoring nobody will do',
    full: ['The schema story is the strongest of any candidate here — structural validation is free rather than hand-written. Nothing else survives: not the diff, not hand-editing, not the render, not the agent writing it.'] },

  { name: 'RDF triples in Turtle',
    one: 'exactly the right graph model with nowhere for the prose to live',
    full: ['The edge model dira wants is a graph, and this is the format that natively is one. Then the reasoning — the thing being recorded — becomes a string literal hanging off a subject, and the record turns into a database with a paragraph stapled to it.'] },

  { name: 'Org-mode',
    one: 'superb for one editor&rsquo;s users, opaque to everyone else',
    full: ['Better outlining, better linking, better literate structure than markdown by some distance. The audience is every developer using every editor, and the forge renders markdown.'] },

  { name: 'An Obsidian vault with wiki-link edges',
    one: 'the right link syntax carrying an application dependency',
    full: ['The double-bracket link is close to what the edge model wants and the graph view is genuinely useful. It also makes the canonical reading experience an application the project does not control and cannot embed in a binary.'] },

  { name: 'Dolt — git semantics over a SQL database',
    one: 'solves versioned data and inherits the unreadable-diff problem',
    full: ['This is the most technically interesting candidate: real branching and merging over tables, which is precisely the operation a per-file ledger has to hand-roll. It is also another runtime to install, and a diff that only its own tooling can render.'] },

  { name: 'A Datasette-published SQLite file',
    one: 'a publishing layer answering a storage question',
    full: ['This is not really a competing store — it is a way to expose one. It inherits every objection to SQLite as source of truth, and adds a Python service to the deployment story of a single static binary.'] },

  { name: 'Plain ADR markdown with no front-matter',
    one: 'the closest miss — the edges have nowhere to live',
    full: ['This is the incumbent practice and the format dira mirrors into. Without structured front-matter, the edges between entries live in prose links that cannot be resolved, checked for drift, or walked, which removes every capability the tool exists to provide.'] },

  { name: 'A Notion database through the API',
    one: 'the record stops existing when the subscription does',
    full: ['The best authoring UI of any option, and the only one where the record is a tenant of a company that can change its export format, its pricing, or its mind. A decision log that can be revoked is not a decision log.'] },
];

// ---- shared page furniture --------------------------------------------------
const head = (title, sub) => `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#0f151c" media="(prefers-color-scheme: dark)">
<meta name="theme-color" content="#f7f4ed" media="(prefers-color-scheme: light)">
<title>${title}</title>
<link rel="stylesheet" href="../tokens.css">
<link rel="stylesheet" href="common.css">`;

const banner = (letter, label) => `
<div class="study">OPEN QUESTION 1 &middot; long content &middot; option <b>${letter}</b> &middot; ${label} &middot; 20 alternatives, one of them 400 words</div>
<header class="topbar">
  <a class="wordmark" href="#">di<b>ra</b></a>
  <nav class="crumb" aria-label="Breadcrumb"><a href="#">kazi-org/dira</a> &middot; <a href="#">ledger</a> &middot; dec-0002</nav>
  <span class="spacer"></span>
  <span class="crumb">decision memory for AI coding agents</span>
</header>`;

// identical in all three options, so the chain is never a comparison variable
const chain = `
  <div class="invoke"><span class="p">$</span><span class="c">dira why</span><span class="a">storage</span></div>

  <section class="chain" aria-label="Why-chain">
    <div class="row"><span class="id-q">int-0001</span><span class="txt">A record that outlives the tool that wrote it</span><span class="lo">active</span></div>
    <div class="node">
      <div class="row"><span class="id">dec-0002</span><span class="txt">Markdown + YAML front-matter, in git</span><span class="lo">accepted 29 Jul</span></div>
      <div class="node">
        <div class="row"><span class="ok">&#10003; markdown</span><span class="dim">readable with cat, reviewable in the diff</span></div>
        <div class="row"><span class="no">&#10007; sqlite</span><span class="dim">the record becomes a file you need a tool to open</span></div>
        <div class="row"><span class="no">&#10007; jsonl</span><span class="dim">append-only; current state must be computed</span></div>
        <div class="row"><span class="no">&#10007; gh issues</span><span class="dim">a writing surface owned by someone else</span></div>
        <div class="row"><span class="lo">&hellip;</span><span class="lo">16 further refusals on record</span></div>
      </div>
    </div>
  </section>

  <section class="chain-stack" aria-label="Why-chain">
    <div class="row"><span class="k">int-0001</span><span class="v">A record that outlives the tool</span></div>
    <div class="row sub"><span class="k">dec-0002</span><span class="v">Markdown + YAML front-matter, in git</span></div>
    <div class="row sub2"><span class="m ok">&#10003;</span><span class="v">markdown &mdash; readable with cat</span></div>
    <div class="row sub2"><span class="m no">&#10007;</span><span class="v">sqlite &mdash; needs a tool to open</span></div>
    <div class="row sub2"><span class="m no">&#10007;</span><span class="v">jsonl &mdash; state must be computed</span></div>
    <div class="row sub2"><span class="m no">&#10007;</span><span class="v">gh issues &mdash; owned by someone else</span></div>
    <div class="row sub2"><span class="m no">&hellip;</span><span class="v">16 further refusals on record</span></div>
  </section>`;

const opening = `
      <div class="tagrow">
        <span class="chip chip-id">dec-0002</span>
        <span class="chip chip-accepted">&#9670; accepted</span>
        <span class="chip chip-date">29 Jul 2026</span>
      </div>
      <p class="arising">Arising from <a href="#">int-0001</a> &mdash; a record that outlives the tool that wrote it.</p>
      <h1 class="ruling">Entries are markdown files in the repo, not rows in a database.</h1>
      <p class="because">
        The ledger&rsquo;s only durable claim is that you can still read it when dira is gone.
        A database is the better store and the worse record &mdash;
        <em>and the index we will eventually want can be rebuilt from the files at any time, while the files cannot be rebuilt from the index.</em>
      </p>`;

const longest = ALTS[1].full.join(' ').replace(/<[^>]+>/g, ' ').replace(/&[a-z]+;/g, '')
  .trim().split(/\s+/).length;

const rail = `
    <aside class="rail">
      <div class="card">
        <h3>What this is</h3>
        <p style="font-size:var(--t-ui);color:var(--ink-mid);margin-bottom:var(--s3)">
          <b style="color:var(--ink)">Nobody typed this page.</b> A coding agent recorded
          it while the work happened, and reads it back at the start of the next session.</p>
        <p style="font-family:var(--mono);font-size: var(--t-chip);color:var(--ink-low)">
          <b style="color:var(--bearing);font-weight:400">int-</b> an intent &middot;
          <b style="color:var(--bearing);font-weight:400">dec-</b> a decision &middot;
          <b style="color:var(--bearing);font-weight:400">converged</b> proven by tests</p>
      </div>
      <div class="card">
        <h3>Edges</h3>
        <div class="kv"><span class="k">derives_from</span><a href="#">int-0001</a></div>
        <div class="kv"><span class="k">informs</span><a href="#">cst-0004</a></div>
        <div class="kv"><span class="k">mirrored</span><a href="#">adr/0002</a></div>
      </div>
      <div class="card">
        <h3>On record</h3>
        <div class="kv"><span class="k">alternatives</span><span>20</span></div>
        <div class="kv"><span class="k">upheld</span><span style="color:var(--converged)">1</span></div>
        <div class="kv"><span class="k">longest grounds</span><span>${longest} w</span></div>
      </div>
    </aside>`;

const tail = `
  <div style="margin-top:var(--s7);padding-top:var(--s4);border-top:1px solid var(--rule);
              display:flex;gap:var(--s3);align-items:baseline;flex-wrap:wrap">
    <p style="font-family:var(--serif);font-size: var(--t-body);color:var(--ink-mid);flex:1;min-width:240px;margin:0">
      Every decision in this repo is recorded like this one.</p>
    <a href="#" style="font-size:var(--t-ui);color:var(--bearing);text-decoration:none;border-bottom:1px solid currentColor">Keep your own &rarr;</a>
  </div>

  <footer class="made">
    <span>2 of 22 entries &middot; <a href="#">see the whole graph</a></span>
    <span class="sep">&middot;</span>
    <span>kept with <a href="#">dira</a>, written by an agent, read by one</span>
  </footer>
</main>
</body>
</html>
`;

// ---- renderers --------------------------------------------------------------
const argBlock = a => `
        <div class="arg${a.upheld ? ' upheld' : ''}">
          <div class="opt"><span class="mark">${a.upheld ? '&#10003;' : '&#10007;'}</span><span class="name">${a.name}</span><span class="tag">${a.upheld ? 'upheld' : 'refused'}</span></div>
          ${a.full.map(p => `<p class="grounds">${p}</p>`).join('\n          ')}
          ${a.revisit ? `<p class="revisit"><b>revisit if</b>${a.revisit}</p>` : ''}
        </div>`;

// (a) let it run --------------------------------------------------------------
const A = `${head('Option A — let it run · long-content study', '')}
</head>
<body>
${banner('A', 'let it run &mdash; no truncation, trust the measure ceilings')}
<main class="wrap">
${chain}
  <div class="body">
    <article>
${opening}
      <section class="args">
        <h2 class="seclabel">Alternatives on record &mdash; 20</h2>
${ALTS.map(argBlock).join('\n')}
      </section>
    </article>
${rail}
  </div>
${tail}`;

// (b) progressive disclosure --------------------------------------------------
const bCss = `
<style>
/* OPTION B — progressive disclosure. The first four alternatives stand as they
   do today; the remaining sixteen sit behind one <details>. Zero JavaScript,
   the same mechanism the rebuilt chain uses for collapse-at-depth. */
.more { margin-top: var(--s5); }
.more > summary {
  list-style: none; cursor: pointer;
  display: flex; align-items: baseline; gap: var(--s3);
  font-family: var(--ui); font-size: var(--t-ui); color: var(--ink-mid);
  padding: var(--s3) var(--s4);
  border: 1px solid var(--rule); border-radius: var(--r-card);
  background: var(--panel);
  transition: border-color var(--dur) var(--ease), background var(--dur) var(--ease);
}
.more > summary::-webkit-details-marker { display: none; }
.more > summary:hover, .more > summary:focus-visible { border-color: var(--ink-low); background: var(--sunk); }
.more > summary .caret {
  display: inline-block; font-family: var(--mono); font-size: var(--t-mono); color: var(--ink-low);
  transition: transform var(--dur) var(--ease);
}
.more[open] > summary .caret { transform: rotate(90deg); }
.more > summary .n { font-family: var(--mono); font-size: var(--t-mono); color: var(--bearing); }
.more > summary .why { color: var(--ink-low); font-size: var(--t-sub); }
.more[open] > summary { margin-bottom: var(--s5); }
</style>`;

const B = `${head('Option B — progressive disclosure · long-content study', '')}
${bCss}
</head>
<body>
${banner('B', 'progressive disclosure &mdash; four shown, sixteen behind &lt;details&gt;')}
<main class="wrap">
${chain}
  <div class="body">
    <article>
${opening}
      <section class="args">
        <h2 class="seclabel">Alternatives on record &mdash; 20</h2>
${ALTS.slice(0, 4).map(argBlock).join('\n')}
        <details class="more">
          <summary><span class="caret">&#9656;</span><span class="n">16</span><span>further alternatives on record</span><span class="why">refused on narrower grounds</span></summary>
${ALTS.slice(4).map(argBlock).join('\n')}
        </details>
      </section>
    </article>
${rail}
  </div>
${tail}`;

// (c) summary / detail split --------------------------------------------------
const cCss = `
<style>
/* OPTION C — summary/detail split. All twenty are visible and scannable; each
   carries a one-line ground on the summary and opens to the full reasoning.
   Zero JavaScript. The strike-through survives on the summary line, which is
   the device this treatment is most at risk of destroying. */
.alt { padding-left: var(--s4); position: relative; border-bottom: 1px solid var(--rule-soft); }
.alt:first-of-type { border-top: 1px solid var(--rule-soft); }
.alt::before { content:""; position:absolute; left:0; top:5px; bottom:5px; width:2px; background: var(--rule); }
.alt.upheld::before { background: var(--converged); width: 3px; }
.alt > summary {
  list-style: none; cursor: pointer; padding: var(--s3) 0 var(--s3) var(--s3);
  transition: background var(--dur) var(--ease);
}
.alt > summary::-webkit-details-marker { display: none; }
.alt > summary:hover, .alt > summary:focus-visible { background: var(--sunk); }
.alt .line1 { display: flex; gap: var(--s2); align-items: baseline; flex-wrap: wrap; }
.alt .caret {
  display: inline-block; flex: none; font-family: var(--mono); font-size: var(--t-mono);
  color: var(--ink-low); transition: transform var(--dur) var(--ease);
}
.alt[open] .caret { transform: rotate(90deg); }
.alt .mark { flex: none; font-family: var(--mono); font-size: var(--t-sub); color: var(--ink-low); }
.alt.upheld .mark { color: var(--converged); }
.alt .name { font-family: var(--serif); font-size: var(--t-body); }
.alt.upheld .name { color: var(--ink); font-weight: 600; }
.alt:not(.upheld) .name {
  color: var(--ink-mid); text-decoration: line-through;
  text-decoration-color: color-mix(in oklab, var(--ink-low) 60%, transparent);
  text-decoration-thickness: 1px;
}
.alt .tag {
  font-family: var(--ui); font-size: var(--t-label); letter-spacing: .12em; text-transform: uppercase;
  padding: 2px 6px; border-radius: var(--r-chip); flex: none; transform: translateY(-2px);
  color: var(--ink-low); background: color-mix(in oklab, var(--ink-low) 12%, transparent);
}
.alt.upheld .tag { color: var(--converged); background: color-mix(in oklab, var(--converged) 14%, transparent); }
.alt .one {
  font-family: var(--serif); font-size: var(--t-small); color: var(--ink-low);
  max-width: var(--m-prose); margin-top: var(--s1); padding-left: var(--s5);
}
.alt .detail { padding: var(--s2) 0 var(--s4) var(--s5); }
.alt .grounds { font-family: var(--serif); font-size: var(--t-body); color: var(--ink-mid); max-width: var(--m-prose); }
.alt .grounds + .grounds { margin-top: var(--s3); }
.alt .revisit { font-size: var(--t-sub); color: var(--ink-low); margin-top: var(--s3); max-width: var(--m-sub); }
.alt .revisit b {
  color: var(--bearing); font-weight: 600; font-size: var(--t-label);
  letter-spacing: .12em; text-transform: uppercase; margin-right: 6px;
}
</style>`;

const altRow = a => `
        <details class="alt${a.upheld ? ' upheld' : ''}"${a.upheld ? ' open' : ''}>
          <summary>
            <span class="line1"><span class="caret">&#9656;</span><span class="mark">${a.upheld ? '&#10003;' : '&#10007;'}</span><span class="name">${a.name}</span><span class="tag">${a.upheld ? 'upheld' : 'refused'}</span></span>
            <span class="one">${a.one}</span>
          </summary>
          <div class="detail">
          ${a.full.map(p => `<p class="grounds">${p}</p>`).join('\n          ')}
          ${a.revisit ? `<p class="revisit"><b>revisit if</b>${a.revisit}</p>` : ''}
          </div>
        </details>`;

const C = `${head('Option C — summary/detail split · long-content study', '')}
${cCss}
</head>
<body>
${banner('C', 'summary / detail &mdash; all twenty scannable, each opens to full grounds')}
<main class="wrap">
${chain}
  <div class="body">
    <article>
${opening}
      <section class="args">
        <h2 class="seclabel">Alternatives on record &mdash; 20</h2>
${ALTS.map(altRow).join('\n')}
      </section>
    </article>
${rail}
  </div>
${tail}`;

await writeFile(resolve(OUT, 'q1-a-run-long.html'), A);
await writeFile(resolve(OUT, 'q1-b-progressive.html'), B);
await writeFile(resolve(OUT, 'q1-c-summary-detail.html'), C);

console.log(`wrote q1-a / q1-b / q1-c — ${ALTS.length} alternatives, longest why_not ${longest} words`);
