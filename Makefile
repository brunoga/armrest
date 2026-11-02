PKG     := ./...
APP     := ./cmd/armrest

.PHONY: build run test check screens lint vet tidy android install-android ci clean

build:
	go build -o armrest $(APP)

run:
	go run $(APP)

test:
	go test -race $(PKG)

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
# in ANDROID_NDK_HOME.
android:
	cd cmd/armrest && fyne package --target android/arm64
	mv cmd/armrest/Armrest.apk .

install-android:
	adb install -r Armrest.apk

# What CI runs on Linux.
ci: check build
	@echo "== ci: passed"

clean:
	rm -f armrest Armrest.apk coverage.out
