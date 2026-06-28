//  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
//
//  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, version 3 of the License.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program. If not, see <https://www.gnu.org/licenses/>

// Implements the Link resources that connect Maps / Applications / Tunnels
// inside a Monitoring Session.

package commonresources

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource               = &Link{}
	_ resource.ResourceWithModifyPlan = &Link{}
)

// NewLink returns a new Link resource instance.
func NewLink() resource.Resource {
	return &Link{}
}

// Link manages a link object inside a Monitoring Session.
type Link struct {
	fmClient *fmclient.FmClient
}

// LinkModel is the Terraform model for a Monitoring Session link.
type LinkModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Id                  types.String `tfsdk:"id"`

	SourceId types.String `tfsdk:"source_id"`

	// SourceType is computed from FM based on SourceId; users normally don't set this.
	SourceType  types.String `tfsdk:"source_type"`   // "map", "application", "tunnel", "raw", ...
	SourceAepId types.Int32  `tfsdk:"source_aep_id"` // optional; used when source is a map (aepId in ruleset) OR Application: Load Balancing

	DestId types.String `tfsdk:"dest_id"`

	// DestType is computed from FM based on DestId; users normally don't set this.
	DestType types.String `tfsdk:"dest_type"` // "map", "application", "tunnel", "raw", ...
}

// FMLinkEndpoint is a single endpoint (source or dest) in the FM link JSON.
type FMLinkEndpoint struct {
	Id    string `json:"id"`
	Type  string `json:"type"`
	AepId int32  `json:"aepId,omitempty"`
}

// FMLink is the FM representation of a link in /cloud/monitoringSessions/{id}.
type FMLink struct {
	Id     string         `json:"id,omitempty"`
	Alias  string         `json:"alias,omitempty"`
	Source FMLinkEndpoint `json:"source"`
	Dest   FMLinkEndpoint `json:"dest"`
}

// Metadata sets the resource type name.
func (l *Link) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_link"
}

