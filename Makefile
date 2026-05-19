IMAGE ?= docker.io/marfillaster/gpon-telemetry
TAG ?= alpine
ARCH_TAG ?= arm64
PLATFORM ?= linux/arm64
TAR ?= gpon-telemetry.tar

.PHONY: build test image tag-release save push push-release clean

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o gponserve ./cmd/gponserve
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o gpontelemetry ./cmd/gpontelemetry

test:
	go test ./...

image: build
	docker build --platform $(PLATFORM) -t $(IMAGE):$(TAG) .

tag-release: image
	docker tag $(IMAGE):$(TAG) $(IMAGE):$(TAG)-$(ARCH_TAG)

save: image
	docker save $(IMAGE):$(TAG) -o $(TAR)

push: image
	docker push $(IMAGE):$(TAG)

push-release: tag-release
	docker push $(IMAGE):$(TAG)
	docker push $(IMAGE):$(TAG)-$(ARCH_TAG)

clean:
	rm -f gponserve gpontelemetry $(TAR)
