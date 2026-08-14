# bbc/tams — vendored ADR corpus (E2-L7-T1)

Shallow-cloned read-only at the pinned commit below, per `qst-0003-neutrality.md`
section 1 and `docs/plan/tasks/E2-L7.md`. Nothing in the source repo was forked,
starred, or otherwise interacted with.

Every `.md` file under `docs/adr/` at this commit is vendored here **except**
`docs/adr/README.md`, which is the directory's own index page and not an ADR —
excluding it is what makes the count exactly 49, `dec-0028`'s own pinned number.

commit: 8cd1ca536322ce0941e58d2c67210b2c7cd3ee80
source: https://github.com/bbc/tams

| file | repo-relative path | sha256 |
|---|---|---|
| 0000-use-markdown-adrs-to-record-design-decisions.md | docs/adr/0000-use-markdown-adrs-to-record-design-decisions.md | 923ba72bae7a0c4c1435d915223d2beab4c698e0b5fc5642aca2fd89c904fb16 |
| 0001-expand-created-modified-metadata.md | docs/adr/0001-expand-created-modified-metadata.md | 62c0246aebf91fd34a46e84f0df92e9627aa2f50b63b9e2a02641805fed8e689 |
| 0002-add-sources-to-api.md | docs/adr/0002-add-sources-to-api.md | 97bdb1b4fb88366a4addfc83c8c8d5cbb16bd7af9bbee3bf481e68865a69c47a |
| 0003-item-timestamps-managed-internally.md | docs/adr/0003-item-timestamps-managed-internally.md | 0661d6f55ebf3025f7707f4a74deae7f39fb55d6cd7c93fece5e092a23b58752 |
| 0004-content-deletion.md | docs/adr/0004-content-deletion.md | 93472356020954f7316d3b7c6e811948ec5f3f31a91b38243ed2a6f65ceac7c6 |
| 0004a-ancestry-relationships.md | docs/adr/0004a-ancestry-relationships.md | 86425b82eca223848328cb61f2e362866574e03a8f861b5eef201824841eea25 |
| 0005-flow-read-write-permissions.md | docs/adr/0005-flow-read-write-permissions.md | 660f9e472f879377e1db3c4c424cb2903619106a95e707a572690b6c908bb6ac |
| 0006-flow-status.md | docs/adr/0006-flow-status.md | e0689eb57b6eae5248a4a0a299e9ebe6a0b70a68f38a5dc5caee3ba9a7d2e464 |
| 0007-use-timerange-in-flow-segments.md | docs/adr/0007-use-timerange-in-flow-segments.md | e8f16de3998df1b46bb53f3dbbf1a2bf07cc5f073bdace93c7104b8b17720191 |
| 0008-move-flow-parameters-into-a-sub-property.md | docs/adr/0008-move-flow-parameters-into-a-sub-property.md | 829dfed517c05d33d6d44b8413c87400a1e70320017d84123f9d06b90b1c84ae |
| 0009-allow-segment-overlap.md | docs/adr/0009-allow-segment-overlap.md | bb60fe307e3aa83f48598b7d1bfd6be49a707ac0935366da786965c3763f4472 |
| 0010-pagination-of-listing-endpoints.md | docs/adr/0010-pagination-of-listing-endpoints.md | 218dbd70eda79a52671eefe227c77d05d20e37f93474259da2aa0f7655483053 |
| 0011-random-storage-object-ids.md | docs/adr/0011-random-storage-object-ids.md | 0aeede118b735803ec35b26304abd42b6cb2ba2996574e5f367f8ffcd046fa1e |
| 0012-add-flow-collections.md | docs/adr/0012-add-flow-collections.md | e38d71da94833f45aebc945107871d2c0daf118f83d76b05329049d0b7533259 |
| 0013-timeline-exposed-by-flows.md | docs/adr/0013-timeline-exposed-by-flows.md | 287c90ebea6217b6dd66e5472e6cfd2392fac0ada0a6de095aa609a43c57b49c |
| 0014-add-event-stream.md | docs/adr/0014-add-event-stream.md | 570526e7ddff1f2c54f6e5001b37b100d276d777071560f03e7b4d96c1b1cca3 |
| 0015-flow-segment-get-url-expectations.md | docs/adr/0015-flow-segment-get-url-expectations.md | c2cc258d073e96d8fad23c0dae0617736e75d3d2e511326109ee54358a7687e9 |
| 0016-checksums-and-filesize.md | docs/adr/0016-checksums-and-filesize.md | 9cd14dea8a28fa72f1bd13ac9f96bbeeb941cfbd2cd3269dd06cbfa294e71164 |
| 0017-container-mapping.md | docs/adr/0017-container-mapping.md | 0c767522445986d5a07d0ef4651a6fa41e5d6122e5546511bdf4369b1bf7674c |
| 0018-restrict-direct-source-modification.md | docs/adr/0018-restrict-direct-source-modification.md | 93af8086016c6cd042e88c9834bf54ce0043e594cdcd5496c5b4a1bce1f8099a |
| 0019-consolidate-modified-updated-terms.md | docs/adr/0019-consolidate-modified-updated-terms.md | b1b8971ea9cc1e50c1b8bcd9f0f21ed0169d01ab9b021319675ce48ba03f25ee |
| 0020-version-signalling.md | docs/adr/0020-version-signalling.md | d6c767356237206b65f678d2dab2e6d3425d0b96b27eff09c3d30ebd9d0a408c |
| 0021-storage-label-format.md | docs/adr/0021-storage-label-format.md | 58e8da273c6da59e4ebc6ea4bf7b41efaa1c07e90dc9751f4e9fddd4d8af1de3 |
| 0022-flow-bit-rate-properties.md | docs/adr/0022-flow-bit-rate-properties.md | 6115e5b7f867516e51394b5edd83df8d0db8d2dce178d6aaf52c2a0dc3b1a653 |
| 0023-filter-segment-get-urls.md | docs/adr/0023-filter-segment-get-urls.md | 67ade39c5fbdcdf139d41f45a877025a87febe656df81fe0984b1f0407e76490 |
| 0024-source-level-edit.md | docs/adr/0024-source-level-edit.md | a286fdaec0070fa7907d6af0ce1e5f3fbbdb74a1e88bdd9079cfdce1866e1d4b |
| 0025-flow-property-updates.md | docs/adr/0025-flow-property-updates.md | 5a3952546ff71cbf0ebc272d71842894a5d5942ee7a8f72c4ba49b25f55dfdbb |
| 0026-updated-webhook-events-and-filters.md | docs/adr/0026-updated-webhook-events-and-filters.md | 3be468e899e70c883055cf6855d9445e9f1f2d6fcd617e65e34016b1e5fceed1 |
| 0027-add-objects-api-endpoint.md | docs/adr/0027-add-objects-api-endpoint.md | 4331ee4b75f2c86f2ff622c78a870ace4e2577bbe33767553af46bf5db6154db |
| 0028-authentication-methods.md | docs/adr/0028-authentication-methods.md | 53132ef4d06d7d630aae818b3ed7da8f9d4e455b735f719b004dfe9dcc02032b |
| 0029-bulk-flow-segments.md | docs/adr/0029-bulk-flow-segments.md | 83bf501750a077b10f8010fe78e0e111f38b9908dd497f5f85a7b9c0a80cfae1 |
| 0030-allow-external-media-objects.md | docs/adr/0030-allow-external-media-objects.md | fafe8c43475f80c7945904fa509c67430b28acd7b5af8a4449aaa3d3f76e9a01 |
| 0031-flow-image-support.md | docs/adr/0031-flow-image-support.md | 2f7dadcc5f898eca5fb58c42d1b67c562592e3649c59bb78410ee965bc896a74 |
| 0032-specifying-storage-backend.md | docs/adr/0032-specifying-storage-backend.md | 3cc7436c9b7c334dd8d9869682cb30888b52a2f480b054f1e957a7be0144f976 |
| 0033-segment-created-metadata.md | docs/adr/0033-segment-created-metadata.md | 91c361131904bfc2232a2bedf65f8beca895068483ec4405528f39f17a32b86c |
| 0034-storage-allow-object_ids.md | docs/adr/0034-storage-allow-object_ids.md | 70526ae9ebcbabbd9a022bf2efcb56c14c730193b8e3526ee445511007e788f3 |
| 0035-fine-grained-auth.md | docs/adr/0035-fine-grained-auth.md | d5e219b67c06f4b01a5c2b6386c68dcad2c5157a0b88ca5dd7bfb82f339d1ede |
| 0036-specifying-partial-segment-usage.md | docs/adr/0036-specifying-partial-segment-usage.md | 8f04d5d39cd391ed3a01cbea7dd9cd63552a8a15ef36e420f7ad21ffacfbac9f |
| 0037-improve-webhooks.md | docs/adr/0037-improve-webhooks.md | c9efee6333fe05dd96f1831f83c76482c0483bd772d0946415e26849e16ba753 |
| 0038-improved-storage-management.md | docs/adr/0038-improved-storage-management.md | ea0d109a0652bf115b4d4f1bfb593f4dce7108425524d001f5152b7416885cea |
| 0039-remove-pre-actions.md | docs/adr/0039-remove-pre-actions.md | 708e8657db5735c3e24644f7c26ac70093e13d86048b1b89b29367d9f8605edb |
| 0040-tag-usability-enhancements.md | docs/adr/0040-tag-usability-enhancements.md | 3750d4b29384fb01172ce88f233dcdf3aa03c600dd310195c0059c1c7bb4e23a |
| 0041-require-explicit-framerate.md | docs/adr/0041-require-explicit-framerate.md | 4e8a922051c0382d5fead409c87262ebf3cd53c24b284a78e95c8d38cd8649f2 |
| 0042-uncontrolled-object-instance-labels.md | docs/adr/0042-uncontrolled-object-instance-labels.md | e41a1ca65dbee53e28f80b9a6ff0247b740a45a2941f9bf6fa218ad8f611023d |
| 0043-signalling-retention-time.md | docs/adr/0043-signalling-retention-time.md | 2b6d527a588278a55c7e4108cba51c72fdc52fc94606105a8c97e148bed99062 |
| 0044-signalling-timeouts.md | docs/adr/0044-signalling-timeouts.md | 9dd8e9e2778e9e110ce42ac6dc91b6d174fbc822e04ea68f9b804023389826d2 |
| 0045-flow-init-segments.md | docs/adr/0045-flow-init-segments.md | 99fe19622e80dc92a1bf5037ec355886c2f6b098fa56d9e3a6e39121d2aeae13 |
| 0046-governance.md | docs/adr/0046-governance.md | e583ed6a5892067c063a438e910a2872acd63841ae5084565a8aad739ca99216 |
| 0048-media-integrity.md | docs/adr/0048-media-integrity.md | f8d800af3107d1aa0c285bc8023e0d69d8895da76aedae46a661fd43686a5239 |
