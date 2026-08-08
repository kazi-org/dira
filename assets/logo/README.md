# dira logo

The mark is a **compass bezel with a two-tone needle, off-north**. It is drawn in a
single warm **antique brass** with no gradient.

## The sibling logic

kazi's ring wraps a **check** — the reconcile loop converging on *objectively done*.
dira's ring wraps a **needle** — the bearing that work was done for. Same ring
language, different interior, so the two read as one family without dira copying
kazi's electric gradient.

The palettes diverge on purpose: kazi is electric because convergence is energetic,
dira is brass because a bearing is a settled thing.

## Two decisions that were arrived at by rendering, not taste

Both were wrong on the first attempt and the renders showed it.

- **The needle is a lozenge, not a line.** A line from the centre reads as a clock
  hand, and the whole mark becomes a stopwatch. Four variants were rendered at 84px,
  28px and 16px: a single needle read as a gauge, a full-diameter chord read as a
  prohibition sign, and an open ring read as a loading spinner. Only the two-tone
  lozenge reads as a compass at every size.
- **The needle sits off-north, at 34 degrees.** A needle at twelve o'clock reads as a
  default. Off-north reads as a bearing someone chose, which is the product.

## Files

| File | Use |
|---|---|
| `dira-mark.svg` | Icon, brass on any light surface. Primary mark. |
| `dira-mark-mono.svg` | Icon in `currentColor`, inherits surrounding text colour. |

## `currentColor` only resolves when the SVG is INLINED

`dira-mark-mono.svg` must be inlined into the document. Loaded through
`<img src="dira-mark-mono.svg">` it is a separate document, `currentColor` falls back
to black, and the mark disappears on a dark background. This was verified by making
the mistake: the `<img>` route renders near-black on `#0f151c`, the inlined route
renders correctly in brass.

The south half of the needle is knocked out with a `mask` rather than filled with a
paper colour, so the mark sits correctly on any background instead of stamping a light
wedge onto a dark one.

## Colours

| Token | Light | Dark |
|---|---|---|
| `--bearing` | `#8a5f18` | `#c99a3e` |

These are the design system's own tokens (`docs/design/tokens.css`). The mark takes no
colour of its own.

## Wordmark

The wordmark is set in the system serif with the second syllable in `--bearing`:
`di` in ink, `ra` in brass. It is CSS, not an asset, so it inherits the type stack and
the theme automatically. See `.wordmark` in `docs/design/tokens.css`.

## Still missing

- A badge/app-icon variant (kazi has `kazi-badge.svg`); needed if dira ever ships an
  app icon or an avatar.
- A favicon. The mark holds at 16px but has not been tested as an `.ico` against a
  browser's own downscaling.
