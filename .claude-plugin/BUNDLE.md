# The dira Claude Code plugin bundle

Lane: `E8-L5` (`docs/plan/tasks/E8-L5.md`). This file is packaging documentation,
not a draft — it carries no `status`/`posted` frontmatter because nothing here
is sent, posted, or published by this lane. See the absolutes in
`docs/plan/lanes/E8.md`.

## What exists today

```
.claude-plugin/
├── plugin.json   # the manifest: metadata + inline hook declarations
└── BUNDLE.md     # this file
```

That is the whole bundle right now. **`skills/dira/` does not exist and this
lane does not create it** — see "Ownership boundary" below.

## What the bundle looks like once E2 ships the skill

```
.claude-plugin/
└── plugin.json
skills/
└── dira/
    ├── SKILL.md       # E2's content
    └── ...            # any other files E2's skill needs
```

Claude Code discovers `skills/dira/SKILL.md` by directory convention — the
manifest needs no `skills` field to declare it, the same way kazi's plugin
manifest (`kazi-org/kazi`, `lib/kazi/plugin/manifest.ex`) needs none for
`skills/kazi/`.

`dira` has no MCP server in scope (`dec-0008`: dira integrates with kazi only
through kazi's public `--json` surface and Claude Code hooks — never a
service dira itself exposes), so unlike kazi's plugin manifest, `plugin.json`
has no `mcpServers` block. If that changes, the manifest gains one; nothing
here should be read as ruling it out permanently.

## Ownership boundary (the collision the lane prompt asks to name, not resolve)

- **This lane (E8-L5) owns `.claude-plugin/plugin.json` and this file** —
  packaging and listing.
- **E2 owns `skills/dira/*`** — the skill's content — and `dira install-hooks`.
- Neither lane should write the other's files. If E2's lane also produces a
  `.claude-plugin/plugin.json`, one of the two silently wins and the other's
  work is lost; the lead should assign this file to exactly one owner (E8-L5,
  since packaging the *bundle* — not authoring the skill — is this lane's
  whole job, and the hook-matching check below already needs a single
  `plugin.json` to check against).

`hooks` in `plugin.json` is a direct, hand-maintained copy of
`hooks/settings.example.json`'s three entries (`SessionStart`, `Stop`,
`PreCompact`) — same event names, same command strings, same timeouts.
`node docs/growth/scripts/check-drafts.mjs` fails if the two drift, so a
rename on one side only is caught rather than silently shipped.

## Versioning contract (the second collision)

kazi already hit this and solved it (ADR-0077, `lib/kazi/plugin/manifest.ex`):
**the plugin version must equal the exact binary release it was cut from**,
rendered and published by the release pipeline in the same CI run that cuts
the binary release — never hand-bumped, never published on its own cadence.
Otherwise the marketplace bundle can lag the binary and a user ends up with a
skill or hook referencing a flag the installed binary doesn't have yet.

`"version": "0.0.0-unreleased"` in `plugin.json` today is a placeholder, not a
claim about this repo's own state — `dira` released v0.1.1 (2026-08-18, brew
install live) and the plugin is published in `kazi-org/claude-plugins`
(second entry beside kazi, `.claude-plugin/marketplace.json` there pins its
own `version`). **This file's `version` field is not that pin and never has
been** — the rendering/pinning step analogous to `mix kazi.plugin` (reading
this manifest shape and stamping in the release tag on every publish) still
does not exist in this checkout's release pipeline, so `plugin.json` here
stays a hand-maintained source the marketplace repo's own entry is cut from,
not a live mirror of it. Do not treat this field as evidence of what version
is actually installable — read the marketplace repo's own manifest for that.
