name := secrets
binary := terraform-provider-${name}
target-dir := ~/.terraform.d/plugins

export GOOS ?= darwin
export GOARCH ?= amd64

define builder
	docker run --rm -v $(shell pwd):/app -e GOOS -e GOARCH -w /app golang:alpine $(1)
endef

go.sum: go.mod

vendor: go.sum
	$(call builder,go mod vendor)

${binary}: secrets vendor
	$(call builder,go build -o ${binary})

${target-dir}/${binary}: ${binary}
	mkdir -p ${target-dir}
	cp ${binary} ${target-dir}

tests/.terraform: tests/test.tf ${target-dir}/${binary}
	cd tests && terraform init

terraform-init: tests/.terraform

test: terraform-init
	cd tests && terraform plan

clean:
	-rm -rf ${binary} vendor ${target-dir}/${binary} tests/.terraform
