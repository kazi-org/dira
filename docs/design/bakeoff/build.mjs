// Typeface bake-off page builder.
//
// Takes the FROZEN snapshot of s1-decision.html (docs/design/bakeoff/_snapshot/)
// and emits one copy per candidate that is byte-identical except for the serif:
// the same markup, the same tokens.css, the same every other token. The only
// difference between two output pages is which woff2 --serif resolves to.
//
// The control page keeps the shipping Palatino stack untouched, so the founder
// is comparing candidates against what exists today rather than against nothing.
//
//   node docs/design/bakeoff/build.mjs

import { readFile, writeFile, copyFile, mkdir, readdir } from 'node:fs/promises';
import { resolve, join } from 'node:path';

const HERE = import.meta.dirname;
const SNAP = join(HERE, '_snapshot');

// candidate -> the bold face's usable weight range. P052 and Pagella ship a 700
// Bold and no 600, so the 600 the design asks for is served by the 700 face
// rather than synthesised; the OFL variable faces are instanced at a true 600.
const CANDIDATES = {
  p052:         { label: 'URW P052 (Palatino metrics)',  boldWeight: '600 700' },
  pagella:      { label: 'TeX Gyre Pagella (Palatino metrics)', boldWeight: '600 700' },
  sourceserif4: { label: 'Source Serif 4',               boldWeight: '600' },
  newsreader:   { label: 'Newsreader',                   boldWeight: '600' },
  literata:     { label: 'Literata',                     boldWeight: '600' },
};

const faceBlock = (cand, boldWeight) => `
/* ---- BAKE-OFF: self-hosted serif under test --------------------------------
   Subsetted to Latin-1 + the punctuation dira's prose actually uses. In the
   product these bytes come out of embed.FS, not off a URL -- the offline
   constraint (int-0002, cst-0004) is preserved either way. */
@font-face {
  font-family: "BakeoffSerif";
  src: url("fonts/${cand}-regular.core.woff2") format("woff2");
  font-weight: 400; font-style: normal; font-display: block;
}
@font-face {
  font-family: "BakeoffSerif";
  src: url("fonts/${cand}-italic.core.woff2") format("woff2");
  font-weight: 400; font-style: italic; font-display: block;
}
@font-face {
  font-family: "BakeoffSerif";
  src: url("fonts/${cand}-bold.core.woff2") format("woff2");
  font-weight: ${boldWeight}; font-style: normal; font-display: block;
}
:root { --serif: "BakeoffSerif", serif; }
`;

const html = await readFile(join(SNAP, 's1-decision.html'), 'utf8');

for (const [cand, { label, boldWeight }] of Object.entries(CANDIDATES)) {
  let out = html
    .replace('href="../tokens.css"', 'href="_snapshot/tokens.css"')
    .replace('<body>', `<body data-candidate="${cand}">`)
    .replace('</style>', `</style>\n<style>${faceBlock(cand, boldWeight)}</style>`)
    .replace(/<title>[^<]*<\/title>/, `<title>${label} — dira serif bake-off</title>`);
  await writeFile(join(HERE, `s1-${cand}.html`), out);
}

// control: the stack that ships today, nothing overridden.
await writeFile(join(HERE, 's1-control-palatino.html'),
  html
    .replace('href="../tokens.css"', 'href="_snapshot/tokens.css"')
    .replace('<body>', '<body data-candidate="control-palatino">')
    .replace(/<title>[^<]*<\/title>/, '<title>CONTROL: shipping Palatino stack — dira serif bake-off</title>'));

const built = (await readdir(HERE)).filter(f => f.startsWith('s1-') && f.endsWith('.html')).sort();
console.log('built:\n  ' + built.join('\n  '));
