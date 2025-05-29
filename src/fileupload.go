package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"wisbot/src/sqlc"
	"wisbot/src/templ"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
)

// rootIdPage is a helper function that renders the root page with the given file.
func getId(w http.ResponseWriter, r *http.Request) {
	ctx, span := StartSpan(r.Context(), "getId")
	defer span.End()

	if !postgresServiceEnabled || db == nil {
		http.Error(w, "Upload feature is unavailable (database disabled)", http.StatusServiceUnavailable)
		return
	}

	id := r.PathValue("id")
	span.SetAttributes(attribute.String("file_id", id))

	// Grab the file where the ID matches.
	queryfile, err := db.GetFileNameAndUploadFromId(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			component := templ.RootIdPage(nil)
			component.Render(r.Context(), w)
			return
		}
		// fmt.Println("error while executing GetFileNameAndUploadFromId query", err.Error())
		LogError(ctx, err, "Error while executing GetFileNameAndUploadFromId query")
	}

	file := &sqlc.File{ID: queryfile.ID, Name: queryfile.Name, Uploaded: queryfile.Uploaded}

	component := templ.RootIdPage(file)
	component.Render(r.Context(), w)
}

// uploadFileFormCompleted is a helper function that renders the upload file form.
func postIdUploadFile(w http.ResponseWriter, r *http.Request) error {
	if !postgresServiceEnabled || db == nil {
		http.Error(w, "Upload feature is unavailable (database disabled)", http.StatusServiceUnavailable)
		return nil
	}

	id := r.PathValue("id")

	// Check if the ID exists and the file has not been uploaded.
	file := &sqlc.File{ID: id}
	id, err := db.GetFileIdWhereIdAndUploadedIsFalse(context.Background(), id)

	// If the Id exists, and the file has not been uploaded, then continue.
	if err != nil {
		if err == sql.ErrNoRows {
			templ.UploadFileFormCompleted(nil, false, "File not found.").Render(r.Context(), w)
			return nil
		}
		return fmt.Errorf("error while executing GetFileIdWhereIdAndUploadedIsFalse query: %w", err)
	}

	// Handle the file upload - 100MB max file maxFileSize.
	var maxSize int64 = maxFileSize * 1024 * 1024
	LogEvent(r.Context(), log.SeverityInfo, "File upload configured", attribute.Int64("max_file_size_bytes", maxSize))
	if err := r.ParseMultipartForm(maxSize); err != nil {
		templ.UploadFileFormCompleted(file, false, "Unable to parse form.").Render(r.Context(), w)
		return nil
	}

	fileObject, fileHeader, err := r.FormFile("file")
	if err != nil {
		templ.UploadFileFormCompleted(file, false, "Unable to read file.").Render(r.Context(), w)
		return nil
	}
	defer fileObject.Close()

	// Read the Data into a buffer.
	buff, _ := io.ReadAll(io.LimitReader(fileObject, maxSize))
	if len(buff) >= int(maxSize) {
		templ.UploadFileFormCompleted(file, false, "File too large.").Render(r.Context(), w)
		return nil
	}

	// fmt.Println("File Name:", fileHeader.Filename)
	// fmt.Println("File Size:", fileHeader.Size)
	// fmt.Println("File Header:", fileHeader.Header)
	// fmt.Println("File *Actual* Size:", len(buff))

	// Update the file.
	file.Name = fileHeader.Filename
	file.Data = buff
	file.Uploaded = true

	// Update the file in the database.
	err2 := db.UpdateFileWhereId(context.Background(),
		sqlc.UpdateFileWhereIdParams{
			ID:       id,
			Name:     file.Name,
			Data:     file.Data,
			Size:     int32(len(buff)),
			Uploaded: file.Uploaded,
		})

	if err2 != nil {
		templ.UploadFileFormCompleted(file, false, "Unable to update file.").Render(r.Context(), w)
		return fmt.Errorf("error while executing UpdateFileWhereId query: %w", err2)
	}

	templ.UploadFileFormCompleted(file, true, "").Render(r.Context(), w)

	return nil
}

// getIdDownloadFile is a handler that serves the file with the given ID.
func getIdDownloadFile(w http.ResponseWriter, r *http.Request) error {
	if !postgresServiceEnabled || db == nil {
		http.Error(w, "Download feature is unavailable (database disabled)", http.StatusServiceUnavailable)
		return nil
	}

	id := r.PathValue("id")

	// Grab the file where the ID matches.
	file, err := db.GetFileFromId(context.Background(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "file not found", http.StatusNotFound)
		}
		return fmt.Errorf("error while executing GetFileFromId query: %w", err)
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+file.Name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
	w.Write(file.Data)

	// Increment the download count.
	err2 := db.UpdateFileDownloadIncrement(context.Background(), id)
	if err2 != nil {
		return fmt.Errorf("error while executing UpdateFileDownloadIncrement query: %w", err2)
	}
	return nil
}
