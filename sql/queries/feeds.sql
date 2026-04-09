-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING id, created_at, updated_at, name, url, user_id;

-- name: GetFeeds :many
SELECT f.id, f.created_at, f.updated_at, f.name, f.url, f.user_id, u.name AS owner_name
FROM feeds f
LEFT JOIN users u ON f.user_id = u.id
ORDER BY f.name;

-- name: GetFeedByURL :one
SELECT id, created_at, updated_at, name, url, user_id FROM feeds WHERE url = $1;

-- name: MarkFeedFetched :exec
UPDATE feeds SET last_fetched_at = now(), updated_at = now() WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT id, created_at, updated_at, name, url, user_id, last_fetched_at FROM feeds
ORDER BY last_fetched_at NULLS FIRST
LIMIT 1;
