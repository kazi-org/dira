# Dira logo

The mark is a **D with a northeast-pointing compass cutout**. The broad vertical
stem and curved bowl establish the initial; the transparent needle records a
chosen direction. The silhouette stays legible as a favicon and works in one ink.

The website pairs it with a bold lowercase sans-serif wordmark, `dira.`, with an
accent-colored period. Editorial headings retain Pagella, so the identity has a
clear typographic hierarchy.

| File | Use |
|---|---|
| `dira-mark.svg` | Primary vector; cobalt in light mode, pale cobalt in dark mode. |
| `dira-mark-mono.svg` | Inline vector inheriting `currentColor`, for monochrome use. |

Both marks use an even-odd path with a transparent cutout: no background-colored
patch, external font, raster image, or mask ID. Keep the 48 × 48 viewBox and its
built-in clear space. Use at 16 pixels or larger; prefer 32–40 pixels in navigation.

`dira-mark.svg` is the source asset. The website build copies it to
`site/public/favicon.svg`, used by the favicon, header, footer, and ledger pages.
The shared wordmark component is `site/src/components/Brand.astro`.

The monochrome variant inherits color only when inlined. When loaded through an
`img`, use the primary mark, which defines its own light and dark colors.

See [the website brand specification](../../site/BRAND.md) for palette roles,
scale provenance, and measured contrast. This redesign replaces the previous
brass compass-bezel mark for the public website and distributable logo assets.
