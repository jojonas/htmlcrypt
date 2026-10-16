package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
)

type header struct {
	Filename    string `json:"filename"`
	MimeType    string `json:"mimeType"`
	Compression string `json:"compression,omitempty"`
	Download    bool   `json:"download"`
	Size        int    `json:"size"`
}

func preparePlaintext(h *header, body []byte) ([]byte, error) {
	serializedHeader, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	headerSize := uint32(len(serializedHeader))

	var b bytes.Buffer
	b.Grow(4 + len(serializedHeader) + len(body))

	binary.Write(&b, binary.BigEndian, headerSize)
	b.Write(serializedHeader)
	b.Write(body)

	return b.Bytes(), nil
}
