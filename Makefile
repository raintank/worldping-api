.PHONY: test build docker all

clean:
	rm -rf build/worldping-api

default:
	$(MAKE) all

test:
	bash -c "./scripts/test.sh $(TEST)"

all: docker

build: build/worldping-api

build/worldping-api:
	bash -c "./scripts/build.sh"

docker: build
	bash -c "./scripts/build_docker.sh"

