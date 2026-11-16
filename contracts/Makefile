.PHONY: test coverage

compile:
	pnpm run build

force-compile:
	pnpm exec hardhat build --force

clean:
	rm -rf coverage/
	rm -rf build/
	rm -rf cache/

fmt:
	pnpm run lint

fmt-fix:
	pnpm run lint:fix

test:
	pnpm test

coverage:
	pnpm run coverage

clean-fmt-test:
	$(MAKE) clean
	$(MAKE) fmt-fix
	$(MAKE) test
