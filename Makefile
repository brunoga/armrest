PKG     := ./...
APP     := ./cmd/armrest

.PHONY: build run test test-live check screens lint vet tidy android bridge-dex install-android ci clean

build:
	go build -o armrest $(APP)

run:
	go run $(APP)

test:
	go test -race $(PKG)

# Against the hubs on the network, one package at a time (they share the
# discovery port); only reads.
test-live:
	go test -tags live -count=1 -p 1 -v -run Live $(PKG)

# The quick pre-push check: tidy, vet, lint and the tests (screenshots
# included).
check: tidy vet lint test
	@echo "== check: passed"

# Accepts the screenshots the last test run wrote to testdata/failed as
# the new masters. Look at them first.
screens:
	@for f in $$(find . -path '*/testdata/failed/*.png'); do \
		dst=$$(echo $$f | sed 's#/failed/#/#'); echo "$$f -> $$dst"; mv $$f $$dst; \
	done
	@find . -type d -path '*/testdata/failed' -empty -delete

vet:
	go vet $(PKG)

tidy:
	go mod tidy -diff

lint:
	@command -v golangci-lint >/dev/null || (echo "golangci-lint not installed: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest" && exit 1)
	golangci-lint run --timeout 5m

# An Android package for arm64 phones, signed with a debug key. Needs the
# fyne tool (go install fyne.io/tools/cmd/fyne@latest) and the Android NDK
# in ANDROID_NDK_HOME (default: the newest under ~/Android).
ANDROID_NDK_HOME ?= $(lastword $(sort $(wildcard $(HOME)/Android/android-ndk-*)))
export ANDROID_NDK_HOME

android:
	sed -e "s/@VERSION@/$$(sed -n 's/^Version = "\(.*\)"/\1/p' cmd/armrest/FyneApp.toml)/" \
		-e "s/@BUILD@/$$(sed -n 's/^Build = //p' cmd/armrest/FyneApp.toml)/" \
		cmd/armrest/AndroidManifest.xml.in > cmd/armrest/AndroidManifest.xml
	cd cmd/armrest && fyne package --target android/arm64; s=$$?; rm -f AndroidManifest.xml; exit $$s
	mv cmd/armrest/Armrest.apk .
	scripts/check-deps.sh Armrest.apk

# The Java side of Android's Bluetooth (internal/androidble), compiled to
# the dex the app embeds: needs a JDK and the Android SDK (ANDROID_SDK,
# default ~/Android/sdk), which nothing else does.
ANDROID_SDK ?= $(HOME)/Android/sdk
JAVA_HOME ?= $(HOME)/Android/jdk
bridge-dex:
	rm -rf .bridge && mkdir .bridge
	$(JAVA_HOME)/bin/javac --release 8 -cp $(ANDROID_SDK)/platforms/android-34/android.jar -d .bridge \
		internal/androidble/java/io/github/brunoga/armrest/BleBridge.java
	PATH=$(JAVA_HOME)/bin:$$PATH $(ANDROID_SDK)/build-tools/34.0.0/d8 --min-api 21 \
		--lib $(ANDROID_SDK)/platforms/android-34/android.jar --output .bridge \
		.bridge/io/github/brunoga/armrest/BleBridge.class
	mv .bridge/classes.dex internal/androidble/bridge.dex
	rm -rf .bridge

install-android:
	adb install -r Armrest.apk

# What CI runs on Linux.
ci: check build
	@echo "== ci: passed"

clean:
	rm -f armrest Armrest.apk coverage.out
