//  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
//
//  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
//
//  This program is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, version 3 of the License.
//
//  This program is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with this program. If not, see <https://www.gnu.org/licenses/>

package commonresources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider-defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                     = &EndpointIfaceMapping{}
	_ resource.ResourceWithConfigValidators = &EndpointIfaceMapping{}
)

// EndpointIfaceMapping implements the VSN endpoint↔iface mapping resource.
type EndpointIfaceMapping struct {
	fmClient *fmclient.FmClient
}

// NewEndpointIfaceMapping returns a new instance.
func NewEndpointIfaceMapping() resource.Resource {
	return &EndpointIfaceMapping{}
}

// EndpointIfaceMappingMappingTFModel is one iface↔endpoint pair in TF.
type EndpointIfaceMappingMappingTFModel struct {
	Iface      types.String `tfsdk:"iface"`
	EndpointId types.String `tfsdk:"endpoint_id"`
}

// EndpointIfaceMappingModel is the Terraform state/plan model.
type EndpointIfaceMappingModel struct {
	MonitoringSessionId types.String                         `tfsdk:"monitoring_session_id"`
	Id                  types.String                         `tfsdk:"id"`
	VseriesNodeIds      types.List                           `tfsdk:"vseries_node_ids"`
	Mappings            []EndpointIfaceMappingMappingTFModel `tfsdk:"mapping"`
}

// ---- FM JSON models ----

type fmEndpointIfaceEntry struct {
	Iface      string `json:"iface"`
	EndpointId string `json:"endpointId"`
}

type fmVseriesEndpointIfaceMapping struct {
	VseriesNodeIds        []string               `json:"vseriesNodeIds"`
	EndpointIfaceMappings []fmEndpointIfaceEntry `json:"endpointIfaceMappings"`
}

type fmEndpointIfaceMappingsRequest struct {
	MonitoringSessionId          string                          `json:"monitoringSessionId"`
	VseriesEndpointIfaceMappings []fmVseriesEndpointIfaceMapping `json:"vseriesEndpointIfaceMappings"`
}

// fmEndpointIfaceMappingsResponse reuses the same shape.
type fmEndpointIfaceMappingsResponse fmEndpointIfaceMappingsRequest

// ---- Resource interface implementation ----

func (r *EndpointIfaceMapping) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_iface_mapping"
}

func (r *EndpointIfaceMapping) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "VSN interface ↔ raw endpoint mapping for a Monitoring Session.",

		Attributes: map[string]schema.Attribute{
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Session ID this mapping belongs to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// Synthetic typed ID for this mapping: endpointIfaceMapping::mapping::<monitoring_session_uuid>.
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic ID for this mapping (typed ID endpointIfaceMapping::mapping::<monitoring_session_uuid>).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"vseries_node_ids": schema.ListAttribute{
				MarkdownDescription: "List of V-Series node IDs this mapping applies to.",
				ElementType:         types.StringType,
				Required:            true,
			},
		},

		Blocks: map[string]schema.Block{
			"mapping": schema.ListNestedBlock{
				MarkdownDescription: "Interface ↔ endpointId pairs for the selected V-Series nodes.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"iface": schema.StringAttribute{
							MarkdownDescription: "Interface name on the V-Series node (e.g., ens162, ens193).",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"endpoint_id": schema.StringAttribute{
							MarkdownDescription: "Typed raw endpoint ID from gigamon_raw_endpoint.id (rawEndpoint::raw::<uuid>).",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
					},
				},
			},
		},
	}
}

func (r *EndpointIfaceMapping) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.fmClient = fmClient
}

func (r *EndpointIfaceMapping) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		endpointIfaceMappingNonEmptyValidator{},
	}
}

type endpointIfaceMappingNonEmptyValidator struct{}

func (v endpointIfaceMappingNonEmptyValidator) Description(ctx context.Context) string {
	return "Requires at least one mapping block"
}

func (v endpointIfaceMappingNonEmptyValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v endpointIfaceMappingNonEmptyValidator) ValidateResource(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var data EndpointIfaceMappingModel

	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(data.Mappings) == 0 {
		resp.Diagnostics.AddError(
			"At least one mapping is required",
			"gigamon_endpoint_iface_mapping must have at least one mapping block.",
		)
	}
}

// ---- CRUD helpers ----

func expandTFModelToFM(
	ctx context.Context,
	data *EndpointIfaceMappingModel,
	diags *diag.Diagnostics,
) (*fmEndpointIfaceMappingsRequest, error) {
	msTypedId := data.MonitoringSessionId.ValueString()

	// Use raw UUID in JSON body (matches UI behavior).
	msUUID, err := commonutils.UUIDFromTypedID(msTypedId)
	if err != nil {
		diags.AddError("Invalid monitoring_session_id", fmt.Sprintf("failed to parse typed ID %q: %v", msTypedId, err))
		return nil, err
	}

	mappings := make([]fmEndpointIfaceEntry, 0, len(data.Mappings))
	for _, m := range data.Mappings {
		eid := m.EndpointId.ValueString()
		rawID := eid

		// If endpoint_id is typed (rawEndpoint::raw::<uuid>), unwrap to raw UUID for FM.
		if strings.Contains(eid, commonutils.TypedIDDelim) {
			uuid, err := commonutils.UUIDFromTypedID(eid)
			if err != nil {
				diags.AddError(
					"Invalid endpoint_id",
					fmt.Sprintf("endpoint_id %q is not a valid typed ID: %v", eid, err),
				)
				continue
			}
			rawID = uuid
		}

		mappings = append(mappings, fmEndpointIfaceEntry{
			Iface:      m.Iface.ValueString(),
			EndpointId: rawID,
		})
	}

	// Build vSeries node IDs list from vseries_node_ids in config/state.
	vsNodeIds := []string{}
	if !data.VseriesNodeIds.IsNull() && !data.VseriesNodeIds.IsUnknown() {
		var tmp []types.String
		diags.Append(data.VseriesNodeIds.ElementsAs(ctx, &tmp, false)...)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to read vseries_node_ids from config/state")
		}
		for _, t := range tmp {
			vsNodeIds = append(vsNodeIds, t.ValueString())
		}
	}

	// Guard against empty list (defensive)
	if len(vsNodeIds) == 0 {
		diags.AddError(
			"No vSeries nodes provided",
			"vseries_node_ids must contain at least one vSeries node ID.",
		)
		return nil, fmt.Errorf("vseries_node_ids empty")
	}

	// Build FM request with real vSeries node IDs.
	req := &fmEndpointIfaceMappingsRequest{
		MonitoringSessionId: msUUID,
		VseriesEndpointIfaceMappings: []fmVseriesEndpointIfaceMapping{
			{
				VseriesNodeIds:        vsNodeIds,
				EndpointIfaceMappings: mappings,
			},
		},
	}
	return req, nil
}

func flattenFMToTFModel(data *EndpointIfaceMappingModel, fmResp *fmEndpointIfaceMappingsResponse) {
	if len(fmResp.VseriesEndpointIfaceMappings) == 0 {
		// FM returned no mappings; keep whatever is already in data.
		return
	}

	v := fmResp.VseriesEndpointIfaceMappings[0]

	// vseries_node_ids
	nodeVals := make([]attr.Value, 0, len(v.VseriesNodeIds))
	for _, id := range v.VseriesNodeIds {
		nodeVals = append(nodeVals, types.StringValue(id))
	}
	list, _ := types.ListValue(types.StringType, nodeVals)
	data.VseriesNodeIds = list

	// mapping blocks
	outMappings := make([]EndpointIfaceMappingMappingTFModel, 0, len(v.EndpointIfaceMappings))
	for _, m := range v.EndpointIfaceMappings {
		// m.EndpointId is raw UUID from FM; wrap back into typed rawEndpoint ID if possible.
		endpointID := types.StringValue(m.EndpointId)
		if m.EndpointId != "" {
			if typedID, err := commonutils.MakeTypedID(
				commonutils.ModuleRawEndpoint,
				commonutils.TypeRawEndpoint,
				m.EndpointId,
			); err == nil {
				endpointID = types.StringValue(typedID)
			}
		}

		outMappings = append(outMappings, EndpointIfaceMappingMappingTFModel{
			Iface:      types.StringValue(m.Iface),
			EndpointId: endpointID,
		})
	}
	data.Mappings = outMappings
}

