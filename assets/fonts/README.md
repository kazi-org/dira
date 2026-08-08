# Fonts

## TeX Gyre Pagella — the serif dira ships

**Why self-hosted at all.** The previous stack was
`"Palatino", "Palatino Linotype", "Book Antiqua", Georgia, serif`. It resolves on macOS,
mostly on Windows, and on a stock Linux install resolves to **none of them** — falling
through to DejaVu Serif, whose metrics are nothing like Palatino's. A git-native
developer tool has a large Linux audience, and every measure, leading and type ratio in
this design was tuned by rendering on macOS with Palatino loaded. That is the exact
failure the system-font rule was adopted to prevent, relocated to where the render
harness could not see it, because the harness runs on macOS.

**Why Pagella specifically.** It is a Palatino metric clone, so every ratio already
tuned stays valid — self-hosting became a determinism fix rather than a re-tune. It was
also the only candidate in the bake-off with **complete glyph coverage** for the
characters this design uses, and the smallest subsetted payload.

Rejected, with reasons, in `docs/decisions-pending/serif-bakeoff.md`:

- **URW P052** — also a Palatino clone, and **eliminated on licence**. It is AGPL-3.0
  with an exception covering PostScript and PDF documents *only*. Embedding a woff2 in a
  Go binary and serving it over HTTP is neither, so the exception does not reach it.
- **Source Serif 4** — cleanest licence (OFL 1.1) and also preserves the ratios exactly,
  but missing U+2011.
- **Literata / Newsreader** — different metrics (full re-tune) and glyph gaps. Newsreader
  has no arrows at all, which is a standing constraint for a tool that renders arbitrary
  agent-written prose.

## Licence — GUST Font Licence (LPPL 1.3c)

Full text in `LICENSE-pagella.txt`. Two obligations, both met:

1. **Renaming a derived work is requested, not required** — GUST states this explicitly.
   These files keep the Pagella name and are unmodified in outline.
2. **LPPL 6b requires prominent notice of the nature of any change.** These are
   **subsets**: the character set is reduced to the Latin, punctuation and symbol
   coverage this design uses. No outline, metric or hinting data was altered. Recorded
   in the repo `NOTICE` and here.

Upstream: https://www.gust.org.pl/projects/e-foundry/tex-gyre/pagella

## Files

| File | Weight |
|---|---|
| `pagella-regular.core.woff2` | 400 |
| `pagella-italic.core.woff2` | 400 italic |
| `pagella-bold.core.woff2` | 700 |

Subsets, not the full faces. Regenerate with the bake-off tooling if the character set
needs to grow — a missing glyph falls back silently, which is the failure mode this
whole change exists to eliminate.
