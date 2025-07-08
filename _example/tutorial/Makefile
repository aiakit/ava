.PHONY: proto data build

proto:
	for d in proto; do \
		for f in $$d/**/*.proto; do \
		    protoc  --ava_out=plugins=ava:. $$f; \
			echo compiled: $$f; \
		done; \
	done

# build
build:
	./bin/build.sh


# build and stop ,then restart
run:
	./bin/build.sh; \
	./bin/stop.sh;\
	./bin/restart.sh;\

# stop
stop:
	./bin/stop.sh

# restart
restart:
	./bin/restart.sh

say:
	curl -H "Content-Type:application/json" -X POST -d '{"ping": "ping"}' http://127.0.0.1:9999/ava/HelloWorld/Say