// Schema defines the Terraform schema for the link resource.
func (l *Link) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon Monitoring Session Link, connecting Maps / Applications / Tunnels.",

		Attributes: map[string]schema.Attribute{
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Session ID in which this link is configured.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this link in the Monitoring Session (used for deletion).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"source_id": schema.StringAttribute{
				MarkdownDescription: "ID of the source object (map/application/tunnel/raw) for this link.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"source_type": schema.StringAttribute{
				MarkdownDescription: "Type of the source object (map, application, tunnel, raw). " +
					"This is computed from FM based on source_id; users do not need to set it.",
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf("map", "application", "tunnel", "raw"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"source_aep_id": schema.Int32Attribute{
				MarkdownDescription: "AEP ID of the source, when source is a map or load balancing app",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int32{
					int32validator.Between(1, 64),
				},
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
			},

			"dest_id": schema.StringAttribute{
				MarkdownDescription: "ID of the destination object (map/application/tunnel/raw) for this link.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"dest_type": schema.StringAttribute{
				MarkdownDescription: "Type of the destination object (map, application, tunnel, raw). " +
					"This is computed from FM based on dest_id; users do not need to set it.",
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf("map", "application", "tunnel", "raw"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// fmTypeFromTypedID derives the FM link endpoint type from a typed ID.
func fmTypeFromTypedID(typedID string) (string, error) {
	parts, err := commonutils.ParseTypedID(typedID)
	if err != nil {
		return "", fmt.Errorf("invalid typed ID %q: %w", typedID, err)
	}
	switch parts.Module {
	case commonutils.ModuleMap:
		return "map", nil
	case commonutils.ModuleApp:
		return "application", nil
	case commonutils.ModuleTunnelIn, commonutils.ModuleTunnelOut:
		return "tunnel", nil
	case commonutils.ModuleRawEndpoint:
		return "raw", nil
	default:
		return "", fmt.Errorf("typed ID %q has module %q which is not a valid link endpoint", typedID, parts.Module)
	}
}

// Configure wires in the FM client.
func (l *Link) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	l.fmClient = fmClient
}

// ModifyPlan performs plan-time validation of link semantics.
func (l *Link) ModifyPlan(
	ctx context.Context,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
) {
	// If the resource is being destroyed, nothing to validate.
	if req.Plan.Raw.IsNull() {
		return
	}

	// IMPORTANT: use Config (user input), not Plan (input + state).
	var cfg LinkModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If source_id is not yet known in config, we can't do typed-ID checks.
	if cfg.SourceId.IsNull() || cfg.SourceId.IsUnknown() {
		return
	}

	// Validate only what the user configured.
	if err := l.validateSourceAep(
		cfg.SourceId.ValueString(),
		cfg.SourceAepId,
	); err != nil {
		resp.Diagnostics.AddError("Invalid source_aep_id", err.Error())
	}
}

// createFMStruct converts the TF model plus raw endpoint IDs into an FM link struct.
func (l *Link) createFMStruct(data *LinkModel, srcIdRaw, destIdRaw string) *FMLink {
	var srcAep int32
	if !data.SourceAepId.IsNull() && !data.SourceAepId.IsUnknown() {
		srcAep = data.SourceAepId.ValueInt32()
	}

	return &FMLink{
		Id: data.Id.ValueString(),
		// Alias is intentionally not set; FM allows empty/omitted alias for links.
		Source: FMLinkEndpoint{
			Id:    srcIdRaw,
			Type:  data.SourceType.ValueString(),
			AepId: srcAep,
		},
		Dest: FMLinkEndpoint{
			Id:   destIdRaw,
			Type: data.DestType.ValueString(),
		},
	}
}

// updateTFStruct copies FM link data into the TF state model.
func (l *Link) updateTFStruct(data *LinkModel, fmData *FMLink) {
	// Source endpoint
	data.SourceType = types.StringValue(fmData.Source.Type)
	if fmData.Source.AepId != 0 {
		data.SourceAepId = types.Int32Value(fmData.Source.AepId)
	} else {
		data.SourceAepId = types.Int32Null()
	}

	// Destination endpoint
	data.DestType = types.StringValue(fmData.Dest.Type)

	// Link id (only overwrite if FM provided one)
	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// validateSourceAep enforces when source_aep_id is allowed.
//
// Allowed when:
//   - source is a map, OR
//   - source is an application of type load balancing ("lb").
//
// Disallowed otherwise.
// validateSourceAep enforces when source_aep_id is allowed, using the typed ID.
//
// Allowed when:
//   - source_id is a typed MAP id (any map type), OR
//   - source_id is a typed APP id of type load_balancing.
//
// Disallowed otherwise, if user provided a source_aep_id.
func (l *Link) validateSourceAep(
	srcTyped string,
	srcAep types.Int32,
) error {
	parts, err := commonutils.ParseTypedID(srcTyped)
	if err != nil {
		return fmt.Errorf(
			"source_aep_id is only valid when source_id is a typed map or load balancing app id; got %q",
			srcTyped,
		)
	}

	isMap := parts.Module == commonutils.ModuleMap
	isLbApp := parts.Module == commonutils.ModuleApp && parts.Type == commonutils.TypeLoadBalancing

	// If user didn't set it:
	if srcAep.IsNull() || srcAep.IsUnknown() {
		// For map or LB source, this is invalid → enforce requirement.
		if isMap || isLbApp {
			return fmt.Errorf(
				"source_aep_id is required when source_id refers to a traffic map or load balancing app; " +
					"please set source_aep_id to the appropriate AEP ID",
			)
		}
		// For other sources, missing source_aep_id is fine.
		return nil
	}

	// If user did set it, only allow for map or load-balancing app.
	if !(isMap || isLbApp) {
		return fmt.Errorf(
			"source_aep_id is only valid when the link source is a map or a load balancing application; got module=%q type=%q",
			parts.Module, parts.Type,
		)
	}

	return nil
}

// Create creates a new link inside the Monitoring Session.
func (l *Link) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LinkModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	msId := data.MonitoringSessionId.ValueString()

	// Decode typed endpoint IDs to raw UUIDs for FM.
	srcTyped := data.SourceId.ValueString()
	destTyped := data.DestId.ValueString()

	srcRaw, err := commonutils.UUIDFromTypedID(srcTyped)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid source_id",
			fmt.Sprintf("source_id %q is not a valid typed ID: %v", srcTyped, err),
		)
		return
	}

	destRaw, err := commonutils.UUIDFromTypedID(destTyped)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid dest_id",
			fmt.Sprintf("dest_id %q is not a valid typed ID: %v", destTyped, err),
		)
		return
	}

	// Derive source/dest endpoint types from typed IDs.
	srcType, err := fmTypeFromTypedID(srcTyped)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to resolve source endpoint type",
			fmt.Sprintf("failed to derive FM type from source_id %q: %s", srcTyped, err),
		)
		return
	}
	destType, err := fmTypeFromTypedID(destTyped)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to resolve destination endpoint type",
			fmt.Sprintf("failed to derive FM type from dest_id %q: %s", destTyped, err),
		)
		return
	}

	// Enforce source_aep_id semantics using typed source_id.
	if err := l.validateSourceAep(srcTyped, data.SourceAepId); err != nil {
		resp.Diagnostics.AddError("Invalid source_aep_id", err.Error())
		return
	}

	data.SourceType = types.StringValue(srcType)
	data.DestType = types.StringValue(destType)

	// source_aep_id is Optional+Computed. If the user didn't provide it, the
	// plan leaves it unknown. validateSourceAep already enforces it is set when
	// required (map/LB source) and absent otherwise, so resolving unknown → null
	// here is always correct and prevents Terraform from rejecting the result.
	if data.SourceAepId.IsUnknown() {
		data.SourceAepId = types.Int32Null()
	}

	fmLink := l.createFMStruct(&data, srcRaw, destRaw)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "link",
				Operation:  "create",
				Link:       fmLink,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		msId,
		l.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create link",
			fmt.Sprintf("link creation failed: %s", err),
		)
		return
	}

	data.Id = types.StringValue(id)

	// State is fully determined from the plan and create response.
	// Skip post-create FM read to avoid eventual-consistency races.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// GetMSLinkData fetches a single link from the Monitoring Session's links[] array.
