#!/bin/bash

#
# Copyright (c) 2015 Dell Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at http://www.apache.org/licenses/LICENSE-2.0
#
# THIS CODE IS PROVIDED ON AN #AS IS* BASIS, WITHOUT WARRANTIES OR
# CONDITIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED, INCLUDING WITHOUT
#  LIMITATION ANY IMPLIED WARRANTIES OR CONDITIONS OF TITLE, FITNESS
# FOR A PARTICULAR PURPOSE, MERCHANTABLITY OR NON-INFRINGEMENT.
#
# See the Apache Version 2.0 License for specific language governing
# permissions and limitations under the License.
#


# platform specific script file which could be
# triggered from systemd service file
export PYTHONPATH=/opt/ngos/lib:/opt/ngos/lib/python

#SMBus Controller 2.0 SPGT register to 0x00000005 to tune 80KHz frequency
/opt/ngos/bin/pcisysfs.py --set --val 0x00000005 --offset 0x300 --res "/sys/devices/pci0000:00/0000:00:13.1/resource0"
#SM Bus HCLK divider register set 0x59 to tune 90khz frequency
/opt/ngos/bin/portiocfg.py --set --offset 0x402 --val 0x59
/opt/ngos/bin/portiocfg.py --set --offset 0x403 --val 0x0

/opt/ngos/bin/bcm_mod_init.sh

# Now create a file to hold the firmware versions.
FIRMWARE_VERSION_FILE=/var/log/firmware_versions
rm -rf ${FIRMWARE_VERSION_FILE}
echo "Bios Version: `dmidecode -s system-version `" > $FIRMWARE_VERSION_FILE

#TODO
# Add CPLD versions too.