// ---- fmclient calls ----

func (r *EndpointIfaceMapping) postMappings(
	ctx context.Context,
	msId string,
	payload *fmEndpointIfaceMappingsRequest,
) error {
	// monitoring_session_id is typed (e.g. "monitoringSession::vmwareEsxi::<uuid>").
	// Convert to bare UUID for the FM URL.
	msUUID, err := commonutils.UUIDFromTypedID(msId)
	if err != nil {
		return fmt.Errorf("invalid monitoring_session_id %q: %w", msId, err)
	}

	path := fmt.Sprintf("/api/v1.3/cloud/monitoringSessions/%s/endpointIfaceMappings", msUUID)

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal endpointIfaceMappings payload: %w", err)
	}

Loop:
	for {
		_, err = r.fmClient.DoRequest(
			ctx,
			http.MethodPost,
			path,
			map[string]string{"deploymentMode": "AUTO"},
			nil, // headers
			bytes.NewReader(body),
			"application/json",
		)
		if err != nil {
			var fmErr *fmclient.FMErrors
			if errors.As(err, &fmErr) {
				if fmErr.ErrorCode() == fmclient.TooManyRequests {
					timer := time.NewTimer(30 * time.Second)
					select {
					case <-timer.C:
						continue
					case <-ctx.Done():
						break Loop
					}
				}
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("endpointIfaceMappings POST for %s timed out", msId)
}

func (r *EndpointIfaceMapping) getMappings(
	ctx context.Context,
	msId string,
) (*fmEndpointIfaceMappingsResponse, error) {
	msUUID, err := commonutils.UUIDFromTypedID(msId)
	if err != nil {
		return nil, fmt.Errorf("invalid monitoring_session_id %q: %w", msId, err)
	}

	path := fmt.Sprintf("/api/v1.3/cloud/monitoringSessions/%s/endpointIfaceMappings", msUUID)

	respBody, err := r.fmClient.DoRequest(
		ctx,
		http.MethodGet,
		path,
		nil, // params
		nil, // headers
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}

	var out fmEndpointIfaceMappingsResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("failed to decode endpointIfaceMappings response: %w", err)
	}

	return &out, nil
}

// ---- CRUD ----

func (r *EndpointIfaceMapping) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EndpointIfaceMappingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmReq, err := expandTFModelToFM(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || err != nil {
		return
	}

	if err := r.postMappings(ctx, data.MonitoringSessionId.ValueString(), fmReq); err != nil {
		resp.Diagnostics.AddError(
			"Unable to create endpoint interface mappings",
			fmt.Sprintf("POST /endpointIfaceMappings failed: %s", err),
		)
		return
	}

	// Synthetic typed ID for this mapping: endpointIfaceMapping::mapping::<monitoring_session_uuid>.
	msTypedId := data.MonitoringSessionId.ValueString()
	msUUID, err := commonutils.UUIDFromTypedID(msTypedId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid monitoring_session_id",
			fmt.Sprintf("failed to parse typed ID %q while building mapping ID: %v", msTypedId, err),
		)
		return
	}
	mappingId, err := commonutils.MakeTypedID(
		commonutils.ModuleEndpointIfaceMapping,
		commonutils.TypeEndpointIfaceMapping,
		msUUID,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to build endpoint interface mapping ID",
			fmt.Sprintf("failed to build typed ID for monitoring_session_id %q: %v", msTypedId, err),
		)
		return
	}
	data.Id = types.StringValue(mappingId)

	// Best-effort read-back
	if fmResp, err := r.getMappings(ctx, data.MonitoringSessionId.ValueString()); err == nil {
		flattenFMToTFModel(&data, fmResp)
	}

	// Ensure vseries_node_ids is known (empty list) after apply
	if data.VseriesNodeIds.IsUnknown() {
		empty, _ := types.ListValue(types.StringType, []attr.Value{})
		data.VseriesNodeIds = empty
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EndpointIfaceMapping) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EndpointIfaceMappingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmResp, err := r.getMappings(ctx, data.MonitoringSessionId.ValueString())
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError(
			"Unable to read endpoint interface mappings from FM",
			err.Error(),
		)
		return
	}

	flattenFMToTFModel(&data, fmResp)

	// Ensure vseries_node_ids is known (empty list)
	if data.VseriesNodeIds.IsUnknown() {
		empty, _ := types.ListValue(types.StringType, []attr.Value{})
		data.VseriesNodeIds = empty
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *EndpointIfaceMapping) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan EndpointIfaceMappingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmReq, err := expandTFModelToFM(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() || err != nil {
		return
	}

	if err := r.postMappings(ctx, plan.MonitoringSessionId.ValueString(), fmReq); err != nil {
		resp.Diagnostics.AddError(
			"Unable to update endpoint interface mappings",
			fmt.Sprintf("POST /endpointIfaceMappings failed: %s", err),
		)
		return
	}

	// Recompute synthetic typed ID from monitoring_session_id.
	msTypedId := plan.MonitoringSessionId.ValueString()
	msUUID, err := commonutils.UUIDFromTypedID(msTypedId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid monitoring_session_id",
			fmt.Sprintf("failed to parse typed ID %q while building mapping ID: %v", msTypedId, err),
		)
		return
	}
	mappingId, err := commonutils.MakeTypedID(
		commonutils.ModuleEndpointIfaceMapping,
		commonutils.TypeEndpointIfaceMapping,
		msUUID,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to build endpoint interface mapping ID",
			fmt.Sprintf("failed to build typed ID for monitoring_session_id %q: %v", msTypedId, err),
		)
		return
	}
	plan.Id = types.StringValue(mappingId)

	if fmResp, err := r.getMappings(ctx, plan.MonitoringSessionId.ValueString()); err == nil {
		flattenFMToTFModel(&plan, fmResp)
	}

	// Ensure vseries_node_ids is known (empty list) after apply
	if plan.VseriesNodeIds.IsUnknown() {
		empty, _ := types.ListValue(types.StringType, []attr.Value{})
		plan.VseriesNodeIds = empty
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *EndpointIfaceMapping) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EndpointIfaceMappingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	msTypedId := data.MonitoringSessionId.ValueString()
	if msTypedId == "" {
		// Nothing we can do; let Terraform drop from state.
		return
	}

	msUUID, err := commonutils.UUIDFromTypedID(msTypedId)
	if err != nil {
		// If the stored ID is invalid, just let Terraform drop the resource from state.
		return
	}

	// Clear mappings for this MS by calling the DELETE endpoint for endpointIfaceMappings.
	path := fmt.Sprintf("/api/v1.3/cloud/monitoringSessions/%s/endpointIfaceMappings", msUUID)

Loop:
	for {
		_, err = r.fmClient.DoRequest(
			ctx,
			http.MethodDelete,
			path,
			map[string]string{"deploymentMode": "AUTO"},
			nil, // headers
			nil,
			"",
		)
		if err == nil {
			return
		}

		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				// Already gone; treat as success.
				return
			}
			if fmErr.ErrorCode() == fmclient.TooManyRequests {
				timer := time.NewTimer(30 * time.Second)
				defer timer.Stop()
				select {
				case <-timer.C:
					continue
				case <-ctx.Done():
					break Loop
				}
			}
		}
		// Any other error is still ignored to keep destroy best-effort.
		return
	}
}
