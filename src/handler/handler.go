package handler

import (
	"bytes"
	"conf"
	"errors"
	"fmt"
	"io"
	"logger"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"
)

func UploadNodeLogFileTask() {
	t := time.Now()
	n := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	d := n.Sub(t)
	if d < 0 {
		n = n.Add(24 * time.Hour)
		d = n.Sub(t)
	}
	for {
		time.Sleep(d)
		d = 24 * time.Hour
		err := sendHTTPLogFile()
		if err != nil {
			logger.Error("Can't upload node log file " + err.Error())
		} else {
			logger.Info("uploading node log file was accepted by daily task")
		}
	}
}

func sendHTTPLogFile() (err error) {
	var bytesBuffer bytes.Buffer
	writer := multipart.NewWriter(&bytesBuffer)
	var fileWriter io.Writer
	var fileReader io.Reader

	fileReader, err = os.Open(conf.Params.Handler.NodeDirPath + "operations.log")
	if err != nil {
		return err
	}

	if uploadedFile, ok := fileReader.(io.Closer); ok {
		defer uploadedFile.Close()
	}
	if uploadedFile, ok := fileReader.(*os.File); ok {
		if fileWriter, err = writer.CreateFormFile("file", uploadedFile.Name()); err != nil {
			return err
		}
	} else {
		return err
	}
	if _, err := io.Copy(fileWriter, fileReader); err != nil {
		return err
	}
	writer.Close()

	url := fmt.Sprint(conf.Params.Service.URL, "/api/v1/log/")
	req, err := http.NewRequest("POST", url, &bytesBuffer)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return errors.New("Wrong http response " + strconv.Itoa(res.StatusCode))
	}
	return nil
}

// Shortcut method for the errors wrapping.
func wrap(message string, err error) error {
	return errors.New(message + " -> " + err.Error())
}
