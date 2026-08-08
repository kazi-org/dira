#!/usr/bin/env python3
"""Compare each candidate's metrics against the face this design was actually
tuned on: macOS Palatino. Everything is normalised to units-per-em so faces with
different UPM are comparable.

Reports, per face:
  - advance of "0"      -> this is literally what the CSS `ch` unit resolves to,
                           and every --m-* measure token in tokens.css is in ch.
  - mean lowercase advance for the real prose of the page (frequency-weighted)
  - x-height, cap-height, ascender/descender (line-box behaviour)
  - max |delta| vs Palatino across the ASCII set -> metric compatibility test
"""
import os, json
from fontTools.ttLib import TTFont, TTCollection
from fontTools.varLib.instancer import instantiateVariableFont

SRC = "fonts/src"

# the real prose this page sets in the serif: .ruling, .because, .arg .name, .grounds
PROSE = (
 "Go, not Elixir/OTP, despite kazi's stack. "
 "dira is invoked from hooks several times a session, in the latency path of a human "
 "waiting on a prompt. OTP's value is supervising long-lived concurrent work - exactly "
 "what kazi needs, and exactly what dira has none of. "
 "The standard library already covers files, JSON and YAML, an embedded HTTP server for "
 "dira ui, and SQLite through a single dependency. A wide contributor pool matters for a "
 "tool that wants drive-by pull requests. "
 "BEAM start-up costs tens to hundreds of milliseconds before any work happens, and a "
 "Burrito-wrapped binary pays a first-run unpacking cost besides. Free CI and free "
 "distribution is a real saving - and it loses anyway, because paying BEAM's start-up for "
 "a process with no concurrency and no uptime buys the one thing that does not apply. "
 "An equivalent start-up and single-binary story with better guarantees, but slower to "
 "write and a smaller contributor pool for an open-source tool that wants casual "
 "contribution. "
 "Node's start-up sits in the same range as BEAM's. Bun fixes that but ships a large "
 "binary and a young ecosystem, for a tool meant to be boring infrastructure."
)

FACES = {
    "Palatino (macOS, the tuned-on face)": ("/System/Library/Fonts/Palatino.ttc", 0, None),
    "URW P052 Roman":                      (f"{SRC}/P052-Roman.otf", None, None),
    "TeX Gyre Pagella Regular":            (f"{SRC}/pagella/qpl2_501otf/texgyrepagella-regular.otf", None, None),
    "Source Serif 4 (400, opsz 16)":       (f"{SRC}/SourceSerif4.ttf", None, {"wght": 400, "opsz": 16}),
    "Newsreader (400, opsz 16)":           (f"{SRC}/Newsreader.ttf", None, {"wght": 400, "opsz": 16}),
    "Literata (400, opsz 16)":             (f"{SRC}/Literata.ttf", None, {"wght": 400, "opsz": 16}),
    "DejaVu Serif (stock-Linux fallback)": ("dejavu", None, None),
}

def load(path, idx, inst):
    if path == "dejavu":
        for p in ("/usr/share/fonts/truetype/dejavu/DejaVuSerif.ttf",
                  "fonts/src/DejaVuSerif.ttf"):
            if os.path.exists(p):
                return TTFont(p)
        return None
    if idx is not None:
        return TTCollection(path).fonts[idx]
    f = TTFont(path)
    if inst:
        f = instantiateVariableFont(f, inst, updateFontNames=False, inplace=True)
    return f

def widths(font):
    upm = font["head"].unitsPerEm
    cmap = font.getBestCmap()
    hmtx = font["hmtx"]
    def adv(ch):
        g = cmap.get(ord(ch))
        return hmtx[g][0] / upm if g and g in hmtx.metrics else None
    return adv, upm

rows = {}
base_adv = None
for name, (path, idx, inst) in FACES.items():
    font = load(path, idx, inst)
    if font is None:
        rows[name] = {"error": "not installed on this machine"}
        continue
    adv, upm = widths(font)
    os2 = font["OS/2"]
    hhea = font["hhea"]

    zero = adv("0")
    # frequency-weighted mean advance over the page's real serif prose
    tot = n = 0
    misses = 0
    for c in PROSE:
        a = adv(c)
        if a is None:
            misses += 1
            continue
        tot += a; n += 1
    mean_prose = tot / n

    ascii_set = [chr(c) for c in range(0x20, 0x7F)]
    a_map = {c: adv(c) for c in ascii_set}

    rows[name] = {
        "upm": upm,
        "adv_zero_em": round(zero, 4),
        "mean_prose_advance_em": round(mean_prose, 4),
        "x_height_em": round(getattr(os2, "sxHeight", 0) / upm, 4) if getattr(os2, "sxHeight", None) else None,
        "cap_height_em": round(getattr(os2, "sCapHeight", 0) / upm, 4) if getattr(os2, "sCapHeight", None) else None,
        "hhea_ascender_em": round(hhea.ascender / upm, 4),
        "hhea_descender_em": round(hhea.descender / upm, 4),
        "hhea_linegap_em": round(hhea.lineGap / upm, 4),
        "default_line_em": round((hhea.ascender - hhea.descender + hhea.lineGap) / upm, 4),
        "prose_chars_unmapped": misses,
        "_ascii": a_map,
    }
    if base_adv is None:
        base_adv = a_map

# metric-compatibility: max abs delta vs Palatino across ASCII
for name, r in rows.items():
    if "error" in r: continue
    d = [abs(r["_ascii"][c] - base_adv[c]) for c in base_adv
         if r["_ascii"].get(c) is not None and base_adv[c] is not None]
    ident = sum(1 for c in base_adv
                if r["_ascii"].get(c) is not None and base_adv[c] is not None
                and abs(r["_ascii"][c] - base_adv[c]) < 1e-9)
    r["max_ascii_delta_vs_palatino_em"] = round(max(d), 5)
    r["mean_ascii_delta_vs_palatino_em"] = round(sum(d) / len(d), 5)
    r["ascii_glyphs_byte_identical_width"] = f"{ident}/{len(d)}"
    del r["_ascii"]

print(json.dumps(rows, indent=1))
with open("fonts/metrics-report.json", "w") as f:
    json.dump(rows, f, indent=1)
