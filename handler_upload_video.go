package main

import (
	"net/http"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxBytes = 1 << 30 // 30 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
}
