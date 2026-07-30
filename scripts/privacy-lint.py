#!/usr/bin/env python3
"""
privacy-lint.py — the enforcement point for cst-0003.

cst-0003 says the private/public boundary must be "enforced by the architecture
rather than by care", and that a violation is a SECURITY bug rather than a UX bug.
Until this file existed, the constraint was a sentence in a markdown document with
nothing checking it — which is care, not architecture.

A private strategic note committed into public git history cannot be un-published.
So this runs in CI and fails the build rather than warning.

    python3 scripts/privacy-lint.py          # non-zero on any violation
    python3 scripts/privacy-lint.py -v       # also print what passed

Four invariants, each crisp enough to be checked rather than judged:

  P1  A public (tier = "repo") ledger contains no entry marked `private: true`.
      Private entries belong to the person tier and must never be committed here.

  P2  No committed file leaks the LABEL of a parent declared `visibility = "private"`.
      Config declares the secret; the lint greps every committed surface for it.

  P3  Every namespaced ref used in an edge resolves to a namespace declared in
      [parents]. An undeclared foreign ref is a typo or an invention (dec-0011) —
      treating it as "withheld" would let real mistakes hide behind the boundary.

  P4  Mirrored ADRs carry refs, never foreign prose. cst-0003 rule 3: cite the ref
      only, never the text. An ADR is a derived public artifact (dec-0009), so it is
      exactly the leak path rule 3 exists to close.

What this does NOT check, stated so nobody mistakes its scope: it cannot see a
private ledger it has no access to, so it cannot verify that inherited context was
never *persisted* (cst-0003 rule 2) beyond the P1 marker check. Rule 2 needs a
runtime assertion in the brief-injection path, which lands with E1/E5 — tracked as
its own obligation rather than assumed covered here.
"""
import re, sys, glob, os
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
os.chdir(ROOT)
VERBOSE = '-v' in sys.argv

def read(p):
    try: return Path(p).read_text()
    except (FileNotFoundError, IsADirectoryError): return ''

# ---- config ---------------------------------------------------------------- #
CFG = read('.dira/config.toml')

def cfg_tier():
    m = re.search(r'^\s*tier\s*=\s*"([^"]+)"', CFG, re.M)
    return m.group(1) if m else 'repo'

def declared_parents():
    """{namespace: {'private': bool, 'label': str|None}} from [parents], including
    commented-out examples? No — commented lines are NOT declarations. Only live
    TOML counts, or the lint would accept refs to namespaces nobody configured."""
    out = {}
    sec = re.search(r'^\[parents\]\s*$(.*?)(?=^\[|\Z)', CFG, re.M | re.S)
    if not sec: return out
    for line in sec.group(1).split('\n'):
        s = line.strip()
        if not s or s.startswith('#'): continue
        m = re.match(r'^([a-z][a-z0-9_-]*)\s*=\s*\{(.*)\}\s*$', s)
        if not m: continue
        ns, body = m.group(1), m.group(2)
        priv = 'visibility' in body and '"private"' in body
        lab = re.search(r'label\s*=\s*"([^"]+)"', body)
        out[ns] = {'private': priv, 'label': lab.group(1) if lab else None}
    return out

# ---- entries --------------------------------------------------------------- #
def entry_files():
    return sorted(glob.glob('.dira/entries/*.md'))

def fm_of(text):
    m = re.match(r'^---\n(.*?)\n---\n', text, re.S)
    return (m.group(1), text[m.end():]) if m else ('', text)

REF = re.compile(r'\b([a-z][a-z0-9_-]*):((?:int|dec|qst|cst|note)-\d{4,})\b')

violations = []
passed = []

def V(check, msg):  violations.append((check, msg))
def P(check, msg):  passed.append((check, msg))

TIER = cfg_tier()
PARENTS = declared_parents()

# ---- P1: no private entries in a public ledger ----------------------------- #
if TIER == 'repo':
    bad = []
    for f in entry_files():
        fm, _ = fm_of(read(f))
        if re.search(r'^\s*private:\s*true\s*$', fm, re.M):
            bad.append(f)
    if bad:
        for f in bad:
            V('P1', f'{f} is marked `private: true` in a tier="repo" (public) ledger')
    else:
        P('P1', f'no private:true entries in {len(entry_files())} entries of a public ledger')
else:
    P('P1', f'skipped — ledger tier is "{TIER}", not a public repo ledger')

# ---- P2: no private parent LABEL leaks into any committed surface ----------- #
secrets = {ns: v['label'] for ns, v in PARENTS.items() if v['private'] and v['label']}
if secrets:
    surfaces = entry_files() + glob.glob('docs/**/*.md', recursive=True) + \
               glob.glob('docs/adr/*.md') + ['README.md', '.dira/config.toml']
    hits = 0
    for ns, label in secrets.items():
        for f in set(surfaces):
            if f == '.dira/config.toml': continue   # the declaration itself
            if label and label in read(f):
                V('P2', f'private parent "{ns}" label "{label}" leaks in {f}')
                hits += 1
    if not hits:
        P('P2', f'{len(secrets)} private parent label(s) absent from all committed surfaces')
else:
    P('P2', 'no private parent declares a label — nothing to leak')

# ---- P3: every namespaced ref resolves to a declared namespace -------------- #
undeclared = {}
for f in entry_files():
    fm, _ = fm_of(read(f))
    for m in re.finditer(r'^\s*to:\s*(\S+)\s*$', fm, re.M):
        target = m.group(1).strip('"\'')
        if target.startswith('kazi:'):   # external execution ref, not a dira ledger
            continue
        r = REF.match(target)
        if r and r.group(1) not in PARENTS:
            undeclared.setdefault(r.group(1), []).append(f)
if undeclared:
    for ns, files in undeclared.items():
        V('P3', f'edge target namespace "{ns}:" is not declared in [parents] '
                f'(used in {", ".join(os.path.basename(x) for x in files)}) — '
                f'an undeclared foreign ref is a typo, not a withheld parent')
else:
    P('P3', 'every namespaced edge target resolves to a declared parent namespace')

# ---- P4: mirrored ADRs cite refs, never foreign prose ---------------------- #
adrs = [f for f in glob.glob('docs/adr/*.md') if not f.endswith('README.md')]
if adrs:
    bad = 0
    for f in adrs:
        txt = read(f)
        for m in REF.finditer(txt):
            ns = m.group(1)
            if ns == 'kazi':
                continue
            # a bare ref is fine; a ref followed by an em-dash/colon and prose is
            # quoting the foreign entry's content into a public derived artifact
            tail = txt[m.end():m.end()+120]
            if re.match(r'\s*(—|--|:)\s*\S{25,}', tail):
                V('P4', f'{f} appears to quote foreign entry text after ref '
                        f'"{m.group(0)}" — cst-0003 rule 3 allows the ref only')
                bad += 1
    if not bad:
        P('P4', f'{len(adrs)} mirrored ADR(s) cite refs without foreign prose')
else:
    P('P4', 'no mirrored ADRs exist yet — nothing to check')

# ---- report ---------------------------------------------------------------- #
if VERBOSE or not violations:
    for c, m in passed:
        print(f'  ok   [{c}] {m}')
if violations:
    print()
    for c, m in violations:
        print(f'  FAIL [{c}] {m}')
    print(f'\nPRIVACY LINT FAIL — {len(violations)} violation(s). '
          f'cst-0003 treats these as security bugs, not style.')
    sys.exit(1)

print(f'\nPRIVACY LINT PASS — cst-0003 enforced by {len(passed)} checks.')
