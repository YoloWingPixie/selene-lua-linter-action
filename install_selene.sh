#!/bin/bash
set -ex

echo "Fetching latest Selene release from ${SELENE_REPO} for pre-installation..."

# SELENE_REPO and SELENE_VARIANT are expected to be passed as environment variables or already set.
# Defaulting them here if not set, though Docker ARG should handle this.
SELENE_REPO=${SELENE_REPO:-Kampfkarren/selene}
SELENE_VARIANT=${SELENE_VARIANT:-selene}

# Target asset format: selene-VERSION-linux.zip or selene-light-VERSION-linux.zip
# The regex needs to handle both selene and selene-light variants correctly.
# The jq filter looks for names like "selene-0.20.0-linux.zip" or "selene-light-0.20.0-linux.zip"
LATEST_RELEASE_ASSET_INFO=$(curl -fsSL "https://api.github.com/repos/${SELENE_REPO}/releases/latest" | \
jq -r ".assets[] | select(.name | test(\"^${SELENE_VARIANT}(-[0-9.]+)?-linux\\\\.zip$\")) | {name, url: .browser_download_url} | @json")

if [ -z "$LATEST_RELEASE_ASSET_INFO" ] || [ "$LATEST_RELEASE_ASSET_INFO" == "null" ]; then
    echo "Error: Could not find the latest Selene asset matching pattern '^${SELENE_VARIANT}(-[0-9.]+)?-linux.zip' in ${SELENE_REPO}." >&2
    # For debugging, show what jq received if possible, or list assets
    echo "Attempting to list assets from ${SELENE_REPO}/releases/latest:"
    curl -fsSL "https://api.github.com/repos/${SELENE_REPO}/releases/latest" | jq -r ".assets[].name" || echo "Failed to list assets."
    exit 1
fi

LATEST_RELEASE_URL=$(echo "$LATEST_RELEASE_ASSET_INFO" | jq -r .url)
LATEST_RELEASE_FILENAME=$(echo "$LATEST_RELEASE_ASSET_INFO" | jq -r .name)

echo "Found asset: $LATEST_RELEASE_FILENAME"
echo "Downloading from: $LATEST_RELEASE_URL"

curl -fsSL -o selene_latest.zip "${LATEST_RELEASE_URL}"
unzip -o selene_latest.zip -d /tmp/selene_extracted

# Find the 'selene' binary, it might be in a subdirectory if the zip structure is like 'selene-0.x.x-linux/selene'
SELENE_BINARY_PATH=$(find /tmp/selene_extracted -name selene -type f | head -n 1)

if [ -z "$SELENE_BINARY_PATH" ]; then
    echo "Error: Could not find 'selene' binary in the extracted zip /tmp/selene_extracted." >&2
    echo "Contents of /tmp/selene_extracted:"
    ls -R /tmp/selene_extracted
    exit 1
fi

echo "Installing pre-loaded Selene from $SELENE_BINARY_PATH to /usr/local/bin/selene"
mv "${SELENE_BINARY_PATH}" /usr/local/bin/selene
chmod +x /usr/local/bin/selene

rm selene_latest.zip
rm -rf /tmp/selene_extracted

echo "Selene pre-installed successfully:"
/usr/local/bin/selene --version