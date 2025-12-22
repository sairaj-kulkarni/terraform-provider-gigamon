// Copyright (c) Gigamon, Inc.

// Implements the APP Resrouces that are common across all environment

package commonresources

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
    //"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TrafficMap{}

// TrafficMap resoruce, which manages the Maps for Traffic Handling
func NewTrafficMap() resource.Resource {
	return &TrafficMap{}
}

// TrafficMap - implements the maps for traffic handling
type TrafficMap struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

func (tm *TrafficMap) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_trafficmap"
}

func (tm *TrafficMap) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = MapSchema()
}

// Initial Configure call, to initialize the Provider
func (tm *TrafficMap) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	fmClient, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *fmclient.FmClient, got: %T. Report the issue to Gigamon", req.ProviderData),
		)
		return
	}
	tm.fmClient = fmClient
}

// Create call for new Traffic Map
func (tm *TrafficMap) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MapModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	trafficMap := ModelMapToGoMap(ctx, &data)
	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "trafficMap",
				Operation:  "create",
				Map:        trafficMap,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		tm.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create dedup app",
			fmt.Sprintf("app creation failed: %v", err),
		)
		return
	}

	data.Id = types.StringValue(id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (tm *TrafficMap) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MapModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmData, err := GetMSMapData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		data.Name.ValueString(),
		"trafficMap",
		tm.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to Get Traffic Map details",
			fmt.Sprintf("unable to get Traffic Map details. error is %v", err),
		)
		return
	}

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(resp.State.Set(ctx, &fmData)...)
}

func (tm *TrafficMap) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data MapModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (tm *TrafficMap) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MapModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "trafficMap",
				Operation:  "delete",
				Map: MapGo{
					Id: data.Id.ValueString(),
				},
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		tm.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete the map",
			fmt.Sprintf("app creation failed: %v", err),
		)
	}
}
