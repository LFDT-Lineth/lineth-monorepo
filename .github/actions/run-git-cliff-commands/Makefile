# Makefile backing the "run-git-cliff-commands" composite action.
#
# It exposes the same units of work the action performs as thin targets over
# scripts/*.sh, and adds *-dry-run targets so you can preview a component's next
# version and changelog locally before a release.
#
# Always run from the repository root (git-cliff --include-path patterns and the
# template path are relative to it), pointing make at this file:
#
#   make -f .github/actions/run-git-cliff-commands/Makefile version-dry-run \
#       COMPONENT=coordinator \
#       INCLUDE_PATH='coordinator/**' \
#       SCOPES=coordinator
#
#   make -f .github/actions/run-git-cliff-commands/Makefile changelog-dry-run \
#       COMPONENT=coordinator \
#       INCLUDE_PATH='coordinator/**' \
#       SCOPES=coordinator
#
# INCLUDE_PATH may list several paths, one per line. Quote a multi-line value;
# either a real newline-delimited string or bash/zsh $'a\nb' works, e.g.:
#
#   make -f .github/actions/run-git-cliff-commands/Makefile prepend-dry-run \
#       COMPONENT=linea-besu-package \
#       FOLDER_NAME=linea-besu/package \
#       SCOPES='linea-besu|tracer|sequencer|deps|misc' \
#       INCLUDE_PATH=$'tracer/**\ntracer-constraints/**\nlinea-besu/besu/**'
#
# Requires: git-cliff, git, awk, sed on PATH.
#
# The action itself calls the plain targets (paths/render/version/changelog);
# those read their inputs from the environment the action sets and write step
# outputs to $GITHUB_OUTPUT.

MK_DIR      := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
SCRIPTS_DIR := $(MK_DIR)/scripts

# Recipes rely on bash (set -o pipefail, errexit, nounset). /bin/sh is dash on
# Debian/Ubuntu, which lacks 'pipefail', so pin bash and apply the flags to every
# recipe here instead of a per-recipe `set -euo pipefail`. bash is already a hard
# dependency (every scripts/*.sh is a bash script).
SHELL       := bash
.SHELLFLAGS := -eu -o pipefail -c

# Overridable inputs (mostly for the *-dry-run targets).
TEMPLATE               ?= cliff.template.toml
RELEASE_TAG_SUFFIX     ?=
# Preview the changelog under the computed next-version header by default.
GENERATE_WITH_NEXT_TAG ?= true

# Export inputs so recipe sub-shells and the scripts read them from the
# environment instead of having their values spliced into the recipe text.
# This is what makes a multi-line INCLUDE_PATH safe — embedded newlines would
# otherwise break the '\'-continued shell command.
export INCLUDE_PATH SCOPES TEMPLATE COMPONENT RELEASE_TAG_SUFFIX
export FOLDER_NAME GENERATE_WITH_NEXT_TAG

.PHONY: help paths render version changelog prepend \
        version-dry-run changelog-dry-run prepend-dry-run

help:
	@echo 'Targets:'
	@echo '  paths            normalize INCLUDE_PATH -> include_paths'
	@echo '  render           render cliff template with SCOPES -> config path'
	@echo '  version          compute next version (needs CLIFF_CONFIG, INCLUDE_PATHS)'
	@echo '  changelog        generate changelog (needs CLIFF_CONFIG, INCLUDE_PATHS)'
	@echo '  prepend          prepend section into FOLDER_NAME/CHANGELOG.md'
	@echo '  version-dry-run    preview next version (COMPONENT, INCLUDE_PATH, SCOPES)'
	@echo '  changelog-dry-run  preview changelog    (COMPONENT, INCLUDE_PATH, SCOPES)'
	@echo '  prepend-dry-run    preview prepended CHANGELOG.md, file untouched'
	@echo '                     (COMPONENT, INCLUDE_PATH, SCOPES, FOLDER_NAME)'

# --- thin wrappers used by the composite action (env-driven) --------------

paths:
	@$(SCRIPTS_DIR)/paths.sh

render:
	@$(SCRIPTS_DIR)/render.sh

