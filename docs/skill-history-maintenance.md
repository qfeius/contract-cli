# Official Skill History

Historical adoption requires an exact whole-directory fingerprint, including references and agent metadata. A matching name or version alone is not trusted. User modifications and extra files continue to require explicit confirmation and installation.

`internal/skillsync/history_fingerprints.json` retains previously verified 1.9.3/1.9.4 installations and adds the pinned GitHub releases in `history_sources.json`. The latter covers 1.9.5 through 1.9.13, including raw GitHub trees and known package-version stamps.

To add another official release:

1. Verify the tag and its peeled commit against `qfeius/contract-cli` on GitHub. Fetch that specific tag without force; do not trust a working tree, a moving branch, or a user's installed directory as provenance.
2. Add the version and verified commit to `internal/skillsync/history_sources.json`.
3. Generate and inspect the catalog:

```sh
node scripts/skill-history.js --write
node scripts/skill-history.js --check
node --test scripts/skill-history.test.js
go test ./internal/skillsync -run 'TestOfficialReleaseHistory|TestHistoryChecksEveryOccupied' -count=1
```

Generation rejects tags that no longer match the pinned commit and rejects symbolic links or non-regular entries. It preserves earlier trusted entries; review catalog removals or unexplained additions explicitly. No runtime download or relaxed version matching is introduced.

The Go provenance test needs the pinned Git objects locally. A shallow checkout skips unavailable releases, so a skipped test is not provenance verification. The offline Node tests and normal installation/upgrade regressions remain part of `make release-check`.
