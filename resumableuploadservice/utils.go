package resumableuploadservice

import (
	"encoding/binary"
	"fmt"
)

func itob(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func btoi(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

func getFileBucketName(fileID string) []byte {
	return []byte(fileID)
}

func getChunkDataKey(fileID string, chunkNumber uint64) []byte {
	return []byte(fmt.Sprintf("data-%s-%d", fileID, chunkNumber))
}

func getChunkSizeKey(fileID string, chunkNumber uint64) []byte {
	return []byte(fmt.Sprintf("size-%s-%d", fileID, chunkNumber))
}

func getFileSizePrefix(fileID string) []byte {
	return []byte(fmt.Sprintf("size-%s", fileID))
}
