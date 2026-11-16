-- name: CreateAccount :execrows
WITH new_account AS (
    INSERT INTO accounts DEFAULT VALUES
        RETURNING id
)
INSERT INTO account_uuids (uuid, account_id)
SELECT $1, id
FROM new_account;

-- name: CheckAccount :one
SELECT account_id FROM account_uuids WHERE uuid = $1;

-- name: AddUUID :execrows
INSERT INTO account_uuids (uuid, account_id)
VALUES (
    $1,
    $2
);