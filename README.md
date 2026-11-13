# event-service

Layanan katalog **event dan kategori tiket** untuk Rusuh Booking Tiket (**BE-2**). Admin mengelola event dan kapasitas kategori; klien membaca katalog dan detail event. Service menyimpan data di PostgreSQL, menerbitkan event integrasi ke Kafka, dan dapat menyiarkan status sold-out ke booking-service melalui Valkey Pub/Sub.

- Arsitektur: Hexagonal (Ports & Adapters), `net/http` + PostgreSQL (`pgx`) + Kafka
- Port lokal: **8082**
- Admin JWT: `Authorization: Bearer <authToken>` dari user-service
- Detail event: `X-Access-Token` dari queue-service

## Prasyarat

- Go 1.26+
- Docker dan Docker Compose
- PostgreSQL, Kafka, dan (opsional) Redis/Valkey dari repo `tools`
- Secret JWT lokal yang sama dengan service terkait; production memakai secret manager

## Menjalankan

Jalankan infrastruktur bersama sekali:

```bash
cd ../tools
make up
```

Lalu jalankan service:

```bash
cd ../event-service
APP_ENV=local APP_PORT=8082 \
DB_PASSWORD=password KAFKA_BROKERS=localhost:29092 \
ACCESS_TOKEN_SECRET=dev-access-token-secret \
USER_JWT_SECRET=dev-user-jwt-change-in-production \
OTEL_ENABLED=false make run
```

PostgreSQL lokal dari `tools` memakai database `rusuh_booking`, user `postgres`, password `password`. `VALKEY_ADDR` boleh dikosongkan untuk menonaktifkan broadcast sold-out; Kafka lokal dapat diakses melalui `localhost:29092`.

## Endpoint

| Method | Path | Akses | Keterangan |
|---|---|---|---|
| GET | `/api/event/v1/list?page=1&size=20` | Publik | Katalog event dengan paginasi |
| GET | `/api/event/v1/detail/{eventId}` | `X-Access-Token` | Detail event dan kategori; menyertakan sisa waktu admission bila tersedia |
| GET | `/api/event/v1/admin/list-event` | Admin | Daftar event untuk pengelolaan |
| POST | `/api/event/v1/admin/create-event` | Admin | Membuat event |
| PUT | `/api/event/v1/admin/update-event/{eventId}` | Admin | Memperbarui event |
| DELETE | `/api/event/v1/admin/delete-event/{eventId}` | Admin | Menghapus event |
| POST | `/api/event/v1/admin/{eventId}/create-category` | Admin | Membuat kategori tiket |
| PUT | `/api/event/v1/admin/update-category/{categoryId}` | Admin | Memperbarui kategori |
| POST | `/internal/event/v1/{eventId}/sold-out` | Internal | Menandai event sold-out dari alur booking |
| GET | `/sys/ping` | Publik | Health check |

Contoh membuat event (gunakan JWT admin dari `POST /auth/login` di user-service):

```bash
curl -X POST http://localhost:8082/api/event/v1/admin/create-event \
  -H 'Authorization: Bearer <admin-authToken>' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Konser Rusuh","description":"Tur 2026","status":"on_sale"}'
```

Status event yang didukung: `draft`, `on_sale`, dan `closed`. Endpoint admin memverifikasi role `admin` dengan `USER_JWT_SECRET`; endpoint detail memvalidasi token admission dengan `ACCESS_TOKEN_SECRET`.

## Konfigurasi

Semua konfigurasi dibaca dari environment variable; file `.env` opsional.

| Env | Default lokal | Keterangan |
|---|---|---|
| `APP_ENV` / `APP_PORT` | `local` / `8082` | Cloud Run dapat mengisi `PORT` |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `json` | Level: `debug`, `info`, `warn`, `error`; format `json` atau `text` |
| `DB_HOST` / `DB_PORT` | `localhost` / `5432` | PostgreSQL bersama |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | `postgres` / kosong / `rusuh_booking` | Isi password `password` untuk Compose lokal |
| `DB_SSL_MODE` | `disable` | SSL mode PostgreSQL |
| `DB_MAX_OPEN_CONNS` / `DB_MAX_IDLE_CONNS` | `25` / `10` | Pool koneksi |
| `DB_CONN_TIMEOUT_SECONDS` | `5` | Timeout koneksi database |
| `ACCESS_TOKEN_SECRET` | wajib diisi | Memverifikasi access token queue-service |
| `USER_JWT_SECRET` | `dev-user-jwt-change-in-production` | Harus sama dengan user-service untuk endpoint admin |
| `KAFKA_BROKERS` | `localhost:9092` | Compose lokal: `localhost:29092` |
| `KAFKA_USERNAME` / `KAFKA_PASSWORD` | kosong | Kredensial broker bila diperlukan |
| `KAFKA_TLS` | `false` | Aktifkan TLS untuk broker |
| `VALKEY_ADDR` / `VALKEY_PASSWORD` | kosong / kosong | Opsional; Valkey/Redis untuk broadcast sold-out |
| `VALKEY_TLS` | `false` | TLS untuk Valkey |
| `OTEL_ENABLED` | aktif | Set `false` untuk menonaktifkan ekspor Cloud Trace saat lokal |
| `OTEL_SAMPLE_RATIO` | `1.0` | Rasio trace yang disimpan, antara `0` dan `1` |

## Pengembangan

```bash
make test
make build
make golint
make scan       # gosec
make vuln       # govulncheck
```

Skema database berada di `migrations/`; database lokal bersama diinisialisasi oleh `tools/docker/postgres/init` saat volume PostgreSQL pertama kali dibuat.