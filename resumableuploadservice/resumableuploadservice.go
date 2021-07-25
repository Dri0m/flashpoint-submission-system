package resumableuploadservice

import (
	"bytes"
	"fmt"
	"github.com/boltdb/bolt"
	"io"
	"time"
)

type ResumableUploadService struct {
	db *bolt.DB
}

func Connect(dbName string) (*ResumableUploadService, error) {
	db, err := bolt.Open(dbName, 0600, &bolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		return nil, err
	}

	rsu := &ResumableUploadService{
		db: db,
	}
	return rsu, nil
}

// Close stops what needs to be stopped
func (rsu *ResumableUploadService) Close() {
	rsu.db.Close()
}

// PutChunk stores chunk, overwrites if exists.
func (rsu *ResumableUploadService) PutChunk(uid int64, fileID string, chunkNumber uint64, chunk []byte) error {
	if len(fileID) == 0 || chunkNumber == 0 || len(chunk) == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(uid, fileID)
	chunkDataKey := getChunkDataKey(uid, fileID, chunkNumber)
	chunkSizeKey := getChunkSizeKey(uid, fileID, chunkNumber)

	return rsu.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(fileBucketName)
		if err != nil {
			return err
		}

		if err := b.Put(chunkDataKey, chunk); err != nil {
			return err
		}
		if err := b.Put(chunkSizeKey, itob(uint64(len(chunk)))); err != nil {
			return err
		}

		return nil
	})
}

// TestChunk returns true if the chunk is already received
func (rsu *ResumableUploadService) TestChunk(uid int64, fileID string, chunkNumber uint64) (bool, error) {
	if len(fileID) == 0 || chunkNumber == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(uid, fileID)
	chunkDataKey := getChunkDataKey(uid, fileID, chunkNumber)
	isReceived := false

	err := rsu.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(fileBucketName)
		if b == nil {
			return nil
		}

		if chunk := b.Get(chunkDataKey); chunk != nil {
			isReceived = true
		}

		return nil
	})

	return isReceived, err
}

// IsUploadFinished compares the total size of stored chunks a to provided size
func (rsu *ResumableUploadService) IsUploadFinished(uid int64, fileID string, expectedSize int64) (bool, error) {
	if len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(uid, fileID)

	var actualSize uint64

	err := rsu.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(fileBucketName)
		if b == nil {
			return fmt.Errorf("bucket does not exist")
		}

		c := b.Cursor()
		prefix := getFileSizePrefix(uid, fileID)

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			actualSize += btoi(v)
		}

		return nil
	})

	if err != nil {
		return false, nil
	}

	return actualSize == uint64(expectedSize), err
}

// DeleteFile deletes the whole file bucket
func (rsu *ResumableUploadService) DeleteFile(uid int64, fileID string) error {
	if len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(uid, fileID)

	return rsu.db.Update(func(tx *bolt.Tx) error {
		return tx.DeleteBucket(fileBucketName)
	})
}

type fileReadCloser struct {
	uid                int64
	fileID             string
	rsu                *ResumableUploadService
	currentChunkNumber uint64
	currentChunkData   []byte
	currentChunkOffset int
	chunkCount         uint64
}

// NewFileReader returns a reader that reconstructs the file from the chunks on the fly. It does not check if the file is complete.
func (rsu *ResumableUploadService) NewFileReader(uid int64, fileID string) (io.ReadCloser, error) {
	if uid == 0 || len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	chunkCount, err := rsu.getChunkCount(uid, fileID)
	if err != nil {
		return nil, err
	}

	return &fileReadCloser{
		uid:                uid,
		fileID:             fileID,
		rsu:                rsu,
		currentChunkNumber: 0,
		currentChunkData:   nil,
		currentChunkOffset: 0,
		chunkCount:         chunkCount,
	}, nil
}

func (fr *fileReadCloser) Read(buf []byte) (n int, err error) {
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
func (fr *fileReadCloser) Close() error {
	return nil
}

func (rsu *ResumableUploadService) getChunkCount(uid int64, fileID string) (uint64, error) {
	if len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(uid, fileID)

	var chunkCount uint64 = 0

	err := rsu.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(fileBucketName)
		if b == nil {
			return fmt.Errorf("bucket does not exist")
		}

		c := b.Cursor()
		prefix := getFileDataPrefix(uid, fileID)

		for k, _ := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			chunkCount++
		}

		return nil
	})

	if err != nil {
		return 0, nil
	}

	return chunkCount, err
}

// getChunk returns chunk
func (rsu *ResumableUploadService) getChunk(uid int64, fileID string, chunkNumber uint64) ([]byte, error) {
	if len(fileID) == 0 || chunkNumber == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(uid, fileID)
	chunkDataKey := getChunkDataKey(uid, fileID, chunkNumber)

	var result []byte

	err := rsu.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(fileBucketName)
		if b == nil {
			return nil
		}

		chunk := b.Get(chunkDataKey)
		if chunk == nil {
			return nil
		}

		result = make([]byte, len(chunk))
		copy(result, chunk)

		return nil
	})

	return result, err
}
