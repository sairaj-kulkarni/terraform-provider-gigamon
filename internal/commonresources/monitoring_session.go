// Copyright (c) Gigamon, Inc.

// Implements the Resrouces that are common across all environment i.e. MS and other such

package commonresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MonSess{}
var _ resource.ResourceWithImportState = &MonSess{}

// MonSess MS resoruce, which manages the MS for all cloud platforms
func NewMonSess() resource.Resource {
	return &MonSess{}
}

// MonSess manages the MS for all cloud platform
type MonSess struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// MonSessModel describes the TF model for the MS
type MonSessModel struct {
	ConnectionId       types.String `tfsdk:"connection_id"`
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	Alias              types.String `tfsdk:"alias"`
	Description        types.String `tfsdk:"description"`
	Id                 types.String `tfsdk:"id"`
	Deployed           types.Bool   `tfsdk:"deployed"`
	DeploymentStatus   types.String `tfsdk:"deployment_status"`
}

// FM response for MS Creation/Get, specifying only the fields relevant to post of MS creation

type FMMonSess struct {
	Alias              string   `json:"alias"`
	Id                 string   `json:"id,omitempty"`
	ConnectionId       []string `json:"connIds"`
	MonitoringDomainId string   `json:"monitoringDomainId"`
	Description        string   `json:"description,omitempty"`
	Platform           string   `json:"platform"`
	Deployed           bool     `json:"deployed"`
	DeploymentStatus   string   `json:"deployStatus,omitempty"`
}

func (ms *MonSess) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitoring_session"
}

func (ms *MonSess) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Monitoring Session",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the monitoring session",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID to use in this monitoring session",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"monitoring_domain_id": schema.StringAttribute{
				MarkdownDescription: "monitoring domain ID to use in this monitoring session",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description for the monitoring sessions",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Session for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"deployed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the MS has been deployed or not",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"deployment_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Deployment status of the monitoring session (e.g. deploymentSuccess / deploymentFailure)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (ms *MonSess) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	ms.fmClient = fmClient
}

// Create call for new monitoring session
func (ms *MonSess) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MonSessModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	//Extract Raw UUID from TypedId
	mdId, err := commonutils.UUIDFromTypedID(data.MonitoringDomainId.ValueString())
	if err != nil {
		return
	}
	platformType, err := commonutils.TypeFromTypedID(data.MonitoringDomainId.ValueString())
	if err != nil {
		return
	}
	connId, err := commonutils.UUIDFromTypedID(data.ConnectionId.ValueString())
	if err != nil {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmMSData := FMMonSess{
		Alias:              data.Alias.ValueString(),
		Platform:           string(platformType),
		ConnectionId:       []string{connId},
		MonitoringDomainId: mdId,
		Description:        data.Description.ValueString(),
	}

	jsonData, err := json.Marshal(fmMSData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct to JSON",
			fmt.Sprintf("converting: %v error is: %v", fmMSData, err),
		)
		return
	}

	respData, err := ms.fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/monitoringSessions/",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the monitoring session",
			fmt.Sprintf("Monitoring session Creaet: %v error is: %v", fmMSData, err),
		)
		return
	}

	var fmMSResp FMMonSess
	err = json.Unmarshal(respData, &fmMSResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert MS Create Resp to struct",
			fmt.Sprintf("unable to get MS data: %s error is %v", string(respData), err),
		)
		return
	}

	// Build typed ID for Monitoring Session: monitoringSession::<platformType>::<uuid>
	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleMonitoringSess,
		platformType, // already computed earlier from MonitoringDomainId
		fmMSResp.Id,  // raw UUID from FM
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	data.Deployed = types.BoolValue(false)
	data.DeploymentStatus = types.StringValue(fmMSResp.DeploymentStatus)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Updates the given monitoring session details and returns the data requested by the caller
func UpdateMSData(
	ctx context.Context,
	monitoringSessId string,
	fmResp any,
	fmClient *fmclient.FmClient,
) error {

	rawID, err := commonutils.UUIDFromTypedID(monitoringSessId)
	if err != nil {
		return err
	}

	fmMSData := struct {
		MonitoringSession any `json:"monitoringSession"`
	}{
		MonitoringSession: fmResp,
	}
	respData, err := fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s", rawID),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return err
	}

	err = json.Unmarshal(respData, &fmMSData)
	if err != nil {
		return fmt.Errorf("unable to convert resp to struct: %s error is: %w", string(respData), err)
	}

	return nil
}

