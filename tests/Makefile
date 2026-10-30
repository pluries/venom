PKI_DIR := ./grpc/pki
PKI_VAR_FILE = ./pki_variables.yml

include ./pki.mk

define docker_run
	docker run --name venom-$(2) -d $(3) $(1) | tee venom-$2.cid
	echo "docker exit code $?"
endef

docker-network := venom-test-net

# Where the suites reach the stack's published ports: localhost on a workstation; the nightly runs them from a Jenkins
# agent that reaches them by vm1's address instead (venom #2121). Passed to the suites as the variable stackHost.
STACK_HOST ?= localhost

# What the Kafka broker tells its clients to connect to. A client bootstraps on the published port, then reconnects to
# the address in the broker's metadata: kafka/broker.env says localhost, which only works on the machine that runs the
# stack (venom #2121). `-e` on the broker overrides that file.
KAFKA_ADVERTISED_LISTENERS = PLAINTEXT://venom-kafka-broker:29092,PLAINTEXT_HOST://$(STACK_HOST):9092

COVER_FILES = $(shell find . -name "*.coverprofile")
PKGS_COMMA_SEP = go list -f '{{ join .Deps "\n" }}{{"\n"}}{{.ImportPath}}' . | grep github.com/ovh/venom | grep -v vendor | tr '\n' ',' | sed 's/,$$//'

venom-postgres.cid:
	$(call docker_run,postgres,postgres,-p 15432:5432 -e POSTGRES_PASSWORD=venom -e POSTGRES_USER=venom -e POSTGRES_DB=venom)
venom-mysql.cid:
	$(call docker_run,mysql/mysql-server,mysql,-p 13306:3306 -e MYSQL_PASSWORD=venom -e MYSQL_USER=venom -e MYSQL_DATABASE=venom)
# mongo:latest refuses to start on Linux 6.19 and newer (SERVER-121912); the nightly's agent (vm1) runs kernel 7.0.
# 8.2 is the newest tag that starts there (venom #2121)
venom-mongo.cid:
	$(call docker_run,mongo:8.2,mongo,-p 27017:27017 -e MONGO_INITDB_ROOT_PASSWORD=venom -e MONGO_INITDB_ROOT_USERNAME=venom -e MONGO_INITDB_DATABASE=venom)
venom-redis.cid:
	$(call docker_run,redis,redis,-p 16379:6379)
venom-imap.cid:
	$(call docker_run,tomadele/mail-server,imap,-p 1025:25 -p 1143:143 -p 1993:993 -e MAILNAME=example.org -e MAIL_ADDRESS=address@example.org -e MAIL_PASS=pass)
# the schema registry is published on host port 18081, not 8081: on vm1, where the nightly runs the stack, 8081 is
# Dependency-Track's API (venom #2121)
venom-kafka.cid:
	docker network create --driver bridge $(docker-network)
	$(call docker_run,confluentinc/cp-zookeeper:7.4.3,kafka-zookeeper,\
		--network $(docker-network) \
		-p 2181:2181 \
		-e ZOOKEEPER_CLIENT_PORT=2181 \
		-e ZOOKEEPER_TICK_TIME=2000 \
	)
	$(call docker_run,confluentinc/cp-kafka:7.4.3,kafka-broker,\
		--network $(docker-network) \
		-p 9092:9092 -p 9101:9101 -p 29092:29092 \
		--env-file kafka/broker.env \
		-e "KAFKA_ADVERTISED_LISTENERS=$(KAFKA_ADVERTISED_LISTENERS)" \
	)
	$(call docker_run,confluentinc/cp-schema-registry:7.4.3,kafka-schema-registry,\
		--network $(docker-network) \
		-p 18081:8081 \
		-e SCHEMA_REGISTRY_HOST_NAME=venom-kafka-schema-registry \
		-e SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS=venom-kafka-broker:29092 \
	)
venom-httpbin.cid:
	docker pull kennethreitz/httpbin
	docker tag kennethreitz/httpbin httpbin 
	$(call docker_run,httpbin,httpbin,-p 9280:80)
venom-rabbit.cid:
	$(call docker_run,rabbitmq,rabbitmq,-p 5672:5672 -p 15672:15672)
venom-sshd.cid:
	$(call docker_run,ghcr.io/linuxserver/openssh-server,sshd,-p 2222:2222 -e PUID=1000 -e PGID=1000 -e TZ=Europe/London -e PUBLIC_KEY="$(shell cat ~/.ssh/id_rsa.pub)" -e USER_NAME=venom -e PASSWORD_ACCESS=true -e USER_PASSWORD=testvenom -e SUDO_ACCESS=true)
venom-mqtt.cid:
	$(call docker_run,eclipse-mosquitto,mqtt-broker,-p 1883:1883 -p 9001:9001 -v $(shell realpath mqtt/mosquitto.conf):/mosquitto/config/mosquitto.conf:ro)
venom-qpid.cid:
	$(call docker_run,scholzj/qpid-cpp:1.39.0,qpid,-p 5673:5672)
	@printf "waiting for QPID broker to be ready..." ; \
	until docker exec venom-qpid qpid-config -b 127.0.0.1:5672 add queue amqp-test 2>/dev/null; do sleep 1; done ; \
	echo " done"
