#!/bin/bash
# Cleanup E2E test organizations from data/orgs/
# Run after E2E tests to remove temporary org databases
set -e
DATA_DIR="${1:-data}"
ORGS_DIR="$DATA_DIR/orgs"

if [ ! -d "$ORGS_DIR" ]; then
  echo "No orgs directory: $ORGS_DIR"
  exit 0
fi

COUNT=0
for dir in "$ORGS_DIR"/e2e-* "$ORGS_DIR"/e2e2-* "$ORGS_DIR"/e2ebot-* "$ORGS_DIR"/e2ewelc-* \
           "$ORGS_DIR"/idor-* "$ORGS_DIR"/iso1-* "$ORGS_DIR"/iso2-* "$ORGS_DIR"/clnorg-* \
           "$ORGS_DIR"/ssrf-* "$ORGS_DIR"/smoketest* "$ORGS_DIR"/testco* "$ORGS_DIR"/testorg* \
           "$ORGS_DIR"/qa-test* "$ORGS_DIR"/acme*; do
  if [ -d "$dir" ]; then
    rm -rf "$dir"
    COUNT=$((COUNT + 1))
  fi
done

# Also clean master.json entries
if [ -f "$DATA_DIR/master.json" ] && command -v python3 &>/dev/null; then
  python3 -c "
import json
with open('$DATA_DIR/master.json') as f:
    orgs = json.load(f)
kept = [o for o in orgs if not any(o['slug'].startswith(p) for p in ['e2e','idor','iso','clnorg','ssrf','smoke','testco','testorg','qa-test','acme'])]
with open('$DATA_DIR/master.json','w') as f:
    json.dump(kept, f, indent=2)
print(f'Kept {len(kept)} orgs, removed {len(orgs)-len(kept)}')
"
fi

echo "Cleaned $COUNT test org directories"
