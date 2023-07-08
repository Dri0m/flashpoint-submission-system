package service

import (
	"archive/zip"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/database"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"hash/crc32"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type ZipIndexer struct {
	stopSignal   chan bool
	status       string
	error        error
	statusMutex  *sync.Mutex
	stopped      bool
	dataPacksDir string
	pool         *pgxpool.Pool
	wg           *sync.WaitGroup
	ctx          context.Context
}

func NewZipIndexer(pool *pgxpool.Pool, dataPacksDir string, l *logrus.Entry) ZipIndexer {
	var syncMutex sync.Mutex
	var wg sync.WaitGroup
	ctx := context.WithValue(context.Background(), utils.CtxKeys.Log, l)
	return ZipIndexer{
		make(chan bool),
		"...",
		nil,
		&syncMutex,
		true,
		dataPacksDir,
		pool,
		&wg,
		ctx,
	}
}

func (z *ZipIndexer) run() {
	defer z.wg.Done()
	// Create DAL
	pgdal := database.NewPostgresDAL(z.pool)

	for {
		select {
		case <-z.stopSignal:
			// Got stop signal, exit this loop
			z.stopped = true
			return
		default:
			// Fetch next data zip
			data, err := pgdal.IndexerGetNext(z.ctx)
			if err != nil {
				if err == pgx.ErrNoRows {
					// Wait 10 seconds and check again for a fresh data pack
					time.Sleep(10 * time.Second)
					continue
				} else {
					z.statusMutex.Lock()
					z.error = err
					utils.LogCtx(z.ctx).
						Error(err)
					z.stopped = true
					z.statusMutex.Unlock()
					return
				}
			} else {
				// Update status
				z.statusMutex.Lock()
				z.status = fmt.Sprintf("Indexing %s", data.GameID)
				utils.LogCtx(z.ctx).
					Debug(z.status)
				z.statusMutex.Unlock()
			}
			err = func() error {
				// Find data path
				newBase := fmt.Sprintf("%s-%d%s", data.GameID, data.DateAdded.UnixMilli(), ".zip")
				filePath := path.Join(z.dataPacksDir, newBase)
				// If zip doesn't exist locally, let the error return handle marking it as failure
				_, err = os.Stat(filePath)
				if err != nil {
					return err
				}
				// Hash the file
				err = func() error {
					zipReader, err := zip.OpenReader(filePath)
					if err != nil {
						return err
					}
					defer zipReader.Close()

					for _, file := range zipReader.File {
						if strings.HasSuffix(file.Name, "/") || file.Name == "content.json" {
							// Directory or content.json, skip
							continue
						}

						// Open each file inside the zip
						fileReader, err := file.Open()
						if err != nil {
							return err
						}

						size := file.UncompressedSize64

						// Use a SHA256 hash.Hash as an io.Writer
						sha256hasher := sha256.New()
						sha1hasher := sha1.New()
						md5hasher := md5.New()
						crc32hasher := crc32.NewIEEE()

						multiWriter := io.MultiWriter(sha256hasher, sha1hasher, md5hasher, crc32hasher)
						_, err = io.Copy(multiWriter, fileReader)
						fileReader.Close()
						if err != nil {
							return err
						}

						cleanName := forceUTF8Compliant(file.Name)

						err = pgdal.IndexerInsert(z.ctx, crc32hasher.Sum(nil), md5hasher.Sum(nil), sha256hasher.Sum(nil),
							sha1hasher.Sum(nil), size, cleanName, data.GameID, data.DateAdded)
						if err != nil {
							return err
						}

					}

					// Print the game just indexed
					utils.LogCtx(z.ctx).
						Debug(fmt.Sprintf("Finished Indexing %s", data.GameID))

					return nil
				}()
				if err != nil {
					return err
				}
				return nil
			}()
			if err != nil {
				if os.IsNotExist(err) {
					// Mark as failure
					utils.LogCtx(z.ctx).
						Error(fmt.Sprintf("Index failure due to missing file %s", data.GameID))
					err = pgdal.IndexerMarkFailure(z.ctx, data.GameID, data.DateAdded)
					if err != nil {
						z.statusMutex.Lock()
						z.error = err
						utils.LogCtx(z.ctx).
							Error(err)
						z.stopped = true
						z.statusMutex.Unlock()
						return
					}
				} else {
					z.statusMutex.Lock()
					z.error = err
					utils.LogCtx(z.ctx).
						Error(err)
					z.stopped = true
					z.statusMutex.Unlock()
					return
				}
			}
		}
	}
}

func (z *ZipIndexer) Start() {
	z.statusMutex.Lock()
	defer z.statusMutex.Unlock()

	if !z.stopped {
		return
	}

	z.stopSignal = make(chan bool)
	z.status = "Starting..."
	z.stopped = false
	z.wg.Add(1)
	go z.run()
}

func (z *ZipIndexer) Stop() {
	if z.stopped {
		return
	}

	z.stopSignal <- true

	z.wg.Wait()
}

func (z *ZipIndexer) GetStatus() (string, error) {
	z.statusMutex.Lock()
	defer z.statusMutex.Unlock()

	return strings.Clone(z.status), z.error
}

func forceUTF8Compliant(str string) string {
	if utf8.ValidString(str) {
		return str
	}

	// If the string is not valid UTF-8, convert it to valid UTF-8.
	validBytes := make([]byte, 0, len(str))
	for i := 0; i < len(str); {
		r, size := utf8.DecodeRuneInString(str[i:])
		if r == utf8.RuneError {
			// Skip invalid runes
			i += size
			continue
		}
		validBytes = append(validBytes, str[i:i+size]...)
		i += size
	}

	return string(validBytes)
}
