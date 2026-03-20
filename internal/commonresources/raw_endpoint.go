// Copyright (c) Gigamon, Inc.

// Implements the Raw Endpoint (REP) resource inside a Monitoring Session.

package commonresources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RawEndpoint{}

// RawEndpoint is the Terraform resource implementation for a Monitoring Session REP.
type RawEndpoint struct {
	fmClient *fmclient.FmClient
}

// NewRawEndpoint returns a new RawEndpoint resource instance.
func NewRawEndpoint() resource.Resource {
	return &RawEndpoint{}
}

// RawEndpointModel is the Terraform state/plan model.
type RawEndpointModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Id                  types.String `tfsdk:"id"`
	Alias               types.String `tfsdk:"alias"`
	Description         types.String `tfsdk:"description"`
}

// FMRaw is the FM-side representation of a raw endpoint used in /update and /monitoringSessions GET.
type FMRaw struct {
	Id          string `json:"id,omitempty"`
	Alias       string `json:"alias,omitempty"`
	Description string `json:"description,omitempty"`
}

// Metadata sets the resource type name.
func (r *RawEndpoint) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_raw_endpoint"
}

// Schema defines the Terraform schema for the raw endpoint resource.
func (r *RawEndpoint) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon Monitoring Session Raw Endpoint (REP).",

		Attributes: map[string]schema.Attribute{
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Session ID in which this raw endpoint is configured.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this raw endpoint (typed ID rawEndpoint::raw::<uuid>, used for links and mappings).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias of the raw endpoint (as shown in FM).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description of the raw endpoint.",
				Optional:            true,
			},
		},
	}
}

// Configure wires in the FM client.
func (r *RawEndpoint) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.fmClient = fmClient
}

// ---- Helpers to read raw endpoints from a Monitoring Session ----

func GetMSRawData(
	ctx context.Context,
	monitoringSessId, rawId string,
	rawData *FMRaw,
	fmClient *fmclient.FmClient,
) error {
	fmResp := struct {
		Id           string  `json:"id,omitempty"`
		RawEndPoints []FMRaw `json:"rawEndPoints"`
	}{
		Id: monitoringSessId,
	}

	if err := UpdateMSData(ctx, monitoringSessId, &fmResp, fmClient); err != nil {
		return fmt.Errorf("failed to get monitoring session %q raw endpoints: %w", monitoringSessId, err)
	}

	for _, re := range fmResp.RawEndPoints {
		if re.Id == "" {
			return fmt.Errorf("monitoring session %q contains raw endpoint with empty id field", monitoringSessId)
		}

		if rawId == "" || rawId == re.Id {
			*rawData = re
			return nil
		}
	}

	// No matching raw found: use generic ObjectNotFound with contextual message.
	return fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf(
			"monitoring session raw endpoint not found: monitoring_session_id=%s raw_id=%s",
			monitoringSessId,
			rawId,
		),
		nil,
	)
}

func GetMSRaws(
	ctx context.Context,
	monitoringSessId string,
	fmClient *fmclient.FmClient,
) ([]FMRaw, error) {
	fmResp := struct {
		Id           string  `json:"id,omitempty"`
		RawEndPoints []FMRaw `json:"rawEndPoints"`
	}{
		Id: monitoringSessId,
	}

	if err := UpdateMSData(ctx, monitoringSessId, &fmResp, fmClient); err != nil {
		return nil, fmt.Errorf("failed to get monitoring session %q raw endpoints: %w", monitoringSessId, err)
	}

	return fmResp.RawEndPoints, nil
}

// ---- FM <-> TF mappers ----

func (r *RawEndpoint) createFMStruct(data *RawEndpointModel) *FMRaw {
	return &FMRaw{
		Id:          data.Id.ValueString(),
		Alias:       data.Alias.ValueString(),
		Description: data.Description.ValueString(),
	}
}

func (r *RawEndpoint) updateTFStruct(data *RawEndpointModel, fmData *FMRaw) {
	data.Alias = types.StringValue(fmData.Alias)

	if fmData.Description != "" {
		data.Description = types.StringValue(fmData.Description)
	} else {
		data.Description = types.StringNull()
	}

	if fmData.Id != "" {
		// fmData.Id is raw UUID from FM; wrap into typed ID for TF
		if typedID, err := commonutils.MakeTypedID(
			commonutils.ModuleRawEndpoint,
			commonutils.TypeRawEndpoint,
			fmData.Id,
		); err == nil {
			data.Id = types.StringValue(typedID)
		}
		// If MakeTypedID fails unexpectedly, keep existing data.Id (match tunnel behavior).
	}
}

// ---- CRUD ----

