# nulib/meadow — vendored ADR corpus (E2-L7-T1)

Shallow-cloned read-only at the pinned commit below, per `qst-0003-neutrality.md`
section 1 and `docs/plan/tasks/E2-L7.md`. Nothing in the source repo was forked,
starred, or otherwise interacted with.

**One exclusion, documented rather than silent.** The ADR directory
(`doc/architecture/decisions/`) holds **32** real `.md` files at this exact
commit — one more than the pinned count of 31 this lane inherits from
`dec-0028`/`qst-0003-neutrality.md`. Two of them are numbered `0024`:

  - `0024-version-management.md`, added upstream 2020-03-05
    (`e7451cbf67fd364da16168310e497a3adc6946f6`), and
  - `0024-iiif-manifests.md`, added upstream 2020-03-06
    (`f422ea9e3d866e0432f74bad0029e24957d2b444`), one day later, which
    explicitly opens "Supercedes ADR 22" but was evidently never renumbered
    off the collision.

This is almost certainly why the original neutrality experiment counted 31: a
number-keyed listing (a dict/set keying documents by their four-digit ADR
number, which is a natural way to walk a directory of `NNNN-slug.md` files)
would silently keep one of the two and drop the other, and 32 files under such
a walk report as 31. It is not possible to tell from the repository alone
which one the original run kept, and it does not change any measured result
either way: both `0024` files are pure-Nygard ADRs with no alternatives
section, so the corpus's 0/31/0 (documents with a reasoned alternative /
alternatives extracted) answer is identical whichever of the two is vendored.

This vendoring keeps `0024-version-management.md` — the one that claimed the
number first in the source repo's own history — and excludes
`0024-iiif-manifests.md`. **Flagged to the integrator in `.orchestrator-status.md`**
as a discrepancy between the pinned count and the real corpus at the pinned
commit, rather than silently resolved.

commit: 10f6ac2cb3f3c4e2894c4cf5dbed67544516faf1
source: https://github.com/nulib/meadow

