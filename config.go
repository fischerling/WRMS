package main

type Config struct {
	Port          int
	Backends      string
	LocalMusicDir string
	UploadDir     string
	HasUpload     bool
}