version:
	@$(SCRIPTS_DIR)/version.sh

changelog:
	@$(SCRIPTS_DIR)/changelog.sh

prepend:
	@$(SCRIPTS_DIR)/prepend.sh

# --- local dry-runs (no $GITHUB_OUTPUT; results go to your terminal) ------

version-dry-run:
	@test -n "$$COMPONENT"    || { echo 'COMPONENT is required (COMPONENT=...)'       >&2; exit 1; }
	@test -n "$$INCLUDE_PATH" || { echo 'INCLUDE_PATH is required (INCLUDE_PATH=...)' >&2; exit 1; }
	@test -n "$$SCOPES"       || { echo 'SCOPES is required (SCOPES=...)'             >&2; exit 1; }
	@include_paths="$$($(SCRIPTS_DIR)/paths.sh)"; \
	 config="$$($(SCRIPTS_DIR)/render.sh | sed 's/^config=//')"; \
	 INCLUDE_PATHS="$$include_paths" CLIFF_CONFIG="$$config" $(SCRIPTS_DIR)/version.sh

changelog-dry-run:
	@test -n "$$COMPONENT"    || { echo 'COMPONENT is required (COMPONENT=...)'       >&2; exit 1; }
	@test -n "$$INCLUDE_PATH" || { echo 'INCLUDE_PATH is required (INCLUDE_PATH=...)' >&2; exit 1; }
	@test -n "$$SCOPES"       || { echo 'SCOPES is required (SCOPES=...)'             >&2; exit 1; }
	@include_paths="$$($(SCRIPTS_DIR)/paths.sh)"; \
	 config="$$($(SCRIPTS_DIR)/render.sh | sed 's/^config=//')"; \
	 vout="$$(INCLUDE_PATHS="$$include_paths" CLIFF_CONFIG="$$config" $(SCRIPTS_DIR)/version.sh)"; \
	 changed="$$(printf '%s\n' "$$vout" | sed -n 's/^changed=//p')"; \
	 version="$$(printf '%s\n' "$$vout" | sed -n 's/^version=//p')"; \
	 if [ "$$changed" != "true" ]; then \
	   echo 'Nothing to bump; no changelog to preview.' >&2; exit 0; \
	 fi; \
	 echo "===== changelog preview ($$COMPONENT $$version) =====" >&2; \
	 INCLUDE_PATHS="$$include_paths" CLIFF_CONFIG="$$config" NEXT_VERSION="$$version" \
	   $(SCRIPTS_DIR)/changelog.sh

# Preview the CHANGELOG.md that `prepend` would produce, without writing the
# file or touching the git remote.
prepend-dry-run:
	@test -n "$$COMPONENT"    || { echo 'COMPONENT is required (COMPONENT=...)'       >&2; exit 1; }
	@test -n "$$INCLUDE_PATH" || { echo 'INCLUDE_PATH is required (INCLUDE_PATH=...)' >&2; exit 1; }
	@test -n "$$SCOPES"       || { echo 'SCOPES is required (SCOPES=...)'             >&2; exit 1; }
	@test -n "$$FOLDER_NAME"  || { echo 'FOLDER_NAME is required (FOLDER_NAME=...)'   >&2; exit 1; }
	@include_paths="$$($(SCRIPTS_DIR)/paths.sh)"; \
	 config="$$($(SCRIPTS_DIR)/render.sh | sed 's/^config=//')"; \
	 vout="$$(INCLUDE_PATHS="$$include_paths" CLIFF_CONFIG="$$config" $(SCRIPTS_DIR)/version.sh)"; \
	 changed="$$(printf '%s\n' "$$vout" | sed -n 's/^changed=//p')"; \
	 version="$$(printf '%s\n' "$$vout" | sed -n 's/^version=//p')"; \
	 if [ "$$changed" != "true" ]; then \
	   echo 'Nothing to bump; no changelog to preview.' >&2; exit 0; \
	 fi; \
	 INCLUDE_PATHS="$$include_paths" CLIFF_CONFIG="$$config" NEXT_VERSION="$$version" \
	   DRY_RUN=true $(SCRIPTS_DIR)/prepend.sh
