.PHONY: bin test all fmt deploy docs server cli setup

all: fmt bin

fmt:
	-go fmt ./...

bin: cli server

ssh-hostkey:
	@[ -f $(HOME)/.ssh/hostkey ] || ssh-keygen -t ed25519 -N '' -f $(HOME)/.ssh/hostkey

cli:
	(cd ./cmd/mcbridgefs; go build)
	(cd ./cmd/mcft; go build)
	(cd ./cmd/mql; go build)

server:
	(cd ./cmd/mchydrad; go build)

servers:
	(cd ./cmd/mcbridgefsd; go build)
	(cd ./cmd/mcftservd; go build)
	(cd ./cmd/mcsshd; go build)
	(cd ./cmd/mqlservd; go build)

deploy: deploy-cli deploy-server

deploy-clis: cli
	sudo cp cmd/mcbridgefs/mcbridgefs /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs
	sudo cp scripts/mcbridgefs.sh /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs.sh
	sudo cp cmd/mql/mql /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mql

deploy-server: ssh-hostkey server
	- sudo supervisorctl stop mchydrad:mchydrad_00
	sudo cp cmd/mchydrad/mchydrad /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mchydrad
	sudo cp operations/supervisord.d/mchydrad.conf /etc/supervisor/conf.d
	- sudo supervisorctl start mchydrad:mchydrad_00

mcbridgefsd:
	(cd ./cmd/mcbridgefsd; go build)

deploy-mcbridgefsd: mcbridgefsd
	- sudo supervisorctl stop mcbridgefsd:mcbridgefsd_00
	sudo cp cmd/mcbridgefsd/mcbridgefsd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefsd
	sudo cp operations/supervisord.d/mcbridgefsd.conf /etc/supervisor/conf.d
	- sudo supervisorctl start mcbridgefsd:mcbridgefsd_00

mcftservd:
	(cd ./cmd/mcftservd; go build)

mcfsd:
	(cd ./cmd/mcfsd; go build)

mcdavd:
	(cd ./cmd/mcdavd; go build)

deploy-mcftservd: mcftservd
	- sudo supervisorctl stop mcftservd:mcftservd_00
	sudo cp cmd/mcftservd/mcftservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcftservd
	sudo cp operations/supervisord.d/mcftservd.conf /etc/supervisor/conf.d/
	- sudo supervisorctl start mcftservd:mcftservd_00

mcsshd:
	(cd ./cmd/mcsshd; go build)

run-mcsshd: mcsshd
	./cmd/mcsshd/mcsshd

deploy-mcsshd: mcsshd ssh-hostkey
	sudo cp cmd/mcsshd/mcsshd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcsshd
	sudo cp operations/supervisord.d/mcsshd.conf /etc/supervisor/conf.d
	- sudo supervisorctl update all

mql-cli:
	(cd ./cmd/mql; go build)

mqlservd:
	(cd ./cmd/mqlservd; go build)

mql-deploy: deploy-mql-cli deploy-mql-server

deploy-mql-cli: mql-cli
	sudo cp cmd/mql/mql /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mql

deploy-mqlservd: mqlservd
	- sudo supervisorctl stop mqlservd:mqlservd_00
	sudo cp cmd/mqlservd/mqlservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mqlservd
	sudo cp operations/supervisord.d/mqlservd.conf /etc/supervisor/conf.d
	- sudo supervisorctl start mqlservd:mqlservd_00
