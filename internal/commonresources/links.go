// Copyright (c) Gigamon, Inc.

// Implements the Link resources that connect Maps / Applications / Tunnels
// inside a Monitoring Session.

package commonresources

import (
	"context"
	"encoding/json"
	"fmt"

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

	Alias types.String `tfsdk:"alias"`
	Id    types.String `tfsdk:"id"`

	SourceId types.String `tfsdk:"source_id"`

	// SourceType is computed from FM based on SourceId; users normally don't set this.
	SourceType  types.String `tfsdk:"source_type"`   // "map", "application", "tunnel", "raw", ...
	SourceAepId types.Int32  `tfsdk:"source_aep_id"` // optional; used when source is a map (aepId in ruleset)

	DestId types.String `tfsdk:"dest_id"`

	// DestType is computed from FM based on DestId; users normally don't set this.
	DestType types.String `tfsdk:"dest_type"` // "map", "application", "tunnel", "raw", ...
}

// FMLinkEndpoint is a single endpoint (source or dest) in the FM link JSON.
type FMLinkEndpoint struct {
	Id    string `json:"id"`
	Type  string `json:"type"`
	AepId *int32 `json:"aepId,omitempty"`
}

// FMLink is the FM representation of a link in /cloud/monitoringSessions/{id}.
type FMLink struct {
	Id     string          `json:"id,omitempty"`
	Alias  string          `json:"alias,omitempty"`
	Source *FMLinkEndpoint `json:"source,omitempty"`
	Dest   *FMLinkEndpoint `json:"dest,omitempty"`
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

			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias/name for this link. Empty string is allowed.",
				Optional:            true,
				Computed:            true,
				// No default: empty if not set.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"source_type": schema.StringAttribute{
				MarkdownDescription: "Type of the source object (map, application, tunnel, raw). " +
					"This is computed from FM based on source_id; users do not need to set it.",
				Optional: true,
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
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
			},

			"dest_id": schema.StringAttribute{
				MarkdownDescription: "ID of the destination object (map/application/tunnel/raw) for this link.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"dest_type": schema.StringAttribute{
				MarkdownDescription: "Type of the destination object (map, application, tunnel, raw). " +
					"This is computed from FM based on dest_id; users do not need to set it.",
				Optional: true,
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
	if sourceId == "" || destId == "" {
		return "", "", fmt.Errorf("source or destination id is empty")
	}

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

// createFMStruct converts the TF model into an FM link struct.
func (l *Link) createFMStruct(data *LinkModel) *FMLink {
	var srcAep *int32
	if !data.SourceAepId.IsNull() && !data.SourceAepId.IsUnknown() {
		v := data.SourceAepId.ValueInt32()
		srcAep = &v
	}

	return &FMLink{
		Id:    data.Id.ValueString(),
		Alias: data.Alias.ValueString(),
		Source: &FMLinkEndpoint{
			Id:    data.SourceId.ValueString(),
			Type:  data.SourceType.ValueString(),
			AepId: srcAep,
		},
		Dest: &FMLinkEndpoint{
			Id:   data.DestId.ValueString(),
			Type: data.DestType.ValueString(),
		},
	}
}

// updateTFStruct copies FM link data into the TF state model.
func (l *Link) updateTFStruct(data *LinkModel, fmData *FMLink) {
	data.Alias = types.StringValue(fmData.Alias)

	if fmData.Source != nil {
		data.SourceId = types.StringValue(fmData.Source.Id)
		data.SourceType = types.StringValue(fmData.Source.Type)
		if fmData.Source.AepId != nil {
			data.SourceAepId = types.Int32Value(*fmData.Source.AepId)
		} else {
			data.SourceAepId = types.Int32Null()
		}
	} else {
		data.SourceId = types.StringNull()
		data.SourceType = types.StringNull()
		data.SourceAepId = types.Int32Null()
	}

	if fmData.Dest != nil {
		data.DestId = types.StringValue(fmData.Dest.Id)
		data.DestType = types.StringValue(fmData.Dest.Type)
	} else {
		data.DestId = types.StringNull()
		data.DestType = types.StringNull()
	}

	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}

// Create creates a new link inside the Monitoring Session.
func (l *Link) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LinkModel

	// Read Terraform plan data into the model.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	msId := data.MonitoringSessionId.ValueString()

	// If source_type / dest_type are not set by the user, derive them from FM
	// based on source_id / dest_id. These attributes are Optional+Computed, so
	// most users will leave them empty.
	needSrc := data.SourceType.IsUnknown() || data.SourceType.IsNull() || data.SourceType.ValueString() == ""
	needDst := data.DestType.IsUnknown() || data.DestType.IsNull() || data.DestType.ValueString() == ""

	if needSrc || needDst {
		srcType, destType, err := resolveEndpointTypes(
			ctx,
			l.fmClient,
			msId,
			data.SourceId.ValueString(),
			data.DestId.ValueString(),
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to resolve link endpoint types",
				fmt.Sprintf("failed to resolve source/dest types for link endpoints in Monitoring Session %q: %s", msId, err),
			)
			return
		}

		if needSrc {
			data.SourceType = types.StringValue(srcType)
		}
		if needDst {
			data.DestType = types.StringValue(destType)
		}
	}

	fmLink := l.createFMStruct(&data)

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

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// GetMSLinkData fetches a single link from the Monitoring Session's links[] array
func GetMSLinkData(
	ctx context.Context,
	monitoringSessId, linkId, linkAlias string,
	linkData any,
	fmClient *fmclient.FmClient,
) (bool, error) {

	fmResp := struct {
		Alias string           `json:"alias"`
		Id    string           `json:"id,omitempty"`
		Links []map[string]any `json:"links"`
	}{
		Id: monitoringSessId,
	}

	err := UpdateMSData(ctx, monitoringSessId, &fmResp, fmClient)
	if err != nil {
		return false, err
	}

	for _, link := range fmResp.Links {
		fmLinkId, ok := link["id"].(string)
		if !ok {
			return false, fmt.Errorf("unable to get the id of the link")
		}
		fmLinkAlias, _ := link["alias"].(string) // alias can be empty string

		if (linkId == "" || linkId == fmLinkId) &&
			(linkAlias == "" || linkAlias == fmLinkAlias) {

			jsonData, err := json.Marshal(link)
			if err != nil {
				return false, err
			}

			if err := json.Unmarshal(jsonData, linkData); err != nil {
				return false, err
			}
			return true, nil
		}
	}

	return false, nil
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

	ok, err := GetMSLinkData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		data.Id.ValueString(),
		data.Alias.ValueString(),
		&linkData,
		l.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get link details from Monitoring Session",
			fmt.Sprintf("unable to get link details. error is %s", err),
		)
		return
	}
	if !ok {
		// Link no longer exists in FM; remove from state.
		resp.State.RemoveResource(ctx)
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

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "link",
				Operation:  "delete",
				Link: FMLink{
					Id: data.Id.ValueString(),
				},
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
		resp.Diagnostics.AddError(
			"Unable to delete link",
			fmt.Sprintf("link deletion failed: %s", err),
		)
	}
}
