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

server:
	(cd ./cmd/mcbridgefsd; go build)

deploy: deploy-cli deploy-server

deploy-cli: cli
	sudo cp cmd/mcbridgefs/mcbridgefs /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs
	sudo cp mcbridgefs.sh /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefs.sh

deploy-server: server
	@sudo supervisorctl stop mcbridgefsd:mcbridgefsd_00
	sudo cp cmd/mcbridgefsd/mcbridgefsd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcbridgefsd
	sudo cp operations/supervisord.d/mcbridgefsd.ini /etc/supervisord.d
	@sudo supervisorctl start mcbridgefsd:mcbridgefsd_00

mcftservd:
	(cd ./cmd/mcftservd; go build)

mcftservd-deploy: mcftservd
	@sudo supervisorctl stop mcftservd:mcftservd_00
	sudo cp cmd/mcftservd/mcftservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mcftservd
	sudo cp operations/supervisord.d/mcftservd.ini /etc/supervisord.d
	@sudo supervisorctl start mcftservd:mcftservd_00

ssh-hostkey:
	@[ -f $(HOME)/.ssh/hostkey ] || ssh-keygen -t ed25519 -N '' -f $(HOME)/.ssh/hostkey

fmt:
	-go fmt ./...

bin: server

server:
	(cd ./cmd/mc-sshd; go build)

run: server
	./cmd/mc-sshd/mc-sshd

deploy: ssh-hostkey deploy-server

deploy-server: server
	sudo cp cmd/mc-sshd/mc-sshd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mc-sshd
	sudo cp operations/supervisord.d/mc-sshd.ini /etc/supervisord.d
	@sudo supervisorctl update all
.PHONY: bin test all fmt deploy docs server cli setup

all: fmt bin

fmt:
	-go fmt ./...

bin: cli server

cli:
	(cd ./cmd/mql; go build)

server:
	(cd ./cmd/mqlservd; go build)

deploy: deploy-cli deploy-server

deploy-cli: cli
	sudo cp cmd/mql/mql /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mql

deploy-server: server
	@sudo supervisorctl stop mqlservd:mqlservd_00
	sudo cp cmd/mqlservd/mqlservd /usr/local/bin
	sudo chmod a+rx /usr/local/bin/mqlservd
	sudo cp operations/supervisord.d/mqlservd.ini /etc/supervisord.d
	@sudo supervisorctl start mqlservd:mqlservd_00
