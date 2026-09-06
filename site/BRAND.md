# Dira — True North

Cobalt marks direction and action; cool slate gives the record a quiet reading
surface. Dark mode uses midnight-blue surfaces and pale cobalt accents. The new
D-shaped compass replaces the brass bezel. The serif remains the voice of the
record; the compact sans-serif wordmark identifies the tool.

## Palette

Colors derive from the MIT-licensed [Radix Colors](https://www.radix-ui.com/colors)
12-step Slate, Indigo, Jade, and Tomato scales. Backgrounds use steps 1–3; text
uses steps 11–12. Light positive and conflict text mix toward step 12 to remain
readable on recessed surfaces. Dark secondary and accent text mix toward step 12
to meet the APCA floor on blue wells. Dark panels mix Slate 2 and Indigo 2.

`public/brand.css` is the shared source for the website and its static ledger
snapshots. `src/styles/tokens.css` imports it for marketing and documentation pages;
the snapshot generator links it after the CLI's original styles. The CLI's
embedded theme and historical design mockups retain their existing palette.

| Role | Light | Dark |
|---|---|---|
| `ground` | `#f9f9fb` | `#11131f` |
| `panel` | `#fcfcfd` | `#161821` |
| `sunk` | `#f0f0f3` | `#182449` |
| `rule` | `#cdced6` | `#43484e` |
| `rule-soft` | `#e0e1e6` | `#2e3135` |
| `ink` | `#1c2024` | `#edeef0` |
| `ink-mid` | `#60646c` | `#c5c8cd` |
| `ink-low` | `#60646c` | `#bcc0c5` |
| `bearing` | `#3a5bc7` | `#a9bbff` |
| `bearing-hover` | `#1f2d5c` | `#d6e1ff` |
| `positive` | `#1f755d` | `#1fd8a4` |
| `conflict` | `#bf3216` | `#fea38d` |
| `action-fill` | `#3e63dd` | `#9eb1ff` |
| `action-hover` | `#3358d4` | `#d6e1ff` |
| `action-ink` | `#fdfdfe` | `#11131f` |
| `control-border` | `#80838d` | `#696e77` |
| `brand-soft` | `#edf2fe` | `#182449` |

Cobalt is reserved for brand/action/focus, jade for accepted or successful states,
and tomato for conflicts. Neutral rules divide content; the stronger control
border identifies interactive controls. Status text always includes a word or
symbol, so hue is never the only indicator. The page uses three saturated hue
families, with neutrals carrying most of its area.

## Contrast evidence

Measured from the role values using WCAG relative luminance and `apca-w3`.
Each text row reports the lowest ratio across ground, panel, and recessed surfaces.
All regular text pairs exceed 4.5:1. Dark text pairs also exceed APCA |Lc| 60;
long-form primary text exceeds |Lc| 90. Focus, control borders, and primary action
fills exceed 3:1 against adjacent page and panel surfaces in both modes.

| Mode | Text role (worst surface) | WCAG ratio | APCA Lc |
|---|---|---:|---:|
| light | ink on sunk | 14.41 | 94.6 |
| light | ink-mid on sunk | 5.22 | 70.9 |
| light | ink-low on sunk | 5.22 | 70.9 |
| light | bearing on sunk | 5.27 | 70.8 |
| light | positive on sunk | 4.91 | 68.8 |
| light | conflict on sunk | 5.01 | 68.3 |
| light | action-ink on action-fill | 5.12 | 79.4 |
| light | action-ink on action-hover | 5.92 | 83.4 |
| dark | ink on sunk | 13.04 | 93.5 |
| dark | ink-mid on sunk | 9.02 | 69.9 |
| dark | ink-low on sunk | 8.28 | 65.1 |
| dark | bearing on sunk | 8.10 | 64.0 |
| dark | positive on sunk | 8.23 | 65.4 |
| dark | conflict on sunk | 7.81 | 62.3 |
| dark | action-ink on action-fill | 8.96 | 62.8 |
| dark | action-ink on action-hover | 14.14 | 87.5 |

## Logo

The source SVG lives at `../assets/logo/dira-mark.svg`; the build copies it to
`public/favicon.svg`. The same SVG supplies header, footer, ledger navigation,
and browser icon. `Brand.astro` owns the website wordmark. The cutout is transparent
and the mark switches color using `prefers-color-scheme` even inside an `img`.

Verified in light and dark mode on the real homepage and ledger, at mobile and
wide viewports. The SVG silhouette was also checked at 16, 24, 32, and 48 pixels.
