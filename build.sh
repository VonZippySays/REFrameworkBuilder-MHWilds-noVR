#!/bin/bash

# Run the build process
# The Go script (buildREFramework) will prompt for rebuild if the file already exists.
./buildREFramework

# Identify the latest built archive
LATEST_ZIP=$(ls -t REFramework_nightly-*.zip 2>/dev/null | head -n 1)

if [ -n "$LATEST_ZIP" ]; then
    echo "==> Copying $LATEST_ZIP to Downloads..."
    cp "$LATEST_ZIP" /mnt/c/Users/Mike/Downloads/
    echo "==> Done."
else
    echo "(!) Error: No REFramework_nightly-*.zip found to copy."
    exit 1
fi
