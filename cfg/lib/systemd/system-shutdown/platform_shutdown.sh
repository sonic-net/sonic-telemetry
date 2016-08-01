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

# Platform specific script to trigger reset/shdutdown of all hw components
# in addition to the CPU.


function platform_reset {
    count=0;

    while [ $count -lt 3 ]
    do
        echo 0 > /sys/class/gpio/gpio1/value
        echo 0 > /sys/class/gpio/gpio2/value

        # Now reset via SYSTEM_CPLD FORCE_RST
        i2cset  -y 0 0x31 1 0xfd

        let "count = count + 1"
    done

    # If reset was successful, this line will not be reached.
    # Hence the fact we are here implies reset failed.
    echo "Platform_reset failed.  Some components might not have been reset" > /dev/console

}

function platform_poweroff {
    echo 0 > /sys/class/gpio/gpio1/value
    echo 0 > /sys/class/gpio/gpio2/value
    
    # Now reset via PS0_ON and PS1_ON registers of MASTER_CPLD
    # But then right now the CPLD seems to reset instead of power-down
    # Hence keeping it commented for now.
    # i2cset  -y 0 0x32 3 0x11
    
}

case "$1" in
    "poweroff")
        platform_poweroff
        ;;
    "reboot")
        platform_reset
        ;;
esac

