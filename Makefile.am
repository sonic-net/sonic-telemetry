#script files
bin_SCRIPTS = cfg/bin/platform_init.sh cfg/lib/systemd/system-shutdown/platform_shutdown.sh \
              cfg/bin/pas_monitor.sh cfg/bin/pas_monitor_cfg_set.py cfg/bin/portiocfg.py cfg/bin/pcisysfs.py 

data_sdidir= $(sysconfdir)/sonic/sdi
data_sdi_DATA = cfg/sdi/device.xml cfg/sdi/entity.xml 

data_pasdir= $(sysconfdir)/sonic/pas
data_pas_DATA= cfg/pas/config.xml

data_saidir=$(sysconfdir)/sonic
data_sai_DATA=cfg/sai/init.xml

#copy sonic-platform-default systemd service file to target
systemdconfdir=/lib/systemd/system
systemdconf_DATA= cfg/init/sonic-platform-default.service
