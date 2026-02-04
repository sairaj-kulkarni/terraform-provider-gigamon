// Copyright (c) Gigamon, Inc.

// Implements the Resources for the (Third Party Orchestratation) Any Cloud Monitoring Domain

package anycloudresources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AnyCloudMD{}
var _ resource.ResourceWithImportState = &AnyCloudMD{}

// AnyCloud MD resoruce, which manages the images for AnyCloud, (Third Party Orchestratation)
func NewAnyCloudMD() resource.Resource {
	return &AnyCloudMD{}
}

// AnyCloudMD manages the MD for AnyCloud
type AnyCloudMD struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// AnyCloudMDModel describes the resource data model.
type AnyCloudMDModel struct {
	Alias                types.String `tfsdk:"alias"`
	Platform             types.String `tfsdk:"platform"`
	UserLaunched         types.Bool   `tfsdk:"user_launched"`
	DualStackPreferIPv6  types.Bool   `tfsdk:"dual_stack_prefer_ipv6"`
	UniformTrafficPolicy types.Bool   `tfsdk:"uniform_traffic_policy"`
	MTU                  types.Int32  `tfsdk:"mtu"`
	ConnectionId         types.String `tfsdk:"connection_id"`
	Id                   types.String `tfsdk:"id"`
}

type AnyCloudMDConn struct {
	Id    string `json:"id,omitempty"`
	Alias string `json:"alias,omitempty"`
}

// FM request/response for Monitoring Domains
type AnyCloudFmMD struct {
	Alias                string           `json:"alias,omitempty"`
	Platform             string           `json:"platform,omitempty"`
	UserLaunched         bool             `json:"userLaunched,omitempty"`
	DualStackPreferIPv6  bool             `json:"dualStackPreferIPv6"`
	UniformTrafficPolicy bool             `json:"uniformTrafficPolicy,omitempty"`
	MTU                  int32            `json:"mtu"`
	ConnectionIds        []string         `json:"connIds,omitempty"`     // Used when we post/patch request
	GetConnectionIds     []AnyCloudMDConn `json:"connections,omitempty"` // Use in the Get only
	Id                   string           `json:"id,omitempty"`
}

func (md *AnyCloudMD) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anycloud_monitoring_domain"
}

