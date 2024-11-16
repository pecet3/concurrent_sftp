package main

import "time"

type File struct {
	FileName   string    `json:"file_name"`
	Path       string    `json:"path"`
	Url        string    `json:"url"`
	Size       int64     `json:"size"`
	Ext        string    `json:"ext"`
	CreatedAt  time.Time `json:"created_at"`
	LastOpenAt time.Time `json:"last_open_at"`
}

func NewFile(fName, path, ext)
