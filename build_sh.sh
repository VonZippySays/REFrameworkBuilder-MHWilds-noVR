#!/bin/bash

# Parse arguments
SILENT=0
for arg in "$@"; do
    if [ "$arg" == "-silent" ]; then
        SILENT=1
        export SILENT=1
    fi
done

# Run the build process
# The Shell script (buildREFramework.sh)
./buildREFramework.sh

# Identify the latest built archive
LATEST_ZIP=$(ls -t REFramework_nightly-*.zip 2>/dev/null | head -n 1)

if [ -z "$LATEST_ZIP" ]; then
    echo "(!) Error: No REFramework_nightly-*.zip found to copy."
    exit 1
fi

# Discover Windows users
USER_LIST=()
while IFS= read -r user_path; do
    user_name=$(basename "$user_path")
    # Filter out system accounts and metadata
    case "$user_name" in
        "All Users"|"Default User"|"Public"|"Default"|"desktop.ini") continue ;;
    esac
    USER_LIST+=("$user_name")
done < <(find /mnt/c/Users/ -maxdepth 1 -mindepth 1 -type d 2>/dev/null)

# Fallback/Default logic
WSL_USER=$(whoami)
DEFAULT_USER=""

# Try to find a match based on current WSL user
for u in "${USER_LIST[@]}"; do
    if [[ "${u,,}" == "${WSL_USER,,}"* || "${WSL_USER,,}" == "${u,,}"* ]]; then
        DEFAULT_USER="$u"
        break
    fi
done

# If no match found, use the first discovered user as the default
[ -z "$DEFAULT_USER" ] && [ ${#USER_LIST[@]} -gt 0 ] && DEFAULT_USER="${USER_LIST[0]}"

# Selection logic
if [ "$SILENT" == "1" ]; then
    TARGET_USER="$DEFAULT_USER"
    DEST_SUBDIR="Downloads"
    echo "Silent Mode: Using default user '$TARGET_USER' and directory '$DEST_SUBDIR'"
else
    # Always prompt for Windows destination user
    echo "Select Windows destination user:"
    for i in "${!USER_LIST[@]}"; do
        marker=" "
        [ "${USER_LIST[$i]}" == "$DEFAULT_USER" ] && marker="*"
        printf " %d) %s %s\n" "$((i+1))" "${USER_LIST[$i]}" "$marker"
    done
    read -p "Choice (1-${#USER_LIST[@]}) [default: $DEFAULT_USER]: " choice
    if [[ -n "$choice" && "$choice" =~ ^[0-9]+$ ]] && [ "$choice" -ge 1 ] && [ "$choice" -le ${#USER_LIST[@]} ]; then
        TARGET_USER="${USER_LIST[$((choice-1))]}"
    else
        # Fallback to DEFAULT_USER if no choice or invalid choice
        TARGET_USER="${DEFAULT_USER:-Unknown}"
    fi

    # Prompt for destination folder
    read -p "Destination folder relative to C:/Users/$TARGET_USER/ [Downloads]: " DEST_SUBDIR
    DEST_SUBDIR="${DEST_SUBDIR:-Downloads}"
fi

DEST="/mnt/c/Users/$TARGET_USER/$DEST_SUBDIR/"

if [ -d "$DEST" ]; then
    echo "==> Copying $LATEST_ZIP to $DEST..."
    if cp "$LATEST_ZIP" "$DEST"; then
        echo "==> Successfully copied to $TARGET_USER's $DEST_SUBDIR folder."
    else
        echo "(!) Error: Failed to copy to $DEST"
        exit 1
    fi
else
    echo "(!) Error: Destination $DEST not found."
    exit 1
fi
