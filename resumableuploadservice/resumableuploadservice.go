package resumableuploadservice

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/Dri0m/flashpoint-submission-system/utils"
)

type ResumableUploadService struct {
	path string
}

func (rsu *ResumableUploadService) getChunkFilename(uid int64, fileID string, chunkNumber int) string {

	if len(fileID) > 64 {
		fileID = fileID[:64]
	}

	return fmt.Sprintf("%s/data-%d-%s-%d", rsu.path, uid, fileID, chunkNumber)
}

func New(path string) (*ResumableUploadService, error) {
	err := os.MkdirAll(path, os.ModeDir)
	if err != nil {
		return nil, err
	}

	rsu := &ResumableUploadService{
		path: path,
	}
	return rsu, nil
}

// Close stops what needs to be stopped
func (rsu *ResumableUploadService) Close() {
}

// PutChunk stores chunk, overwrites if exists.
func (rsu *ResumableUploadService) PutChunk(uid int64, fileID string, chunkNumber int, chunk []byte) error {
	if uid == 0 || len(fileID) == 0 || chunkNumber == 0 || len(chunk) == 0 {
		panic("invalid arguments provided")
	}

	chunkFilename := rsu.getChunkFilename(uid, fileID, chunkNumber)
	chunkFilenameTmp := chunkFilename + ".part"

	if err := os.WriteFile(chunkFilenameTmp, chunk, 0644); err != nil {
		return err
	}
	if err := os.Rename(chunkFilenameTmp, chunkFilename); err != nil {
		return err
	}

	return nil
}

// TestChunk returns true if the chunk is already received
func (rsu *ResumableUploadService) TestChunk(uid int64, fileID string, chunkNumber int, chunkSize int64) (bool, error) {
	if uid == 0 || len(fileID) == 0 || chunkNumber == 0 || chunkSize == 0 {
		panic("invalid arguments provided")
	}

	chunkFilename := rsu.getChunkFilename(uid, fileID, chunkNumber)

	if !utils.FileExists(chunkFilename) {
		return false, nil
	}

	fi, err := os.Stat(chunkFilename)
	if err != nil {
		return false, err
	}

	if fi.Size() != chunkSize {
		return false, nil
	}

	return true, nil
}

// IsUploadFinished compares the total size of stored chunks a to provided size
func (rsu *ResumableUploadService) IsUploadFinished(uid int64, fileID string, chunkCount int, expectedSize int64) (bool, error) {
	if uid == 0 || len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	var totalSize int64

	for i := 1; i <= chunkCount; i++ {
		chunkFilename := rsu.getChunkFilename(uid, fileID, i)

		if !utils.FileExists(chunkFilename) {
			return false, nil
		}

		fi, err := os.Stat(chunkFilename)
		if err != nil {
			return false, err
		}

		totalSize += fi.Size()
	}

	return totalSize == expectedSize, nil
}

// DeleteFile deletes the whole file bucket
func (rsu *ResumableUploadService) DeleteFile(uid int64, fileID string, chunkCount int) error {
	if uid == 0 || len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	for i := 1; i <= chunkCount; i++ {
		chunkFilename := rsu.getChunkFilename(uid, fileID, i)

		err := os.Remove(chunkFilename)
		if err != nil {
			return err
		}
	}

	return nil
}

type ReadCloserInformer interface {
	Read(buf []byte) (n int, err error)
	Close() error
	GetFractionRead() float32
}

type ReadCloserInformerProvider interface {
	GetReadCloserInformer() (ReadCloserInformer, error)
}

type fileReadCloserInformer struct {
	uid                int64
	fileID             string
	rsu                *ResumableUploadService
	currentChunkNumber int
	currentChunkData   []byte
	currentChunkOffset int
	chunkCount         int
}

// NewFileReader returns a reader that reconstructs the file from the chunks on the fly. It does not check if the file is complete.
func (rsu *ResumableUploadService) NewFileReader(uid int64, fileID string, chunkCount int) (ReadCloserInformer, error) {
	if uid == 0 || len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	return &fileReadCloserInformer{
		uid:                uid,
		fileID:             fileID,
		rsu:                rsu,
		currentChunkNumber: 0,
		currentChunkData:   nil,
		currentChunkOffset: 0,
		chunkCount:         chunkCount,
	}, nil
}

func (fr *fileReadCloserInformer) Read(buf []byte) (n int, err error) {
	// case 0: init the reader
	if fr.currentChunkNumber == 0 {
		fr.currentChunkNumber++
		fr.currentChunkData, err = fr.rsu.getChunk(fr.uid, fr.fileID, fr.currentChunkNumber)
		if err != nil {
			return
		}
	}

	for {
		// buffer full
		if len(buf[n:]) == 0 {
			return
		}

		remainingChunkBytes := len(fr.currentChunkData) - fr.currentChunkOffset

		// case 1: reader has more data available than the caller wants
		if remainingChunkBytes >= len(buf[n:]) {
			low := fr.currentChunkOffset
			high := fr.currentChunkOffset + len(buf[n:])
			copy(buf[n:], fr.currentChunkData[low:high])
			fr.currentChunkOffset = high
			n += high - low
			return
		}

		// case 2: caller wants more data than the reader currently has
		if remainingChunkBytes < len(buf[n:]) {
			// use up the rest of the chunk
			if remainingChunkBytes > 0 {
				low := fr.currentChunkOffset
				high := len(fr.currentChunkData)
				copy(buf[n:], fr.currentChunkData[low:high])
				fr.currentChunkOffset = high
				n += high - low
			}

			// fetch new chunk
			fr.currentChunkNumber++

			if fr.currentChunkNumber > fr.chunkCount {
				return n, io.EOF
			}

			fr.currentChunkOffset = 0
			fr.currentChunkData, err = fr.rsu.getChunk(fr.uid, fr.fileID, fr.currentChunkNumber)
			if err != nil {
				return
			}

			// no more chunks, terminate
			if fr.currentChunkData == nil {
				return n, io.EOF
			}
		}
	}
}

// Close is just a dummy close in case it's needed later
func (fr *fileReadCloserInformer) Close() error {
	return nil
}

func (fr *fileReadCloserInformer) GetFractionRead() float32 {
	return float32(fr.currentChunkNumber) / float32(fr.chunkCount)
}

// getChunk returns chunk
func (rsu *ResumableUploadService) getChunk(uid int64, fileID string, chunkNumber int) ([]byte, error) {
	if uid == 0 || len(fileID) == 0 || chunkNumber == 0 {
		panic("invalid arguments provided")
	}

	chunkFilename := rsu.getChunkFilename(uid, fileID, chunkNumber)

	if !utils.FileExists(chunkFilename) {
		return nil, fmt.Errorf("file %s does not exist", chunkFilename)
	}

	chunk, err := ioutil.ReadFile(chunkFilename)
	if err != nil {
		return nil, err
	}

	return chunk, nil
}
