// Copyright (c) Gigamon, Inc.

// Implements utility functions that are common across cloud platforms

package fmcommon

// Represents the generic struct that is used to provide updates to the MS for the
// different components like links, maps etc.
// The update can contain any one of the below elements and an array of such structs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"
)

type UpdateReq struct {
	Requests []UpdateObject `json:"requests"`
}

type UpdateObject struct {
	EntityType  string `json:"entityType"`
	Operation   string `json:"operation"`
	ReferenceId string `json:"referenceId,omitempty"`
	Link        any    `json:"link,omitempty"`
	Tunnel      any    `json:"tunnel,omitempty"`
	Raw         any    `json:"raw,omitempty"`
	Application any    `json:"application,omitempty"`
	Map any `json:"map,omitempty"`
}

// The update response
type UpdateResp struct {
	OperationResponses []ResponseObject `json:"operationResponses"`
}

type ResponseObject struct {
	EntityType string `json:"entityType"`
	Id         string `json:"id"`
	Alias      string `json:"alias"`
	Status     string `json:"status"`
}

// Function to post an update request with an APP/Map/Link etc. to the MS. This can be a 
// create/change/delete request
func UpdateMonSess(
	ctx context.Context,
	req *UpdateReq,
	monSessId string,
	fmClient *fmclient.FmClient,
) (string, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("Unable to encode into Json: %v, error: %s", req, err)
	}

	respData, err := fmClient.DoRequest(
		ctx,
		"POST",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/update", monSessId),
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)

	if err != nil {
		return "", fmt.Errorf("Unable to perform update  error: %s", err)
	}

	var fmResp UpdateResp
	err = json.Unmarshal(respData, &fmResp)
	if err != nil {
		return "", fmt.Errorf("Unable to decode update response: %s , err: %s", string(respData), err)
	}

	return fmResp.OperationResponses[0].Id, err
}

