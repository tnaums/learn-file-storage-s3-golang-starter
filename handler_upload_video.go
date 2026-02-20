package main

import (
	"log"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxBytes = 1 << 30 // 30 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	id := r.PathValue("videoID")
	uid, err := uuid.Parse(id)
	if err != nil {
		log.Printf("couldn't create uid from id")
	}

	// get userid from access token
	token, _ := auth.GetBearerToken(r.Header)
	userid, err := auth.ValidateJWT(token, cfg.secretPhrase)
	if err != nil {
		log.Printf("token is invalid: %s", tokenid)
		w.WriteHeader(401)
		return
	}

	c, err := cfg.queries.GetVideo(uid)
	if err != nil {
		log.Printf("Error retrieving video by id: %s", err)
		w.WriteHeader(401)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		http.Error(w, "Error retrieving the file", http.StatusBadRequest)
		return
	}
	defer file.Close()

}
