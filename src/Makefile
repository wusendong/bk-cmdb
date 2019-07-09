
IMAGE?=image_placeholder
NPM?=npm

GITHASH?=$(shell git rev-parse HEAD)
GITTAG?=$(shell git describe --all --tags --exact-match 2>/dev/null | awk -F '/' '{print $$NF}')
VERSION?=$(shell git symbolic-ref -q --short HEAD || git describe --all --tags --exact-match 2>/dev/null | awk -F '-' '{print $$NF}')
ISDEBUG?=false
ifeq ($(ISDEBUG),true)
	BUILDTIME:=$(shell date +%Y-%m-%d)
	GOBUILD_FLAGS = -ldflags "-X configcenter/src/common/version.CCRunMode=dev -X configcenter/src/common/version.CCVersion=${VERSION} -X configcenter/src/common/version.CCBuildTime=${BUILDTIME} -X configcenter/src/common/version.CCGitHash=${GITHASH} -X configcenter/src/common/version.CCTag=${GITTAG}"
else
	BUILDTIME:=$(shell date +%Y-%m-%dT%T%z)
	GOBUILD_FLAGS = -ldflags "-X configcenter/src/common/version.CCVersion=${VERSION} -X configcenter/src/common/version.CCBuildTime=${BUILDTIME} -X configcenter/src/common/version.CCGitHash=${GITHASH} -X configcenter/src/common/version.CCTag=${GITTAG}"
endif

ifeq ($(GOPATH),)
	PROJECT_ROOT?=$(shell readlink -f $(firstword $(subst :, ,${GOPATH}))/src/configcenter)
else
	PROJECT_ROOT?=`git rev-parse --show-toplevel`
endif

SCRIPT_DIR?=$(shell readlink -f $(PROJECT_ROOT)/scripts)
RESOURCE_DIR?=$(shell readlink -f $(PROJECT_ROOT)/resources)
SOURCE_ROOT?=$(shell readlink -f $(PROJECT_ROOT)/src)
BUILD_DIR?=$(SOURCE_ROOT)/bin/build/$(VERSION)
BIN_PATH?=$(BUILD_DIR)

targets = $(shell find . -name 'build_info' | xargs awk 'FNR==1{print $$1}')

.PHONY: default ui server linux clean cleanall package enterprise
default: server ;

server: $(targets)
	@echo "\033[34mbuild server finish\033[0m"
	@cd $(SCRIPT_DIR) && python ./generate.py -t '$(BIN_PATH)' -i '${IMAGE}' -o '$(BIN_PATH)/docker'

$(targets):
	@echo "\033[34mbuild target [ $@ ] to $(BIN_PATH)/$@/$@ ... \033[0m" ;\
	export TARGET_PATH=$(dir $(shell find $(SOURCE_ROOT) -name 'build_info' | xargs grep -l $@))  ;\
	export TARGET_PORT=$(shell find $(SOURCE_ROOT) -name 'build_info' | xargs grep -l $@ | xargs awk 'FNR==1{print $$2}')  ;\
	cd $$TARGET_PATH && go build $(GOBUILD_FLAGS) -o $(BIN_PATH)/$@/$@ ;\
	python $(SCRIPT_DIR)/prepare_cfg.py $(BIN_PATH) $@ $(RESOURCE_DIR) $(SCRIPT_DIR) $$TARGET_PORT $(VERSION)

linux:
	@export CGO_ENABLED=0
	@export GOOS=linux 
	@export GOARCH=amd64
	$(MAKE)

ui:
	@echo ""
	@echo "\033[34mbuilding web... \033[0m"
	@rm -rf "./ui/node_modules"
	@cd $(SOURCE_ROOT)/ui && bash ./build.sh ${NPM} ${BIN_PATH}

package:
	rm -rf $(SOURCE_ROOT)/bin/pub/cmdb
	mkdir -p $(SOURCE_ROOT)/bin/pub && cp -R $(BIN_PATH) ./bin/pub/cmdb
	cd $(SOURCE_ROOT)/bin/pub && tar -zvcf cmdb.tar.gz cmdb
	@echo -e "\033[34mpackage success and store in $(SOURCE_ROOT)/bin/pub/cmdb.tar.gz\033[0m"

enterprise:
	rm -rf $(SOURCE_ROOT)/bin/enterprise/cmdb
	mkdir -p $(SOURCE_ROOT)/bin/enterprise/cmdb/server/bin/
	mkdir -p $(SOURCE_ROOT)/bin/enterprise/cmdb/server/conf/
	mkdir -p $(SOURCE_ROOT)/bin/enterprise/cmdb/errors/
	mkdir -p $(SOURCE_ROOT)/bin/enterprise/cmdb/language/
	mkdir -p $(SOURCE_ROOT)/bin/enterprise/cmdb/web/

	cp -R ${RESOURCE_DIR}/errors/*  $(SOURCE_ROOT)/bin/enterprise/cmdb/errors
	cp -R ${RESOURCE_DIR}/language/*  $(SOURCE_ROOT)/bin/enterprise/cmdb/language
	cp  ${BIN_PATH}/cmdb_*/cmdb_*  $(SOURCE_ROOT)/bin/enterprise/cmdb/server/bin/

clean:
	rm -rf ${BIN_PATH}

cleanall:
	rm -rf $(SOURCE_ROOT)/bin

