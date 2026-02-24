#!/usr/bin/env bash
#
# Seed the Jiraiya app with sample release data from scripts/data/releases.json.
#
# Usage:
#   ./scripts/seed.sh              # default: http://localhost:8080
#   ./scripts/seed.sh http://host:port
#
# Tree structure (platform: android):
#
#              1.0.0 (Root)
#                |
#              2.0.0            {PROJ-1}
#            /   |   \
#        3.0.0  2.1.0  2.2.0   3.0.0:{PROJ-2,3,4}  2.1.0:{PROJ-5}  2.2.0:{PROJ-10}
#       /   \     |
#      /     \  2.1.1           2.1.1:{PROJ-6,7}
#     /       \
#  3.1.0     3.2.0             3.1.0:{PROJ-5,6,7,8}  3.2.0:{PROJ-5,6,7,10}
#
# CalcChgs test cases (GET /api/jiras?from=X&to=Y):
#   TC1: to=3.1.0  from=2.1.1  → PROJ-2,3,4,8
#   TC2: to=3.0.0  from=2.1.0  → PROJ-2,3,4         (cross-branch)
#   TC3: to=3.2.0  from=2.2.0  → PROJ-2,3,4,5,6,7
#   TC4: to=3.1.0  from=9.9.9  → error               (version not found)
#   TC5: to=3.2.0  from=2.0.0  → PROJ-2,3,4,5,6,7,10
#
# Tree structure (platform: ios):
#
#   ios-1.0.0 (Root)
#       |
#   ios-1.1.0          {IOS-1, IOS-2}
#      / \
#     /   \
#  ios-1.2.0  ios-1.1.1   ios-1.2.0:{IOS-3,4}  ios-1.1.1:{IOS-5}
#     |
#  ios-2.0.0               {IOS-6, IOS-7}

set -euo pipefail

BASE_URL="${1:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATA_FILE="${SCRIPT_DIR}/data/releases.json"

if [ ! -f "$DATA_FILE" ]; then
  echo "Error: $DATA_FILE not found"
  exit 1
fi

count=$(jq length "$DATA_FILE")

echo "Seeding ${BASE_URL} with ${count} releases from ${DATA_FILE} ..."
echo ""

for i in $(seq 0 $((count - 1))); do
  entry=$(jq ".[$i]" "$DATA_FILE")
  version=$(echo "$entry" | jq -r '.release.version')
  platform=$(echo "$entry" | jq -r '.release.platform')

  status=$(curl -s -o /dev/null -w "%{http_code}" -X PUT \
    -H "Content-Type: application/json" \
    -d "$entry" \
    "${BASE_URL}/api/releases")

  if [ "$status" = "200" ]; then
    printf "  ✓ %-16s %s\n" "$version" "$platform"
  else
    printf "  ✗ %-16s %s — HTTP %s\n" "$version" "$platform" "$status"
    exit 1
  fi
done

echo ""
echo "Done! Test queries:"
echo ""
echo "  # TC1: 3.1.0 vs 2.1.1 → PROJ-2,PROJ-3,PROJ-4,PROJ-8"
echo "  curl '${BASE_URL}/api/jiras?from=2.1.1&to=3.1.0'"
echo ""
echo "  # TC2: 3.0.0 vs 2.1.0 → PROJ-2,PROJ-3,PROJ-4 (cross-branch)"
echo "  curl '${BASE_URL}/api/jiras?from=2.1.0&to=3.0.0'"
echo ""
echo "  # TC3: 3.2.0 vs 2.2.0 → PROJ-2,PROJ-3,PROJ-4,PROJ-5,PROJ-6,PROJ-7"
echo "  curl '${BASE_URL}/api/jiras?from=2.2.0&to=3.2.0'"
echo ""
echo "  # TC4: 3.1.0 vs 9.9.9 → error (version not found)"
echo "  curl '${BASE_URL}/api/jiras?from=9.9.9&to=3.1.0'"
echo ""
echo "  # TC5: 3.2.0 vs 2.0.0 → PROJ-2,PROJ-3,PROJ-4,PROJ-5,PROJ-6,PROJ-7,PROJ-10"
echo "  curl '${BASE_URL}/api/jiras?from=2.0.0&to=3.2.0'"
echo ""
echo "  # iOS: ios-2.0.0 vs ios-1.1.1 → IOS-1,IOS-2,IOS-3,IOS-4,IOS-6,IOS-7"
echo "  curl '${BASE_URL}/api/jiras?from=ios-1.1.1&to=ios-2.0.0'"
echo ""
echo "  # Tree views"
echo "  curl '${BASE_URL}/api/admin/tree?platform=android'"
echo "  curl '${BASE_URL}/api/admin/tree?platform=ios'"
