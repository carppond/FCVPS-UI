# Embedded agent binaries

This directory is populated by `scripts/build-release.sh` (T-32). Each file
follows the naming convention `agent-<os>-<arch>` (e.g. `agent-linux-amd64`,
`agent-darwin-arm64`). At v1 development time only this README is present
so the `embed.FS` declaration compiles — the install handler returns 404
for any platform that has not yet been built into the hub.
