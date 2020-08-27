define builder
	docker run --rm -v $(shell pwd):/app -w /app golang:alpine $(1)
endef

go.sum: go.mod
	$(call builder,go mod vendor)

terraform-provider-secrets: go.sum
	$(call builder,go build -o terraform-provider-secrets)

clean:
	-rm -rf terraform-provider-secrets vendor
