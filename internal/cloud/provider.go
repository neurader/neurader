package cloud

// Uploader defines how we ship logs to the cloud
type Uploader interface {
    Upload(localPath string, targetName string) error
}
