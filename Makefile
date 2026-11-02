# Binary output name
BINARY_NAME=bin/secret-api
MAIN_PACKAGE=.

.PHONY: all help run build test test-race vet tidy clean

# Target default: menampilkan menu bantuan
all: help

## help: Tampilkan daftar perintah yang tersedia
help:
	@echo "Perintah yang tersedia:"
	@echo "  make run        - Menjalankan server secara langsung (go run)"
	@echo "  make build      - Mengompilasi aplikasi menjadi binary di ./bin"
	@echo "  make test       - Menjalankan unit test"
	@echo "  make test-race  - Menjalankan test dengan deteksi race condition (wajib untuk in-memory)"
	@echo "  make vet        - Menjalankan static code analysis (go vet)"
	@echo "  make tidy       - Merapikan dependensi go.mod dan go.sum"
	@echo "  make clean      - Menghapus binary hasil build"

## run: Menjalankan server aplikasi
run:
	go run $(MAIN_PACKAGE)

## build: Mengompilasi aplikasi
build:
	@mkdir -p bin
	go build -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build sukses! Binary tersimpan di $(BINARY_NAME)"

## test: Menjalankan unit test
test:
	go test -v ./...

## test-race: Menjalankan test dengan deteksi race condition
test-race:
	go test -race -v ./...

## vet: Analisis kode statis
vet:
	go vet ./...

## tidy: Merapikan dependensi modul
tidy:
	go mod tidy

## clean: Membersihkan file kompilasi
clean:
	rm -rf bin/
	@echo "File binary di ./bin berhasil dibersihkan"
