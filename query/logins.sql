-- name: CreateLogin :execrows
INSERT INTO logins (account_id, uuid, token, ip, protocol_version, brand, lang)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
);