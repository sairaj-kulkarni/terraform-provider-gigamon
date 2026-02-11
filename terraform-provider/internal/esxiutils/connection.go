// Copyright (c) Gigamon, Inc.

// Implements utility functions that interact with ESXI Connection APIs

package esxiutils

import (
	"context"
	"encoding/json"
	"fmt"

	"terraform-provider-gigamon/internal/fmclient"
)

// FM Connection struct
type EsxiFmConnection struct {
	MonitoringDomainId  string `json:"monitoringDomainId"`
	TappingMethod       string `json:"tappingMethod"`
	Alias               string `json:"alias"`
	VcenterIP           string `json:"vcenterIp"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	ResourceAllocation  string `json:"resourceAllocation"`
	MaximumNodesPerHost int32  `json:"maximumNodesPerHost"`
	Id                  string `json:"id,omitempty"`
	Status              string `json:"status,omitempty"`
}

// Get the ESXI Connection details given the connection ID
func GetConnectionById(
	ctx context.Context,
	connectionId string,
	fmclient *fmclient.FmClient,
) (*EsxiFmConnection, error) {

	fmConn := struct {
		VmwareConnection EsxiFmConnection `json:"vmwareConnection"`
	}{}

	fmResp, err := fmclient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf(
			"api/v1.3/cloud/vmware/connections/%s",
			connectionId,
		),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"Get request of Vmware Connections failed: %s: %s",
			connectionId,
			err,
		)
	}

	err = json.Unmarshal(fmResp, &fmConn)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert Connection resp to struct: %s error is: %s",
			string(fmResp),
			err,
		)
	}
	return &fmConn.VmwareConnection, nil
}