func (md *AnyCloudMD) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Third Party Orchestratation (Any Cloud) Monitoring Domain",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the monitoring domain",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[A-Za-z0-9_-]+$`),
						`Invalid characters (Only alphanumeric, "-" and "_" are allowed) monitoring domain name`,
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"platform": schema.StringAttribute{
				MarkdownDescription: "Platform on which the monitoring domain has been created",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_launched": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates that the vseries nodes are launched and managed by the user. Default true",
				Computed:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"dual_stack_prefer_ipv6": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates IPv6 tunnels are preferred between UCT‑V and V Series nodes. Default false",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"uniform_traffic_policy": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates same monitoring session configuration is applied to all V Series Nodes in the monitoring domain. Default false",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"mtu": schema.Int32Attribute{
				MarkdownDescription: "MTU between UCT‑V and V Series nodes, when Traffic Acquisiotn method is UCT-V. Default value is 1450",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(1450),
				Validators: []validator.Int32{
					int32validator.Between(1280, 9000),
				},
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID associated with this MD",
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Domain for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (md *AnyCloudMD) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	md.fmClient = fmClient
}

func (md *AnyCloudMD) getMDByName(ctx context.Context, alias string) (*AnyCloudFmMD, error) {

	fmMDData := struct {
		MonitoringDomains []AnyCloudFmMD `json:"monitoringDomains"`
	}{}

	mdResp, err := md.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/monitoringDomains",
		map[string]string{"platform": "anyCloud"},
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(mdResp, &fmMDData)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert MD Get resp to struct: %s error is: %s",
			string(mdResp),
			err,
		)
	}

	for i := range fmMDData.MonitoringDomains {
		if fmMDData.MonitoringDomains[i].Alias == alias {
			return &fmMDData.MonitoringDomains[i], nil
		}
	}

	return nil, fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf("Unable to find anyCloud MD by name: %s", alias),
		err,
	)
}

func (md *AnyCloudMD) getMDByID(ctx context.Context, id string) (*AnyCloudFmMD, error) {
	fmMDData := struct {
		MonitoringDomain AnyCloudFmMD `json:"monitoringDomain"`
	}{}

	//Extract Raw UUID from TypedId for GET call
	rawID, err := commonutils.UUIDFromTypedID(id)
	if err != nil {
		return nil, err
	}

	mdResp, err := md.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains/%s", rawID),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(mdResp, &fmMDData)
	if err != nil {
		return nil, fmt.Errorf(
			"Unable to convert anyCloud MD resp to struct: %s error is: %s",
			string(mdResp),
			err,
		)
	}
	return &fmMDData.MonitoringDomain, nil
}

// Given the MD Alias / ID, get details from FM and updates the TF state
func (md *AnyCloudMD) updateMD(ctx context.Context, data *AnyCloudMDModel, alias, id string) error {

	var err error
	var mdDetails *AnyCloudFmMD
	if alias != "" {
		mdDetails, err = md.getMDByName(ctx, alias)
	} else {
		mdDetails, err = md.getMDByID(ctx, id)
	}
	if err != nil {
		return err
	}

	//Make TypeID from raw UUID recieved from FM
	typedID, err := commonutils.MakeTypedID(commonutils.ModuleMonitoringDomain, commonutils.TypeAnyCloud, mdDetails.Id)
	if err != nil {
		return err
	}
	data.Id = types.StringValue(typedID)

	data.Alias = types.StringValue(mdDetails.Alias)
	data.Platform = types.StringValue(mdDetails.Platform)
	data.UserLaunched = types.BoolValue(mdDetails.UserLaunched)
	data.DualStackPreferIPv6 = types.BoolValue(mdDetails.DualStackPreferIPv6)
	data.MTU = types.Int32Value(mdDetails.MTU)
	data.UniformTrafficPolicy = types.BoolValue(mdDetails.UniformTrafficPolicy)

	if len(mdDetails.GetConnectionIds) != 0 {
		//Make TypeID from raw UUID recieved from FM
		typedID, err := commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeAnyCloud, mdDetails.GetConnectionIds[0].Id)
		if err != nil {
			return err
		}
		data.ConnectionId = types.StringValue(typedID)
	} else {
		data.ConnectionId = types.StringValue("Unknown")
	}

	return nil
}

func (md *AnyCloudMD) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AnyCloudMDModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmMDData := AnyCloudFmMD{
		Alias:               data.Alias.ValueString(),
		Platform:            "anyCloud",
		UserLaunched:        true,
		DualStackPreferIPv6: data.DualStackPreferIPv6.ValueBool(),
		MTU:                 data.MTU.ValueInt32(),
	}

	jsonData, err := json.Marshal(fmMDData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert AnyCloudFmMD struct to JSON",
			fmt.Sprintf("converting: %v error is: %v", fmMDData, err),
		)
		return
	}

	tflog.Info(ctx, "Creating monitoring domain", map[string]any{
		"struct":   fmMDData,
		"jsonBody": string(jsonData),
	})

	_, err = md.fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/monitoringDomains",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the monitoring domain for anyCloud",
			fmt.Sprintf("Monitoring Domain Create: %v error is: %v", fmMDData, err),
		)
		return
	}

	err = md.updateMD(ctx, &data, fmMDData.Alias, "")
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get the updated data on anyCloud MD from FM",
			fmt.Sprintf("%v", err),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (md *AnyCloudMD) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AnyCloudMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ID should always be present in state for Read
	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Missing AnyCloud Monitoring Domain ID",
			"Cannot read AnyCloud Monitoring Domain because 'id' is null/unknown/empty in state.",
		)
		return
	}

	err := md.updateMD(ctx, &data, "", data.Id.ValueString())
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Unable to read AnyCloud Monitoring Domain",
			fmt.Sprintf("Failed to read AnyCloud Monitoring Domain (id=%s): %v", data.Id.ValueString(), err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (md *AnyCloudMD) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData AnyCloudMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate ID
	if stateData.Id.IsNull() || stateData.Id.IsUnknown() || stateData.Id.ValueString() == "" {
		resp.Diagnostics.AddError("Missing AnyCloud MD ID", "Cannot update because 'id' is missing in state.")
		return
	}

	//Extract Raw UUID from TypedId for GET call
	typedId := stateData.Id.ValueString()
	mdId, err := commonutils.UUIDFromTypedID(typedId)
	if err != nil {
		return
	}
	connId, err := commonutils.UUIDFromTypedID(stateData.ConnectionId.ValueString())
	if err != nil {
		return
	}

	fmMDData := struct {
		MonitoringDomains []AnyCloudFmMD `json:"monitoringDomains"`
	}{
		MonitoringDomains: []AnyCloudFmMD{
			{
				Platform:            stateData.Platform.ValueString(),
				ConnectionIds:       []string{connId},
				Id:                  mdId,
				DualStackPreferIPv6: planData.DualStackPreferIPv6.ValueBool(),
				MTU:                 planData.MTU.ValueInt32(),
			},
		},
	}

	if connId == "Unknown" {
		fmMDData.MonitoringDomains[0].ConnectionIds = nil
	}
	jsonData, err := json.Marshal(fmMDData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct to JSON",
			fmt.Sprintf("converting: %v error is: %v", fmMDData, err),
		)
		return
	}

	tflog.Info(ctx, "Updating monitoring domain", map[string]any{
		"struct":   fmMDData,
		"jsonBody": string(jsonData),
	})

	_, err = md.fmClient.DoRequest(
		ctx,
		"PATCH",
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains/%s", mdId),
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update the any cloud monitoring domain",
			fmt.Sprintf("Monitoring Domain update: %v error is: %v", fmMDData, err),
		)
		return
	}

	err = md.updateMD(ctx, &stateData, "", typedId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get the updated data on MD from FM",
			fmt.Sprintf("%v", err),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
}

func (md *AnyCloudMD) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AnyCloudMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If ID is missing, nothing to delete
	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		return
	}

	//Extract Raw UUID from TypedId for GET call
	mdId, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	_, err = md.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains/%s", mdId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete AnyCloud Monitoring Domain",
			fmt.Sprintf("Unable to delete monitoring domain: %s (%s) error is: %v", data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
}

func (md *AnyCloudMD) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
}
