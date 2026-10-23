VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/ujizin/ujibox/internal/cli.Version=$(VERSION)

.PHONY: build install test docs docs-serve app-android app-ios app-test

build:
	go build -ldflags "$(LDFLAGS)" -o bin/uji ./cmd/uji

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/uji

test:
	go vet ./...
	go test ./...

docs:
	scripts/docs.sh build

docs-serve:
	scripts/docs.sh

# The ujibox app (app/). The Android tasks need ANDROID_HOME or app/local.properties.
app-android:
	cd app && ./gradlew :androidApp:assembleDebug

app-ios:
	cd app/iosApp && xcodebuild -project iosApp.xcodeproj -scheme iosApp -configuration Debug \
		-destination "generic/platform=iOS Simulator" -derivedDataPath build build

app-test:
	cd app && ./gradlew :shared:testAndroidHostTest
