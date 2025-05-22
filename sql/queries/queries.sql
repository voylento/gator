-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, name)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetUser :one
SELECT * FROM users
WHERE name = $1 LIMIT 1;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUsers :many
SELECT * FROM users;

-- name: DeleteAllUsers :exec
TRUNCATE TABLE users CASCADE;

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
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds
WHERE url = $1 LIMIT 1;

-- name: GetAllFeeds :many
SELECT * FROM feeds;

-- name: GetFeedsByUser :many
SELECT * FROM feeds
WHERE user_id = $1;

-- name: DeleteAllFeeds :exec
TRUNCATE TABLE feeds;

-- name: CreateFeedFollow :many
WITH inserted_feed_follow AS (
  INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
  VALUES(
    $1,
    $2,
    $3,
    $4,
    $5
  )
  RETURNING *
)
SELECT 
  ff.*,
  f.name AS feed_name,
  u.name AS user_name
FROM inserted_feed_follow ff
INNER JOIN feeds f ON ff.feed_id = f.id
INNER JOIN users u ON ff.user_id = u.id;

-- name: GetFollowsByUser :many
SELECT
  ff.id,
  ff.user_id,
  ff.feed_id,
  f.name AS feed_name,
  u.name AS user_name
FROM feed_follows ff
INNER JOIN feeds f ON ff.feed_id = f.id
INNER JOIN users u ON ff.user_id = u.id
WHERE ff.user_id = $1;

-- name: GetAllFeedFollows :many
SELECT * FROM feed_follows;

-- name: DeleteFeedFollows :execresult
DELETE FROM feed_follows WHERE user_id = $1 AND feed_id = $2;

-- name: UpdateFeedFetchTime :exec
UPDATE feeds
SET last_fetched_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT id, created_at, updated_at, last_fetched_at, name, url, user_id
FROM feeds
ORDER BY 
    -- Prioritize feeds that have never been fetched (NULL values first)
    CASE WHEN last_fetched_at IS NULL THEN 0 ELSE 1 END,
    -- Then sort by oldest fetch time
    last_fetched_at ASC NULLS FIRST,
    -- If multiple feeds have the same last_fetched_at, use id for consistent ordering
    id
LIMIT 1;

-- name: CreatePost :one
INSERT INTO posts (id, created_at, updated_at, title, url, description, published_at, feed_id)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING *;

-- name: GetPostsForUser :many
SELECT posts.*, feeds.name AS feed_name 
FROM posts 
INNER JOIN feed_follows ON posts.feed_id = feed_follows.feed_id 
INNER JOIN feeds ON posts.feed_id = feeds.id
WHERE feed_follows.user_id = $1
ORDER BY posts.published_at DESC
LIMIT $2;
