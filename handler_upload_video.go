package main

import (
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
	userJwt, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		log.Printf("token is invalid: %s", token)
		w.WriteHeader(401)
		return
	}

	c, err := cfg.db.GetVideo(uid)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Error retrieving video by id: %s", err)
		return
	}

	if userJwt != c.UserID {
		respondWithError(w, http.StatusUnauthorized, "user does not have access to video", nil)
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

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error seeking start of file", err)
		return
	}

	fileKey := getAssetPath(mediaType)

	input := &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(fileKey),
		Body:        f,
		ContentType: aws.String(mediaType),
	}

	_, err = cfg.s3Client.PutObject(r.Context(), input)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error putting object in bucket", err)
		return
	}

	newUrl := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, fileKey)
	fmt.Printf("New url is: %s\n\n\n", newUrl)
	c.VideoURL = &newUrl
	err = cfg.db.UpdateVideo(c)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, c)
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	//	cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}	
	return "", nil
}
