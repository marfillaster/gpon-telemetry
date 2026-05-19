IMAGE ?= docker.io/marfillaster/gpon-telemetry
TAG ?= latest
VERSION ?= $(shell date +%Y.%m.%d)
BASE_TAG ?= alpine3.22
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
	docker tag $(IMAGE):$(TAG) $(IMAGE):$(VERSION)
	docker tag $(IMAGE):$(TAG) $(IMAGE):$(ARCH_TAG)
	docker tag $(IMAGE):$(TAG) $(IMAGE):$(VERSION)-$(ARCH_TAG)
	docker tag $(IMAGE):$(TAG) $(IMAGE):$(BASE_TAG)-$(ARCH_TAG)
	docker tag $(IMAGE):$(TAG) $(IMAGE):$(VERSION)-$(BASE_TAG)-$(ARCH_TAG)

save: image
	docker save $(IMAGE):$(TAG) -o $(TAR)

push: image
	docker push $(IMAGE):$(TAG)

push-release: tag-release
	docker push $(IMAGE):$(TAG)
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):$(ARCH_TAG)
	docker push $(IMAGE):$(VERSION)-$(ARCH_TAG)
	docker push $(IMAGE):$(BASE_TAG)-$(ARCH_TAG)
	docker push $(IMAGE):$(VERSION)-$(BASE_TAG)-$(ARCH_TAG)

clean:
	rm -f gponserve gpontelemetry $(TAR)
