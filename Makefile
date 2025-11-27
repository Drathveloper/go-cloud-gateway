PARENT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

include $(PARENT_DIR).parent/parent.mk

SOURCE.DIR   := ./...
LINT.CONFIG  := .golangci.yml