package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
)

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(base)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

type aspectRatio struct {
	Streams []struct {
		DisplayAspectRatio string `json:"display_aspect_ratio"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	buf := bytes.NewBuffer(make([]byte, 0))
	cmd.Stdout = buf

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var ar aspectRatio
	err = json.Unmarshal(buf.Bytes(), &ar)
	if err != nil {
		return "", err
	}

	widthHeight := strings.Split(ar.Streams[0].DisplayAspectRatio, ":")
	if len(widthHeight) != 2 {
		return "", errors.New("incorrect display_aspect_ratio format")
	}

	l, err := strconv.ParseFloat(widthHeight[0], 64)
	if err != nil {
		return "", err
	}

	r, err := strconv.ParseFloat(widthHeight[1], 64)
	if err != nil {
		return "", err
	}

	width := int(math.Round(l))
	height := int(math.Round(r))

	if width == 16 && height == 9 {
		return "landscape", nil
	} else if width == 9 && height == 16 {
		return "portrait", nil
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outPath := filePath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outPath)
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return outPath, nil
}

// func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
// 	presignClient := s3.NewPresignClient(s3Client)
// 	presignedURL, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
// 		Bucket: aws.String(bucket),
// 		Key:    aws.String(key),
// 	}, s3.WithPresignExpires(expireTime))

// 	if err != nil {
// 		return "", err
// 	}
// 	return presignedURL.URL, nil
// }

// func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
// 	if video.VideoURL == nil {
// 		return video, nil
// 	}

// 	parts := strings.Split(*video.VideoURL, ",")
// 	if len(parts) != 2 {
// 		return video, errors.New("invalid video url")
// 	}
// 	bucket := parts[0]
// 	key := parts[1]

// 	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 5*time.Minute)
// 	if err != nil {
// 		return video, err
// 	}

// 	video.VideoURL = &presignedURL
// 	return video, nil
// }
