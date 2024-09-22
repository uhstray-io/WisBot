package main

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
)

func getId(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Grab the file where the ID matches.
	file := &File{}
	err := db.QueryRow("SELECT Id, Name, Uploaded FROM File WHERE Id = ?", id).Scan(&file.Id, &file.Name, &file.Uploaded)

	if err != nil && err == sql.ErrNoRows {
		component := rootIdPage(nil)
		component.Render(r.Context(), w)
		return
	}

	if err != nil {
		fmt.Println("Error in getId", err)
	}

	component := rootIdPage(file)
	component.Render(r.Context(), w)
}

func postIdUploadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	fmt.Println("uploading file")
	fmt.Println(r.Header.Get("HX-Request"))

	// Check if the ID exists and the file has not been uploaded.
	file := &File{}
	err := db.QueryRow("SELECT Id FROM File WHERE Id = ? AND Uploaded = False", id).Scan(&file.Id)

	// If the Id exists, and the file has not been uploaded, then continue.
	if err == sql.ErrNoRows {
		uploadFileFormCompleted(nil, false, "File not found.").Render(r.Context(), w)
		return
	}

	// If there is an error, print it out
	if err != nil {
		fmt.Println("Error", err)
	}

	// Handle the file upload - 100MB max file size.
	var size int64 = 100 * 1024 * 1024
	fmt.Println("Size", size)
	if err := r.ParseMultipartForm(size); err != nil {
		uploadFileFormCompleted(file, false, "Unable to parse form.").Render(r.Context(), w)
		return
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		uploadFileFormCompleted(file, false, "Unable to read file.").Render(r.Context(), w)
		return
	}
	defer f.Close()

	// Read the Data into a buffer.
	buff, _ := io.ReadAll(io.LimitReader(f, size))

	if len(buff) == int(size) {
		uploadFileFormCompleted(file, false, "File too large.").Render(r.Context(), w)
		return
	}

	fmt.Println("file header", fh.Filename, fh.Size, fh.Header)
	fmt.Println("file uploaded len", len(buff))

	// Update the file.
	file.Name = fh.Filename
	file.Data = buff
	file.Size = len(buff)
	file.Uploaded = true
	fmt.Println("File Uploaded:", file.Name)

	// Update the file in the database.
	_, err = db.Exec("UPDATE File SET Name = ?, Data = ?, Size = ?, Uploaded = ? WHERE Id = ?", file.Name, file.Data, file.Size, file.Uploaded, id)

	if err != nil {
		fmt.Println("Error", err)
		uploadFileFormCompleted(file, false, "Unable to update file.").Render(r.Context(), w)
		return
	}

	uploadFileFormCompleted(file, true, "").Render(r.Context(), w)

}

func getIdDownloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Grab the file where the ID matches.
	file := &File{}
	err := db.QueryRow("SELECT Name, Data, Size FROM File WHERE Id = ?", id).Scan(&file.Name, &file.Data, &file.Size)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "file not found", http.StatusNotFound)
		}
		fmt.Println("Error", err)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+file.Name)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
	w.Write(file.Data)

	db.Exec("UPDATE File SET Downloads = Downloads + 1 WHERE Id = ?", id)
}
