package resumableuploadservice

import (
	"bytes"
	"fmt"
	"github.com/boltdb/bolt"
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
func (rsu *ResumableUploadService) PutChunk(fileID string, chunkNumber uint64, chunk []byte) error {
	if len(fileID) == 0 || chunkNumber == 0 || len(chunk) == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(fileID)
	chunkDataKey := getChunkDataKey(fileID, chunkNumber)
	chunkSizeKey := getChunkSizeKey(fileID, chunkNumber)

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
func (rsu *ResumableUploadService) TestChunk(fileID string, chunkNumber uint64) (bool, error) {
	if len(fileID) == 0 || chunkNumber == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(fileID)
	chunkDataKey := getChunkDataKey(fileID, chunkNumber)
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
func (rsu *ResumableUploadService) IsUploadFinished(fileID string, expectedSize uint64) (bool, error) {
	if len(fileID) == 0 {
		panic("invalid arguments provided")
	}

	fileBucketName := getFileBucketName(fileID)

	var actualSize uint64

	err := rsu.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(fileBucketName)
		if b == nil {
			return fmt.Errorf("bucket does not exist")
		}

		c := b.Cursor()
		prefix := getFileSizePrefix(fileID)

		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			actualSize += btoi(v)
		}

		return nil
	})

	if err != nil {
		return false, nil
	}

	return actualSize == expectedSize, err
}
