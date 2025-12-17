// Copyright (c) Gigamon, Inc.

// Fetches the requested APP data from the monitoring session.

package commonresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"terraform-provider-gigamon/internal/fmclient"
)
// FM response for DedupConfig Put/Get
type FMDedupConfig struct {
	Action   string `json:"action,omitempty"`
	IPTClass string `json:"ipTclass,omitempty"`
	IPTos    string `json:"ipTos,omitempty"`
	TCPSeq   string `json:"tcpSeq,omitempty"`
	Timer    int32  `json:"timer,omitempty"`
	Vlan     string `json:"vlan,omitempty"`
}

// FM response for Slicing Creation/Get
type FMSlicing struct {
	Id       string `json:"id,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Name     string `json:"name,omitempty"` // Will be always slicing
	Protocol string `json:"protocol,omitempty"`
	Offset int32 `json:"offset,omitempty"`
}

// FM struct to get GS Params data
type GsParams struct {
	GsParamsName string `json:"gsparamsName"`
	Dedup FMDedupConfig `json:"dedup"`
}

type FMDedup struct {
	Id       string `json:"id,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Name     string `json:"name,omitempty"` // Will be always dedup
    Description string `json:"description,omitempty"`
}



// GetMSAppData - gets the app details from the MS and returns an error in case it is not
//  available. error implies that there was an error in fetching the data and ok indicates
//  if the data was there in the response or not
func GetMSAppData(
	ctx context.Context,
	monitoringSessId, appId string,
	appName, appAlias string,
	appData any,
	fmClient *fmclient.FmClient,
) (bool, error) {

	fmResp := struct {
	    Alias              string   `json:"alias"`
	    Id                 string   `json:"id,omitempty"`
	    ConnectionId       []string `json:"connIds"`
	    MonitoringDomainId string   `json:"monitoringDomainId"`
		Applications []map[string]any `json:"applications"`
	}{
		Id: monitoringSessId,
	}

	err := updateMSData(ctx, monitoringSessId, &fmResp, fmClient)
	if err != nil {
		return false, err
	}

	// Go through and check if this app is present or not
	for _, app := range(fmResp.Applications) {
		fmAppName, ok := app["name"].(string)
		if !ok {
			return false, fmt.Errorf("Unable to get the name of the app")
		}
		fmAppId, ok := app["id"].(string)
		if !ok {
			return false, fmt.Errorf("Unable to get the id of the app")
		}

		fmAppAlias, ok := app["alias"].(string)
		if !ok {
			return false, fmt.Errorf("Unable to get the alias of the app")
		}

		if appName == fmAppName &&
		   (appId == "" || appId == fmAppId) &&
		   (appAlias == "" || appAlias == fmAppAlias) {
			   // Convert this to a JSON and then read it back into the app data
	           jsonData, err := json.Marshal(app)
	           if err != nil {
		           return false, err
	           }

			   // Convert it back to our struct
	           err = json.Unmarshal(jsonData, appData)
			   if err != nil {
				   return false, err
			   }
			   return true, nil
		}
	   }
    return false, nil
}

// GetGsparams - Gets the gsParams for the specified MD
func GetGsParams(
	ctx context.Context,
	monitoringDomainId string,
	fmClient *fmclient.FmClient,
) (*GsParams, error) {

	gsData := struct {
		VseriesGsParams GsParams `json:"vseriesGsParams"`
	}{
		VseriesGsParams: GsParams{},
	}

	fmResp, err := fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("/api/v1.3/cloud/vseriesGsParams/%s",monitoringDomainId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(fmResp, &gsData)
	if err != nil {
		return nil, err
	}

	return &gsData.VseriesGsParams, nil
}

// SetGsParanms - Sets the given gsParams for the specified MD
func SetGsParams(
	ctx context.Context,
	monitoringDomainId string,
	gsParams *GsParams,
	fmClient *fmclient.FmClient,
) error {

	jsonData, err := json.Marshal(gsParams)
	if err != nil {
		return err
	}

	_, err = fmClient.DoRequest(
		ctx,
		"PATCH",
		fmt.Sprintf("/api/v1.3/cloud/vseriesGsParams/%s", monitoringDomainId),
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)

	if err != nil {
		return err
	}
	return nil
}
