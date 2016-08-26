default:
	$(MAKE) all
test:
	bash -c "./scripts/test.sh $(TEST)"
check:
	$(MAKE) test
all:
	bash -c "./scripts/depends.sh"
	bash -c "./scripts/build.sh"
docker:
	bash -c "./scripts/build_docker.sh"

