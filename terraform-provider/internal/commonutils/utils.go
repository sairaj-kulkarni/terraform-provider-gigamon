// Copyright (c) Gigamon, Inc.

package commonutils

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"terraform-provider-gigamon/internal/fmclient"
)

// -----------------------------------------------------------------------------
// Types used for Monitoring Session updates
// -----------------------------------------------------------------------------

// UpdateReq is the generic request used to update a Monitoring Session.
type UpdateReq struct {
	Requests []UpdateObject `json:"requests"`
}

type UpdateObject struct {
	EntityType  string `json:"entityType"`
	Operation   string `json:"operation"`
	ReferenceId string `json:"referenceId,omitempty"`

	Link        any `json:"link,omitempty"`
	Tunnel      any `json:"tunnel,omitempty"`
	Raw         any `json:"raw,omitempty"`
	Application any `json:"application,omitempty"`
	Map         any `json:"map,omitempty"`
}

type UpdateResp struct {
	OperationResponses []ResponseObject `json:"operationResponses"`
}

type ResponseObject struct {
	EntityType string `json:"entityType"`
	Id         string `json:"id"`
	Alias      string `json:"alias"`
	Status     string `json:"status"`
}

// UpdateMonSess posts an update request for a Monitoring Session (create/change/delete).
func UpdateMonSess(
	ctx context.Context,
	req *UpdateReq,
	monSessId string,
	fmClient *fmclient.FmClient,
) (string, error) {

	rawID, err := UUIDFromTypedID(monSessId)
	if err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("Unable to encode into Json: %v, error: %w", req, err)
	}

Loop:
	for {
		respData, err := fmClient.DoRequest(
			ctx,
			"POST",
			fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/update", rawID),
			map[string]string{"useTimerBasedDeployment": "true"},
			nil,
			bytes.NewBuffer(jsonData),
			"application/json",
		)
		if err != nil {
			var fmErr *fmclient.FMErrors
			if errors.As(err, &fmErr) {
				if fmErr.ErrorCode() == fmclient.TooManyRequests {
					timer := time.NewTimer(30 * time.Second)
					select {
					case <-timer.C:
						continue
					case <-ctx.Done():
						break Loop
					}
				}
			}
			return "", err
		}
		var fmResp UpdateResp
		if err := json.Unmarshal(respData, &fmResp); err != nil {
			return "", fmt.Errorf(
				"Unable to decode update response: %s , err: %w",
				string(respData),
				err,
			)
		}

		if len(fmResp.OperationResponses) == 0 {
			return "", fmt.Errorf(
				"update response has no OperationResponses: %s",
				string(respData),
			)
		}

		return fmResp.OperationResponses[0].Id, nil
	}
	return "", fmt.Errorf("Monitoring Session Update for %s timed out", monSessId)
}
