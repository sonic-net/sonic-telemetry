# sonic-pas-config-s6000
This repo holds the configuration files for the SONiC PAS component.

## Description
The sonic PAS component will read the configuration files to determine how to initialize the system and understand which devices are expected to be present and should be tracked.


Additionally this repo has python services using the PAS to monitor startup and optionally events.


A good example of using the object-library interfaces to communicate with the pas can be found in the cfg/bin directory.


Building
--------
Please see the instructions in the sonic-nas-manifest repo for more details on the common build tools.  [Sonic-nas-manifest](https://stash.force10networks.com/projects/SONIC/repos/sonic-nas-manifest/browse)

BUILD CMD: sonic_build -- clean binary

(c) Dell 2016