| file | repo-relative path | sha256 |
|---|---|---|
| 0001-record-architecture-decisions.md | doc/architecture/decisions/0001-record-architecture-decisions.md | 3173cf0a216c2b32d2c924914c155cca12216cf64472e3be2c2a9d18058db4a4 |
| 0002-ulids.md | doc/architecture/decisions/0002-ulids.md | 57fde145135d97f4dc31de349447a5de309cfd28cc887429a4dded98711f74ea |
| 0003-terraform.md | doc/architecture/decisions/0003-terraform.md | 4321dda6610484c4e62cb5c46b0cc8241208b46f1afc71773770b07522f21dc0 |
| 0004-api.md | doc/architecture/decisions/0004-api.md | d56c6ab6e71cb064e508efced19bfa630841b53dc2d5ddfca513f9654af0fa15 |
| 0005-multistage-docker.md | doc/architecture/decisions/0005-multistage-docker.md | 2f586113fc561e954ebccd79db6a77c259707922f10fc08969fdca6f8072093c |
| 0006-honeybadger.md | doc/architecture/decisions/0006-honeybadger.md | c4f4b4f0797db4c3db29d991250e5f62e548b8e33312192e5e8a34e69f706815 |
| 0007-code-analysis.md | doc/architecture/decisions/0007-code-analysis.md | 273aba59c9a02cd0002dc93141a57ccd49a5f232472a6b03685227e9b2f023d6 |
| 0008-api-documentation.md | doc/architecture/decisions/0008-api-documentation.md | 25fc399bacec583d77f79211b19ae225ba836795f654aa1509ee7af9714708cc |
| 0009-tailwind-css-framework.md | doc/architecture/decisions/0009-tailwind-css-framework.md | d231b278988783560f944498fff427d736c8722a70c2dae1d0316f0dbf3b3872 |
| 0010-dependencies.md | doc/architecture/decisions/0010-dependencies.md | 184a4f0d083871a83cb03e5a15b7e20f22147a9177fa4eb961af5c1db133a9eb |
| 0011-yarn.md | doc/architecture/decisions/0011-yarn.md | c6ccd7da63c282ef6ad08bc28273c5525ca7cecdf0e883cad5a8f6165a34b2e4 |
| 0012-websockets.md | doc/architecture/decisions/0012-websockets.md | d5faaa64ad7f3a872a126ec6d8153cca73f0e901b3d3b40c64001061530e9bf9 |
| 0013-use-graphql-for-api.md | doc/architecture/decisions/0013-use-graphql-for-api.md | 02ead94195f68823649a9ff6ac64beb4a007c60afcbd0a0a4fdd5f6f5ac7b6c1 |
| 0014-active-directory-groups.md | doc/architecture/decisions/0014-active-directory-groups.md | 5cb6477861278161e0a91ecb69a020eb28c7a0b7227cee236d6418bb81ed0e60 |
| 0015-phoenix-context-organization.md | doc/architecture/decisions/0015-phoenix-context-organization.md | a69ef07df27f194c9d9e2658a64290448ed703783d79f770140c8cf2c8c37d46 |
| 0016-ingest-pipeline-spec.md | doc/architecture/decisions/0016-ingest-pipeline-spec.md | 5b3cd5c9ec77a0bde29cf445b98714aed400eca06183d15235b3ce3326d761ed |
| 0017-preservation-strategy.md | doc/architecture/decisions/0017-preservation-strategy.md | 90b9aea0ae888bb8f0031c70c4272e2e9c267e494100f2b3ae41e1ec45293edc |
| 0018-preservation-storage-object-naming-scheme.md | doc/architecture/decisions/0018-preservation-storage-object-naming-scheme.md | 290f1c73f81fd28ac307904fb73ede5adba405c3d3af0c121a9fad1e77c70df5 |
| 0019-directory-layout-revisions.md | doc/architecture/decisions/0019-directory-layout-revisions.md | d0acd7c8669f0279385e5b467e7458c511b9c36f4304db4627ecbc16cdafd1fc |
| 0020-test-coverage-strategy.md | doc/architecture/decisions/0020-test-coverage-strategy.md | ad80300ffae0a680fedbc12d35ae2d000c730148808247c86da3304c0b45e9a1 |
| 0021-elasticsearch-indexing.md | doc/architecture/decisions/0021-elasticsearch-indexing.md | 82f6ef1033d8bb7e5b992d060e8f6c0df2f8de5efec27f0858f1f822b0736471 |
| 0022-iiif-manifests.md | doc/architecture/decisions/0022-iiif-manifests.md | fd53d1b289854daed457925c62818cb867c396eb42f028c1ca876bfb26bdd2ec |
| 0023-uuids.md | doc/architecture/decisions/0023-uuids.md | ee1a1f8846a4f65ed26d0dab95cbc34df95266d21e9a21217ab982ac3c68bc07 |
| 0024-version-management.md | doc/architecture/decisions/0024-version-management.md | afa0a536773b031a868d1fc97b5bef6072934c546ae8ed09fcf0085f960771f1 |
| 0025-ui-component-directory-structure.md | doc/architecture/decisions/0025-ui-component-directory-structure.md | 382ada3e37c5f0fce616220a185fc5dd285b244dba2877a622172a1debc088a7 |
| 0026-reentrant-processes.md | doc/architecture/decisions/0026-reentrant-processes.md | bdea30341e846f7e56dd6db3555e4df7c92a4a871414161710171b4442572551 |
| 0027-semantic-versioning.md | doc/architecture/decisions/0027-semantic-versioning.md | 67e5de802d13d51c03f1e2d606afaf7553cf4d4120b4492dd80b28675eed10f5 |
| 0028-use-only-fileset-digest-for-preservation-object-name.md | doc/architecture/decisions/0028-use-only-fileset-digest-for-preservation-object-name.md | 7943c995bee80fc815430c631cbf7bf26c9b1d33df567db72759c285c9e642fd |
| 0029-npm.md | doc/architecture/decisions/0029-npm.md | 95a0af2019825014de2fde03f2596fb5a1c430f89af4982371076ba6c51decdf |
| 0030-bun.md | doc/architecture/decisions/0030-bun.md | e4677a1665fd13438e1e74754a922e616676f98770dc4326ca6bc87512420fed |
| 0031-ai-provenance.md | doc/architecture/decisions/0031-ai-provenance.md | dfdfc661e90c3fd054f53eb4ef712f8827b5d61dbe9e3ceeb5852b2afc8401f8 |
