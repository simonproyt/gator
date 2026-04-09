-- name: CreateFeedFollow :one
WITH ins AS (
  INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
  VALUES ($1, $2, $3, $4, $5)
  RETURNING id, created_at, updated_at, user_id, feed_id
)
SELECT ins.id, ins.created_at, ins.updated_at, ins.user_id, ins.feed_id, u.name AS user_name, f.name AS feed_name
FROM ins
JOIN users u ON ins.user_id = u.id
JOIN feeds f ON ins.feed_id = f.id;

-- name: GetFeedFollowsForUser :many
SELECT ff.id, ff.created_at, ff.updated_at, ff.user_id, ff.feed_id, u.name AS user_name, f.name AS feed_name, f.url AS feed_url
FROM feed_follows ff
JOIN users u ON ff.user_id = u.id
JOIN feeds f ON ff.feed_id = f.id
WHERE ff.user_id = $1
ORDER BY f.name;

-- name: DeleteFeedFollow :exec
DELETE FROM feed_follows WHERE user_id = $1 AND feed_id = $2;

-- name: GetFeedByURLForFollow :one
SELECT id, created_at, updated_at, name, url, user_id FROM feeds WHERE url = $1;
