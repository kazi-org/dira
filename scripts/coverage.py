#!/usr/bin/env python3
"""
coverage.py — the nothing-is-forgotten check.

Extracts every OBLIGATION mechanically from the repo's structured sources, then
cross-references them against the dispositions registered in docs/coverage.md.

Exits non-zero when an obligation has no disposition, or when a registered
disposition points at an obligation that no longer exists. That non-zero exit is
the guarantee — not the register, and not any plan document.

The register is hand-maintained; the OBLIGATION LIST IS NOT. That asymmetry is the
whole design: a human can forget to add a row, and the checker catches it. If both
sides were hand-maintained this would guarantee nothing.

    python3 scripts/coverage.py            # check; non-zero on any gap
    python3 scripts/coverage.py --list     # print extracted obligations as TSV
    python3 scripts/coverage.py --stub     # emit register rows for uncovered items

Honest limit: this guarantees nothing is forgotten *from the sources it reads*. It
cannot guarantee something never written down anywhere is remembered. Mitigation:
those sources are exactly where the capture hooks write, so new decisions land in
scope automatically rather than needing to be remembered.
"""
import re, sys, os, glob, json, hashlib
from pathlib import Path

def sid(s, n=6):
    """Stable short id. NEVER use hash() — it is randomised per process, so ids
    would differ on every run and the register could never match."""
    return hashlib.sha1(s.encode()).hexdigest()[:n]

def first_cell(line):
    """Text of a markdown table's first cell, bold stripped, parenthetical
    qualifiers dropped. Written this way because a regex anchored on `**...**|`
    silently skipped every row whose bold was followed by a qualifier — a
    silently-dropped obligation is worse than no checker at all."""
    parts = [c.strip() for c in line.strip().strip('|').split('|')]
    if not parts: return ''
    cell = parts[0]
    cell = re.sub(r'\*\*(.+?)\*\*', r'\1', cell)
    cell = re.sub(r'\s*\([^)]*\)\s*$', '', cell)
    return cell.strip()

ROOT = Path(__file__).resolve().parent.parent
os.chdir(ROOT)

def read(p):
    try: return Path(p).read_text()
    except FileNotFoundError: return ""

# --------------------------------------------------------------------------- #
# frontmatter parsing — deliberately dependency-free so this runs anywhere,
# including CI before any Go toolchain exists.
# --------------------------------------------------------------------------- #
def frontmatter(text):
    m = re.match(r'^---\n(.*?)\n---\n', text, re.S)
    if not m: return {}, text
    raw, body = m.group(1), text[m.end():]
    d, cur_list, cur_key = {}, None, None
    for line in raw.split('\n'):
        if not line.strip() or line.lstrip().startswith('#'): continue
        if re.match(r'^\s+', line):
            if cur_key: d.setdefault('_nested', []).append(line.strip())
            continue
        m2 = re.match(r'^([A-Za-z_]+):\s*(.*)$', line)
        if m2:
            k, v = m2.group(1), m2.group(2).strip()
            cur_key = k
            if v.startswith('[') and v.endswith(']'):
                d[k] = [x.strip() for x in v[1:-1].split(',') if x.strip()]
            elif v in ('', '>', '|'):
                d[k] = ''
            else:
                d[k] = v.strip('"\'')
    return d, body

def entries():
    out = []
    for f in sorted(glob.glob('.dira/entries/*.md')):
        txt = read(f)
        fm, body = frontmatter(txt)
        fm['_file'] = f
        fm['_body'] = body
        fm['_raw'] = txt
        out.append(fm)
    return out

