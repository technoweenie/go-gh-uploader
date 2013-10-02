package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

var (
	App         = filepath.Base(os.Args[0])
	FlagSet     = flag.NewFlagSet(App, flag.ExitOnError)
	Token       = FlagSet.String("token", "", "Your access token")
	Name        = FlagSet.String("name", "", "A custom file name for the asset.  Defaults to the local file's base name")
	ContentType = FlagSet.String("content_type", "", "The local file's content type")
)

func main() {
	asseturl := getAssetUrl()
	localfile, localstat := getLocalfile()

	file, err := os.Open(localfile)
	if err != nil {
		printUsageAndExit("Error opening %s: %s", localfile, err)
	}
	defer file.Close()

	if args := len(os.Args); args > 3 {
		FlagSet.Parse(os.Args[3:args])
	}

	if len(*Name) == 0 {
		*Name = localstat.Name()
	}

	q := asseturl.Query()
	q.Set("name", *Name)
	asseturl.RawQuery = q.Encode()

	if len(*ContentType) == 0 {
		*ContentType = "application/octet-stream"
	}

	fmt.Printf("Sending %s (%d bytes) to %s\n", localfile, localstat.Size(), asseturl.String())

	res := upload(asseturl, localstat, file)
	defer res.Body.Close()
	dec := json.NewDecoder(res.Body)
	if res.StatusCode == 201 {
		asset := &Asset{}
		dec.Decode(asset)
		fmt.Printf("Successfully uploaded to %s\n", asset.Url)
	} else {
		apierr := &ApiError{}
		dec.Decode(apierr)
		printUsageAndExit("%d: %s\nRequest ID: %s", res.StatusCode, apierr.Message, apierr.RequestId)
	}
}

type Asset struct {
	Url string `json:"url"`
}

type ApiError struct {
	Message          string            `json:"message"`
	RequestId        string            `json:"request_id,omitempty"`
	DocumentationUrl string            `json:"documentation_url,omitempty"`
	Errors           []ValidationError `json:"errors,omitempty"`
}

type ValidationError struct {
	Resource string `json:"resource"`
	Code     string `json:"code"`
	Field    string `json:"field"`
	Message  string `json:"message,omitempty"`
}

func getAssetUrl() *url.URL {
	if len(os.Args) < 2 {
		printUsageAndExit("No asset URL specified.")
	}
	uri, err := url.Parse(os.Args[1])
	if err != nil {
		printUsageAndExit("Invalid URL: %s", err)
	}

	normalizeUri(uri)

	return uri
}

func getLocalfile() (string, os.FileInfo) {
	if len(os.Args) < 3 {
		printUsageAndExit("No local file specified.")
	}

	localfile := os.Args[2]
	stat, err := os.Stat(localfile)
	if err != nil {
		printUsageAndExit("Error opening local file: %s", err)
	}

	return localfile, stat
}

func upload(asseturl *url.URL, stat os.FileInfo, reader io.Reader) *http.Response {
	req, err := http.NewRequest("POST", asseturl.String(), reader)
	if err != nil {
		printUsageAndExit("Error creating POST request: %s", err)
	}

	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	enc.Write([]byte(*Token))
	enc.Close()
	req.Header.Set("Authorization", "basic "+buf.String())
	req.Header.Set("Accept", "application/vnd.github.manifold-preview")
	req.Header.Set("Content-Type", *ContentType)
	req.ContentLength = int64(stat.Size())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		printUsageAndExit("POST response error: %s", err)
	}

	return res
}

func normalizeUri(uri *url.URL) {
	if !strings.HasPrefix(uri.Path, "/") {
		if len(uri.Host) == 0 {
			pieces := strings.Split(uri.Path, "/")
			uri.Host = pieces[0]
			uri.Path = strings.Join(pieces[1:len(pieces)], "/")
		}

		uri.Path = "/" + uri.Path
	}

	if len(uri.Host) == 0 {
		uri.Host = "uploads.github.com"
	}

	if len(uri.Scheme) == 0 {
		uri.Scheme = "https"
	}
}

func printUsageAndExit(msg string, a ...interface{}) {
	if len(msg) > 0 {
		fmt.Printf("%s\n\n", fmt.Sprintf(msg, a...))
	}

	fmt.Printf("%s asseturl localurl [options]\n", App)
	FlagSet.PrintDefaults()
	os.Exit(1)
}
