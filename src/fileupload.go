package main

import (
	"fmt"
	"io"
	"net/http"
	"runtime"
)

type TempFile struct {
	Name     string
	Id       string // uuid
	Uploaded bool   // Shows if the file has been uploaded or not

	Data []byte
	Size int
}

func getId(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Grab the file.
	var file *TempFile
	for _, tempfile := range TempFiles {
		if tempfile.Id == id {
			file = tempfile
			break
		}
	}

	if file == nil {
		component := rootIdPage(id, false, false, "File Not Found")
		component.Render(r.Context(), w)
	} else {
		component := rootIdPage(id, true, file.Uploaded, file.Name)
		component.Render(r.Context(), w)
	}
}

func postIdUploadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	fmt.Println("uploading file")
	fmt.Println(r.Header.Get("HX-Request"))

	// Check if the ID exists
	var file *TempFile
	for _, tempfile := range TempFiles {
		if tempfile.Id == id && !tempfile.Uploaded {
			file = tempfile
			break
		}
	}

	if file == nil {
		// http.Error(w, "file not found", http.StatusNotFound)
		uploadFileFormCompleted(id, false, "File not found.").Render(r.Context(), w)
		return
	}

	// Handle the file upload.
	// 100MB max file size.
	var size int64 = 100 * 1024 * 1024
	fmt.Println("Size", size)
	if err := r.ParseMultipartForm(size); err != nil {
		// http.Error(w, "unable to parse form", http.StatusBadRequest)
		uploadFileFormCompleted(id, false, "Unable to parse form.").Render(r.Context(), w)
		return
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		// http.Error(w, "unable to read file", http.StatusBadRequest)
		uploadFileFormCompleted(id, false, "Unable to read file.").Render(r.Context(), w)
		return
	}
	defer f.Close()

	// Read the Data into a buffer.
	buff, _ := io.ReadAll(io.LimitReader(f, size))

	if len(buff) == int(size) {
		// http.Error(w, "file too large", http.StatusBadRequest)
		uploadFileFormCompleted(id, false, "File too large.").Render(r.Context(), w)
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

	uploadFileFormCompleted(id, true, "").Render(r.Context(), w)

	runtime.GC()
}

func getIdDownloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	for _, file := range TempFiles {
		if file.Id == id {
			w.Header().Set("Content-Disposition", "attachment; filename="+file.Name)
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))
			w.Write(file.Data)
			return
		}
	}

	http.Error(w, "file not found", http.StatusNotFound)
}