// Create creates a new raw endpoint inside the Monitoring Session using the
// /cloud/monitoringSessions/{msId}/update batch API:
//
//	{
//	  "requests": [
//	    {
//	      "entityType": "raw",
//	      "operation": "create",
//	      "raw": { "alias": "raw-1" }
//	    }
//	  ]
//	}
func (r *RawEndpoint) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RawEndpointModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	msId := data.MonitoringSessionId.ValueString()
	fmRaw := r.createFMStruct(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "raw",
				Operation:  "create",
				Raw:        fmRaw,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		msId,
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create raw endpoint",
			fmt.Sprintf("raw endpoint creation failed: %s", err),
		)
		return
	}

	// id is raw UUID from FM; do not set data.Id directly here.
	// We'll read back from FM and let updateTFStruct set the typed ID.

	var rawData FMRaw
	if err := GetMSRawData(ctx, msId, id, &rawData, r.fmClient); err != nil {
		resp.Diagnostics.AddError(
			"Unable to read created raw endpoint from Monitoring Session",
			fmt.Sprintf("raw endpoint created but fetching details failed: %s", err),
		)
		return
	}

	r.updateTFStruct(&data, &rawData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes the raw endpoint state from FM.
func (r *RawEndpoint) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RawEndpointModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawData := FMRaw{}

	rawID := data.Id.ValueString()
	// If ID is typed, unwrap to raw UUID for FM
	if strings.Contains(rawID, commonutils.TypedIDDelim) {
		uuid, err := commonutils.UUIDFromTypedID(rawID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid raw endpoint ID",
				fmt.Sprintf("failed to parse typed ID %q: %v", rawID, err),
			)
			return
		}
		rawID = uuid
	}

	err := GetMSRawData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawID,
		&rawData,
		r.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			// Raw endpoint no longer exists in FM; remove from state (idempotent read).
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Unable to get raw endpoint details from Monitoring Session",
			fmt.Sprintf("unable to get raw endpoint details. error is %s", err),
		)
		return
	}

	r.updateTFStruct(&data, &rawData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update modifies an existing raw endpoint.
//
// FM supports raw update via:
//
//	{
//	  "requests": [
//	    {
//	      "entityType": "raw",
//	      "operation": "update",
//	      "raw": { "id": "<uuid>", "alias": "...", "description": "..." }
//	    }
//	  ]
//	}
func (r *RawEndpoint) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RawEndpointModel
	var state RawEndpointModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	msId := state.MonitoringSessionId.ValueString()

	// Unwrap typed ID to raw UUID for FM
	rawID := state.Id.ValueString()
	if strings.Contains(rawID, commonutils.TypedIDDelim) {
		uuid, err := commonutils.UUIDFromTypedID(rawID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid raw endpoint ID",
				fmt.Sprintf("failed to parse typed ID %q: %v", rawID, err),
			)
			return
		}
		rawID = uuid
	}

	// Build FM payload using raw UUID
	fmRaw := &FMRaw{
		Id:          rawID,
		Alias:       plan.Alias.ValueString(),
		Description: plan.Description.ValueString(),
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "raw",
				Operation:  "update",
				Raw:        fmRaw,
			},
		},
	}

	if _, err := commonutils.UpdateMonSess(ctx, &updateReq, msId, r.fmClient); err != nil {
		resp.Diagnostics.AddError(
			"Unable to update raw endpoint",
			fmt.Sprintf("raw endpoint update failed: %s", err),
		)
		return
	}

	var rawData FMRaw
	if err := GetMSRawData(ctx, msId, rawID, &rawData, r.fmClient); err != nil {
		resp.Diagnostics.AddError(
			"Unable to read updated raw endpoint from Monitoring Session",
			fmt.Sprintf("raw endpoint updated but fetching details failed: %s", err),
		)
		return
	}

	r.updateTFStruct(&plan, &rawData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the raw endpoint from the Monitoring Session.
//
// Payload:
//
//	{
//	  "requests": [
//	    {
//	      "entityType": "raw",
//	      "operation": "delete",
//	      "raw": { "id": "<uuid>" }
//	    }
//	  ]
//	}
func (r *RawEndpoint) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RawEndpointModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	msId := data.MonitoringSessionId.ValueString()

	rawID := data.Id.ValueString()
	if strings.Contains(rawID, commonutils.TypedIDDelim) {
		uuid, err := commonutils.UUIDFromTypedID(rawID)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid raw endpoint ID",
				fmt.Sprintf("failed to parse typed ID %q: %v", rawID, err),
			)
			return
		}
		rawID = uuid
	}

	deletePayload := struct {
		Id string `json:"id"`
	}{
		Id: rawID,
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "raw",
				Operation:  "delete",
				Raw:        deletePayload,
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		msId,
		r.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			// Raw endpoint does not exist in FM; delete is idempotent.
			return
		}

		resp.Diagnostics.AddError(
			"Unable to delete raw endpoint",
			fmt.Sprintf("raw endpoint deletion failed: %s", err),
		)
	}
}
