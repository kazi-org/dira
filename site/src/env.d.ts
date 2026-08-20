/// <reference types="astro/client" />

// Injected by vite's `define` in astro.config.mjs from `git describe --tags`
// at build time — "pre-release" on a tagless checkout, the real tag
// otherwise. Declared here so pages that read it type-check.
declare const __DIRA_VERSION__: string;
