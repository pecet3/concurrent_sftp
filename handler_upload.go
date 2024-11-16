package main

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/pecet3/concurrent_sftp/multi_sftp"
)

var tmpl = template.Must(template.New("upload").Parse(`
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Upload file</title>
</head>
<body>
    <h1>Upload File</h1>
    <form enctype="multipart/form-data" action="/files" method="post">
        <input type="file" name="uploadedFile" />
        <input type="submit" value="Upload" />
    </form>
</body>
</html>
`))

// Handler obsługujący GET i POST
func (a app) handleUploadView(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if err := tmpl.Execute(w, nil); err != nil {
			http.Error(w, "Error rendering template", http.StatusInternalServerError)
		}
		return
	}
}
func (a app) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		file, header, err := r.FormFile("uploadedFile")
		if err != nil {
			http.Error(w, "Error uploading file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		f := &multi_sftp.File{
			Path: header.Filename,
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		a.m.Manager.Upload(ctx, f, file)

		fmt.Fprintf(w, "File %s uploaded successfully!", header.Filename)
	}
}
