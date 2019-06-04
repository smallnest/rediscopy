package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strconv"
)

const (
	SIMPLE_STRING = '+'
	BULK_STRING   = '$'
	INTEGER       = ':'
	ARRAY         = '*'
	ERROR         = '-'
)

var (
	ErrInvalidSyntax = errors.New("resp: invalid syntax")
)

// RESPReader reads the raw reply from redis server.
type RESPReader struct {
	*bufio.Reader
}

// NewRESPReader creates a new reader with the given buffer size.
func NewRESPReader(reader io.Reader, bufSize int) *RESPReader {
	return &RESPReader{
		Reader: bufio.NewReaderSize(reader, bufSize),
	}
}

// ReadRaw reads the raw byte slice.
func (r *RESPReader) ReadRaw() ([]byte, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	switch line[0] {
	case SIMPLE_STRING, INTEGER, ERROR:
		return line, nil
	case BULK_STRING:
		return r.readBulkString(line)
	case ARRAY:
		return r.readArray(line)
	default:
		return nil, ErrInvalidSyntax
	}
}

// readLine reads a line which contains "\r\n"
func (r *RESPReader) readLine() (line []byte, err error) {
	line, err = r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}

	if len(line) > 1 && line[len(line)-2] == '\r' {
		return line, nil
	}

	// Line was too short or \n wasn't preceded by \r.
	return nil, ErrInvalidSyntax
}

func (r *RESPReader) readBulkString(line []byte) ([]byte, error) {
	count, err := r.getCount(line)
	if err != nil {
		return nil, err
	}
	if count == -1 {
		return line, nil
	}

	buf := make([]byte, len(line)+count+2)
	copy(buf, line)
	_, err = io.ReadFull(r, buf[len(line):])
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (r *RESPReader) getCount(line []byte) (int, error) {
	end := bytes.IndexByte(line, '\r')
	return strconv.Atoi(string(line[1:end]))
}

func (r *RESPReader) readArray(line []byte) ([]byte, error) {
	// Get number of array elements.
	count, err := r.getCount(line)
	if err != nil {
		return nil, err
	}

	// Read `count` number of RESP objects in the array.
	for i := 0; i < count; i++ {
		buf, err := r.ReadRaw()
		if err != nil {
			return nil, err
		}
		line = append(line, buf...)
	}

	return line, nil
}
