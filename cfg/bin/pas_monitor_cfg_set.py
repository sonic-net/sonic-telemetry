#!/usr/bin/python


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



import cps
import subprocess

if __name__=='__main__':

    handle = cps.event_connect()
    key = cps.key_from_name('observed','base-pas/ready')
    print "Key:"+key

    print "Registered for PAS event object; Entering while loop"
    cps.event_register(handle, cps.key_from_name('observed', 'base-pas/ready'))

    while(1):
        cps.event_wait(handle)
        print "PAS READY EVENT RECEIVED" 
        print "set fan speed set to 75%"
        subprocess.call("python /usr/bin/cps_set_oid.py set base-pas/fan speed_pct=75", shell=True)
        print "set tx enable for all media connected"
        subprocess.call("python /usr/bin/cps_set_oid.py set base-pas/media-channel slot=1 state=1", shell=True)

