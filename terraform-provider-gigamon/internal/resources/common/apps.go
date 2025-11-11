// Copyright (c) Gigamon, Inc.

// Implements the APP Resrouces that are common across all environment

package commonresources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"
	"gigamon.com/terraform-provider-gigamon/internal/utils/fmcommon"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &Dedup{}

// Dedup app resoruce, which manages the dedup applications
func NewDedup() resource.Resource {
	return &Dedup{}
}

// Dedup manages the dedup app
type Dedup struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Dedup App model
type DedupModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias types.String `tfsdk:"alias"`
	Action types.String `tfsdk:"action"`
	Timer types.Int32	`tfsdk:"timer"`
	IPTClass types.String  `tfsdk:"ipv6_traffic_class"`
	IPTos types.String`tfsdk:"ipv4_tos_field"`
	TCPSeq types.String `tfsdk:"tcp_sequence"`
	Vlan types.String `tfsdk:"vlan"`
	Id types.String `tfsdk:"id"`
}

// FM response for Dedup Creation/Get

type FMDedup struct {
	Id string `json:"id,omitempty"`
	Alias string `json:"alias"`
	Name string `json:"name"` // Will be always dedup
	Action string `json:"action"`
	IPTClass string `json:"ipTclass"`
	IPTos string `json:"ipTos"`
	TCPSeq string `json:"tcpSeq"`
	Timer int32 `json:"timer"`
	Vlan string `json:"vlan"`
}

	
func (de *Dedup) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_dedup"
}

func (de *Dedup) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon APP Dedup Schema",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this dedup application",
				Required:            true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session ID on which to deploy this APP",
				Required:            true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Action to take on the duplicate packets",
				Optional:            true,
				Computed: true,
				Default: stringdefault.StaticString("drop"),
				Validators: []validator.String{
					stringvalidator.OneOf("drop", "count"),
				},
			},
			"timer": schema.Int32Attribute{
				MarkdownDescription: "Time to wait for duplicates in micro seconds",
				Optional:            true,
				Computed: true,
				Default:  int32default.StaticInt32(50000),
			},
			"ipv6_traffic_class": schema.StringAttribute{
				MarkdownDescription: "include or ignore the IPv6 Traffic Class filed",
				Optional:            true,
				Computed: true,
				Default: stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"ipv4_tos_field": schema.StringAttribute{
				MarkdownDescription: "include or ignore the IPv4 TOS/DSCP field",
				Optional:            true,
				Computed: true,
				Default: stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"tcp_sequence": schema.StringAttribute{
				MarkdownDescription: "include or ignore the TCP Sequence Number field",
				Optional:            true,
				Computed: true,
				Default: stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"vlan": schema.StringAttribute{
				MarkdownDescription: "include or ignore the VLAN ID field in l2 header",
				Optional:            true,
				Computed: true,
				Default: stringdefault.StaticString("ignore"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Session for later use",
				PlanModifiers: []planmodifier.String{
                   stringplanmodifier.UseStateForUnknown(),
               },
		   },
		},
	}
}

// Initial Configure call, to initialize the Provider
func (de *Dedup) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	de.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func createFMStruct(data *DedupModel) *FMDedup {
	return &FMDedup{
		Alias: data.Alias.ValueString(),
		Name: "dedup",
		Action: data.Action.ValueString(),
		Timer: data.Timer.ValueInt32(),
		IPTClass: data.IPTClass.ValueString(),
		IPTos: data.IPTos.ValueString(),
		Vlan: data.Vlan.ValueString(),
		TCPSeq: data.TCPSeq.ValueString(),
		Id: data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func updateTFStruct(data *DedupModel, fmData *FMDedup) {
	data.Action = types.StringValue(fmData.Action)
	data.IPTClass = types.StringValue(fmData.IPTClass)
	data.IPTos = types.StringValue(fmData.IPTos)
	data.Vlan = types.StringValue(fmData.Vlan)
	data.TCPSeq = types.StringValue(fmData.TCPSeq)
	data.Timer = types.Int32Value(fmData.Timer)
	if fmData.Id != "" {
		data.Id = types.StringValue(fmData.Id)
	}
}


// Create call for new monitoring session
func (de *Dedup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DedupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmData := createFMStruct (&data)

	updateReq := fmcommon.UpdateReq {
		Requests: []fmcommon.UpdateObject {
			fmcommon.UpdateObject {
				EntityType: "application",
			    Operation: "create",
			    Application: fmData,
			},
		},
	}
	tflog.Info(ctx, "Dedup create request  ", map[string]any {
		"update_struct": updateReq,
	})
	
	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct to JSON",
			fmt.Sprintf("converting: %v error is: %s", updateReq,  err),
		)
		return
	}

	respData, err := de.fmClient.DoRequest(
		ctx,
		"POST",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/update", data.MonitoringSessionId.ValueString()),
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create the dedup app",
			fmt.Sprintf("Dedup  Creaet: %v error is: %s", fmData, err),
		)
		return
	}

	var fmDedupResp fmcommon.UpdateResp
	err = json.Unmarshal(respData, &fmDedupResp)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert MS Create Resp to struct",
			fmt.Sprintf("unable to get MS data: %s error is %s", string(respData), err),
		)
		return
	}
	data.Id = types.StringValue(fmDedupResp.OperationResponses[0].Id)

	// Deploy the MS if it is not already deployed
	err = deployIfNeeded(ctx, de.fmClient, data.MonitoringSessionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to deploy MS",
			fmt.Sprintf("unable to deploy MS. error is %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (de *Dedup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DedupModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (de *Dedup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    resp.Diagnostics.AddError(
         "Dedup APP does not support any modifications",
		 "Dedup App can only be created/deleted. They cannot be modified",
	)
}

func (de *Dedup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DedupModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	return
}
