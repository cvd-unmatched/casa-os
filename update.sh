#!/usr/bin/env bash
# Pulls the latest release from this fork's GitHub repo and swaps it into an
# existing CasaOS install, backing up what's currently installed first.
#
# Usage: sudo ./update.sh
#
# Before running for the first time, verify BINARY_PATH, WWW_PATH, and
# APP_MANAGEMENT_BINARY_PATH below actually match your install (see FORK.md
# for how to check). This script refuses to run if any of them don't already
# exist, rather than guessing.
#
# Every setting below can be overridden per-invocation with an environment
# variable instead of editing this file - handy when running the same script
# unmodified across several machines with different layouts, e.g.:
#   sudo BACKUP_ROOT=/srv/backups ./update.sh

set -euo pipefail

REPO="${REPO:-cvd-unmatched/casa-os}"
SERVICE="${SERVICE:-casaos}"
BINARY_PATH="${BINARY_PATH:-/usr/bin/casaos}"
WWW_PATH="${WWW_PATH:-/var/lib/casaos/www}"
APP_MANAGEMENT_SERVICE="${APP_MANAGEMENT_SERVICE:-casaos-app-management}"
APP_MANAGEMENT_BINARY_PATH="${APP_MANAGEMENT_BINARY_PATH:-/usr/bin/casaos-app-management}"
BACKUP_ROOT="${BACKUP_ROOT:-/mnt/mydata/casaos-backups/update.sh}"
# Every run adds a new timestamped backup and never overwrites an old one -
# without this, they'd accumulate forever. Kept ones are only pruned after
# *this* run succeeds, so a run that itself fails never deletes a backup it
# might still be needed to roll back to.
KEEP_BACKUPS="${KEEP_BACKUPS:-5}"

if [[ $EUID -ne 0 ]]; then
  echo "Run this as root (sudo ./update.sh) - it needs to replace $BINARY_PATH and restart $SERVICE." >&2
  exit 1
fi

if [[ ! -f "$BINARY_PATH" ]]; then
  echo "Expected the current CasaOS binary at $BINARY_PATH but didn't find it." >&2
  echo "Edit BINARY_PATH at the top of this script to match your install, then re-run." >&2
  exit 1
fi

if [[ ! -d "$WWW_PATH" ]]; then
  echo "Expected the current CasaOS web UI at $WWW_PATH but didn't find it." >&2
  echo "Edit WWW_PATH at the top of this script to match your install, then re-run." >&2
  exit 1
fi

if [[ ! -f "$APP_MANAGEMENT_BINARY_PATH" ]]; then
  echo "Expected the current CasaOS App Management binary at $APP_MANAGEMENT_BINARY_PATH but didn't find it." >&2
  echo "Edit APP_MANAGEMENT_BINARY_PATH at the top of this script to match your install, then re-run." >&2
  exit 1
fi