// On success, it copies the link into linkData and returns nil.
// If the link is not found, it returns a generic ObjectNotFound FM error
func GetMSLinkData(
	ctx context.Context,
	monitoringSessId, linkId string,
	linkData *FMLink,
	fmClient *fmclient.FmClient,
) error {
	fmResp := struct {
		Id    string   `json:"id,omitempty"`
		Links []FMLink `json:"links"`
	}{
		Id: monitoringSessId,
	}

	if err := UpdateMSData(ctx, monitoringSessId, &fmResp, fmClient); err != nil {
		return fmt.Errorf("failed to get monitoring session %q links: %w", monitoringSessId, err)
	}

	for _, link := range fmResp.Links {
		if link.Id == "" {
			return fmt.Errorf("monitoring session %q contains link with empty id field", monitoringSessId)
		}

		if linkId == "" || linkId == link.Id {
			// Copy the found link into the caller-provided struct.
			*linkData = link
			return nil
		}
	}

	// No matching link found: use generic ObjectNotFound with contextual message.
	return fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf(
			"monitoring session link not found: monitoring_session_id=%s link_id=%s",
			monitoringSessId,
			linkId,
		),
		nil,
	)
}

// GetMSLinks fetches all links from the Monitoring Session's links[] array.
func GetMSLinks(
	ctx context.Context,
	monitoringSessId string,
	fmClient *fmclient.FmClient,
) ([]FMLink, error) {
	fmResp := struct {
		Id    string   `json:"id,omitempty"`
		Links []FMLink `json:"links"`
	}{
		Id: monitoringSessId,
	}

	if err := UpdateMSData(ctx, monitoringSessId, &fmResp, fmClient); err != nil {
		return nil, fmt.Errorf("failed to get monitoring session %q links: %w", monitoringSessId, err)
	}

	return fmResp.Links, nil
}

// Read refreshes the link state from FM.
func (l *Link) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LinkModel

	// Read Terraform prior state data into the model.
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	linkData := FMLink{}

	err := GetMSLinkData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		&linkData,
		l.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			// Link no longer exists in FM; remove from state (idempotent read).
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Unable to get link details from Monitoring Session",
			fmt.Sprintf("unable to get link details. error is %s", err),
		)
		return
	}

	l.updateTFStruct(&data, &linkData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update modifies an existing link. For now, we treat links as create/delete only.
// replace mention it instead of throwing error or we can update same link with same id - check
func (l *Link) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Link does not support in-place modifications",
		"Links can only be created/deleted. Please recreate the resource if changes are needed.",
	)
}

// Delete removes the link from the Monitoring Session.
func (l *Link) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LinkModel

	// Read Terraform prior state data into the model.
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	deletePayload := struct {
		Id string `json:"id"`
	}{
		Id: data.Id.ValueString(),
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "link",
				Operation:  "delete",
				Link:       deletePayload,
			},
		},
	}

	_, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		l.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			// Link does not exist in FM; delete is idempotent.
			return
		}

		resp.Diagnostics.AddError(
			"Unable to delete link",
			fmt.Sprintf("link deletion failed: %s", err),
		)
	}
}
