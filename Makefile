SHELL=/bin/bash
test:
	env $$(cat .env | xargs) go test -v
