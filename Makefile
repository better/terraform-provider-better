export GOOS ?= linux
export GOARCH ?= amd64

name := better
version := 1.0.0
build-image := golang:alpine
plugins-dir := .terraform.d/plugins
dir := ${plugins-dir}/terraform.better.com
binary := ${dir}/better/${name}/${version}/${GOOS}_${GOARCH}/terraform-provider-${name}_v${version}
sdm-binary := ${dir}/strongdm/sdm/1/${GOOS}_${GOARCH}/terraform-provider-sdm_v1

define builder
	docker run --rm -v $(shell pwd):/app -e GOOS -e GOARCH -w /app ${build-image} $(1)
endef

define get-provider
	docker run --rm -e GOOS -e GOARCH -e GO111MODULE=on -v $(shell pwd)/${plugins-dir}:/go/bin ${build-image} sh -c "apk add --no-cache git && go get github.com/$(1)/terraform-provider-$(2)" && \
	mkdir -p ${dir}/$(1)/$(2)/$(3)/${GOOS}_${GOARCH} && mv ${plugins-dir}/terraform-provider-$(2) ${dir}/$(1)/$(2)/$(3)/${GOOS}_${GOARCH}/terraform-provider-$(2)_v$(3)
endef

define terraform
	docker run --rm -it \
		-v $(shell pwd):/app \
		-v $(shell pwd)/${dir}:/root/${dir} \
		-e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY \
		-e AWS_SESSION_TOKEN \
		-e AWS_SECURITY_TOKEN \
		-e SDM_API_ACCESS_KEY=$(shell $(call secret,SDM_API_ACCESS_KEY)) \
		-e SDM_API_SECRET_KEY=$(shell $(call secret,SDM_API_SECRET_KEY)) \
		-w /app/tests hashicorp/terraform:0.13.5 $(1)
endef

define secret
	aws secretsmanager get-secret-value --secret-id terraform | jq -r ".SecretString | fromjson | .$(1)"
endef

go.sum: go.mod

vendor: go.sum
	$(call builder,go mod vendor)

${binary}: ${name} vendor
	$(call builder,go build -o ${binary})

${sdm-binary}:
	$(call get-provider,strongdm,sdm,1)

build: ${binary}
plugins: ${sdm-binary}

tests/.terraform: tests/test.tf plugins build
	$(call terraform,init)

terraform-%: tests/.terraform
	$(call terraform,$*)

test: terraform-apply

clean:
	-rm -rf ${dir} vendor tests/.terraform tests/terraform.tfstate*
