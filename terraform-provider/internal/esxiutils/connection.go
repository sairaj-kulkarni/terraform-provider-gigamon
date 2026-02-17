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
	client *fmclient.FmClient,
) (*EsxiFmConnection, error) {

	fmConn := struct {
		VmwareConnection EsxiFmConnection `json:"vmwareConnection"`
	}{}

	fmResp, err := client.DoRequest(
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
			"Get request of Vmware Connections failed: %s: %w",
			connectionId,
			err,
		)
	}

	err = json.Unmarshal(fmResp, &fmConn)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert Connection resp to struct: %s error is: %w",
			string(fmResp),
			err,
		)
	}
	return &fmConn.VmwareConnection, nil
}

func GetConnectionByAlias(
	ctx context.Context,
	alias string,
	client *fmclient.FmClient,
) (*EsxiFmConnection, error) {

	connResp := struct {
		VmwareConnections []EsxiFmConnection `json:"vmwareConnections"`
	}{
		VmwareConnections: []EsxiFmConnection{},
	}

	fmResp, err := client.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/connections",
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf(
			"Get request of Vmware Connections failed: %s: %w",
			alias,
			err,
		)
	}

	err = json.Unmarshal(fmResp, &connResp)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert resp to struct: %s error is: %w",
			string(fmResp),
			err,
		)
	}

	for _, conn := range connResp.VmwareConnections {
		if conn.Alias == alias {
			return &conn, nil
		}
	}

	// This connection is not found, return a not found object error
	return nil, fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf("unable to find MD by name: %s", alias),
		nil,
	)
}
