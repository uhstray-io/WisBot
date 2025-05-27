
-- name: CountFilesFromUser :one
SELECT COUNT(*) FROM files
WHERE Discord_Username = $1;


-- name: DeleteFileWhereUsersCountIsProvided :exec
DELETE FROM files 
WHERE Id IN (
  SELECT Id 
  FROM files f1
  WHERE f1.Discord_Username = $1
  ORDER BY f1.Created_At
  LIMIT $2
);


-- name: InsertFile :exec
INSERT INTO files 
(Id, Uploaded, Discord_Username, Name)
VALUES 
($1, $2, $3, $4);


-- name: GetFileFromId :one
SELECT Name, Data, Size 
FROM files 
WHERE Id = $1;


-- name: UpdateFileWhereId :exec
UPDATE files
SET Name = $1, Data = $2, Size =  $3, Uploaded = $4
WHERE Id = $5;


-- name: UpdateFileDownloadIncrement :exec
UPDATE files 
SET Downloads = Downloads + 1 
WHERE Id = $1;


-- name: GetFileIdWhereIdAndUploadedIsFalse :one
SELECT Id 
FROM files 
WHERE Id = $1
AND Uploaded = False;


-- name: GetFileNameAndUploadFromId :one
SELECT Id, Name, Uploaded 
FROM files
WHERE Id = $1;


-- name: DeleteFileWhereOlderThan :exec
DELETE FROM files 
WHERE Created_At < (NOW() - make_interval(days => $1));