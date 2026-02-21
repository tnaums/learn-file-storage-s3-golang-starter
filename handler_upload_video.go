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

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid Content-Type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Invalid file type", nil)
		return
	}

	f, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(f.Name()) // clean up
	defer f.Close()

	if _, err = io.Copy(f, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error saving file", err)
		return
	}

	_, err := f.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error seeking start of file", err)
		return
	}

	fileKey := getAssetPath(mediaType)
	
	input := &s3.PutObjectInput{
		Bucket: aws.String(cfg.s3Bucket),
		Key:    aws.String(fileKey),
		Body:   f,
		ContentType: aws.String(mediaType),
	}

	_, err = cfg.s3Client.PutObject(r.Context(), input)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error putting object in bucket", err)
		return
	}

}
