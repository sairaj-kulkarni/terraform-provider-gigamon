// Copyright (c) Gigamon, Inc.

// Implements utility functions that are common across cloud platforms

package fmcommon

// Represents the generic struct that is used to provide updates to the MS for the
// different components like links, maps etc.
// The update can contain any one of the below elements and an array of such structs

type UpdateReq struct{
	Requests []UpdateObject `json:"requests"`
}

type UpdateObject struct{
	EntityType string `json:"entityType"`
	Operation string `json:"operation"`
	ReferenceId string `json:"referenceId,omitempty"`
	Link any `json:"link,omitempty"`
	Tunnel any `json:"tunnel,omitempty"`
	Raw any `json:"raw,omitempty"`
	Application any `json:"application,omitempty"`
}

// The update response
type UpdateResp struct{
	OperationResponses []ResponseObject `json:"operationResponses"`
}

type ResponseObject struct{
	EntityType string `json:"entityType"`
	Id string `json:"id"`
	Alias string `json:"alias"`
	Status string `json:"status"`
}
