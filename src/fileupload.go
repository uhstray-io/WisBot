package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"wisbot/src/sqlgo"

	"github.com/rotisserie/eris"
)

// rootIdPage is a helper function that renders the root page with the given file.
func getId(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Grab the file where the ID matches.
	queryfile, err := wisQueries.GetFileNameAndUploadFromId(context.Background(), id)

	if err != nil {
		if err == sql.ErrNoRows {
			component := rootIdPage(nil)
			component.Render(r.Context(), w)
			return
		}
		fmt.Println("Error while executing GetFileNameAndUploadFromId query", err.Error())
	}

	file := &sqlgo.File{ID: queryfile.ID, Name: queryfile.Name, Uploaded: queryfile.Uploaded}

	component := rootIdPage(file)
	component.Render(r.Context(), w)
}

// uploadFileFormCompleted is a helper function that renders the upload file form.
func postIdUploadFile(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	// fmt.Println("-=+=- Uploading file -=+=-")
	// fmt.Println("HX-Request: ", r.Header.Get("HX-Request"))

	// Check if the ID exists and the file has not been uploaded.
	file := &sqlgo.File{ID: id}
	id, err := wisQueries.GetFileIdWhereIdAndUploadedIsFalse(context.Background(), id)

	// If the Id exists, and the file has not been uploaded, then continue.
	if err != nil {
		if err == sql.ErrNoRows {
			uploadFileFormCompleted(nil, false, "File not found.").Render(r.Context(), w)
			return nil
		}
		return eris.Wrap(err, "Error while executing GetFileIdWhereIdAndUploadedIsFalse query")
	}

	// Handle the file upload - 100MB max file maxFileSize.
	var maxSize int64 = maxFileSize * 1024 * 1024
	fmt.Println("Max File Size:", maxSize)
	if err := r.ParseMultipartForm(maxSize); err != nil {
		uploadFileFormCompleted(file, false, "Unable to parse form.").Render(r.Context(), w)
		return nil
	}

	fileObject, fileHeader, err := r.FormFile("file")
	if err != nil {
		uploadFileFormCompleted(file, false, "Unable to read file.").Render(r.Context(), w)
		return nil
	}
	defer fileObject.Close()

	// Read the Data into a buffer.
	buff, _ := io.ReadAll(io.LimitReader(fileObject, maxSize))
	if len(buff) >= int(maxSize) {
		uploadFileFormCompleted(file, false, "File too large.").Render(r.Context(), w)
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
	err2 := wisQueries.UpdateFileWhereId(context.Background(),
		sqlgo.UpdateFileWhereIdParams{
			ID:       id,
			Name:     file.Name,
			Data:     file.Data,
			Size:     int32(len(buff)),
			Uploaded: file.Uploaded,
		})

	if err2 != nil {
		uploadFileFormCompleted(file, false, "Unable to update file.").Render(r.Context(), w)
		return eris.Wrap(err2, "Error while executing UpdateFileWhereId query")
	}

	uploadFileFormCompleted(file, true, "").Render(r.Context(), w)

	return nil
}

// getIdDownloadFile is a handler that serves the file with the given ID.
func getIdDownloadFile(w http.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	// Grab the file where the ID matches.
	file, err := wisQueries.GetFileFromId(context.Background(), id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "file not found", http.StatusNotFound)
		}
		return eris.Wrap(err, "Error while executing GetFileFromId query")
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+file.Name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
	w.Write(file.Data)

	// Increment the download count.
	err2 := wisQueries.UpdateFileDownloadIncrement(context.Background(), id)
	if err2 != nil {
		return eris.Wrap(err2, "Error while executing UpdateFileDownloadIncrement query")
	}
	return nil
}
