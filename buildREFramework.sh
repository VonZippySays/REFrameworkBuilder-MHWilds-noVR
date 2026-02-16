#!/bin/bash

# Function to display a progress-like header
status() {
    echo -e "\033[1;34m==>\033[0m $1"
}

# 1. Fetching latest tag
status "Fetching latest nightly tag..."
RELEASE_JSON=$(curl -s https://api.github.com/repos/praydog/REFramework-nightly/releases | head -n 100)
TAG=$(echo "$RELEASE_JSON" | grep -m 1 '"tag_name":' | cut -d '"' -f 4)
PUB_DATE_RAW=$(echo "$RELEASE_JSON" | grep -m 1 '"published_at":' | cut -d '"' -f 4)

if [ -z "$TAG" ]; then
    echo "Error: Could not find the latest nightly tag."
    exit 1
fi

TIMESTAMP_DATE=$(date -d "$PUB_DATE_RAW" +"%d%b%y")

# Extract version: nightly-[numbers]-[first 6 of hash]
HASH=$(echo $TAG | grep -oE "^nightly-[0-9]+-[a-zA-Z0-9]{6}")

# Fallback if the specific tag format isn't found
if [ -z "$HASH" ]; then
    HASH=$(echo $TAG | cut -d'-' -f1,2,3 | cut -c1-20)
fi

TIMESTAMP="${HASH}_$TIMESTAMP_DATE"
EXPECTED_ZIP="REFramework_$TIMESTAMP.zip"

if [ -f "$EXPECTED_ZIP" ]; then
    echo -e "\033[1;33m(!)\033[0m Archive $EXPECTED_ZIP already exists."
    read -p "Do you want to rebuild it anyway? (y/N): " confirm
    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        status "Skipping rebuild. Exiting."
        exit 0
    fi
fi

# 2. Downloading with progress bar
URL="https://github.com/praydog/REFramework-nightly/releases/download/$TAG/MHWILDS.zip"
status "Downloading MHWILDS.zip ($TAG)..."
curl -L --progress-bar -o MHWILDS.zip "$URL"

# 3. Unzipping
status "Extracting MHWILDS.zip..."
mkdir -p MHWILDS
unzip -qo MHWILDS.zip -d MHWILDS

# 4. Cleaning
status "Filtering files..."
find MHWILDS \( -name "*RE*" -o -name "*vr*" -o -name "*DELETE*" -o -name "*xr*" \) -exec rm -rf {} +

# 5. Zipping
status "Creating optimized archive..."
zip -rq "$EXPECTED_ZIP" MHWILDS

# 6. Final Cleanup
status "Cleaning up temporary files..."
rm -f MHWILDS.zip
rm -rf MHWILDS

status "Finished! Created: $EXPECTED_ZIP"
