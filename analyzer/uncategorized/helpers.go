package common

import (
	"io"

	"github.com/oasisprotocol/nexus/log"
)

func CloseOrLog(c io.Closer, logger *log.Logger) {
	if err := c.Close(); err != nil {
		logger.Warn("error closing", "closer", c, "err", err)
	}
}

func WriteOrLog(w io.Writer, p []byte, logger *log.Logger) {
	if _, err := w.Write(p); err != nil {
		logger.Warn("error writing", "writer", w, "err", err)
	}
}
