#!/bin/bash
for i in {0..2}
do
echo "- - -" > /sys/class/scsi_host/host$i/scan
done
exit 0