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
INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id)
VALUES(
  $1,
  $2,
  $3,
  $4,
  $5
)
RETURNING *;

