#!/bin/bash
cd /home/anton/Koding/ngawur/pov-ai-indonesia
# Preserve the SQLite database + generated insights across restarts.
# Only wipe data when POV_RESET=1 is explicitly set (for a clean re-seed).
if [ "${POV_RESET}" = "1" ]; then
    echo "[restart] POV_RESET=1 set — wiping data directory for clean re-seed"
    rm -rf data
fi
./pov-ai
