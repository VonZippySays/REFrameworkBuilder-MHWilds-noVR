#!/bin/bash

# Function to display a progress-like header
status() {
    echo -e "\033[1;34m==>\033[0m $1"
}

# 1. Fetching latest tag
status "Fetching latest nightly tag..."
TAG=$(curl -s https://api.github.com/repos/praydog/REFramework-nightly/releases | grep -m 1 '"tag_name": "nightly-' | cut -d '"' -f 4)

if [ -z "$TAG" ]; then
    echo "Error: Could not find the latest nightly tag."
    exit 1
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
# Try to get the hash from the tag (6e28fb)
HASH=$(echo $TAG | cut -d'-' -f3 | cut -c1-6)
if [ -z "$HASH" ]; then
    # Fallback to the awk method if tag format changed
    HASH=$(awk '{print substr($0,1,6)}' ./MHWILDS/*.txt | head -n 1)
fi

TIMESTAMP="${HASH}_$(date +"%d%b%y")"
zip -rq "REFramework_$TIMESTAMP.zip" MHWILDS

# 6. Final Cleanup
status "Cleaning up temporary files..."
rm -f MHWILDS.zip
rm -rf MHWILDS

status "Finished! Created: REFramework_$TIMESTAMP.zip"
