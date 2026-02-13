package main

import (
	"fmt"
	"io"
	"net/http"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	// "thumbnail" should match the HTML form input name
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()
	the_string_i_want := header.Header.Get("content-type")

	//	mediaType := r.Header.Get("content-type")
	b, _ := io.ReadAll(file)

	videoData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to find video in database", err)
		return
	}

	if userID != videoData.UserID {
		respondWithError(w, http.StatusBadRequest, "You do not own that video", err)
		return
	}
	
	newThumbnail := thumbnail{
		data:      b,
		mediaType: the_string_i_want,
	}
	videoThumbnails[videoID] = newThumbnail
	newURL := fmt.Sprintf("http://localhost:%s/api/thumbnails/%s", cfg.port, videoID)
	videoData.ThumbnailURL = &newURL
	err = cfg.db.UpdateVideo(videoData)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to update video with new url", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoData)
}
