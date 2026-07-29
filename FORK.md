# About this fork

This repository is a personal fork combining two upstream projects, both licensed
under Apache License 2.0:

- [IceWhaleTech/CasaOS](https://github.com/IceWhaleTech/CasaOS) (backend), forked
  from `main` at commit `0d3b2f444ec0193193cf03eef6d43c6e35b0183e`.
- [IceWhaleTech/CasaOS-UI](https://github.com/IceWhaleTech/CasaOS-UI) (frontend),
  vendored into [`UI/`](UI/) as plain files instead of a git submodule so both
  live in one repository.

All credit for the original design and implementation goes to the CasaOS
authors and contributors. See [LICENSE](LICENSE) for the full license text.

## Changes made in this fork

- Dependency updates for known vulnerabilities (see commit history).
- Fixed an auth bypass in the JWT middleware (it trusted a spoofable
  `X-Forwarded-For` header to decide "is this request local") and moved
  `/v1/sys/debug` behind authentication - see commit history for details.
- Added a "folders" feature to the home dashboard, so installed apps can be
  grouped instead of living in one flat grid.
- Mobile-friendliness fixes to the dashboard.
- `common.VERSION` is set to `999.0.0` so CasaOS's built-in "Settings > Update"
  (which checks IceWhale's own api.casaos.io) never thinks an official update
  is available - that update path would `curl | bash` IceWhale's installer as
  root and overwrite this fork. Use `update.sh` instead (below).

## Updating a running install from this repo

Pushing a tag like `v1.2.3` runs [`.github/workflows/release.yml`](.github/workflows/release.yml),
which builds the backend (linux/amd64 + linux/arm64) and the UI, and attaches
`casaos-linux-amd64.tar.gz` / `casaos-linux-arm64.tar.gz` to a GitHub Release
on this repo.

[`update.sh`](update.sh) (run on the server, as root) downloads the release
matching the server's architecture, **backs up the current binary and web UI
first**, swaps them in, restarts the `casaos` service, and automatically rolls
back if the service fails to come back up.

Before running it for the first time, confirm the two paths at the top of the
script actually match your install:

```bash
systemctl cat casaos            # confirms the binary path (ExecStart=...)
sudo find /var/lib/casaos /usr/share/casaos -maxdepth 3 -iname index.html
                                 # confirms where the web UI static files live
```

Then:

```bash
sudo ./update.sh
```

The original `.goreleaser.yaml` and the previous `release.yml` (which called
`IceWhaleTech/github`'s private reusable workflow and needed OAuth secrets for
Google Drive/Dropbox/OneDrive) are IceWhale's own release pipeline and are not
used by this fork's simplified workflow.
