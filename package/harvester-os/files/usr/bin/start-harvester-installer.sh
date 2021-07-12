#!/bin/bash

export HARVESTER_DASHBOARD=true
if grep -q root=live:CDLABEL=COS_LIVE /proc/cmdline || [ -n "$(blkid -L COS_SYSTEM)" ]; then
    # Live environment: installation mode.
    export HARVESTER_DASHBOARD=false
fi
export DEBUG=true

export TTY=$(tty)
export TERM="linux"

# Set a default window size for un-attached terminals like serial consoles
MIN_ROWS=24
MIN_COLS=80
read ROWS COLS <<< $(/usr/bin/stty -F ${TTY} size)
if [ $ROWS -lt $MIN_ROWS ] || [ $COLS -lt $MIN_COLS ]; then
    /usr/bin/stty -F "${TTY}" rows "${MIN_ROWS}" cols "${MIN_COLS}"
fi

harvester-installer
