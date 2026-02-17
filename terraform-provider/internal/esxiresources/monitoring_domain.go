// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI cloud Monitoring Domain

package esxiresources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiMD{}
var _ resource.ResourceWithImportState = &EsxiMD{}

// Esxi MD resoruce, which manages the images for ESXI platform
func NewEsxiMD() resource.Resource {
	return &EsxiMD{}
}

// EsxiMD manages the MD for the ESXI platform
type EsxiMD struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiMDModel describes the resource data model.
type EsxiMDModel struct {
	Alias                       types.String `tfsdk:"alias"`
	Platform                    types.String `tfsdk:"platform"`
	UserLaunched                types.Bool   `tfsdk:"user_launched"`
	UsePublicIpForNotifications types.Bool   `tfsdk:"use_public_ip_for_notifications"`
	ConnectionId                types.String `tfsdk:"connection_id"`
	Id                          types.String `tfsdk:"id"`
}

type EsxiMDConn struct {
	Id    string `json:"id,omitempty"`
	Alias string `json:"alias,omitempty"`
}

// FM request/response for Monitoring Domains
type EsxiFmMD struct {
	Alias                       string       `json:"alias,omitempty"`
	Platform                    string       `json:"platform,omitempty"`
	UserLaunched                bool         `json:"userLaunched,omitempty"`
	UsePublicIpForNotifications bool         `json:"usePublicIpForNotifications"`
	ConnectionIds               []string     `json:"connIds,omitempty"`     // Used when we post/patch request
	GetConnectionIds            []EsxiMDConn `json:"connections,omitempty"` // Use in the Get only
	Id                          string       `json:"id,omitempty"`
}

func (md *EsxiMD) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_monitoring_domain"
}

func (md *EsxiMD) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Esxi Monitoring Domain",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the monitoring domain",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"platform": schema.StringAttribute{
				MarkdownDescription: "Platform on which the monitoring domain has been created",
				Computed:            true,
			},
			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID associated with this MD",
				Computed:            true,
				Optional:            true,
			},
			"user_launched": schema.BoolAttribute{
				MarkdownDescription: "true indicates that the vseries nodes are launched and managed by the user. Default false",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"use_public_ip_for_notifications": schema.BoolAttribute{
				MarkdownDescription: "Set the destination IP to public address for Vseries to send its event notifications",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
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

func (md *EsxiMD) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (md *EsxiMD) getMDByName(ctx context.Context, alias string) (*EsxiFmMD, error) {

	fmMDData := struct {
		MonitoringDomains []EsxiFmMD `json:"monitoringDomains"`
	}{
		MonitoringDomains: make([]EsxiFmMD, 10),
	}

	mdResp, err := md.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/monitoringDomains",
		map[string]string{"platform": "vmware"},
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
			"Unable to convert MD Get resp to struct: %s error is: %w",
			string(mdResp),
			err,
		)
	}
	for _, mdDetails := range fmMDData.MonitoringDomains {
		if mdDetails.Alias == alias {
			return &mdDetails, nil
		}
	}
	return nil, fmclient.NewFMError(
		fmclient.ObjectNotFound,
		fmt.Sprintf("unable to find MD by name: %s", alias),
		err,
	)
}

func (md *EsxiMD) getMDByID(ctx context.Context, id string) (*EsxiFmMD, error) {
	fmMDData := struct {
		MonitoringDomain EsxiFmMD `json:"monitoringDomain"`
	}{}

	//Extract Raw UUID from TypedId
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
			"Unable to convert MD resp to struct: %s error is: %w",
			string(mdResp),
			err,
		)
	}
	return &fmMDData.MonitoringDomain, nil
}

// Given the MD Alias / ID, get details from FM and updates the TF state
func (md *EsxiMD) updateMD(ctx context.Context, data *EsxiMDModel, alias, id string) error {

	var err error
	var mdDetails *EsxiFmMD
	if alias != "" {
		mdDetails, err = md.getMDByName(ctx, alias)
	} else {
		mdDetails, err = md.getMDByID(ctx, id)
	}
	if err != nil {
		return err
	}

	//Make TypeID from raw UUID recieved from FM
	typedID, err := commonutils.MakeTypedID(commonutils.ModuleMonitoringDomain, commonutils.TypeVMWareESXi, mdDetails.Id)
	if err != nil {
		return err
	}
	data.Id = types.StringValue(typedID)

	data.Alias = types.StringValue(mdDetails.Alias)
	data.Platform = types.StringValue(mdDetails.Platform)
	data.UserLaunched = types.BoolValue(mdDetails.UserLaunched)
	data.UsePublicIpForNotifications = types.BoolValue(mdDetails.UsePublicIpForNotifications)
	if len(mdDetails.GetConnectionIds) != 0 {
		//Make TypeID from raw UUID recieved from FM
		typedID, err := commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeVMWareESXi, mdDetails.GetConnectionIds[0].Id)
		if err != nil {
			return err
		}
		data.ConnectionId = types.StringValue(typedID)
	} else {
		data.ConnectionId = types.StringValue("Unknown")
	}
	return nil
}

func (md *EsxiMD) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EsxiMDModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmMDData := EsxiFmMD{
		Alias:                       data.Alias.ValueString(),
		Platform:                    "vmwareEsxi",
		UsePublicIpForNotifications: data.UsePublicIpForNotifications.ValueBool(),
		UserLaunched:                data.UserLaunched.ValueBool(),
	}

	jsonData, err := json.Marshal(fmMDData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct to JSON",
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
		"api/v1.3/cloud/monitoringDomains/",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the monitoring domain",
			fmt.Sprintf("Monitoring Domain Creaet: %v error is: %v", fmMDData, err),
		)
		return
	}

	err = md.updateMD(ctx, &data, fmMDData.Alias, "")
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get the updated data on MD from FM",
			fmt.Sprintf("%v", err),
		)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (md *EsxiMD) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := md.updateMD(ctx, &data, "", data.Id.ValueString())
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to get Monitoring Domain details",
			fmt.Sprintf("unable to get Monitoring Domain details. error is %v", err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (md *EsxiMD) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData EsxiMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	//Extract Raw UUID from TypedId
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
		MonitoringDomains []EsxiFmMD `json:"monitoringDomains"`
	}{
		MonitoringDomains: []EsxiFmMD{
			{
				Platform:                    stateData.Platform.ValueString(),
				ConnectionIds:               []string{connId},
				Id:                          mdId,
				UsePublicIpForNotifications: planData.UsePublicIpForNotifications.ValueBool(),
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
			"Unable to update the monitoring domain",
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
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
}

func (md *EsxiMD) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	//Extract Raw UUID from TypedId
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
			"Unable to Delete the Monitoring Domain from FM",
			fmt.Sprintf("Unable to delete monitoring domain: %s (%s) error is: %v", data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
}

// Allows the user to import the existing configuration into their TF files. If the id is
// sufficient for the Read function to get hte current state, than just simply call the
// ImportStatePassThroughID. Otherwise set things up so that the read function can get the
// details, and populate that into the data in resp state
func (md *EsxiMD) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {

	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
