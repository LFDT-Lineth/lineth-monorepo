.PHONY: test coverage

compile:
	pnpm run build

force-compile: export TS_NODE_TRANSPILE_ONLY := 1
force-compile:
	pnpm exec hardhat compile --force

clean:
	rm -rf coverage/
	rm -rf build/
	rm -rf cache/

fmt:
	pnpm run lint

fmt-fix:
	pnpm run lint:fix

test: export SKIP_DEPLOY_LOG := true
test:
	pnpm test -- --parallel

coverage: export SKIP_DEPLOY_LOG := true
coverage:
	pnpm run coverage

clean-fmt-test:
	make clean
	make fmt-fix
	make test
