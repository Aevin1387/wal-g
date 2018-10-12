package walg

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
)

// S3TarBall represents a tar file that is
// going to be uploaded to S3.
type S3TarBall struct {
	trim             string
	backupName       string
	partCount        int
	size             int64
	writeCloser      io.WriteCloser
	tarWriter        *tar.Writer
	tarUploader      *TarUploader
	Lsn              *uint64
	IncrementFromLsn *uint64
	IncrementFrom    string
	Files            BackupFileList
}

// SetUp creates a new tar writer and starts upload to S3.
// Upload will block until the tar file is finished writing.
// If a name for the file is not given, default name is of
// the form `part_....tar.[Compressor file extension]`.
func (tarBall *S3TarBall) SetUp(crypter Crypter, names ...string) {
	if tarBall.tarWriter == nil {
		var name string
		if len(names) > 0 {
			name = names[0]
		} else {
			name = fmt.Sprintf("part_%0.3d.tar.%v", tarBall.partCount, tarBall.FileExtension())
		}
		writeCloser := tarBall.StartUpload(name, crypter)

		tarBall.writeCloser = writeCloser
		tarBall.tarWriter = tar.NewWriter(writeCloser)
	}
}

// CloseTar closes the tar writer, flushing any unwritten data
// to the underlying writer before also closing the underlying writer.
func (tarBall *S3TarBall) CloseTar() error {
	err := tarBall.tarWriter.Close()
	if err != nil {
		return errors.Wrap(err, "CloseTar: failed to close tar writer")
	}

	err = tarBall.writeCloser.Close()
	if err != nil {
		return errors.Wrap(err, "CloseTar: failed to close underlying writer")
	}
	Logger.Infof("Finished writing part %d.\n", tarBall.partCount)
	return nil
}
func (tarBall *S3TarBall) AwaitUploads() {
	tarBall.tarUploader.waitGroup.Wait()
	if !tarBall.tarUploader.Success {
		Logger.Fatal("Unable to complete uploads")
	}
}

// StartUpload creates a compressing writer and runs upload in the background once
// a compressed tar member is finished writing.
func (tarBall *S3TarBall) StartUpload(name string, crypter Crypter) io.WriteCloser {
	pipeReader, pipeWriter := io.Pipe()
	tarUploader := tarBall.tarUploader

	path := tarUploader.server + BaseBackupsPath + tarBall.backupName + "/tar_partitions/" + name
	input := tarUploader.createUploadInput(path, NewNetworkLimitReader(pipeReader))

	Logger.Infof("Starting part %d ...", tarBall.partCount)

	tarUploader.waitGroup.Add(1)
	go func() {
		defer tarUploader.waitGroup.Done()

		err := tarUploader.upload(input, path)
		if compressingError, ok := err.(CompressingPipeWriterError); ok {
			Logger.Errorf("FATAL: Could not upload '%s' due to compression error\n%+v", path, compressingError)
		}
		if err != nil {
			Logger.Errorf("FATAL upload: could not upload '%s'\n%v", path, err)
		}
	}()

	if crypter.IsUsed() {
		encryptedWriter, err := crypter.Encrypt(pipeWriter)

		if err != nil {
			Logger.Fatalf("upload: encryption error %s", err)
		}

		return &CascadeWriteCloser{tarUploader.compressor.NewWriter(encryptedWriter), &CascadeWriteCloser{encryptedWriter, pipeWriter}}
	}

	return &CascadeWriteCloser{tarUploader.compressor.NewWriter(pipeWriter), pipeWriter}
}

// Trim suffix
func (tarBall *S3TarBall) Trim() string { return tarBall.trim }

// Size accumulated in this tarball
func (tarBall *S3TarBall) Size() int64 { return tarBall.size }

// AddSize to total Size
func (tarBall *S3TarBall) AddSize(i int64) { tarBall.size += i }

func (tarBall *S3TarBall) TarWriter() *tar.Writer { return tarBall.tarWriter }

func (tarBall *S3TarBall) FileExtension() string {
	return tarBall.tarUploader.compressor.FileExtension()
}

// Finish writes a .json file description and uploads it with the
// the backup name. Finish will wait until all tar file parts
// have been uploaded. The json file will only be uploaded
// if all other parts of the backup are present in S3.
// an alert is given with the corresponding error.
func (tarBall *S3TarBall) Finish(sentinelDto *S3TarBallSentinelDto) error {
	var err error
	name := tarBall.backupName + "_backup_stop_sentinel.json"
	tarUploader := tarBall.tarUploader

	tarUploader.Finish()

	//If other parts are successful in uploading, upload json file.
	if tarUploader.Success && sentinelDto != nil {
		sentinelDto.UserData = GetSentinelUserData()
		dtoBody, err := json.Marshal(*sentinelDto)
		if err != nil {
			return err
		}
		path := tarUploader.server + BaseBackupsPath + name
		input := &s3manager.UploadInput{
			Bucket:       aws.String(tarUploader.bucket),
			Key:          aws.String(path),
			Body:         bytes.NewReader(dtoBody),
			StorageClass: aws.String(tarUploader.StorageClass),
		}

		if tarUploader.ServerSideEncryption != "" {
			input.ServerSideEncryption = aws.String(tarUploader.ServerSideEncryption)

			if tarUploader.SSEKMSKeyId != "" {
				// Only aws:kms implies sseKmsKeyId, checked during validation
				input.SSEKMSKeyId = aws.String(tarUploader.SSEKMSKeyId)
			}
		}

		tarUploader.waitGroup.Add(1)
		go func() {
			defer tarUploader.waitGroup.Done()

			e := tarUploader.upload(input, path)
			if e != nil {
				Logger.Errorf("upload: could not upload '%s'", path)
				Logger.Fatal("S3TarBall Finish: json failed to upload")
			}
		}()

		tarUploader.Finish()
	} else {
		Logger.Infof("Uploaded %d compressed tar Files.", tarBall.partCount)
		Logger.Warnf("Sentinel was not uploaded %v", name)
		return errors.New("Sentinel was not uploaded due to timeline change during backup")
	}

	if err == nil && tarUploader.Success {
		Logger.Infof("Uploaded %d compressed tar Files.", tarBall.partCount)
	}
	return err
}
