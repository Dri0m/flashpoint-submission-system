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
	"strings"
	"time"
)

// https://stackoverflow.com/a/31832326
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
	letterBytes   = "abcdefghijklmnopqrstuvwxyz0123456789"
)

var src = rand.NewSource(time.Now().UnixNano())

// RandomString returns random alphanumeric string, fast but not crypto safe
func RandomString(n int) string {
	sb := strings.Builder{}
	sb.Grow(n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
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
	return fmt.Sprintf("https://cdn.discordapp.com/avatars/%d/%s", uid, avatar)
}

func LogIfErr(ctx context.Context, err error) {
	if err != nil {
		LogCtx(ctx).Error(err)
	}
}

// UploadFile POSTs a given file to a given URL via multipart writer and returns the response body if OK
func UploadFile(ctx context.Context, url string, filePath string) ([]byte, error) {
	LogCtx(ctx).WithField("filepath", filePath).Debug("opening file for upload")
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	client := http.Client{}
	// Prepare a form that you will submit to that URL.
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	var fw io.Writer

	if fw, err = w.CreateFormFile("file", f.Name()); err != nil {
		return nil, err
	}

	LogCtx(ctx).WithField("filepath", filePath).Debug("copying file into multipart writer")
	if _, err = io.Copy(fw, f); err != nil {
		return nil, err
	}

	// Don't forget to close the multipart writer.
	// If you don't close it, your request will be missing the terminating boundary.
	w.Close()

	// Now that you have a form, you can submit it to your handler.
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return nil, err
	}
	// Don't forget to set the content type, this will contain the boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Submit the request
	LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("uploading file")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	LogCtx(ctx).WithField("url", url).WithField("filepath", filePath).Debug("response OK")

	return bodyBytes, nil
}

func FormatLike(s string) string {
	return "%" + strings.Replace(strings.Replace(s, "%", `\%`, -1), "_", `\_`, -1) + "%"
}

func WriteTarball(w io.Writer, filePaths []string) error {
	tarWriter := tar.NewWriter(w)
	defer tarWriter.Close()

	for _, filePath := range filePaths {
		err := addFileToTarWriter(filePath, tarWriter)
		if err != nil {
			return fmt.Errorf("add file to tar: %w", err.Error())
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
