package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	allowedMIME := "video/mp4"
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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Failed to locate video", err)
		return
	}
	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "You are not the video owner", nil)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// one gigabyte
	const maxMemory = 1 << 30

	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)
	if err = r.ParseForm(); err != nil {
		if _, ok := err.(*http.MaxBytesError); ok {
			respondWithError(w, http.StatusRequestEntityTooLarge, "File too large for upload", err)
			return
		} else {
			respondWithError(w, http.StatusBadRequest, "Failed to parse request", err)
			return
		}
	}

	f, videoHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not find video", err)
		return
	}
	defer f.Close()

	mediaType, _, err := mime.ParseMediaType(videoHeader.Header.Get("Content-Type"))
	if mediaType != allowedMIME {
		respondWithError(w, http.StatusUnsupportedMediaType, "Video must be an mp4", err)
		return
	}

	temp, err := os.CreateTemp("", uuid.NewString()+".mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	_, err = io.Copy(temp, f)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	_, err = temp.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	processedFilePath, err := processVideoForFastStart(temp.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	tempProcessed, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}
	defer os.Remove(processedFilePath)
	defer tempProcessed.Close()

	var prefix string
	prefix, err = getVideoAspectRatio(tempProcessed.Name())
	if err != nil {
		fmt.Println(err.Error())
		prefix = "other"
	}

	key := filepath.Join(prefix, getAssetPath(mediaType))
	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(key),
		Body:        tempProcessed,
		ContentType: aws.String(mediaType),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "", err)
		return
	}

	// presigning the url already includes the amazomaws url so no longer need to add that stuff
	// bucketURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
	// bucketAndKey := fmt.Sprintf("%s,%s", cfg.s3Bucket, bucketURL)

	// url storage for the presigning step. split on the comma character to get the bucket and key value
	// url := fmt.Sprintf("%s,%s", cfg.s3Bucket, key)

	url := "https://" + filepath.Join(cfg.s3CfDistribution, key)
	video.VideoURL = &url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	// video, err = cfg.dbVideoToSignedVideo(video)
	// if err != nil {
	// 	respondWithError(w, http.StatusInternalServerError, "Couldn't generate presigned URL", err)
	// 	return
	// }

	respondWithJSON(w, http.StatusOK, video)
}
