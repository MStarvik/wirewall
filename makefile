INSTALL_PREFIX = /usr/local

$(INSTALL_PREFIX)/bin:
	mkdir -p $@

$(INSTALL_PREFIX)/bin/wirewalld: bin/wirewalld | $(INSTALL_PREFIX)/bin
	install -Dsm755 $< $@

$(INSTALL_PREFIX)/bin/wirewallctl: bin/wirewallctl | $(INSTALL_PREFIX)/bin
	install -Dsm755 $< $@

/etc/wirewall:
	mkdir -p $@

/etc/wirewall/clients: /etc/wirewall
	mkdir -m 700 $@

$(INSTALL_PREFIX)/lib/systemd/system:
	mkdir -p $@

$(INSTALL_PREFIX)/lib/systemd/system/wirewalld.service: lib/systemd/system/wirewalld.service | $(INSTALL_PREFIX)/lib/systemd/system
	cp $< $@

$(INSTALL_PREFIX)/share/dbus-1/system-services:
	mkdir -p $@

$(INSTALL_PREFIX)/share/dbus-1/system-services/no.mstarvik.wirewall.service: share/dbus-1/system-services/no.mstarvik.wirewall.service | $(INSTALL_PREFIX)/share/dbus-1/system-services
	cp $< $@

/etc/dbus-1/system.d:
	mkdir -p $@

/etc/dbus-1/system.d/no.mstarvik.wirewall.conf: etc/dbus-1/system.d/no.mstarvik.wirewall.conf | /etc/dbus-1/system.d
	cp $< $@

.PHONY: install-bin
install-bin: $(INSTALL_PREFIX)/bin/wirewalld $(INSTALL_PREFIX)/bin/wirewallctl

.PHONY: install-config
install-config: /etc/wirewall/clients

.PHONY: install-systemd
install-systemd: $(INSTALL_PREFIX)/lib/systemd/system/wirewalld.service

.PHONY: install-dbus
install-dbus: $(INSTALL_PREFIX)/share/dbus-1/system-services/no.mstarvik.wirewall.service /etc/dbus-1/system.d/no.mstarvik.wirewall.conf

.PHONY: install
install: install-bin install-config install-systemd install-dbus

.PHONY: uninstall
uninstall:
	rm -f $(INSTALL_PREFIX)/bin/wirewalld
	rm -f $(INSTALL_PREFIX)/bin/wirewallctl
	rm -f $(INSTALL_PREFIX)/lib/systemd/system/wirewalld.service
	rm -f $(INSTALL_PREFIX)/share/dbus-1/system-services/no.mstarvik.wirewall.service
	rm -f /etc/dbus-1/system.d/no.mstarvik.wirewall.conf
