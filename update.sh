#!/usr/bin/env bash
# Pulls the latest release from this fork's GitHub repo and swaps it into an
# existing CasaOS install, backing up what's currently installed first.
#
# Usage: sudo ./update.sh
#
# Before running for the first time, verify BINARY_PATH and WWW_PATH below
# actually match your install (see FORK.md for how to check). This script
# refuses to run if either path doesn't already exist, rather than guessing.

set -euo pipefail

REPO="cvd-unmatched/casa-os"
SERVICE="casaos"
BINARY_PATH="/usr/bin/casaos"
WWW_PATH="/var/lib/casaos/www"
BACKUP_ROOT="/mnt/mydata/casaos-backups/update.sh"

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

BACKUP_MOUNT="$(dirname "$BACKUP_ROOT")"
while [[ "$BACKUP_MOUNT" != "/" && ! -d "$BACKUP_MOUNT" ]]; do
  BACKUP_MOUNT="$(dirname "$BACKUP_MOUNT")"
done
if ! mountpoint -q "$BACKUP_MOUNT" 2>/dev/null && [[ "$BACKUP_MOUNT" != "/" ]]; then
  echo "$BACKUP_MOUNT doesn't look like a mounted filesystem right now." >&2
  echo "Refusing to back up there - it would silently land on the root disk instead. Check your mount (df -h) and re-run." >&2
  exit 1
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

if [[ ! -x "$WORK_DIR/casa" || ! -d "$WORK_DIR/www" ]]; then
  echo "Downloaded archive didn't contain the expected casa binary + www directory." >&2
  exit 1
fi

BACKUP_DIR="$BACKUP_ROOT/$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"
echo "==> Backing up current install to $BACKUP_DIR"
cp -a "$BINARY_PATH" "$BACKUP_DIR/casa"
cp -a "$WWW_PATH" "$BACKUP_DIR/www"

echo "==> Stopping $SERVICE"
systemctl stop "$SERVICE"

echo "==> Installing new binary + web UI"
cp "$WORK_DIR/casa" "$BINARY_PATH"
chmod +x "$BINARY_PATH"
rm -rf "$WWW_PATH"
cp -a "$WORK_DIR/www" "$WWW_PATH"

echo "==> Starting $SERVICE"
systemctl start "$SERVICE"
sleep 2

if systemctl is-active --quiet "$SERVICE"; then
  echo "==> $SERVICE is running on $TAG."
  echo "    Check the dashboard still loads and you can log in before assuming this is fine."
else
  echo "==> $SERVICE failed to start! Rolling back automatically." >&2
  systemctl stop "$SERVICE" || true
  cp "$BACKUP_DIR/casa" "$BINARY_PATH"
  chmod +x "$BINARY_PATH"
  rm -rf "$WWW_PATH"
  cp -a "$BACKUP_DIR/www" "$WWW_PATH"
  systemctl start "$SERVICE"
  echo "==> Rolled back to the previous version. Check 'journalctl -u $SERVICE -e' for what went wrong." >&2
  exit 1
fi

echo ""
echo "Backup of the previous version kept at: $BACKUP_DIR"
echo "To roll back manually later:"
echo "  sudo systemctl stop $SERVICE"
echo "  sudo cp $BACKUP_DIR/casa $BINARY_PATH"
echo "  sudo rm -rf $WWW_PATH && sudo cp -a $BACKUP_DIR/www $WWW_PATH"
echo "  sudo systemctl start $SERVICE"
