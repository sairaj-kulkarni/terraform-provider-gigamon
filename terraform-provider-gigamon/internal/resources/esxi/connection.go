// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI cloud Connection

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"

)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiConnection{}

// Esxi Connection resoruce, which manages the images for ESXI platform
func NewEsxiConnection() resource.Resource {
	return &EsxiConnection{}
}

// EsxiConnetion manages the connection for the ESXI platform
type EsxiConnection struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiConnectionModel describes the resource data model.
type EsxiConnectionModel struct {
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	TappingMethod types.String `tfsdk:"tapping_method"`
	Alias types.String `tfsdk:"alias"`
	VcenterIP types.String `tfsdk:"vcenter_address"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	ResourceAllocation types.String `tfsdk:"resource_allocation"`
	MaximumNodesPerHost types.Int32 `tfsdk:"maximum_nodes_per_host"`
	Id types.String `tfsdk:"id"`
	Status types.String `tfsdk:"status"`
}

// FM response for Connection API
type EsxiFmConnection struct {
	MonitoringDomainId string `json:"monitoringDomainId"`
	TappingMethod string `json:"tappingMethod"`
	Alias string `json:"alias"`
	VcenterIP string `json:"vcenterIp"`
	Username string `json:"username"`
	Password string `json:"password"`
	ResourceAllocation string `json:"resourceAllocation"`
	MaximumNodesPerHost int32 `json:"maximumNodesPerHost"`
	Id string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
}

func (c *EsxiConnection) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_connection"
}

func (c *EsxiConnection) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Esxi Connection",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the Connection",
				Required:            true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},

			"monitoring_domain_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Domain ID to attach this connection to",
				Required: true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},
			"tapping_method": schema.StringAttribute{
				MarkdownDescription: "Type of tapping method to use",
				Optional: true,
				Computed: true,
				Default:     stringdefault.StaticString("platform"),
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"platform", "none"}...),
				},
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},
			"vcenter_address": schema.StringAttribute{
				MarkdownDescription: "Vcenter Address - numerical IP or FQDN",
				Required: true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},
			"username": schema.StringAttribute{
				MarkdownDescription: "Username for authentication to the Vcenter",
				Required: true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for authentication to the Vcenter",
				Required: true,
			},
			"resource_allocation": schema.StringAttribute{
				MarkdownDescription: "Determines the mapping of customer VM to Vseries. Can be either TargetVM based or based on the switch on which the targetVM resides",
				Optional: true,
				Computed: true,
				Default:     stringdefault.StaticString("TargetVMBased"),
				Validators: []validator.String{
					stringvalidator.OneOf([]string{"TargetVMBased", "SwitchBased", "none"}...),
				},
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},
			"maximum_nodes_per_host": schema.Int32Attribute{
				MarkdownDescription: "Maximum number of Vsereis nodes to spin up per host",
				Optional: true,
				Computed: true,
				Default: int32default.StaticInt32(1),
				PlanModifiers: []planmodifier.Int32{
                    int32planmodifier.RequiresReplace(),
                },
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(10),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Connectivity status of this connection",
				Computed: true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Connection for later use",
				PlanModifiers: []planmodifier.String{
                   stringplanmodifier.UseStateForUnknown(),
               },
			},
		},
	}
}

func (c *EsxiConnection) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	c.fmClient = fmClient
}

// Given the Connection Alias, gets the details from FM and updates the TF state with the latest values
func (c *EsxiConnection) readAndUpdate(ctx context.Context, data *EsxiConnectionModel, alias string) error {

	fmConnectionData := struct {
		VmwareConnections []EsxiFmConnection `json:"vmwareConnections"`
	} {
		VmwareConnections: make([]EsxiFmConnection, 10),
	}

	mdResp, err := c.fmClient.DoRequest(
		ctx,
		"GET",
		0,
		fmt.Sprintf("api/v1.3/cloud/vmware/connections"),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("Get request of Vmware Connections failed: %s: %s", alias, err)
	}

	err = json.Unmarshal(mdResp, &fmConnectionData)
	if err != nil {
		return fmt.Errorf("Unable to convert resp to struct: %s error is: %s", string(mdResp), err)
	}

	// save into the Terraform state.
	for _, connDetails := range fmConnectionData.VmwareConnections {
		if connDetails.Alias == alias {
			data.MonitoringDomainId = types.StringValue(connDetails.MonitoringDomainId)
			data.TappingMethod = types.StringValue(connDetails.TappingMethod)
	        data.Alias = types.StringValue(connDetails.Alias)
			data.VcenterIP = types.StringValue(connDetails.VcenterIP)
			data.Username = types.StringValue(connDetails.Username)
			data.ResourceAllocation = types.StringValue(connDetails.ResourceAllocation)
			data.MaximumNodesPerHost = types.Int32Value(connDetails.MaximumNodesPerHost)
	        data.Id = types.StringValue(connDetails.Id)
			data.Status = types.StringValue(connDetails.Status)
			return nil
		}
	}
	return fmt.Errorf("Unable to find %s in FM Response %s and JSON Struct %v", alias, string(mdResp), fmConnectionData)
}

func (c *EsxiConnection) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EsxiConnectionModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmConnection := EsxiFmConnection{
		MonitoringDomainId: data.MonitoringDomainId.ValueString(),
		TappingMethod: data.TappingMethod.ValueString(),
		Alias: data.Alias.ValueString(),
		VcenterIP: data.VcenterIP.ValueString(),
		Username: data.Username.ValueString(),
		Password: data.Password.ValueString(),
		ResourceAllocation: data.ResourceAllocation.ValueString(),
		MaximumNodesPerHost: data.MaximumNodesPerHost.ValueInt32(),
	}
	
	jsonData, err := json.Marshal(fmConnection)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct to JSON",
			fmt.Sprintf("converting: %v error is: %s", fmConnection,  err),
		)
		return
	}

	tflog.Info(ctx, "Creating Connection", map[string]any{
		"struct": fmConnection,
		"jsonBody": string(jsonData),
	})

	_, err = c.fmClient.DoRequest(
		ctx,
		"POST",
		0,
		"api/v1.3/cloud/vmware/connections",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the connection",
			fmt.Sprintf("Connection Creaet: %v error is: %s", fmConnection, err),
		)
		return
	}

    err = c.readAndUpdate(ctx, &data, fmConnection.Alias)
	if err != nil {
        resp.Diagnostics.AddError(
             "Could not get the updated data on Connection from FM",
		     fmt.Sprintf("%s", err),
	    )
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (c *EsxiConnection) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiConnectionModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := c.readAndUpdate(ctx, &data, data.Alias.ValueString())
	if err != nil {
        resp.Diagnostics.AddError(
             "Could not get the updated Connection Details from FM",
			 fmt.Sprintf("alias: %s error: %s", data.Alias.ValueString(), err),
	    )
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (c *EsxiConnection) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    resp.Diagnostics.AddError(
         "Esxi Monitoring Domain does not support any modifications",
		 "ESXI Montitoring Domain  can only be created/deleted. They cannot be modified",
	)
}

func (c *EsxiConnection) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiConnectionModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := c.fmClient.DoRequest(
		ctx,
		"DELETE",
		0,
		fmt.Sprintf("api/v1.3/cloud/vmware/connections/%s", data.Id.ValueString()),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete the Connection from FM",
			fmt.Sprintf("Unable to delete Connection: %s (%s) error is: %s", data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
	return
}
