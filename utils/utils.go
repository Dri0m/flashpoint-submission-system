package utils

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// https://stackoverflow.com/a/31832326
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	letterBytes   = "abcdefghijklmnopqrstuvwxyz0123456789"
)

type RealRandomString struct {
	src rand.Source
}

var MetadataMutex sync.Mutex

func NewRealRandomStringProvider() *RealRandomString {
	return &RealRandomString{
		src: rand.NewSource(time.Now().UnixNano()),
	}
}

func (r *RealRandomString) RandomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, r.src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = r.src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			sb.WriteByte(letterBytes[idx])
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return sb.String()
}

func FormatAvatarURL(uid int64, avatar string) string {
	if len(avatar) == 0 {
		return ""
	}
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s", uid, avatar)
}

func FormatLike(s string) string {
	return "%" + s + "%"
}

func WriteTarball(w io.Writer, filePaths []string) error {
	tarWriter := tar.NewWriter(w)
	defer tarWriter.Close()

	for _, filePath := range filePaths {
		err := addFileToTarWriter(filePath, tarWriter)
		if err != nil {
			return fmt.Errorf("add file to tar: %s", err.Error())
		}
	}

	return nil
}

func addFileToTarWriter(filePath string, tarWriter *tar.Writer) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    filePath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	err = tarWriter.WriteHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(tarWriter, file)
	if err != nil {
		return err
	}

	return nil
}

// Unpointify is for template
func Unpointify(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Convert size in bytes (123456789B) to a human readable string (Extensions B through EB)
func SizeToString(size int64) string {
	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB",
		float64(size)/float64(div), "kMGTPE"[exp])
}

func SplitMultilineText(s *string) []string {
	if s == nil {
		return nil
	}
	return strings.Split(*s, "\n")
}

// NewBucketLimiter creates a ticker channel that fills a bucket with one token every d and has a given capacity for burst usage
func NewBucketLimiter(d time.Duration, capacity int) (chan bool, *time.Ticker) {
	bucket := make(chan bool, capacity)
	ticker := time.NewTicker(d)
	go func() {
		for {
			<-ticker.C
			bucket <- true
		}
	}()
	return bucket, ticker
}

func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func CapitalizeASCII(s string) string {
	if len(s) == 0 {
		return s
	}

	result := strings.ToUpper(string(s[0]))
	result += s[1:]
	return result
}

func GetURL(url string) ([]byte, error) {
	var client = &http.Client{Timeout: 600 * time.Second}

	r, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("response not OK: %d", r.StatusCode)
	}

	var result bytes.Buffer

	_, err = io.Copy(&result, r.Body)
	if err != nil {
		return nil, err
	}

	return result.Bytes(), nil
}

func GetMemStats() *runtime.MemStats {
	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)
	return m
}

func BoolToString(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func UploadMultipartFile(ctx context.Context, url string, f io.Reader, filename string) ([]byte, error) {
	body, writer := io.Pipe()
	client := http.Client{Timeout: 86400 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	mwriter := multipart.NewWriter(writer)
	req.Header.Add("Content-Type", mwriter.FormDataContentType())

	errchan := make(chan error)

	LogCtx(ctx).WithField("url", url).Debug("uploading file")

	go func() {
		defer close(errchan)
		defer writer.Close()
		defer mwriter.Close()

		w, err := mwriter.CreateFormFile("file", filename)
		if err != nil {
			errchan <- err
			return
		}

		if written, err := io.Copy(w, f); err != nil {
			errchan <- fmt.Errorf("error copying %s (%d bytes written): %v", filename, written, err)
			return
		}

		if err := mwriter.Close(); err != nil {
			errchan <- err
			return
		}
	}()

	resp, err := client.Do(req)
	merr := <-errchan

	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}

	if err != nil || merr != nil {
		return nil, fmt.Errorf("http error: %v, multipart error: %v", err, merr)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check the response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upload to remote error: %s", string(bodyBytes))
	}

	LogCtx(ctx).WithField("url", url).Debug("response OK")

	return bodyBytes, nil
}

type ValueOnlyContext struct {
	context.Context
}

func (ValueOnlyContext) Deadline() (deadline time.Time, ok bool) {
	return
}
func (ValueOnlyContext) Done() <-chan struct{} {
	return nil
}
func (ValueOnlyContext) Err() error {
	return nil
}

func Int64Ptr(n int64) *int64 {
	return &n
}

func StrPtr(s string) *string {
	return &s
}

func NilTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
