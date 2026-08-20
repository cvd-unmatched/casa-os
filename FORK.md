# About this fork

This repository is a personal fork combining three upstream projects, all
licensed under Apache License 2.0:

- [IceWhaleTech/CasaOS](https://github.com/IceWhaleTech/CasaOS) (main backend,
  `casaos` service), forked from `main` at commit
  `0d3b2f444ec0193193cf03eef6d43c6e35b0183e`.
- [IceWhaleTech/CasaOS-UI](https://github.com/IceWhaleTech/CasaOS-UI) (frontend),
  vendored into [`UI/`](UI/) as plain files instead of a git submodule.
- [IceWhaleTech/CasaOS-AppManagement](https://github.com/IceWhaleTech/CasaOS-AppManagement)
  (`casaos-app-management` service - actually applies compose changes and
  manages containers), vendored into [`AppManagement/`](AppManagement/) at
  commit `debfa317f0f996b91b43210e8d57799461388704`.

All three live in this one repository as plain subdirectories, each still its
own Go module (`AppManagement/` has its own `go.mod`) or npm project (`UI/`),
built as separate artifacts by the same release workflow.

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
- Fixed changing an app's icon recreating its containers: `AppManagement`
  used to bake the icon into every service's Docker labels purely so an old
  stats-polling loop could read it back, but Compose treats labels as part of
  a service's config, so any icon-only edit looked like a real config change.

## Updating a running install from this repo

Pushing a tag like `v1.2.3` runs [`.github/workflows/release.yml`](.github/workflows/release.yml),
which builds both backends (linux/amd64 + linux/arm64) and the UI, and
attaches `casaos-linux-amd64.tar.gz` / `casaos-linux-arm64.tar.gz` (each
containing `casa`, `casaos-app-management`, and `www`) to a GitHub Release on
this repo.

[`update.sh`](update.sh) (run on the server, as root) downloads the release
matching the server's architecture, **backs up the current binaries and web UI
first**, swaps them in, restarts the `casaos` and `casaos-app-management`
services, and automatically rolls both back if either service fails to come
back up.

Before running it for the first time, confirm the paths at the top of the
script actually match your install:

```bash
systemctl cat casaos                       # confirms the main binary path (ExecStart=...)
systemctl cat casaos-app-management        # confirms the app-management binary path
sudo find /var/lib/casaos /usr/share/casaos -maxdepth 3 -iname index.html
                                            # confirms where the web UI static files live
```

Then:

```bash
sudo ./update.sh
```

If any of those don't match this server's actual layout (or you just want
backups written somewhere other than the default `/mnt/mydata/casaos-backups/update.sh`),
override them with environment variables instead of editing the script -
useful for running the same unmodified copy across several machines:

```bash
sudo BACKUP_ROOT=/srv/backups BINARY_PATH=/usr/local/bin/casaos ./update.sh
```

The original `.goreleaser.yaml` and the previous `release.yml` (which called
`IceWhaleTech/github`'s private reusable workflow and needed OAuth secrets for
Google Drive/Dropbox/OneDrive) are IceWhale's own release pipeline and are not
used by this fork's simplified workflow.
