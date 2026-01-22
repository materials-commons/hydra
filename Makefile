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
	(cd ./cmd/mcdavd; go build)
	(cd ./cmd/mcfsd; go build)
	(cd ./cmd/mctusd; go build)

deploy: deploy-cli deploy-server

deploy-clis: cli
	sudo cp cmd/mcbridgefs/mcbridgefs /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs
	sudo cp scripts/mcbridgefs.sh /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs.sh
	sudo cp cmd/mql/mql /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mql

deploy-server: ssh-hostkey server
	sudo cp cmd/mchydrad/mchydrad /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mchydrad
	sudo cp operations/supervisord.d/mchydrad.conf /etc/supervisor/conf.d

deploy-servers: deploy-mcbridgefsd deploy-mcftservd deploy-mcsshd deploy-mqlservd deploy-mcdavd deploy-mcfsd deploy-mctusd

stop-servers:
	-sudo supervisorctl stop 'mcbridgefsd:*' 'mcdavd:*' 'mcfsd:*' 'mcftservd:*' 'mcsshd:*' 'mqlservd:*' 'mctusd:*'

start-servers:
	-sudo supervisorctl start 'mcbridgefsd:*' 'mcdavd:*' 'mcfsd:*' 'mcftservd:*' 'mcsshd:*' 'mqlservd:*' 'mctusd:*'

mcbridgefsd:
	(cd ./cmd/mcbridgefsd; go build)

deploy-mcbridgefsd: mcbridgefsd
	sudo cp cmd/mcbridgefsd/mcbridgefsd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefsd
	sudo cp operations/supervisord.d/mcbridgefsd.conf /etc/supervisor/conf.d

mcftservd:
	(cd ./cmd/mcftservd; go build)

mcfsd:
	(cd ./cmd/mcfsd; go build)

mcdavd:
	(cd ./cmd/mcdavd; go build)

mctusd:
	(cd ./cmd/mctusd; go build)

deploy-mcftservd: mcftservd
	sudo cp cmd/mcftservd/mcftservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcftservd
	sudo cp operations/supervisord.d/mcftservd.conf /etc/supervisor/conf.d/

mcsshd:
	(cd ./cmd/mcsshd; go build)

run-mcsshd: mcsshd
	./cmd/mcsshd/mcsshd

deploy-mcsshd: mcsshd ssh-hostkey
	sudo cp cmd/mcsshd/mcsshd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcsshd
	sudo cp operations/supervisord.d/mcsshd.conf /etc/supervisor/conf.d

mql-cli:
	(cd ./cmd/mql; go build)

mqlservd:
	(cd ./cmd/mqlservd; go build)

mchubd:
	(cd ./cmd/mchubd; go build)

mql-deploy: deploy-mql-cli deploy-mql-server

deploy-mql-cli: mql-cli
	sudo cp cmd/mql/mql /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mql

deploy-mqlservd: mqlservd
	sudo cp cmd/mqlservd/mqlservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mqlservd
	sudo cp operations/supervisord.d/mqlservd.conf /etc/supervisor/conf.d

deploy-mcdavd: mcdavd
	- sudo cp operations/supervisord.d/mcdavd.conf /etc/supervisor/conf.d
	- sudo cp cmd/mcdavd/mcdavd /usr/local/bin
	- sudo chmod a+rx /usr/local/bin/mcdavd


deploy-mctusd: mctusd
	- sudo cp operations/supervisord.d/mctusd.conf /etc/supervisor/conf.d
	- sudo cp cmd/mctusd/mctusd /usr/local/bin
	- sudo chmod a+rx /usr/local/bin/mctusd

deploy-mcfsd: mcfsd
	- sudo cp operations/supervisord.d/mcfsd.conf /etc/supervisor/conf.d
	- sudo cp cmd/mcfsd/mcfsd /usr/local/bin
	- sudo chmod a+rx /usr/local/bin/mcfsd
