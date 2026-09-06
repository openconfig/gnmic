.DEFAULT_GOAL := serve-docs

include versions.env

DOCS_UV = sh scripts/docs.sh
DOCS_ADDR ?= 127.0.0.1:8000
DOCKER_TAG ?= gnmic:$(GNMIC_TEST_VERSION)

.PHONY: build test build-docker serve-docs build-docs deploy-docs update-docs sync-versions check-versions

sync-versions:
	sh scripts/sync-versions.sh

check-versions:
	sh scripts/sync-versions.sh --check

build:
	sh scripts/go.sh build -o gnmic .

test:
	./tests/run_tests.sh

build-docker:
	sh scripts/docker-build.sh -t "$(DOCKER_TAG)"

serve-docs:
	$(DOCS_UV) run --locked zensical serve --config-file mkdocs.yml --dev-addr "$(DOCS_ADDR)"

build-docs:
	$(DOCS_UV) run --locked zensical build --config-file mkdocs.yml --clean --strict

# Publish the generated site to the existing GitHub Pages branch.
deploy-docs: build-docs
	$(DOCS_UV) run --locked ghp-import --no-jekyll --push --force --branch gh-pages site

# Resolve current releases; commit uv.lock after checking the resulting site.
update-docs:
	$(DOCS_UV) lock --upgrade
