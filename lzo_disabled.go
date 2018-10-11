// +build !lzo

package walg

import (
	"io"

	log "github.com/sirupsen/logrus"
)

func NewLzoReader(r io.Reader) (io.ReadCloser, error) {
	log.Fatal("lzo support not compiled into this WAL-G binary")
	return nil, nil
}

func NewLzoWriter(w io.Writer) io.WriteCloser {
	log.Fatal("lzo support not compiled into this WAL-G binary")
	return nil
}
