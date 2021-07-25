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

func getFileBucketName(uid int64, fileID string) []byte {
	return []byte(fmt.Sprintf("%d-%s", uid, fileID))
}

func getChunkDataKey(uid int64, fileID string, chunkNumber uint64) []byte {
	return []byte(fmt.Sprintf("data-%d-%s-%d", uid, fileID, chunkNumber))
}

func getChunkSizeKey(uid int64, fileID string, chunkNumber uint64) []byte {
	return []byte(fmt.Sprintf("size-%d-%s-%d", uid, fileID, chunkNumber))
}

func getFileSizePrefix(uid int64, fileID string) []byte {
	return []byte(fmt.Sprintf("size-%d-%s", uid, fileID))
}

func getFileDataPrefix(uid int64, fileID string) []byte {
	return []byte(fmt.Sprintf("data-%d-%s", uid, fileID))
}
