package cloud

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
)

// GCSUploader handles all log archival to Google Cloud Storage
type GCSUploader struct {
	BucketName string
}

// Upload takes the raw log data and pushes it to the specified GCS bucket
func (u *GCSUploader) Upload(fileName string, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %v", err)
	}
	defer client.Close()

	// Create a writer for the bucket/object
	bucket := client.Bucket(u.BucketName)
	obj := bucket.Object(fileName)

	w := obj.NewWriter(ctx)
	
	// Set metadata for easier searching in GCS console
	w.ContentType = "text/plain"
	w.Metadata = map[string]string{
		"origin": "neurader-gke-daemon",
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write data to bucket: %v", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to close GCS writer: %v", err)
	}

	return nil
}
