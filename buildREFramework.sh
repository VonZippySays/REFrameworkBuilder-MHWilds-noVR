#!/bin/bash
set -euo pipefail

# Function to display a progress-like header
status() {
    printf "\033[1;34m==>\033[0m %s\n" "$1"
}

# Check dependencies
for cmd in curl zip unzip; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        echo "Error: Required command '$cmd' not found." >&2
        exit 1
    fi
done

# Setup cleanup trap
# Use RAM disk if available for better performance
TMP_ROOT=$(mktemp -d -p /dev/shm 2>/dev/null || mktemp -d)
cleanup() {
    rm -rf "$TMP_ROOT" MHWILDS.zip
}
trap cleanup EXIT
TMP_EXTRACT_DIR="$TMP_ROOT/extract"

# 1. Fetching recent dev releases and let user choose
status "Fetching recent dev releases..."
DEV_PREFIX="${DEV_PREFIX:-}"
MAX_LIST="${MAX_LIST:-20}"

# If running interactively, allow the user to change how many releases to display
if [ -t 0 ]; then
    read -p "How many releases to display? [${MAX_LIST}]: " USER_MAX
    if [[ -n "$USER_MAX" && "$USER_MAX" =~ ^[0-9]+$ ]] && [ "$USER_MAX" -gt 0 ]; then
        MAX_LIST="$USER_MAX"
    fi
fi

CACHE_DIR=".cache_github"
mkdir -p "$CACHE_DIR"
CACHE_BODY="$CACHE_DIR/releases.json"
CACHE_HEADERS="$CACHE_DIR/headers.tmp"
CACHE_ETAG="$CACHE_DIR/etag"

ETAG_VAL=""
if [ -f "$CACHE_ETAG" ]; then
    ETAG_VAL=$(cat "$CACHE_ETAG")
fi

API_URL="https://api.github.com/repos/praydog/REFramework-nightly/releases?per_page=100"
PARSED_CACHE="$CACHE_DIR/parsed_${MAX_LIST}.list"

CURL_FLAGS=("-L" "--compressed" "-sS" "-D" "$CACHE_HEADERS")
if [ -n "$ETAG_VAL" ]; then
    CURL_FLAGS+=("-H" "If-None-Match: $ETAG_VAL")
fi

curl "${CURL_FLAGS[@]}" -o "$CACHE_BODY.tmp" "$API_URL"

HTTP_STATUS=$(awk '/^HTTP/{status=$2} END{print status}' "$CACHE_HEADERS" 2>/dev/null || echo "")
if [ "$HTTP_STATUS" = "200" ]; then
    mv "$CACHE_BODY.tmp" "$CACHE_BODY"
    NEW_ETAG=$(awk '/^[Ee][Tt][Aa][Gg]:/ { sub(/^[Ee][Tt][Aa][Gg]:[[:space:]]*/,"",$0); print $0; exit }' "$CACHE_HEADERS" | tr -d '\r')
    if [ -n "$NEW_ETAG" ]; then
        echo "$NEW_ETAG" > "$CACHE_ETAG"
    fi
elif [ "$HTTP_STATUS" = "304" ] && [ -f "$CACHE_BODY" ]; then
    rm -f "$CACHE_BODY.tmp"
else
    if [ ! -f "$CACHE_BODY" ]; then
        echo "Error: failed to fetch releases and no cache available." >&2
        exit 1
    fi
    rm -f "$CACHE_BODY.tmp"
fi

# If we already have a parsed list cached and it's newer than the JSON body, reuse it
if [ -f "$PARSED_CACHE" ] && [ "$PARSED_CACHE" -nt "$CACHE_BODY" ]; then
    RELEASE_LIST=$(cat "$PARSED_CACHE")
    COUNT=$(wc -l < "$PARSED_CACHE" | tr -d ' ')
