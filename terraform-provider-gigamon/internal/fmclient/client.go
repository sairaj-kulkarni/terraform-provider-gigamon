// Copyright (c) Gigamon, Inc.

// Client library for Gigamon FM.

package fmclient

import (
	"bytes"
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type FmClient struct {
	token string // Toekn for authentication and authorization to FM. Currently we only support APi based token, we can add other methods later if required
	fmAddress string // FM address to reach to
	skipVerify bool // Verify the certificate presented by FM
	client *http.Client // The Client instance for talking to FM
	version string // Version of the FM that we are talking to
}

type FmInfo struct {
	Version string // FM version
}

// Create a new instance of FM client, and validate reachability by doing a Version call
func NewFmClient (token, fmAddress string, skipVerify bool) (*FmClient, error) {
	var fmInfo FmInfo

	// For now we will limit parallel request to FM to just one, so limit connections
	httpClient := &http.Client {
		Transport: &http.Transport {
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify,
			},
		    MaxIdleConns: 2,
		    MaxConnsPerHost: 2,
		},
	}

	fmClient := &FmClient {
		token: token,
		fmAddress: fmAddress,
		skipVerify: skipVerify,
		client: httpClient,
	}

	resp, err := fmClient.DoRequest("GET", 10, "/api/0.9/sys/info", nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to get the version of FM: %s", err)
	}

	err = json.Unmarshal(resp, &fmInfo)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse the version response: %s,", err)
	}
	fmClient.version = fmInfo.Version
	return fmClient, nil
}

// Performs an operation on FM
//   method  -> The method to execute, one of GET, POST, PATCH, DELETE, PUT
//   timeout -> amount of seconds to wait for the response, 0 means indefinite wait
//   path    -> The path for the request. does not include the host/port
//   params  -> The request URL parameters to be added to the request
//   headers -> Headers to be added to the request, on top of any standard headers always added
//   body    -> an interface body which is sent as the body. Should be somethign that can be
//              mapped to a JSON body
//
//   The function returns the body of the response (if any, otherwis is null), and an error in
//   case the request could not be completed
func (c *FmClient) DoRequest (
	method string, // Method to invoke
	timeout int, // wait period for the request in seconds, 0 => default which is 60 seconds
	path string, // The path of the URL, the host/port is added to this
	params map[string]string, // URL parameters to be added
	headers map[string]string, // headers to be added to the request
	body any, // The body that is to be sent, will be added as JSON to the request
) ([]byte, error) {
	fmUrl, err := url.Parse(fmt.Sprintf("https://%s/%s", c.fmAddress, path))
	if err != nil {
		return nil, fmt.Errorf("Unable to form the URL %s %s: %s", c.fmAddress, path, err)
	}
	urlParams := fmUrl.Query()
	for p, v := range params {
		urlParams.Add(p, v)
	}
	fmUrl.RawQuery = urlParams.Encode()
	if timeout <= 0 {
		timeout = 60
	}
	var jsonBody bytes.Buffer
	writer := bufio.NewWriter(&jsonBody)
	if body != nil {
		err := json.NewEncoder(writer).Encode(body)
		if err != nil {
			return nil, fmt.Errorf("Unable to encode the Json body: %s", err)
		}
		err = writer.Flush()
		if err != nil {
			return nil, fmt.Errorf("Unable to flush the writed: %s", err)
		}
	}
	reader := bufio.NewReader(&jsonBody)

	ctx, _ := context.WithTimeout(context.Background(), time.Duration(timeout) * time.Second)
	httpReq, err := http.NewRequestWithContext(ctx, method, fmUrl.String(), reader)
	if err != nil {
		return nil, fmt.Errorf("Failed to form the http request: %s", err)
	}

	// Add our standard headers
	httpReq.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))

	// Add any additional headers
	for k,v := range headers {
		httpReq.Header.Add(k, v)
	}

	// Perform the operation
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("Failed to get proper response: %s", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("FM request error: %s", http.StatusText(resp.StatusCode))
	}

	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to read the response body: %s", err)
	}
	return respBody, nil
}
