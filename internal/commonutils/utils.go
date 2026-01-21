// Copyright (c) Gigamon, Inc.

package commonutils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// monitoringSessionStatusResp is a minimal view of the MS for deployment state.
type monitoringSessionStatusResp struct {
	MonitoringSession struct {
		Id       string `json:"id"`
		Deployed bool   `json:"deployed"`
	} `json:"monitoringSession"`
}

func GetMonitoringSessionDeployedState(
	ctx context.Context,
	monSessId string,
	fmClient *fmclient.FmClient,
) (bool, error) {
	respData, err := fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s", monSessId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return false, fmt.Errorf("failed to get monitoring session %s: %w", monSessId, err)
	}

	var wrapper monitoringSessionStatusResp
	if err := json.Unmarshal(respData, &wrapper); err != nil {
		return false, fmt.Errorf("failed to decode monitoring session %s: %v; body=%s",
			monSessId, err, string(respData))
	}

	return wrapper.MonitoringSession.Deployed, nil
}

// WaitForMonitoringSessionUndeployed polls until 'deployed' is false or timeout.
func WaitForMonitoringSessionUndeployed(
	ctx context.Context,
	monSessId string,
	fmClient *fmclient.FmClient,
	timeout time.Duration,
	interval time.Duration,
) error {
	deadline := time.Now().Add(timeout)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for monitoring session %s to undeploy", monSessId)
		}

		deployed, err := GetMonitoringSessionDeployedState(ctx, monSessId, fmClient)
		if err == nil && !deployed {
			return nil
		}

		time.Sleep(interval)
	}
}

// UndeployMonitoringSession undeploys an MS and waits until it is undeployed.
func UndeployMonitoringSession(
	ctx context.Context,
	monSessId string,
	fmClient *fmclient.FmClient,
) error {
	_, err := fmClient.DoRequest(
		ctx,
		"POST",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/undeploy", monSessId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("failed to undeploy monitoring session %s: %w", monSessId, err)
	}

	if err := WaitForMonitoringSessionUndeployed(ctx, monSessId, fmClient, 2*time.Minute, 5*time.Second); err != nil {
		return fmt.Errorf("undeploy of monitoring session %s did not complete: %w", monSessId, err)
	}

	return nil
}

func isLastEntityConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg,
		"Cannot deploy the monitoring session: it must have at least one of traffic map, application, tunnel, transport interface or raw endpoints")
}

// isMsEntityType tracks which entity types participate in the "must have at least one" rule.
func isMsEntityType(t string) bool {
	switch t {
	case "trafficMap", "application", "tunnel", "raw", "transportInterface", "link":
		return true
	default:
		return false
	}
}

// -----------------------------------------------------------------------------
// Core update function
// -----------------------------------------------------------------------------

// UpdateMonSess posts an update request for a Monitoring Session (create/change/delete).
//
// For deletes of MS entities (map/app/tunnel/etc):
//   - First try /update normally.
//   - If FM returns the specific "must have at least one ..." conflict:
//   - Undeploy the MS and wait for 'deployed' to become false.
//   - Retry the same /update once.
//   - If it still returns the same conflict, return a clear error
//     indicating the user is trying to delete the last entity.
func UpdateMonSess(
	ctx context.Context,
	req *UpdateReq,
	monSessId string,
	fmClient *fmclient.FmClient,
) (string, error) {
	// Detect if this is a single delete of an MS entity type.
	isDelete := false
	if len(req.Requests) == 1 {
		r := req.Requests[0]
		if strings.EqualFold(r.Operation, "delete") && isMsEntityType(r.EntityType) {
			isDelete = true
		}
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("Unable to encode into Json: %v, error: %s", req, err)
	}

	// First attempt.
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
		// If it's not the known conflict on a delete, just return the error.
		if !(isDelete && isLastEntityConflict(err)) {
			return "", fmt.Errorf("Unable to perform update error: %s", err)
		}

		// Conflict on delete: undeploy and retry.
		if uErr := UndeployMonitoringSession(ctx, monSessId, fmClient); uErr != nil {
			return "", fmt.Errorf("update conflict deleting entity; also failed to undeploy MS %s: undeployErr=%v, deleteErr=%v",
				monSessId, uErr, err)
		}

		// Retry once after confirmed undeploy.
		respData, err = fmClient.DoRequest(
			ctx,
			"POST",
			fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/update", monSessId),
			nil,
			nil,
			bytes.NewBuffer(jsonData),
			"application/json",
		)
		if err != nil {
			// Still the same conflict after undeploy → true "last entity" case.
			if isLastEntityConflict(err) && isDelete && len(req.Requests) == 1 {
				r := req.Requests[0]
				return "", fmt.Errorf(
					"FM does not allow deleting the last %q entity from monitoring session %s. "+
						"Please delete the monitoring session resource as well or keep at least one "+
						"traffic map, application, tunnel, transport interface, or raw endpoint.",
					r.EntityType, monSessId)
			}
			return "", fmt.Errorf("Unable to perform update after undeploy error: %s", err)
		}
	}

	var fmResp UpdateResp
	if err := json.Unmarshal(respData, &fmResp); err != nil {
		return "", fmt.Errorf("Unable to decode update response: %s , err: %v", string(respData), err)
	}

	if len(fmResp.OperationResponses) == 0 {
		return "", fmt.Errorf("update response has no OperationResponses: %s", string(respData))
	}

	return fmResp.OperationResponses[0].Id, nil
}