else
    # Parse using jq when available (fast and correct JSON handling)
    if command -v jq >/dev/null 2>&1; then
        RELEASE_LIST=$(jq -r --argjson MAX "$MAX_LIST" '
            map(select(.tag_name|test("^nightly-[0-9]{4,}-")))
            | map({
                num:(.tag_name|capture("nightly-(?<n>[0-9]{4,})-[A-Za-z0-9]+")|.n),
                tag:.tag_name,
                pub:.published_at,
                disp:(.published_at | sub("\\.[0-9]+Z$"; "Z") | fromdateiso8601 | strftime("%Y-%m-%d %H:%M:%S"))
              })
            | sort_by(.pub) | reverse
            | unique_by(.num)
            | (if ($MAX>0) then .[:$MAX] else . end)
            | .[] | "\(.num)||\(.tag)||\(.pub)||\(.disp)"' "$CACHE_BODY")
    elif command -v python3 >/dev/null 2>&1; then
        RELEASE_LIST=$(python3 - "$MAX_LIST" "$CACHE_BODY" <<'PY'
import sys, json, re, datetime
maxn=int(sys.argv[1])
body=sys.argv[2]
with open(body,'r') as f:
    data=json.load(f)
items=[]
pat=re.compile(r"^nightly-(?P<n>[0-9]{4,})-(?P<h>[A-Za-z0-9]+)")
for r in data:
    tag = r.get('tag_name','')
    m = pat.match(tag)
    if m:
        pub = r.get('published_at','')
        try:
            dt = datetime.datetime.fromisoformat(pub.replace('Z', '+00:00'))
            disp = dt.strftime('%Y-%m-%d %H:%M:%S')
        except:
            disp = pub
        items.append({'num':m.group('n'),'tag':tag,'pub':pub,'disp':disp})
items.sort(key=lambda x:x.get('pub',''), reverse=True)
seen=set(); out=[]
for it in items:
    if it['num'] in seen: continue
    seen.add(it['num'])
    out.append(it)
    if maxn>0 and len(out)>=maxn: break
for it in out:
    print(f"{it['num']}||{it['tag']}||{it['pub']}||{it['disp']}")
PY
)
    else
        echo "Error: neither 'jq' nor 'python3' are available for JSON parsing." >&2
        exit 1
    fi

    COUNT=$(echo "$RELEASE_LIST" | grep -c "||" || echo "0")
    printf "%s\n" "$RELEASE_LIST" > "$PARSED_CACHE"
fi

if [ "$COUNT" -eq 0 ]; then
    echo "Error: Could not find any releases matching nightly-<digits>."
    exit 1
fi

# Build arrays and present a menu
declare -a NUMS TAGS DATES
i=0
if [ -n "$DEV_PREFIX" ]; then
    echo "Filtering nightly versions by prefix: $DEV_PREFIX"
fi
echo "Found $COUNT nightly version(s)."
echo "Available versions (showing up to $MAX_LIST newest -> oldest):"

# Use while read to populate arrays safely
# Display date is now pre-formatted in the parser for speed
idx=0
while IFS='|' read -r num tag date disp; do
    [ -z "$num" ] && continue
    NUMS[$idx]="$num"
    TAGS[$idx]="$tag"
    DATES[$idx]="$date"
    printf " %2d. %s  (%s)  %s\n" "$((idx+1))" "$num" "$tag" "$disp"
    idx=$((idx+1))
done <<< "${RELEASE_LIST//||/|}"

read -p "Choose numeric version (1-$idx) [1]: " choice
choice="${choice:-1}"

if ! [[ "$choice" =~ ^[0-9]+$ ]] || [ "$choice" -lt 1 ] || [ "$choice" -gt "$idx" ]; then
    echo "Invalid choice. Exiting."
    exit 1
fi

idx=$((choice-1))
NUM="${NUMS[$idx]}"
TAG="${TAGS[$idx]}"
PUB_DATE_RAW="${DATES[$idx]}"

TIMESTAMP_DATE=$(date -d "$PUB_DATE_RAW" +"%d%b%y")

if [[ "$TAG" =~ ^nightly-([0-9]+)-([A-Za-z0-9]+)$ ]]; then
    HASH="nightly-${BASH_REMATCH[1]}-${BASH_REMATCH[2]:0:6}"
else
    # Fallback using bash parameter expansion instead of sed/cut
    clean_tag="${TAG//[^a-zA-Z0-9._-]/_}"
    HASH="${clean_tag:0:30}"
fi

TIMESTAMP="${HASH}_$TIMESTAMP_DATE"
EXPECTED_ZIP="REFramework_$TIMESTAMP.zip"

if [ "${SKIP_DOWNLOAD:-0}" = "1" ]; then
    echo "SKIP_DOWNLOAD=1 - test mode"
    echo "Selected TAG: $TAG"
    echo "Publish date: $PUB_DATE_RAW"
    echo "Would create: $EXPECTED_ZIP"
    exit 0
fi

if [ -f "$EXPECTED_ZIP" ]; then
    printf "\033[1;33m(!)\033[0m Archive %s already exists.\n" "$EXPECTED_ZIP"
    read -p "Do you want to rebuild it anyway? (y/N): " confirm
    if [[ ! $confirm =~ ^[Yy]$ ]]; then
        status "Skipping rebuild. Exiting."
        exit 0
    fi
fi

# 2. Downloading
URL="https://github.com/praydog/REFramework-nightly/releases/download/$TAG/MHWILDS.zip"
status "Downloading MHWILDS.zip ($TAG)..."
curl -L --progress-bar -o MHWILDS.zip "$URL"

# 3. Unzipping with integrated filtering
status "Extracting and filtering..."
mkdir -p "$TMP_EXTRACT_DIR/MHWILDS"
# Exclude files matching patterns during extraction for efficiency
# Patterns cover README, VR/XR runtimes, and other unwanted files
# We suppress "caution: excluded filename not matched" warnings to keep output clean
{ unzip -qo MHWILDS.zip -d "$TMP_EXTRACT_DIR/MHWILDS" \
    -x "*RE*" "*vr*" "*xr*" "*VR*" "*XR*" "*DELETE*" "*OpenVR*" "*OpenXR*" 2>"$TMP_ROOT/unzip_error" || [ $? -eq 1 ]; }
if [ -s "$TMP_ROOT/unzip_error" ]; then
    grep -v "caution: excluded filename not matched" "$TMP_ROOT/unzip_error" >&2 || true
fi

# 4. Creating optimized archive
status "Creating optimized archive: $EXPECTED_ZIP"
# Use absolute path for output zip to avoid issues when zipping from subshell
OUTPUT_ZIP="$(pwd)/$EXPECTED_ZIP"
(cd "$TMP_EXTRACT_DIR" && zip -rq "$OUTPUT_ZIP" MHWILDS)

status "Finished! Created: $EXPECTED_ZIP"

# 5. Show summary of archive contents
echo "Archive Summary ($EXPECTED_ZIP):"
unzip -l "$EXPECTED_ZIP" | head -n -2 | tail -n +4 | awk '{print "  " $4}'
echo "Total files: $(unzip -l "$EXPECTED_ZIP" | grep -c "MHWILDS/")"
