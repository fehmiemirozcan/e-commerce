# Proje Değişkenleri
APP_NAME=ecommerce
MAIN_FILE=cmd/ecommerce/main.go

# Bu hedefler gerçek dosya değil.
.PHONY: help dev build rebuild clean up down restart logs logs-app logs-worker migrate migrate-down create-admin seed test test-backend lint tidy

## help: Kullanılabilecek komutları listeler.
help:
	@echo "Kullanılabilecek Komutlar:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## dev: Uygulamayı geliştirme modunda başlatır.
dev:
	@echo "Uygulama Başlatılıyor."
	go run $(MAIN_FILE) server

## build: Uygulamayı tek bir çalıştırılabilir dosya olarak derler.
build:
	@echo "Uygulama Derleniyor."
	go build -o $(APP_NAME) $(MAIN_FILE)

## rebuild: Eski derlemeyi temizleyip yeniden derler.
rebuild: clean build

## clean: Derlenmiş eski dosyaları temizler.
clean:
	@echo "Derlenmiş dosyalar temizleniyor."
	rm -f $(APP_NAME)

## up: Docker servislerini (Veritabanı, Redis, Mailpit vb.) ayağa kaldırır.
up:
	@echo "Application stack ayağa kalkıyor."
	docker compose up -d --build

## down: Çalışan tüm Docker servislerini durdurur ve kapatır.
down:
	@echo "Servisler Durduruluyor."
	docker compose down

## restart: Docker servislerini yeniden başlatır.
restart: down up

## logs: Tüm Docker servis loglarını takip eder.
logs:
	docker compose logs -f

## logs-app: Sadece app servisinin loglarını takip eder.
logs-app:
	docker compose logs -f app

## logs-worker: Sadece worker servisinin loglarını takip eder.
logs-worker:
	docker compose logs -f worker

## migrate: Tablolar oluşturulur.
migrate: 
	@echo "Migration UP işlemi başlatılıyor..."
	go run $(MAIN_FILE) migrate

## migrate-down: Oluşturulan tablolar silinir.
migrate-down: 
	@echo "Migration DOWN işlemi başlatılıyor..."
	go run $(MAIN_FILE) migrate-down

## create-admin: Sisteme yetkili bir admin kullanıcısı ekler.
create-admin:
	@echo "Admin kullanıcısı oluşturuluyor..."
	go run $(MAIN_FILE) create-admin

## seed: Veritabanını sahte/test verileriyle doldurur.
seed:
	@echo "Veritabanına sahte veriler (seed) ekleniyor..."
	go run $(MAIN_FILE) seed

## test: Tüm testleri çalıştırır.
test:
	@echo "Testler çalıştırılıyor..."
	go test ./...

## test-backend: Backend testlerini çalıştırır.
test-backend:
	@echo "Backend testleri çalıştırılıyor..."
	go test ./internal/...

## lint: Kodda olası hataları ve kötü kullanımları tarar.
lint:
	@echo "Linter çalıştırılıyor..."
	go vet ./...

## tidy: Modüller temizlenir.
tidy:
	@echo "Modüller düzenleniyor..."
	go mod tidy