// Copyright (c) Gigamon, Inc.

// Fetches the requested APP data from the monitoring session.

package commonresources

import (
	"context"
	"encoding/json"
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

// The generic strucutre that we will use to receive the MS data for getting the app data
type AppParams map[string]any
type FMMonSessApp struct {
	// The below are common for all apps and will always be present
	Id       string `json:"id,omitempty"`
	Alias    string `json:"alias,omitempty"`
	Name     string `json:"name,omitempty"` // Will be always dedup

	// The below is specific to the app
	AppParams
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
		Applications []FMMonSessApp `json:"applications"`
	}{
		Id: monitoringSessId,
	}

	err := updateMSData(ctx, monitoringSessId, &fmResp, fmClient)
	if err != nil {
		return false, err
	}

	// Go through and check if this app is present or not
	for _, app := range(fmResp.Applications) {
		if appName == app.Name &&
		   (appId == "" || appId == app.Id) &&
		   (appAlias == "" || appAlias == app.Alias) {
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