# Only enforce "must be a real separate mount" for paths under /mnt/ - that's
# the one convention where writing there while it's unmounted would silently
# land backups on the root disk instead of the intended bulk-storage disk.
# A custom BACKUP_ROOT elsewhere (e.g. /root/casaos-backups) is trusted as-is.
if [[ "$BACKUP_ROOT" == /mnt/* ]]; then
  # Walk up checking `mountpoint` at every level (not just the first
  # existing directory - a subdirectory like .../casaos-backups can easily
  # already exist from a previous run without itself being the mount).
  check_path="$BACKUP_ROOT"
  found_mount=""
  while [[ "$check_path" != "/" && "$check_path" != "." ]]; do
    check_path="$(dirname "$check_path")"
    if mountpoint -q "$check_path" 2>/dev/null; then
      found_mount="$check_path"
      break
    fi
  done
  if [[ -z "$found_mount" ]]; then
    echo "No mounted filesystem found above $BACKUP_ROOT." >&2
    echo "Refusing to back up under /mnt/... - it would silently land on the root disk instead. Check your mount (df -h) and re-run, or point BACKUP_ROOT somewhere else." >&2
    exit 1
  fi
fi

case "$(uname -m)" in
  x86_64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "Unsupported architecture $(uname -m). This fork's release workflow only builds amd64 and arm64." >&2; exit 1 ;;
esac

echo "==> Checking latest release for $REPO ($ARCH)"
RELEASE_JSON=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")
TAG=$(echo "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
ASSET_URL=$(echo "$RELEASE_JSON" | grep -o "https://[^\"]*casaos-linux-$ARCH.tar.gz")

if [[ -z "$ASSET_URL" ]]; then
  echo "Couldn't find a casaos-linux-$ARCH.tar.gz asset on the latest release ($TAG)." >&2
  echo "Check https://github.com/$REPO/releases - has the release workflow finished running?" >&2
  exit 1
fi

echo "==> Latest release: $TAG"

WORK_DIR=$(mktemp -d)
trap 'rm -rf "$WORK_DIR"' EXIT

echo "==> Downloading $ASSET_URL"
curl -fsSL -o "$WORK_DIR/casaos.tar.gz" "$ASSET_URL"
tar -xzf "$WORK_DIR/casaos.tar.gz" -C "$WORK_DIR"

if [[ ! -x "$WORK_DIR/casa" || ! -d "$WORK_DIR/www" || ! -x "$WORK_DIR/casaos-app-management" ]]; then
  echo "Downloaded archive didn't contain the expected casa binary, casaos-app-management binary, and www directory." >&2
  exit 1
fi

BACKUP_DIR="$BACKUP_ROOT/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"
echo "==> Backing up current install to $BACKUP_DIR"
cp -a "$BINARY_PATH" "$BACKUP_DIR/casa"
cp -a "$WWW_PATH" "$BACKUP_DIR/www"
cp -a "$APP_MANAGEMENT_BINARY_PATH" "$BACKUP_DIR/casaos-app-management"

restore_backup() {
  cp "$BACKUP_DIR/casa" "$BINARY_PATH"
  chmod +x "$BINARY_PATH"
  rm -rf "$WWW_PATH"
  cp -a "$BACKUP_DIR/www" "$WWW_PATH"
  cp "$BACKUP_DIR/casaos-app-management" "$APP_MANAGEMENT_BINARY_PATH"
  chmod +x "$APP_MANAGEMENT_BINARY_PATH"
}

echo "==> Stopping $SERVICE and $APP_MANAGEMENT_SERVICE"
systemctl stop "$SERVICE" "$APP_MANAGEMENT_SERVICE"

echo "==> Installing new binaries + web UI"
cp "$WORK_DIR/casa" "$BINARY_PATH"
chmod +x "$BINARY_PATH"
rm -rf "$WWW_PATH"
cp -a "$WORK_DIR/www" "$WWW_PATH"
cp "$WORK_DIR/casaos-app-management" "$APP_MANAGEMENT_BINARY_PATH"
chmod +x "$APP_MANAGEMENT_BINARY_PATH"

echo "==> Starting $SERVICE and $APP_MANAGEMENT_SERVICE"
systemctl start "$SERVICE" "$APP_MANAGEMENT_SERVICE"
sleep 2

if systemctl is-active --quiet "$SERVICE" && systemctl is-active --quiet "$APP_MANAGEMENT_SERVICE"; then
  echo "==> $SERVICE and $APP_MANAGEMENT_SERVICE are running on $TAG."
  echo "    Check the dashboard still loads and you can log in before assuming this is fine."

  # Only prune once this run's own backup is confirmed to have been for a
  # successful update - keep the newest $KEEP_BACKUPS (directory names sort
  # chronologically since they're timestamps) and remove anything older.
  mapfile -t old_backups < <(ls -1 "$BACKUP_ROOT" | sort -r | tail -n "+$((KEEP_BACKUPS + 1))")
  if [[ ${#old_backups[@]} -gt 0 ]]; then
    echo "==> Pruning ${#old_backups[@]} backup(s) older than the newest $KEEP_BACKUPS"
    for old in "${old_backups[@]}"; do
      rm -rf "${BACKUP_ROOT:?}/$old"
    done
  fi
else
  echo "==> One of the services failed to start! Rolling back automatically." >&2
  systemctl stop "$SERVICE" "$APP_MANAGEMENT_SERVICE" || true
  restore_backup
  systemctl start "$SERVICE" "$APP_MANAGEMENT_SERVICE"
  echo "==> Rolled back to the previous version. Check 'journalctl -u $SERVICE -e' and 'journalctl -u $APP_MANAGEMENT_SERVICE -e' for what went wrong." >&2
  exit 1
fi

echo ""
echo "Backup of the previous version kept at: $BACKUP_DIR"
echo "To roll back manually later:"
echo "  sudo systemctl stop $SERVICE $APP_MANAGEMENT_SERVICE"
echo "  sudo cp $BACKUP_DIR/casa $BINARY_PATH"
echo "  sudo rm -rf $WWW_PATH && sudo cp -a $BACKUP_DIR/www $WWW_PATH"
echo "  sudo cp $BACKUP_DIR/casaos-app-management $APP_MANAGEMENT_BINARY_PATH"
echo "  sudo systemctl start $SERVICE $APP_MANAGEMENT_SERVICE"