// getMSByAlias looks up a Monitoring Session by alias using the FM list API.
func (ms *MonSess) getMSByAlias(
	ctx context.Context,
	alias string,
) (*FMMonSess, error) {

	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil, fmt.Errorf("monitoring session alias cannot be empty")
	}

	fmResp := struct {
		MonitoringSessions []FMMonSess `json:"monitoringSessions"`
	}{}

	respBytes, err := ms.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/monitoringSessions",
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list monitoring sessions: %w", err)
	}

	if err := json.Unmarshal(respBytes, &fmResp); err != nil {
		return nil, fmt.Errorf(
			"unable to convert monitoring sessions list to struct: %s error is: %w",
			string(respBytes), err,
		)
	}

	for i := range fmResp.MonitoringSessions {
		if fmResp.MonitoringSessions[i].Alias == alias {
			return &fmResp.MonitoringSessions[i], nil
		}
	}

	return nil, fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf("monitoring session not found for alias %q", alias),
		nil,
	)
}

func (ms *MonSess) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MonSessModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	fmResp := FMMonSess{}
	err := UpdateMSData(ctx, data.Id.ValueString(), &fmResp, ms.fmClient)
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Rebuild typed ID
	platformType, err := commonutils.TypeFromTypedID(data.MonitoringDomainId.ValueString())
	if err != nil {
		return
	}
	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleMonitoringSess,
		platformType,
		fmResp.Id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	data.Alias = types.StringValue(fmResp.Alias)

	data.Deployed = types.BoolValue(fmResp.Deployed)
	data.DeploymentStatus = types.StringValue(fmResp.DeploymentStatus)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (ms *MonSess) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state MonSessModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get raw MS ID from typed state.Id.
	rawMSID, err := commonutils.UUIDFromTypedID(state.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Monitoring Session ID", err.Error())
		return
	}

	// Derive platform from monitoring_domain_id typed ID.
	platformType, err := commonutils.TypeFromTypedID(state.MonitoringDomainId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Monitoring Domain ID", err.Error())
		return
	}

	// Build PATCH payload directly from the plan.
	payload := struct {
		Alias       string `json:"alias,omitempty"`
		Description string `json:"description,omitempty"`
		Platform    string `json:"platform"`
	}{
		Alias:    plan.Alias.ValueString(),
		Platform: string(platformType),
	}

	if !plan.Description.IsNull() {
		payload.Description = plan.Description.ValueString()
	}

	body, _ := json.Marshal(payload)

	// PATCH in FM.
	_, err = ms.fmClient.DoRequest(
		ctx,
		"PATCH",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s", rawMSID),
		nil,
		nil,
		bytes.NewBuffer(body),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Monitoring Session", err.Error())
		return
	}

	// Update state from plan and refresh deploy status.
	state.Alias = plan.Alias
	state.Description = plan.Description

	fmResp := FMMonSess{}
	if err := UpdateMSData(ctx, state.Id.ValueString(), &fmResp, ms.fmClient); err == nil {
		state.Deployed = types.BoolValue(fmResp.Deployed)
		state.DeploymentStatus = types.StringValue(fmResp.DeploymentStatus)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (ms *MonSess) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MonSessModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}
	msID := rawID

	_, err = ms.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s", msID),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete the Monitoring Session from FM",
			fmt.Sprintf("Unable to delete monitoring Session: %s (%s) error is: %v",
				data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
}

// ImportState implements terraform import for gigamon_monitoring_session.
//
// Expected import ID:
//   - Monitoring Session alias (must be unique), e.g. "demo-ms"
//
// Example:
//
//	terraform import gigamon_monitoring_session.my_ms demo-ms
func (ms *MonSess) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	alias := strings.TrimSpace(req.ID)
	if alias == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Import ID must be the monitoring session alias (non-empty).",
		)
		return
	}

	fmMS, err := ms.getMSByAlias(ctx, alias)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to import Monitoring Session",
			fmt.Sprintf("Failed to find monitoring session with alias %q: %v", alias, err),
		)
		return
	}

	platformType := commonutils.Type(fmMS.Platform)

	// Build typed IDs
	typedMSID, err := commonutils.MakeTypedID(
		commonutils.ModuleMonitoringSess,
		platformType,
		fmMS.Id,
	)
	if err != nil {
		return
	}

	typedMDID, err := commonutils.MakeTypedID(
		commonutils.ModuleMonitoringDomain,
		platformType,
		fmMS.MonitoringDomainId,
	)
	if err != nil {
		return
	}

	var typedConnID string
	if len(fmMS.ConnectionId) > 0 {
		typedConnID, err = commonutils.MakeTypedID(
			commonutils.ModuleConnection,
			platformType,
			fmMS.ConnectionId[0],
		)
		if err != nil {
			return
		}
	}

	// Read() will fill derived / computed fields.
	state := MonSessModel{
		Id:                 types.StringValue(typedMSID),
		MonitoringDomainId: types.StringValue(typedMDID),
		Alias:              types.StringValue(fmMS.Alias),
	}
	if typedConnID != "" {
		state.ConnectionId = types.StringValue(typedConnID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
