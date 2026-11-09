-- migrations/003_add_poster_description.sql
-- Tambah kolom poster_url dan description ke tabel event

ALTER TABLE event ADD COLUMN IF NOT EXISTS poster_url TEXT NOT NULL DEFAULT '';
ALTER TABLE event ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
