# these targets are not defined as depending on source files,
# so they should be recompiled by default “just in case”.
.PHONY: \
	test \
	ci-lint

##
## Run all the unit-tests
##
test:
	go test -tags debug ./...

##
## Run the CI linting
##
ci-lint:
	golangci-lint run --timeout 5m

