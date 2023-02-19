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
	(cd ./cmd/mc-sshd; go build)
	(cd ./cmd/mqlservd; go build)

deploy: deploy-cli deploy-server

deploy-cli: cli
	sudo cp cmd/mcbridgefs/mcbridgefs /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs
	sudo cp mcbridgefs.sh /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs.sh
	sudo cp cmd/mql/mql /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mql

deploy-server: ssh-hostkey server
	@sudo supervisorctl stop mchydrad:mchydrad_00
	sudo cp cmd/mchydrad/mchydrad /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mchydrad
	sudo cp operations/supervisord.d/mchydrad.ini /etc/supervisord.d
	@sudo supervisorctl start mchydrad:mchydrad_00

mcbridgefsd:
	(cd ./cmd/mcbridgefsd; go build)

deploy-mcbridgefsd: mcbridgefsd
	@sudo supervisorctl stop mcbridgefsd:mcbridgefsd_00
	sudo cp cmd/mcbridgefsd/mcbridgefsd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefsd
	sudo cp operations/supervisord.d/mcbridgefsd.ini /etc/supervisord.d
	@sudo supervisorctl start mcbridgefsd:mcbridgefsd_00

mcftservd:
	(cd ./cmd/mcftservd; go build)

deploy-mcftservd: mcftservd
	@sudo supervisorctl stop mcftservd:mcftservd_00
	sudo cp cmd/mcftservd/mcftservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcftservd
	sudo cp operations/supervisord.d/mcftservd.ini /etc/supervisord.d
	@sudo supervisorctl start mcftservd:mcftservd_00

mc-sshd:
	(cd ./cmd/mc-sshd; go build)

run-mc-sshd: mc-sshd
	./cmd/mc-sshd/mc-sshd

deploy-mc-sshd: mc-sshd ssh-hostkey
	sudo cp cmd/mc-sshd/mc-sshd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mc-sshd
	sudo cp operations/supervisord.d/mc-sshd.ini /etc/supervisord.d
	@sudo supervisorctl update all

mql-cli:
	(cd ./cmd/mql; go build)

mqlservd:
	(cd ./cmd/mqlservd; go build)

mql-deploy: deploy-mql-cli deploy-mql-server

deploy-mql-cli: mql-cli
	sudo cp cmd/mql/mql /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mql

deploy-mql-server: mqlservd
	@sudo supervisorctl stop mqlservd:mqlservd_00
	sudo cp cmd/mqlservd/mqlservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mqlservd
	sudo cp operations/supervisord.d/mqlservd.ini /etc/supervisord.d
	@sudo supervisorctl start mqlservd:mqlservd_00
