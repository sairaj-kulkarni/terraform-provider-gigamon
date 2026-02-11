// Copyright (c) Gigamon, Inc.

// Implements the Resrouces that are common across all environment i.e. MS and other such

package commonresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MonSess{}

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
}

// FM response for MS Creation/Get, specifying only the fields relevant to post of MS creation

type FMMonSess struct {
	Alias              string   `json:"alias"`
	Id                 string   `json:"id,omitempty"`
	ConnectionId       []string `json:"connIds"`
	MonitoringDomainId string   `json:"monitoringDomainId"`
	Description        string   `json:"description"`
	Platform           string   `json:"platform"`
	Deployed           bool     `json:"deployed"`
}

func (ms *MonSess) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_monitoring_session"
}

func (ms *MonSess) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Esxi Monitoring Session",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the monitoring session",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
				MarkdownDescription: "descirption for the monitoring sessions",
				Optional:            true,
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
	data.Id = types.StringValue(fmMSResp.Id)
	data.Deployed = types.BoolValue(false)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Updates the given monitoring session details and returns the data requested by the caller
func UpdateMSData(
	ctx context.Context,
	monitoringSessId string,
	fmResp any,
	fmClient *fmclient.FmClient,
) error {

	fmMSData := struct {
		MonitoringSession any `json:"monitoringSession"`
	}{
		MonitoringSession: fmResp,
	}
	respData, err := fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s", monitoringSessId),
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
		return fmt.Errorf("unable to convert resp to struct: %s error is: %s", string(respData), err)
	}

	return nil
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
	data.Id = types.StringValue(fmResp.Id)
	data.Deployed = types.BoolValue(fmResp.Deployed)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (ms *MonSess) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Esxi Monitoring Session does not support any modifications",
		"ESXI Montitoring Session  can only be created/deleted. They cannot be modified",
	)
}

func (ms *MonSess) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MonSessModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := ms.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s", data.Id.ValueString()),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete the Monitoring Session from FM",
			fmt.Sprintf("Unable to delete monitoring Session: %s (%s) error is: %v", data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
}
