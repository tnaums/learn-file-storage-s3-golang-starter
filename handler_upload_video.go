package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"

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
	aspectRatio, err := getVideoAspectRatio(f.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error finding aspect ratio", err)
		return
	}
	fileKey := getAssetPath(mediaType)
	if aspectRatio == "16:9" {
		fileKey = "landscape/" + fileKey
	} else if aspectRatio == "9:16" {
		fileKey = "portrait/" + fileKey
	} else {
		fileKey = "other/" + fileKey
	}

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

func processVideoForFastStart(filePath string) (string, error) {
	return "", nil
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	ffprobeStruct := Ffprobe{}
	err = json.Unmarshal(out.Bytes(), &ffprobeStruct)
	if err != nil {
		fmt.Println("Error in JSON unmarshalling from json marshalled object:", err)
		return "", err
	}
	videoWidth := ffprobeStruct.Streams[0].Width
	videoHeight := ffprobeStruct.Streams[0].Height
	ratio := videoWidth / videoHeight
	if ratio == int(16)/int(9) {
		return "16:9", nil
	}
	if ratio == int(9)/int(16) {
		return "9:16", nil
	}
	return "other", nil
}

type Ffprobe struct {
	Streams []struct {
		Index              int    `json:"index"`
		CodecName          string `json:"codec_name,omitempty"`
		CodecLongName      string `json:"codec_long_name,omitempty"`
		Profile            string `json:"profile,omitempty"`
		CodecType          string `json:"codec_type"`
		CodecTagString     string `json:"codec_tag_string"`
		CodecTag           string `json:"codec_tag"`
		Width              int    `json:"width,omitempty"`
		Height             int    `json:"height,omitempty"`
		CodedWidth         int    `json:"coded_width,omitempty"`
		CodedHeight        int    `json:"coded_height,omitempty"`
		ClosedCaptions     int    `json:"closed_captions,omitempty"`
		HasBFrames         int    `json:"has_b_frames,omitempty"`
		SampleAspectRatio  string `json:"sample_aspect_ratio,omitempty"`
		DisplayAspectRatio string `json:"display_aspect_ratio,omitempty"`
		PixFmt             string `json:"pix_fmt,omitempty"`
		Level              int    `json:"level,omitempty"`
		ColorRange         string `json:"color_range,omitempty"`
		ColorSpace         string `json:"color_space,omitempty"`
		ColorTransfer      string `json:"color_transfer,omitempty"`
		ColorPrimaries     string `json:"color_primaries,omitempty"`
		ChromaLocation     string `json:"chroma_location,omitempty"`
		Refs               int    `json:"refs,omitempty"`
		IsAvc              string `json:"is_avc,omitempty"`
		NalLengthSize      string `json:"nal_length_size,omitempty"`
		RFrameRate         string `json:"r_frame_rate"`
		AvgFrameRate       string `json:"avg_frame_rate"`
		TimeBase           string `json:"time_base"`
		StartPts           int    `json:"start_pts"`
		StartTime          string `json:"start_time"`
		DurationTs         int    `json:"duration_ts"`
		Duration           string `json:"duration"`
		BitRate            string `json:"bit_rate,omitempty"`
		BitsPerRawSample   string `json:"bits_per_raw_sample,omitempty"`
		NbFrames           string `json:"nb_frames"`
		Disposition        struct {
			Default         int `json:"default"`
			Dub             int `json:"dub"`
			Original        int `json:"original"`
			Comment         int `json:"comment"`
			Lyrics          int `json:"lyrics"`
			Karaoke         int `json:"karaoke"`
			Forced          int `json:"forced"`
			HearingImpaired int `json:"hearing_impaired"`
			VisualImpaired  int `json:"visual_impaired"`
			CleanEffects    int `json:"clean_effects"`
			AttachedPic     int `json:"attached_pic"`
			TimedThumbnails int `json:"timed_thumbnails"`
		} `json:"disposition"`
		Tags struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			VendorID    string `json:"vendor_id"`
			Encoder     string `json:"encoder"`
			Timecode    string `json:"timecode"`
		} `json:"tags,omitempty"`
		SampleFmt     string `json:"sample_fmt,omitempty"`
		SampleRate    string `json:"sample_rate,omitempty"`
		Channels      int    `json:"channels,omitempty"`
		ChannelLayout string `json:"channel_layout,omitempty"`
		BitsPerSample int    `json:"bits_per_sample,omitempty"`
		Tags0         struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			VendorID    string `json:"vendor_id"`
		} `json:"tags,omitempty"`
		Tags1 struct {
			Language    string `json:"language"`
			HandlerName string `json:"handler_name"`
			Timecode    string `json:"timecode"`
		} `json:"tags,omitempty"`
	} `json:"streams"`
}
