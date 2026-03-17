package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadFile saves an uploaded file either to Leapcell Object Storage
// (when OBJ_ENDPOINT is set) or to local disk (development).
//
// This is the single point in the codebase that decides where files go.
// All other code — handlers, templates, models — is unchanged regardless
// of which storage backend is active.
//
// Returns the public URL to store in the database, or "" on failure.
func UploadFile(file multipart.File, originalFilename string) string {
	// Prefix with Unix timestamp to guarantee unique filenames
	filename := fmt.Sprintf("%d-%s", time.Now().Unix(), originalFilename)

	if os.Getenv("OBJ_ENDPOINT") == "" {
		// No object storage configured — save to local disk
		// This is the default in development
		return saveLocally(file, filename)
	}

	return uploadToLeapcell(file, filename)
}

// saveLocally writes the file to static/uploads/ on disk.
// Used in local development only.
func saveLocally(file multipart.File, filename string) string {
	if err := os.MkdirAll("static/uploads", os.ModePerm); err != nil {
		log.Println("storage: could not create uploads dir:", err)
		return ""
	}

	dst, err := os.Create("static/uploads/" + filename)
	if err != nil {
		log.Println("storage: could not create file:", err)
		return ""
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Println("storage: could not write file:", err)
		return ""
	}

	return "/static/uploads/" + filename
}

// uploadToLeapcell uploads a file to Leapcell Object Storage via the
// S3-compatible API and returns the public CDN URL.
//
// Required environment variables:
//
//	OBJ_ENDPOINT  — https://objstorage.leapcell.io
//	OBJ_BUCKET    — your bucket name
//	OBJ_ACCESS_KEY — your access key ID
//	OBJ_SECRET_KEY — your secret access key
//	OBJ_CDN_URL   — https://subdomain.leapcellobj.com/bucket-name
func uploadToLeapcell(file multipart.File, filename string) string {
	endpoint := os.Getenv("OBJ_ENDPOINT")
	bucket := os.Getenv("OBJ_BUCKET")
	accessKey := os.Getenv("OBJ_ACCESS_KEY")
	secretKey := os.Getenv("OBJ_SECRET_KEY")
	cdnBase := os.Getenv("OBJ_CDN_URL")

	if bucket == "" || accessKey == "" || secretKey == "" || cdnBase == "" {
		log.Println("storage: object storage env vars incomplete — falling back to local")
		return ""
	}

	// Read file into memory.
	// S3 PutObject needs an io.Reader that supports seeking or a
	// known content length; reading to []byte is the simplest approach.
	data, err := io.ReadAll(file)
	if err != nil {
		log.Println("storage: could not read upload:", err)
		return ""
	}

	// Build the S3 client pointed at Leapcell's endpoint.
	// region "us-east-1" is required by the AWS SDK but ignored by Leapcell.
	cfg := aws.Config{
		Region: "us-east-1",
		Credentials: credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		),
		BaseEndpoint: aws.String(endpoint),
	}

	client := s3.NewFromConfig(cfg)

	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		log.Println("storage: PutObject failed:", err)
		return ""
	}

	log.Printf("storage: uploaded %s to object storage\n", filename)

	// The CDN URL is what gets saved in the database and rendered in
	// <img src="..."> tags in templates.
	return cdnBase + "/" + filename
}
