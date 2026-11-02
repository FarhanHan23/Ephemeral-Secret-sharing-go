# 🔐 Ephemeral Secret Sharing API (Mini Privnote)

[![Go Version](https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Dependencies](https://img.shields.io/badge/Dependencies-Zero%20(Stdlib)-success)](#)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](#)

RESTful API untuk berbagi pesan, token, dan data rahasia sementara yang disimpan secara **in-memory** (RAM) dengan fitur **Self-Destruct (Burn After Read)** dan batas masa aktif **(Time-To-Live / TTL)**. 

Dibangun **100% menggunakan Go Standard Library (`net/http`) murni** tanpa framework pihak ketiga, mendemonstrasikan arsitektur *concurrency-safe*, *structured logging*, dan *graceful shutdown*.

---

## 📸 Arsitektur HTTP Request & Response

![Siklus HTTP Request dan Response](./image.png)

---

## 🚀 Fitur Utama

- **🔥 Burn After Read:** Rahasia otomatis dimusnahkan seketika dari RAM saat pertama kali dibaca.
- **⏳ Custom Time-To-Live (TTL):** Tentukan batas masa aktif pesan (contoh: `5m`, `1h`, `24h`).
- **🛡️ Concurrency-Safe:** Menggunakan `sync.RWMutex` untuk mencegah *race condition* pada akses simultan tinggi.
- **🚦 Body Size Limiting:** Menggunakan `http.MaxBytesReader` (maksimal 1 MB) untuk mencegah serangan *Denial-of-Service* (DoS).
- **📝 Structured Logging (`slog`):** Pencatatan log terstruktur modern dengan penangkap status HTTP dan latensi request.
- **🛑 Graceful Shutdown:** Menangani sinyal OS (`SIGINT`, `SIGTERM`) dengan aman tanpa memutus request yang sedang berjalan.
- **✨ Zero External Dependencies:** Ringan dan portabel, hanya bergantung pada Go runtime.

---

## 🛠️ Tech Stack & Primitives

- **Language:** Go (Golang)
- **Routing:** Modern Go 1.22+ `http.ServeMux` (Method + Path Matching)
- **Concurrency:** `sync.RWMutex`
- **Validation:** `errors.Join`, `time.ParseDuration`
- **Logging:** `log/slog`

---

## 📋 Dokumentasi Endpoint REST API

Base URL: `http://localhost:8080`

| Method | Endpoint | Deskripsi | Status Sukses |
| :--- | :--- | :--- | :--- |
| `GET` | `/health` | Cek status kesehatan server | `200 OK` |
| `POST` | `/api/secret` | Membuat data rahasia baru | `201 Created` |
| `GET` | `/api/secret/{id}` | Mengambil & membakar rahasia | `200 OK` / `404 Not Found` |
| `PUT` | `/api/secret/{id}` | Memperbarui isi atau durasi rahasia | `200 OK` |
| `DELETE` | `/api/secret/{id}` | Menghapus rahasia secara manual | `204 No Content` |
| `GET` | `/api/secret` | Melihat daftar secret yang masih aktif | `200 OK` |

---

## 💻 Panduan Menjalankan

Proyek ini dilengkapi dengan `Makefile` untuk mempermudah eksekusi:

### 1. Menjalankan Server Langsung
```bash
make run
```
Server akan aktif di `http://localhost:8080`.

### 2. Menjalankan Static Code Analysis
```bash
make vet
```

### 3. Mengompilasi Binary
```bash
make build
./bin/secret-api
```

---

## 🧪 Contoh Pengujian (cURL)

### 1. Simpan Rahasia Baru (Burn After Read)
```bash
curl -i -X POST http://localhost:8080/api/secret \
  -H "Content-Type: application/json" \
  -d '{
    "content": "postgres://admin:secret123@prod-db:5432/core",
    "ttl": "10m",
    "burn_after_read": true
  }'
```

**Respon (`201 Created`):**
```json
{
  "data": {
    "burn_after_read": true,
    "expires_at": "2026-11-02T16:45:00Z",
    "id": 1
  },
  "success": true
}
```

### 2. Baca Rahasia (Pertama Kali)
```bash
curl -i http://localhost:8080/api/secret/1
```
**Respon (`200 OK`):**
```json
{
  "data": {
    "burned": true,
    "content": "postgres://admin:secret123@prod-db:5432/core",
    "expired_at": "2026-11-02T16:45:00Z"
  },
  "success": true
}
```

### 3. Coba Baca Ulang (Sudah Hangus)
```bash
curl -i http://localhost:8080/api/secret/1
```
**Respon (`404 Not Found`):**
```json
{
  "error": "Secret tidak ditemukan atau sudah expired/dihanguskan."
}
```

---

## 📄 Lisensi
Didistribusikan di bawah lisensi [MIT](LICENSE).
