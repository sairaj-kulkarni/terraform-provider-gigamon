// Copyright (c) Gigamon, Inc.

// Client library for Gigamon FM.

package fmclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Custom Errors that FM would return
// Codes 1xx - 6xx carry the corresponding HTTP response code, and others imply
// code/communication errors

const (
	ObjectNotFound      int = 404
	CommunicationErrors int = 1000
	GeneralErrors       int = 2000
)

type FMErrors struct {
	Code    int
	Message string
	Err     error
}

func (e *FMErrors) Error() string {
	return fmt.Sprintf("fmclient: %s. %v", e.Message, e.Err)
}

func (e *FMErrors) Unwrap() error {
	return e.Err
}

func (e *FMErrors) ErrorCode() int {
	return e.Code
}

func NewFMError(code int, message string, err error) *FMErrors {
	return &FMErrors{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

type FmClient struct {
	token      string       // Toekn for authentication and authorization to FM. Currently we only support APi based token, we can add other methods later if required
	fmAddress  string       // FM address to reach to
	skipVerify bool         // Verify the certificate presented by FM
	client     *http.Client // The Client instance for talking to FM
	version    string       // Version of the FM that we are talking to
}

type FmInfo struct {
	Version string // FM version
}

// Create a new instance of FM client, and validate reachability by doing a Version call
func NewFmClient(
	ctx context.Context,
	token, fmAddress string,
	skipVerify bool,
) (*FmClient, error) {
	var fmInfo FmInfo

	// For now we will limit parallel request to FM to just one, so limit connections
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify,
			},
			MaxIdleConns:    2,
			MaxConnsPerHost: 2,
		},
	}

	fmClient := &FmClient{
		token:      token,
		fmAddress:  fmAddress,
		skipVerify: skipVerify,
		client:     httpClient,
	}

	// Do a Get Version call to make sure FM is reachable and credentials are ok
	myCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := fmClient.DoRequest(myCtx, "GET", "/api/0.9/sys/info", nil, nil, nil, "")
	if err != nil {
		return nil, NewFMError(CommunicationErrors, "Error in getting FM Version", err)
	}

	err = json.Unmarshal(resp, &fmInfo)
	if err != nil {
		return nil, NewFMError(GeneralErrors, "Error in decoding FM Version", err)
	}
	fmClient.version = fmInfo.Version
	return fmClient, nil
}

// Prepare the request content for a file upload. Currently it reads the entire file into
// memory, but later will make it use streaming mode
func (c *FmClient) PrepareFileUpload(ctx context.Context, fileName string) (io.Reader, string, error) {
	var b bytes.Buffer

	w := multipart.NewWriter(&b)
	defer w.Close()

	fhdl, err := os.Open(fileName)
	if err != nil {
		return nil, "", NewFMError(
			GeneralErrors,
			fmt.Sprintf("%s: upload failed", fileName),
			err,
		)
	}
	defer fhdl.Close()

	filePart, err := w.CreateFormFile("file", filepath.Base(fileName))
	if err != nil {
		return nil, "", NewFMError(
			GeneralErrors,
			fmt.Sprintf("%s: upload failed", fileName),
			err,
		)
	}
	_, err = io.Copy(filePart, fhdl)
	if err != nil {
		return nil, "", NewFMError(
			GeneralErrors,
			fmt.Sprintf("%s: copy file content failed", fileName),
			err,
		)
	}
	return &b, w.FormDataContentType(), nil
}

// Performs an operation on FM
//
//	ctx     -> The user provided ctx, to cancel this operation if user aborts
//	method  -> The method to execute, one of GET, POST, PATCH, DELETE, PUT
//	path    -> The path for the request. does not include the host/port
//	params  -> The request URL parameters to be added to the request
//	headers -> Headers to be added to the request, on top of any standard headers always added
//	body    -> an interface body which is sent as the body. Should be somethign that can be
//	           mapped to a JSON body
//	contentType -> String to be added to the Content-Type header
//
//	The function returns the body of the response (if any, otherwis is null), and an error in
//	case the request could not be completed
func (c *FmClient) DoRequest(
	ctx context.Context, // User provided context to cancel if user aborts the run
	method string, // Method to invoke
	path string, // The path of the URL, the host/port is added to this
	params map[string]string, // URL parameters to be added
	headers map[string]string, // headers to be added to the request
	body io.Reader, // The body that is to be sent, should be nil (no content) or body content
	contentType string, // Content Type that is being sent, if body is not nil
) ([]byte, error) {

	// Form the URL and add query parameters if any
	fmUrl, err := url.Parse(fmt.Sprintf("https://%s/%s", c.fmAddress, path))
	if err != nil {
		return nil, NewFMError(
			GeneralErrors,
			fmt.Sprintf("Unable to form the URL %s %s", c.fmAddress, path),
			err,
		)
	}
	urlParams := fmUrl.Query()
	for p, v := range params {
		urlParams.Add(p, v)
	}
	fmUrl.RawQuery = urlParams.Encode()
	tflog.Info(ctx, "FM Client DoRequest Calling: ", map[string]any{
		"url":          fmUrl.String(),
		"method":       method,
		"params":       params,
		"headers":      headers,
		"content-type": contentType,
	})

	httpReq, err := http.NewRequestWithContext(ctx, method, fmUrl.String(), body)
	if err != nil {
		return nil, NewFMError(
			GeneralErrors,
			fmt.Sprintf("Error in creating request for %s:%s", method, fmUrl.String()),
			err,
		)
	}

	// Add the default authorization header
	httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	httpReq.Header.Add("Accept", "application/json, text/plain, */*")
	httpReq.Header.Add("Accept-Language", "en-IN,en;q=0.9")
	if body != nil {
		httpReq.Header.Add("Content-Type", contentType)
	}

	// Add any additional headers
	for k, v := range headers {
		httpReq.Header.Add(k, v)
	}

	// Perform the operation
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, NewFMError(
			CommunicationErrors,
			fmt.Sprintf("error when making request:  %s:%s", method, fmUrl.String()),
			err,
		)
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewFMError(
			GeneralErrors,
			fmt.Sprintf(
				"FM request %s:%s failed when reading the response body.",
				method,
				fmUrl.String(),
			),
			err,
		)
	}

	if resp.StatusCode >= 300 {
		return nil, NewFMError(
			resp.StatusCode,
			fmt.Sprintf(
				"FM request %s:%s failed with error: %s, error content: %s",
				method,
				fmUrl.String(),
				http.StatusText(resp.StatusCode),
				string(respBody),
			),
			err,
		)
	}
	return respBody, nil
}
