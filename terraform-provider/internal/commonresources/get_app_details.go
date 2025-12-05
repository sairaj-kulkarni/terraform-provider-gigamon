// Copyright (c) Gigamon, Inc.

// Fetches the requested APP data from the monitoring session.

package commonresources

import (
	"context"
	"encoding/json"
	"fmt"
	"terraform-provider-gigamon/internal/fmclient"
)
// FM response for Dedup Creation/Get
type FMDedup struct {
	Id       string `json:"id,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Name     string `json:"name,omitempty"` // Will be always dedup
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
	Name     string `json:"name,omitempty"` // Will be always dedup
	Protocol string `json:"protocol,omitempty"`
	Offset int32 `json:"offset,omitempty"`
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
