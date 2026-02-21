// Copyright (c) Gigamon, Inc.

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
var _ resource.Resource = &Link{}

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
				MarkdownDescription: "AEP ID of the source, when source is a map (optional).",
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

type endpointRef struct {
	Id string `json:"id"`
}

type msEndpoints struct {
	Id string `json:"id,omitempty"`

	TrafficMaps  []endpointRef `json:"trafficMaps"`
	Applications []endpointRef `json:"applications"`
	Tunnels      []endpointRef `json:"tunnels"`
	RawEndPoints []endpointRef `json:"rawEndPoints"`
}

// resolveEndpointTypes determines the FM endpoint type strings ("map", "application",
// "tunnel", "raw") for the given source and dest IDs by inspecting the Monitoring Session
// once. It returns srcType, destType.
func resolveEndpointTypes(
	ctx context.Context,
	fmClient *fmclient.FmClient,
	monitoringSessId string,
	sourceId string,
	destId string,
) (string, string, error) {
	resp := msEndpoints{
		Id: monitoringSessId,
	}

	if err := UpdateMSData(ctx, monitoringSessId, &resp, fmClient); err != nil {
		return "", "", err
	}

	var srcType, destType string

	// Helper closure to check and fill src/dest types for a given id/type label.
	check := func(list []endpointRef, t string) {
		for _, e := range list {
			if e.Id == sourceId && srcType == "" {
				srcType = t
			}
			if e.Id == destId && destType == "" {
				destType = t
			}
			if srcType != "" && destType != "" {
				return
			}
		}
	}

	check(resp.Applications, "application")
	check(resp.Tunnels, "tunnel")
	check(resp.TrafficMaps, "map")
	check(resp.RawEndPoints, "raw")

	if srcType == "" {
		return "", "", fmt.Errorf("unable to determine source endpoint type for id %q in monitoring session %q", sourceId, monitoringSessId)
	}
	if destType == "" {
		return "", "", fmt.Errorf("unable to determine destination endpoint type for id %q in monitoring session %q", destId, monitoringSessId)
	}

	return srcType, destType, nil
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
	// If user didn't set it, nothing to do.
	if srcAep.IsNull() || srcAep.IsUnknown() {
		return nil
	}

	parts, err := commonutils.ParseTypedID(srcTyped)
	if err != nil {
		return fmt.Errorf(
			"source_aep_id is only valid when source_id is a typed map or load balancing app id; got %q",
			srcTyped,
		)
	}

	allowed := false

	switch parts.Module {
	case commonutils.ModuleMap:
		allowed = true
	case commonutils.ModuleApp:
		if parts.Type == commonutils.TypeLoadBalancing {
			allowed = true
		}
	}

	if !allowed {
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
		return
	}

	destRaw, err := commonutils.UUIDFromTypedID(destTyped)
	if err != nil {
		return
	}

	// Resolve source_type / dest_type from FM using raw IDs.
	srcType, destType, err := resolveEndpointTypes(
		ctx,
		l.fmClient,
		msId,
		srcRaw,
		destRaw,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to resolve link endpoint types",
			fmt.Sprintf("failed to resolve source/dest types for link endpoints in Monitoring Session %q: %s", msId, err),
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

	var linkData FMLink
	if err := GetMSLinkData(ctx, msId, id, &linkData, l.fmClient); err != nil {
		resp.Diagnostics.AddError(
			"Unable to read created link from Monitoring Session",
			fmt.Sprintf("link created but fetching details failed: %s", err),
		)
		return
	}

	l.updateTFStruct(&data, &linkData)
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