# --------------------------------------------------------------------------- #
# OBLIGATION EXTRACTORS
# Each returns (obligation_id, source, one-line statement).
# Add an extractor here and the checker immediately demands dispositions for it.
# --------------------------------------------------------------------------- #
def extract():
    obs = []
    E = entries()

    for e in E:
        eid, kind, state = e.get('id',''), e.get('kind',''), e.get('state','')
        title = e.get('title','').strip()
        if not eid: continue

        # An accepted decision must be implemented somewhere, or explicitly deferred.
        if kind == 'decision' and state == 'accepted':
            obs.append((f'impl:{eid}', eid, f'Implement: {title}'))

        # An open question must be answered or explicitly parked with a trigger.
        if kind == 'question' and state == 'open':
            obs.append((f'answer:{eid}', eid, f'Answer or park: {title}'))

        # An active constraint must have an enforcement point, or it is decoration.
        if kind == 'constraint' and state == 'active':
            obs.append((f'enforce:{eid}', eid, f'Enforcement point for: {title}'))

        # An active intent must be served by at least one child.
        if kind == 'intent' and state == 'active':
            obs.append((f'serve:{eid}', eid, f'Work serving intent: {title}'))

        # Every revisit_if is a future trigger that must be watched, not lost.
        for rv in re.findall(r'revisit_if:\s*[>|]?\s*\n?\s*(.+)', e.get('_raw','')):
            cond = rv.strip().strip('"\'')[:90]
            if cond and not cond.startswith('never'):
                obs.append((f'trigger:{eid}:{sid(cond)}', eid,
                            f'Watch revisit trigger ({eid}): {cond}'))

    # docs/design/DESIGN.md — the open design questions section
    dm = read('docs/design/DESIGN.md')
    sec = re.search(r'## Open design questions\n(.*?)(?=\n## |\Z)', dm, re.S)
    if sec:
        # id from the question TEXT, not its position. Position-derived ids shift
        # whenever an item is answered and struck through, silently orphaning every
        # register row below it -- the same instability as a randomised hash.
        for item in re.findall(r'^\d+\.\s+\*\*(.+?)\*\*', sec.group(1), re.M):
            obs.append((f'design-q:{sid(item)}', 'DESIGN.md', f'Close design question: {item}'))

    # docs/roadmap.md — Blocked rows and upstream asks
    rm = read('docs/roadmap.md')
    blocked = re.search(r'## Blocked\n(.*?)(?=\n## |\Z)', rm, re.S)
    if blocked:
        for line in blocked.group(1).split('\n'):
            if not line.strip().startswith('|'): continue
            cell = first_cell(line)
            if not cell or cell.lower().startswith('item') or set(cell) <= set('-: '): continue
            obs.append((f'blocked:{sid(cell)}', 'roadmap', f'Unblock: {cell}'))
    ups = re.search(r'## Upstream asks.*?\n(.*?)(?=\n## |\Z)', rm, re.S)
    if ups:
        for n, ask in re.findall(r'^\|\s*(\d+)\s*\|\s*(.+?)\s*\|', ups.group(1), re.M):
            obs.append((f'upstream:{n}', 'roadmap', f'Upstream kazi ask {n}: {ask}'))

    # docs/plan.md — every epic must have a lane file, or be explicitly not-yet-planned
    for eid in re.findall(r'^### (E\d+)\s+—', read('docs/plan.md'), re.M):
        obs.append((f'lanes:{eid}', 'plan.md', f'Lanes decomposed for epic {eid}'))

    # dedupe, stable order
    seen, out = set(), []
    for o in obs:
        if o[0] in seen: continue
        seen.add(o[0]); out.append(o)
    return out

# --------------------------------------------------------------------------- #
# the register
# --------------------------------------------------------------------------- #
DISPOSITIONS = ('done', 'covered', 'deferred', 'blocked')

def register():
    """Parse docs/coverage.md rows: | id | disposition | note |"""
    reg = {}
    for line in read('docs/coverage.md').split('\n'):
        m = re.match(r'^\|\s*`([^`]+)`\s*\|\s*([a-z]+)(?::([^|]*))?\s*\|\s*(.*?)\s*\|\s*$', line)
        if m:
            reg[m.group(1)] = (m.group(2), (m.group(3) or '').strip(), m.group(4).strip())
    return reg

SOURCES = ['.dira/entries', 'docs/design/DESIGN.md', 'docs/roadmap.md', 'docs/plan.md']

