// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI cloud Monitoring Domain

package esxiresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"

)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiMD{}

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
	Alias types.String `tfsdk:"alias"`
	Platform types.String `tfsdk:"platform"`
	UserLaunched types.Bool `tfsdk:"user_launched"`
	UsePublicIpForNotifications types.Bool `tfsdk:"use_public_ip_for_notifications"`
	Id types.String `tfsdk:"id"`
}

// FM response for images
type EsxiFmMD struct {
	Alias string `json:"alias"`
	Platform string `json:"platform"`
	UserLaunched bool `json:"userLaunched"`
	UsePublicIpForNotifications bool `json:"usePublicIpForNotifications"`
	Id string `json:"id,omitempty"`
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
				Computed: true,
			},
			"user_launched": schema.BoolAttribute{
				MarkdownDescription: "true indicates that the vseries nodes are launched and managed by the user. Default false",
				Optional: true,
				Computed: true,
				Default: booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
                    boolplanmodifier.RequiresReplace(),
                },
			},
			"use_public_ip_for_notifications": schema.BoolAttribute{
				MarkdownDescription: "Set the destination IP to public address for Vseries to send its event notifications",
				Optional: true,
				Computed: true,
				Default: booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
                    boolplanmodifier.RequiresReplace(),
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

// Given the MD Alias, gets the details from FM and updates the TF state with the latest values
func (md *EsxiMD) readAndUpdate(ctx context.Context, data *EsxiMDModel, alias string) error {

	fmMDData := struct {
		MonitoringDomains []EsxiFmMD `json:"monitoringDomains"`
	} {
		MonitoringDomains: make([]EsxiFmMD, 10),
	}

	mdResp, err := md.fmClient.DoRequest(
		ctx,
		"GET",
		0,
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains"),
		map[string]string {"platform": "vmware"},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("Get request of Vmware MDs failed: %s: %s", alias, err)
	}

	err = json.Unmarshal(mdResp, &fmMDData)
	if err != nil {
		return fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(mdResp), err)
	}

	// save into the Terraform state.
	for _, mdDetails := range fmMDData.MonitoringDomains {
		if mdDetails.Alias == alias {
	        data.Id = types.StringValue(mdDetails.Id)
	        data.Alias = types.StringValue(mdDetails.Alias)
	        data.Platform = types.StringValue(mdDetails.Platform)
            data.UserLaunched = types.BoolValue(mdDetails.UserLaunched)
	        data.UsePublicIpForNotifications = types.BoolValue(mdDetails.UsePublicIpForNotifications)
			return nil
		}
	}
	return fmt.Errorf("Unable to find %s in FM Response %s and JSON Struct %v", alias, string(mdResp), fmMDData)
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
		Alias: data.Alias.ValueString(),
		Platform: "vmwareEsxi",
		UsePublicIpForNotifications: data.UsePublicIpForNotifications.ValueBool(),
		UserLaunched: data.UserLaunched.ValueBool(),
	}
	
	jsonData, err := json.Marshal(fmMDData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct to JSON",
			fmt.Sprintf("converting: %v error is: %s", fmMDData,  err),
		)
		return
	}

	tflog.Info(ctx, "Creating monitoring domain", map[string]any{
		"struct": fmMDData,
		"jsonBody": string(jsonData),
	})

	_, err = md.fmClient.DoRequest(
		ctx,
		"POST",
		0,
		"api/v1.3/cloud/monitoringDomains/",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the monitoring domain",
			fmt.Sprintf("Monitoring Domain Creaet: %v error is: %s", fmMDData, err),
		)
		return
	}

    err = md.readAndUpdate(ctx, &data, fmMDData.Alias)
	if err != nil {
        resp.Diagnostics.AddError(
             "Could not get the updated data on MD from FM",
		     fmt.Sprintf("%s", err),
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

	err := md.readAndUpdate(ctx, &data, data.Alias.ValueString())
	if err != nil {
        resp.Diagnostics.AddError(
             "Could not get the updated MD Details from FM",
			 fmt.Sprintf("alias: %s error: %s", data.Alias.ValueString(), err),
	    )
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (md *EsxiMD) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    resp.Diagnostics.AddError(
         "Esxi Monitoring Domain does not support any modifications",
		 "ESXI Montitoring Domain  can only be created/deleted. They cannot be modified",
	)
}

func (md *EsxiMD) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiMDModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := md.fmClient.DoRequest(
		ctx,
		"DELETE",
		0,
		fmt.Sprintf("api/v1.3/cloud/monitoringDomains/%s", data.Id.ValueString()),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete the Monitoring Domain from FM",
			fmt.Sprintf("Unable to delete monitoring domain: %s (%s) error is: %s", data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
	return
}
