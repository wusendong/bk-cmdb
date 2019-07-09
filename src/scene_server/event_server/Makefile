TARGET_NAME?=$(shell awk 'FNR==1{print $$1}' build_info)
TARGET_PORT?=$(shell awk 'FNR==1{print $$2}' build_info)
include $(firstword $(subst :, ,${GOPATH}))/src/configcenter/scripts/Makefile
