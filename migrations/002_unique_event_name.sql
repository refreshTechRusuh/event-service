-- Migration 002: enforce uniqueness on event name (case-insensitive)
-- Usecase sudah ada ExistsByName guard, ini lapisan defense-in-depth di DB.

-- Hapus duplicate dulu sebelum constraint dibuat.
-- Pertahankan baris created_at paling lama per nama (case-insensitive).
DELETE FROM event
WHERE id NOT IN (
    SELECT DISTINCT ON (LOWER(name)) id
    FROM event
    ORDER BY LOWER(name), created_at ASC
);

-- Unique index case-insensitive via LOWER()
CREATE UNIQUE INDEX IF NOT EXISTS idx_event_name_unique ON event (LOWER(name));
