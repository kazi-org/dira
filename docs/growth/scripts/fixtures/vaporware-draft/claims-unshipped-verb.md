# fixture: a draft claiming a verb dira does not have

This file exists only so `check-launch-accuracy.mjs` can prove it fails loudly against
a false claim, before it is ever pointed at the real drafts. It is not a real draft and
carries none of the `DRAFT-CONTRACT.md` frontmatter — `check-drafts.mjs` never scans
this directory, since it is named `fixtures/`.

The claim under test:

```
$ dira sync
syncing your ledger to the team dashboard...
```

`dira sync` is not a real verb. It does not appear in `dira --help`'s command list and
it does not appear in README.md's `## The core verbs` table. A draft that promises this
would be promising a hosted, paid capability that does not exist — exactly what
`dec-0007` and `dec-0012` forbid.
