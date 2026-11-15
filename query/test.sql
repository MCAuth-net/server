-- name: GetPlayer :one
SELECT *
FROM minecraft_accounts
WHERE id = $1
    LIMIT 1;

-- sqlc wouldnt build without one