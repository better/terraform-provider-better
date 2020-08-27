app-name := better-secrets
version := 0.0.1

define builder
	docker run --rm -v $(shell pwd):/app -w /app golang:alpine $(1)
endef

go.sum: go.mod
	$(call builder,go mod vendor)

vendor: go.sum

${app-name}: database vendor
	$(call builder,go build -o ${app-name})

test: dir ?= ~/.terraform.d/plugins/${app-name}/${version}
test: ${app-name}
	mkdir -p ${dir}
	mv ${app-name} ${dir}/darwin_amd64
	cd tests && terraform init && terraform plan

clean:
	-rm -rf ${app-name} vendor ~/.terraform.d/plugins/${app-name} tests/.terraform
