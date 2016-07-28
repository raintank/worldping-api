default:
	$(MAKE) all
test:
	bash -c "./rt-pkg/test.sh $(TEST)"
check:
	$(MAKE) test
all:
	bash -c "./rt-pkg/build.sh"

