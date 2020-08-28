name := better
dir := .terraform.d
plugins-dir := ${dir}/plugins
binary := ${plugins-dir}/terraform-provider-${name}
build-image := golang:alpine

export GOOS ?= linux
export GOARCH ?= amd64

define builder
	docker run --rm -v $(shell pwd):/app -e GOOS -e GOARCH -w /app ${build-image} $(1)
endef

define get-provider
	docker run --rm -e GOOS -e GOARCH -e GO111MODULE=on -v $(shell pwd)/${plugins-dir}:/go/bin ${build-image} sh -c "apk add --no-cache git && go get $(1)"
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
		-w /app/tests hashicorp/terraform:0.12.29 $(1)
endef

define secret
	aws secretsmanager get-secret-value --secret-id terraform | jq -r ".SecretString | fromjson | .$(1)"
endef

go.sum: go.mod

vendor: go.sum
	$(call builder,go mod vendor)

${plugins-dir}:
	$(call get-provider,github.com/strongdm/terraform-provider-sdm)

${binary}: ${name} vendor
	$(call builder,go build -o ${binary})

tests/.terraform: tests/test.tf ${plugins-dir} ${binary}
	$(call terraform,init)

terraform-%: tests/.terraform
	$(call terraform,$*)

test: terraform-apply

clean:
	-rm -rf ${dir} vendor tests/.terraform tests/terraform.tfstate*
