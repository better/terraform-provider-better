name := better
dir := .terraform.d
binary := ${dir}/terraform-provider-${name}

export GOOS ?= darwin
export GOARCH ?= amd64

define builder
	docker run --rm -v $(shell pwd):/app -e GOOS -e GOARCH -w /app golang:alpine $(1)
endef

define terraform
	docker run --rm -it -v $(shell pwd):/app -v $(shell pwd)/${dir}:/root/${dir} -w /app/tests hashicorp/terraform:0.12.29 $(1)
endef

go.sum: go.mod

vendor: go.sum
	$(call builder,go mod vendor)

${binary}: ${name} vendor
	$(call builder,go build -o ${binary})

tests/.terraform: tests/test.tf ${binary}
	$(call terraform,init)

terraform-%: tests/.terraform
	$(call terraform,$*)

test: terraform-apply

clean:
	-rm -rf ${dir} vendor tests/.terraform tests/terraform.tfstate*
