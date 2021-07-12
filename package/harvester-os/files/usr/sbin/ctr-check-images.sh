#!/bin/bash -eu
# The script reads a image list and waits until images in the list are presented.

sorted_list_file="/tmp.$(basename $1).sorted"
sort $1 > $sorted_list_file

trap "rm -f $sorted_list_file" EXIT

while true; do
    missing=$(ctr -n k8s.io images ls -q | grep -v ^sha256 | sort | comm -23 $sorted_list_file -)
    if [ -z "$missing" ]; then
        break
    fi
    sleep 2
done