venom-grpc.cid:
	docker build -t venom-grpc-greeter ./grpc
	$(call docker_run,venom-grpc-greeter,grpc,-p 50051:50051)
venom-couchbase.cid:
	$(call docker_run,couchbase/server-sandbox:7.1.1,couchbase,-p 8091-8096:8091-8096 -p 11210:11210)
	@echo "must wait 30 to 60 seconds to be sure couchbase is ready"

start-test-stack: venom-postgres.cid
start-test-stack: venom-mysql.cid
start-test-stack: venom-mongo.cid
start-test-stack: venom-redis.cid
start-test-stack: venom-imap.cid
start-test-stack: venom-kafka.cid
start-test-stack: venom-rabbit.cid
start-test-stack: venom-sshd.cid
start-test-stack: venom-mqtt.cid
start-test-stack: venom-qpid.cid
start-test-stack: $(PKI_DIR)
start-test-stack: venom-httpbin.cid
start-test-stack: venom-grpc.cid
start-test-stack: venom-couchbase.cid
start-test-stack:
	@$(MAKE) --no-print-directory check-test-stack

# docker_run pipes `docker run` into tee, so a container that fails to start or exits at once is invisible to make: fail
# here, with its name and its last log lines, when a container of the stack is not running (venom #2121)
check-test-stack:
	@sleep 5; bad=0; for f in venom-*.cid; do \
	  n=$${f%.cid}; s=$$(docker inspect -f '{{.State.Status}}' $$n 2>/dev/null) || s=missing; \
	  if [ "$$s" != running ]; then echo "$$n is $$s"; docker logs --tail 20 $$n 2>&1 | sed "s|^|  $$n: |"; bad=1; fi; \
	done; exit $$bad

stop-test-stack:
	@for f in `ls -1 *.cid`; do docker stop `cat $${f}`; docker rm `cat $${f}`; done; rm -f *.cid; docker network rm $(docker-network)

build-test-binary:
	cd ../cmd/venom; \
	TEMP=`$(PKGS_COMMA_SEP)`; \
	CGO_ENABLED=1 go test -coverpkg $$TEMP -c . -o ../../tests/venom.test -ldflags "-X github.com/ovh/venom.IsTest=true" -tags testbincover;

# the binary that suites call as ./venom (verbose_output, retry_if, user_executor_*): the README builds it with
# `make build` and `cp`, the nightly builds it here (venom #2121)
build-venom:
	cd ../cmd/venom && go build -o ../../tests/venom .

build-test-binary-docker:
	docker run -v `pwd`/..:/workspace golang:1.27 sh -c "cd /workspace/tests && make build-test-binary"

# The hello plugin is a compiled Go plugin: venom only loads it when the host and the plugin were built with the same Go
# toolchain, the same module versions and the same instrumentation. venom.test (built with -coverpkg, run by
# venom_wrapper.sh) is instrumented, so the plugin suite runs with the plain ./venom of build-venom instead (#2847).
# Proof it is the plugin that runs: ./plugin/lib has no hello.yml, and the same suite fails with a plugin built on
# another Go patch.
build-test-plugin:
	$(MAKE) -C ../executors/plugins/hello build
	mkdir -p ./plugin/lib
	cp ../executors/plugins/hello/dist/lib/hello.so ./plugin/lib/hello.so

run-test-plugin: build-venom build-test-plugin
	cd ./plugin && ../venom run -vv --format=xml --output-dir=. hello.yml

run-test: generate-venom-pki
	VENOM_VAR_MY_ENVAR=foo  ./venom_wrapper.sh run \
	-vv --format=xml --output-dir=. --html-report --lib-dir=./lib_custom --var=stackHost=$(STACK_HOST) --var='array_from_var=["biz","buz"]' --var-from-file ./kafka/testVariables.yml --var-from-file=$(PKI_VAR_FILE) --var-from-file ./vars/vars.yml ./*.yml ./assertions/*.yml && \
	(cd ./vars_override && VENOM_VAR_foo=from-env VENOM_VAR_FOO2=from-env2 VENOM_VAR_FOO3=from-env3 ../venom_wrapper.sh run -vv --format=xml --output-dir=. --var foo='from-cmd-arg' --var='array_from_var=["biz","buz"]' ./mytc.yml);
	$(MAKE) --no-print-directory run-test-plugin

.PHONY: wait-for-kafka
wait-for-kafka:
	@printf "waiting for Kafka infrastructure startup ... (it may take a few minutes)" ; \
	until curl --retry 30 --retry-connrefused --retry-max-time 100 --connect-timeout 10 -sf http://$(STACK_HOST):18081/config/ -o /dev/null; do sleep 2; done ; \
	echo "\n\033[0;32mdone\033[0m"

clean:
	@rm -f *.prof *.html *.xml *.log *.dump.json *.args.file *.error.out *.out *.test.out *.coverprofile

merge-coverage:
	@docker run -v `pwd`:/workspace golang:1.27 sh -c "\
			go install github.com/wadey/gocovmerge@latest && \
			cd /workspace && \
    gocovmerge $(COVER_FILES) > /workspace/venom.cover.out \
		"