def untracked_sources():
    """Every file this checker reads must be committed. A source that exists only on
    one machine makes the guarantee hollow — the extractor finds obligations locally
    that vanish on a fresh clone, and the register orphans en masse for a reason that
    has nothing to do with the work. Found the hard way: a global ~/.gitignore
    containing `plan.md` silently excluded docs/plan.md for this repo's whole history."""
    import subprocess
    bad = []
    for src in SOURCES:
        if not Path(src).exists():
            bad.append((src, 'does not exist')); continue
        r = subprocess.run(['git', 'ls-files', '--error-unmatch', src],
                           capture_output=True, text=True)
        if r.returncode != 0:
            bad.append((src, 'exists but is NOT tracked by git'))
    return bad

def main():
    obs = extract()
    if '--list' in sys.argv:
        for oid, src, stmt in obs: print(f'{oid}\t{src}\t{stmt}')
        return 0

    reg = register()
    obs_ids = {o[0] for o in obs}
    bad_sources = untracked_sources()

    uncovered = [o for o in obs if o[0] not in reg]

    # ---- VERIFIED DISPOSITIONS ---------------------------------------------- #
    # Where a disposition makes a claim that can be checked mechanically, check it.
    # A register that accepts unverifiable claims is just prose again. Caught a real
    # false "covered" on first run: a lane agent died before writing its file.
    unverified = []
    for oid, (disp, tgt, note) in reg.items():
        if oid.startswith('lanes:') and disp == 'covered':
            epic = oid.split(':')[1]
            f = f'docs/plan/lanes/{epic}.md'
            if not Path(f).exists():
                unverified.append((oid, f'claims covered but {f} does not exist'))
        if disp == 'covered' and tgt in ('privacy-lint',):
            if not Path('scripts/privacy-lint.py').exists():
                unverified.append((oid, 'claims covered:privacy-lint but the script is missing'))
    bad_disp  = [(k,v) for k,v in reg.items() if v[0] not in DISPOSITIONS]
    orphaned  = [k for k in reg if k not in obs_ids]

    if '--stub' in sys.argv:
        for oid, src, stmt in uncovered:
            # escape pipes or the emitted row breaks the very table it feeds
            safe = stmt.replace('|', r'\|')
            print(f'| `{oid}` | UNASSIGNED | {safe} — source: {src} |')
        return 0

    print(f'obligations extracted : {len(obs)}')
    print(f'registered            : {len(reg)}')
    print(f'uncovered             : {len(uncovered)}')
    print(f'invalid disposition   : {len(bad_disp)}')
    print(f'orphaned register rows: {len(orphaned)}')
    print(f'unverified dispositions: {len(unverified)}')
    print(f'untracked sources      : {len(bad_sources)}')

    fail = False
    if bad_sources:
        fail = True
        print('\nUNTRACKED SOURCE — this checker reads a file that is not in the repo,'
              '\nso its obligations vanish on a fresh clone and the guarantee is hollow:')
        for src, why in bad_sources: print(f'  {src}  ({why})')
    if uncovered:
        fail = True
        print('\nUNCOVERED — no disposition registered in docs/coverage.md:')
        for oid, src, stmt in uncovered: print(f'  {oid}\n      {stmt}  [{src}]')
    if bad_disp:
        fail = True
        print(f'\nINVALID DISPOSITION (must be one of {", ".join(DISPOSITIONS)}):')
        for k,v in bad_disp: print(f'  {k} -> {v[0]}')
    if unverified:
        fail = True
        print('\nUNVERIFIED DISPOSITION — the register claims something checkable that is not true:')
        for oid, why in unverified: print(f'  {oid}\n      {why}')
    if orphaned:
        fail = True
        print('\nORPHANED — registered but the obligation no longer exists '
              '(entry deleted or retitled? remove the row or fix the id):')
        for k in orphaned: print(f'  {k}')

    print('\n' + ('COVERAGE FAIL — something is unaccounted for.' if fail
                  else 'COVERAGE PASS — every obligation has a disposition.'))
    return 1 if fail else 0

if __name__ == '__main__':
    sys.exit(main())
