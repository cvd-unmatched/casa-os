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
- Added a "folders" feature to the home dashboard, so installed apps can be
  grouped instead of living in one flat grid.
- Mobile-friendliness fixes to the dashboard.
