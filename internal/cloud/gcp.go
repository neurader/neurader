package cloud

import (
    "context"
    "cloud.google.com/go/storage"
    "io"
    "os"
)

type GCSUploader struct {
    BucketName string
}

func (g *GCSUploader) Upload(localPath string, targetName string) error {
    ctx := context.Background()
    client, _ := storage.NewClient(ctx)
    defer client.Close()

    f, _ := os.Open(localPath)
    defer f.Close()

    wc := client.Bucket(g.BucketName).Object(targetName).NewWriter(ctx)
    if _, err := io.Copy(wc, f); err != nil {
        return err
    }
    return wc.Close()
}
