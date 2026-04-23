// Copyright (c) Gigamon, Inc.

// Implements the Resources that are common across all environment i.e. MS and other such

package commonresources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/float32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MonSess{}
var _ resource.ResourceWithImportState = &MonSess{}
var _ resource.ResourceWithModifyPlan = &MonSess{}

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
	DistributeTraffic  types.Bool   `tfsdk:"distribute_traffic"`
	DeploymentStatus   types.String `tfsdk:"deployment_status"`
	TappingMethod      types.String `tfsdk:"tapping_method"`
	TrafficAcquisition types.Object `tfsdk:"traffic_acquisition"`

	//App Intel Attributes
	FastMode  types.Bool    `tfsdk:"fast_mode"`
	ScaleUnit types.Float32 `tfsdk:"scale_unit"`
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
	DistributeTraffic  bool     `json:"distributeTraffic"`
	DeploymentStatus   string   `json:"deployStatus,omitempty"`
	FastMode           bool     `json:"fastMode"`
	ScaleUnit          float32  `json:"scaleUnit,omitempty"`

	// Tapping Method = UCTV
	FMMonSessUCTV
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

			"tapping_method": schema.StringAttribute{
				MarkdownDescription: "Used for HCL validation and Read functionality",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("uctv", "none", "platform"),
				},
			},

			"fast_mode": schema.BoolAttribute{
				MarkdownDescription: "Enable fast mode for AppIntel Solution",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},

			"scale_unit": schema.Float32Attribute{
				MarkdownDescription: "Configure Scale for AppIntel Solution",
				Optional:            true,
				Validators: []validator.Float32{
					float32validator.OneOf(0.5, 1.0, 2.0, 3.0, 4.0, 5.0),
				},
				PlanModifiers: []planmodifier.Float32{
					float32planmodifier.RequiresReplace(),
				},
			},

			// UCT-V Traffic Acquisition attributes
			"traffic_acquisition": TrafficAcquisitionSchemaAttribute(),

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
			"distribute_traffic": schema.BoolAttribute{
				MarkdownDescription: "If true, indicates distributed deduplication is enabled. Default false.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
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

func (ms *MonSess) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Config.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plan MonSessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ValidateTrafficAcquisitionConfig(ctx, req, resp, plan)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := ValidateProtoFilterValues(ctx, plan); err != nil {
		resp.Diagnostics.AddError("Invalid proto filter value", err.Error())
		return
	}

	if err := DeriveComputedAttributesFromPolicy(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Invalid filtering policy", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// After Update, call deploy Monitoring Session
func (ms *MonSess) deployMonitoringSession(ctx context.Context, typedMSID string) error {
	rawID, err := commonutils.UUIDFromTypedID(typedMSID)
	if err != nil {
		return err
	}

	_, err = ms.fmClient.DoRequest(
		ctx,
		"POST",
		fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s/deploy", rawID),
		nil,
		nil,
		nil,
		"",
	)

	// Ignore fmclient.RequestConflict : Monitoring Session has no deployable resources
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.RequestConflict {
			// fmclient.RequestConflict means "nothing to deploy yet" – safe to ignore
			tflog.Info(ctx, "Monitoring Session deploy skipped (not deployable yet)", map[string]any{
				"monitoring_session_id": typedMSID,
			})
			return nil
		}
		return err
	}

	return nil
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
		resp.Diagnostics.AddError("Invalid Monitoring Domain ID", err.Error())
		return
	}
	platformType, err := commonutils.TypeFromTypedID(data.MonitoringDomainId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Monitoring Domain ID", err.Error())
		return
	}
	connId, err := commonutils.UUIDFromTypedID(data.ConnectionId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Connection ID", err.Error())
		return
	}

	// Build request payload as map so we can add only items needed
	payload := map[string]any{
		"alias":              data.Alias.ValueString(),
		"platform":           string(platformType),
		"connIds":            []string{connId},
		"monitoringDomainId": mdId,
		"distributeTraffic":  data.DistributeTraffic.ValueBool(),
	}

	if !data.Description.IsNull() {
		payload["description"] = data.Description.ValueString()
	}

	payload["fastMode"] = data.FastMode.ValueBool()

	if !data.ScaleUnit.IsNull() {
		payload["scaleUnit"] = data.ScaleUnit.ValueFloat32()
	}

	// If traffic_acquisition is present, compute and include mirroring and precryption attributes
	if err := AddTrafficAcquisitionIntoPayload(ctx, payload, data.TrafficAcquisition); err != nil {
		resp.Diagnostics.AddError("Invalid traffic_acquisition configuration", err.Error())
		return
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert payload to JSON",
			fmt.Sprintf("payload: %v error is: %v", payload, err),
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
			fmt.Sprintf("Monitoring session Create: %v error is: %v", payload, err),
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
		resp.Diagnostics.AddError("Unable to build Monitoring Session MS ID", err.Error())
		return
	}
	data.Id = types.StringValue(typedID)

	data.Deployed = types.BoolValue(fmMSResp.Deployed)
	data.DeploymentStatus = types.StringValue(fmMSResp.DeploymentStatus)
	data.DistributeTraffic = types.BoolValue(fmMSResp.DistributeTraffic)
	data.FastMode = types.BoolValue(fmMSResp.FastMode)

	if fmMSResp.ScaleUnit > 0 {
		data.ScaleUnit = types.Float32Value(fmMSResp.ScaleUnit)
	} else {
		data.ScaleUnit = types.Float32Null()
	}

	// Store TA attributes when tapping_method is uctv; otherwise Null
	taObj, taDiags := ComputeTrafficAcquisitionStateFromFM(data.TappingMethod, fmMSResp)
	resp.Diagnostics.Append(taDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.TrafficAcquisition = taObj

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
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

	// Pull latest from FM into the struct you already use
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
	data.DistributeTraffic = types.BoolValue(fmResp.DistributeTraffic)
	data.FastMode = types.BoolValue(fmResp.FastMode)

	if fmResp.ScaleUnit > 0 {
		data.ScaleUnit = types.Float32Value(fmResp.ScaleUnit)
	} else {
		data.ScaleUnit = types.Float32Null()
	}

	// Store TA attributes when tapping_method is uctv; otherwise Null
	taObj, taDiags := ComputeTrafficAcquisitionStateFromFM(data.TappingMethod, fmResp)
	resp.Diagnostics.Append(taDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.TrafficAcquisition = taObj

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
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

	// Build PATCH payload as map so we can add only items needed.
	payload := map[string]any{
		"alias":             plan.Alias.ValueString(),
		"platform":          string(platformType),
		"distributeTraffic": plan.DistributeTraffic.ValueBool(),
	}

	if !plan.Description.IsNull() {
		payload["description"] = plan.Description.ValueString()
	}

	payload["fastMode"] = plan.FastMode.ValueBool()

	if !plan.ScaleUnit.IsNull() {
		payload["scaleUnit"] = plan.ScaleUnit.ValueFloat32()
	}

	// Traffic_acquisition attributes
	if err := ApplyTrafficAcquisitionUpdatesToPayload(ctx, payload, plan.TrafficAcquisition, state.TrafficAcquisition, plan.TappingMethod); err != nil {
		resp.Diagnostics.AddError("Invalid traffic_acquisition configuration", err.Error())
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Unable to build Monitoring Session update payload", err.Error())
		return
	}

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
	state.TappingMethod = plan.TappingMethod
	state.TrafficAcquisition = plan.TrafficAcquisition
	state.DistributeTraffic = plan.DistributeTraffic

	fmResp := FMMonSess{}
	if err := UpdateMSData(ctx, state.Id.ValueString(), &fmResp, ms.fmClient); err == nil {
		state.Deployed = types.BoolValue(fmResp.Deployed)
		state.DeploymentStatus = types.StringValue(fmResp.DeploymentStatus)
		state.FastMode = types.BoolValue(fmResp.FastMode)

		if fmResp.ScaleUnit > 0 {
			state.ScaleUnit = types.Float32Value(fmResp.ScaleUnit)
		} else {
			state.ScaleUnit = types.Float32Null()
		}
	}

	// Store TA attributes when tapping_method is uctv; otherwise Null
	taObj, taDiags := ComputeTrafficAcquisitionStateFromFM(state.TappingMethod, fmResp)
	resp.Diagnostics.Append(taDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.TrafficAcquisition = taObj

	// Deploy Monitoring Session so that changes are pushed to nodes
	if err := ms.deployMonitoringSession(ctx, state.Id.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to deploy Monitoring Session", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
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
		resp.Diagnostics.AddError("Invalid Monitoring Session ID", err.Error())
		return
	}

Loop:
	for {
		_, err = ms.fmClient.DoRequest(
			ctx,
			"DELETE",
			fmt.Sprintf("api/v1.3/cloud/monitoringSessions/%s", rawID),
			map[string]string{"deploymentMode": "AUTO"},
			nil,
			nil,
			"",
		)
		if err == nil {
			return
		}

		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.TooManyRequests {
			timer := time.NewTimer(30 * time.Second)
			select {
			case <-timer.C:
				continue
			case <-ctx.Done():
				timer.Stop()
				break Loop
			}
		}

		resp.Diagnostics.AddError(
			"Unable to Delete the Monitoring Session from FM",
			fmt.Sprintf("Unable to delete monitoring Session: %s (%s) error is: %v",
				data.Alias.ValueString(), data.Id.ValueString(), err),
		)
		return
	}

	resp.Diagnostics.AddError(
		"Unable to Delete the Monitoring Session from FM",
		fmt.Sprintf("Unable to delete monitoring Session: %s (%s) timed out while retrying after too many requests",
			data.Alias.ValueString(), data.Id.ValueString()),
	)
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

	// Build null objects with attribute type information for optional nested blocks.
	_, _, taAttrTypes := trafficAcqAttrTypes()

	// Read() will fill derived / computed fields.
	state := MonSessModel{
		Id:                 types.StringValue(typedMSID),
		MonitoringDomainId: types.StringValue(typedMDID),
		Alias:              types.StringValue(fmMS.Alias),
		DistributeTraffic:  types.BoolValue(fmMS.DistributeTraffic),
		TrafficAcquisition: types.ObjectNull(taAttrTypes),
	}
	if typedConnID != "" {
		state.ConnectionId = types.StringValue(typedConnID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
