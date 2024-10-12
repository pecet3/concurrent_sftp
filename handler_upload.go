package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
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

		dst, err := os.Create("./uploads/" + header.Filename)
		if err != nil {
			http.Error(w, "Error saving file", http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		f := &File{
			Path: header.Filename,
		}
		a.m.Upload(f, file)

		fmt.Fprintf(w, "File %s uploaded successfully!", header.Filename)
	}
}
