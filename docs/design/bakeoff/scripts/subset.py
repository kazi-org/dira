#!/usr/bin/env python3
"""Subset each bake-off candidate to the character set dira's prose actually needs.

Two tiers per face:
  core = Basic Latin + Latin-1 Supplement + the punctuation/arrows this design uses
  ext  = core + Latin Extended-A (European names in real decision records)

The serif never sets the chain, the ids or the status marks -- those are --mono --
so no box-drawing, no geometric marks, are included. Verified: the current screens
contain zero U+2500..257F.
"""
import os, sys, subprocess, json
from fontTools.ttLib import TTFont
from fontTools import subset
from fontTools.varLib.instancer import instantiateVariableFont

SRC = "fonts/src"
OUT = "fonts/out"
os.makedirs(OUT, exist_ok=True)

def rng(a, b): return set(range(a, b + 1))

CORE = (
    rng(0x0020, 0x007E) |            # basic latin
    rng(0x00A0, 0x00FF) |            # latin-1 supplement (incl. x00D7 multiplication)
    {0x2010, 0x2011, 0x2013, 0x2014} |               # hyphens, dashes
    {0x2018, 0x2019, 0x201A, 0x201C, 0x201D, 0x201E} |  # curly quotes
    {0x2039, 0x203A} |                               # single guillemets
    {0x2022, 0x2026, 0x2032, 0x2033} |               # bullet, ellipsis, primes
    {0x2212} |                                       # minus
    {0x2190, 0x2191, 0x2192, 0x2193, 0x2197, 0x2198} |  # arrows
    {0x2264, 0x2265} |                               # <= >=
    {0x00B7}                                         # middot (in latin-1, explicit)
)
EXT = CORE | rng(0x0100, 0x017F)     # Latin Extended-A

# candidate -> [(style, src file, css weight, css style, variable-instance dict|None)]
CANDIDATES = {
    "p052": [
        ("regular", "P052-Roman.otf",  "400", "normal", None),
        ("italic",  "P052-Italic.otf", "400", "italic", None),
        ("bold",    "P052-Bold.otf",   "600 700", "normal", None),
    ],
    "pagella": [
        ("regular", "pagella/qpl2_501otf/texgyrepagella-regular.otf", "400", "normal", None),
        ("italic",  "pagella/qpl2_501otf/texgyrepagella-italic.otf",  "400", "italic", None),
        ("bold",    "pagella/qpl2_501otf/texgyrepagella-bold.otf",    "600 700", "normal", None),
    ],
    "sourceserif4": [
        ("regular", "SourceSerif4.ttf",        "400", "normal", {"wght": 400, "opsz": 16}),
        ("italic",  "SourceSerif4-Italic.ttf", "400", "italic", {"wght": 400, "opsz": 16}),
        ("bold",    "SourceSerif4.ttf",        "600", "normal", {"wght": 600, "opsz": 16}),
    ],
    "newsreader": [
        ("regular", "Newsreader.ttf",        "400", "normal", {"wght": 400, "opsz": 16}),
        ("italic",  "Newsreader-Italic.ttf", "400", "italic", {"wght": 400, "opsz": 16}),
        ("bold",    "Newsreader.ttf",        "600", "normal", {"wght": 600, "opsz": 16}),
    ],
    "literata": [
        ("regular", "Literata.ttf",        "400", "normal", {"wght": 400, "opsz": 16}),
        ("italic",  "Literata-Italic.ttf", "400", "italic", {"wght": 400, "opsz": 16}),
        ("bold",    "Literata.ttf",        "600", "normal", {"wght": 600, "opsz": 16}),
    ],
}

report = {}
for cand, styles in CANDIDATES.items():
    report[cand] = {"styles": {}, "missing": {}}
    for style, src, weight, fstyle, inst in styles:
        path = os.path.join(SRC, src)
        for tier, uni in (("core", CORE), ("ext", EXT)):
            font = TTFont(path)
            if inst:
                font = instantiateVariableFont(font, inst, updateFontNames=False, inplace=True)
            have = set()
            for t in font["cmap"].tables:
                have |= set(t.cmap.keys())
            missing = sorted(uni - have)
            if tier == "core":
                report[cand]["missing"][style] = ["U+%04X" % m for m in missing]
            opts = subset.Options()
            opts.layout_features = ["kern", "liga", "calt", "ccmp", "locl",
                                    "onum", "lnum", "tnum", "pnum", "frac", "case", "mark", "mkmk"]
            opts.name_IDs = ["*"]
            opts.name_legacy = True
            opts.notdef_outline = True
            opts.recalc_bounds = True
            opts.drop_tables += ["DSIG"]
            opts.desubroutinize = False
            sub = subset.Subsetter(options=opts)
            sub.populate(unicodes=sorted(uni & have))
            sub.subset(font)
            font.flavor = "woff2"
            out = os.path.join(OUT, f"{cand}-{style}.{tier}.woff2")
            font.save(out)
            font.close()
            report[cand]["styles"].setdefault(style, {})[tier] = os.path.getsize(out)

print(json.dumps(report, indent=1))
with open("fonts/subset-report.json", "w") as f:
    json.dump(report, f, indent=1)
