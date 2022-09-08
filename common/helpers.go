package common

import (
	"fmt"
	"io"
)

func CloseOrLog(c io.Closer) {
	if err := c.Close(); err != nil {
		fmt.Printf("close: %v", err)
	}
}

func WriteOrLog(w io.Writer, p []byte) {
	if _, err := w.Write(p); err != nil {
		fmt.Printf("write: %v", err)
	}
}
