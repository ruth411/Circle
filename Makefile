PHASE ?= phase-0
ITER ?=

.PHONY: phase-check

phase-check:
	@./scripts/phase-check.sh "$(PHASE)" "$(ITER)"
