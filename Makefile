.DEFAULT_GOAL := serve-docs

DOCS_UV = sh scripts/docs.sh
DOCS_ADDR ?= 127.0.0.1:8000

.PHONY: serve-docs build-docs deploy-docs update-docs

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
