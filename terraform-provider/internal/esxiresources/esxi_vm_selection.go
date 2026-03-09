// Copyright (c) Gigamon, Inc.
//
// ESXi-only resource to model "Select VM" (macFilterList) for a traffic map.
//
// This resource is ESXi-scoped so that: It is obvious to users that this feature is only for ESXi.

package esxiresources

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonresources"
	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiVmSelection{}

// EsxiVmSelection models the ESXi-only "Select VM" (macFilterList) behavior
// for a specific traffic map in a specific monitoring session.
type EsxiVmSelection struct {
	fmClient *fmclient.FmClient
}

// NewEsxiVmSelection returns a new ESXi VM selection resource.
func NewEsxiVmSelection() resource.Resource {
	return &EsxiVmSelection{}
}

// EsxiVmSelectionModel is the TF model for this resource.
type EsxiVmSelectionModel struct {
	// ID of this VM selection resource.
	// We derive this as map::esxiVmwareSelection::<trafficmap_id> so it is clearly
	// tied to a specific map, but distinct from the map itself.
	Id types.String `tfsdk:"id"`

	// ESXi monitoring session ID that owns the map.
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`

	// ID of the traffic map within that monitoring session.
	TrafficMapId types.String `tfsdk:"trafficmap_id"`

	// List of VM MAC addresses to select for this map.
	MacAddresses []types.String `tfsdk:"mac_addresses"`
}

func (r *EsxiVmSelection) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	// Type name will be "gigamon_esxi_vm_selection"
	resp.TypeName = req.ProviderTypeName + "_esxi_vm_selection"
}

func (r *EsxiVmSelection) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "ESXi-only VM selection (macFilterList) for a traffic map. " +
			"This resource is only applicable for VMware ESXi monitoring sessions.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "ID of this VM selection resource. " +
					`Internally derived as "map::esxiVmwareSelection::<trafficmap_id>" so it is clearly ` +
					"distinct from the traffic map itself but tied to it.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "ID of the ESXi monitoring session that owns the target traffic map.",
				Required:            true,
			},

			"trafficmap_id": schema.StringAttribute{
				MarkdownDescription: "ID of the traffic map on which to apply VM selection (macFilterList).",
				Required:            true,
			},

			"mac_addresses": schema.ListAttribute{
				MarkdownDescription: "List of VM MAC addresses to select on this map. " +
					"Each MAC must be in the format 00:11:22:33:44:55.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					// Ensure list is non-empty
					listvalidator.SizeAtLeast(1),

					// Per-element MAC validation – same regex as L2MacSchema
					listvalidator.ValueStringsAre(
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`),
							"must be a valid MAC address format (e.g., 00:1A:2B:3C:4D:5E)",
						),
					),
				},
			},
		},
	}
}

// Configure initializes the FM client.
func (r *EsxiVmSelection) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	fmClient, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *fmclient.FmClient, got: %T. Report the issue to Gigamon.", req.ProviderData),
		)
		return
	}
	r.fmClient = fmClient
}

// Create sets macFilterList on the specified map based on the requested MAC addresses.
func (r *EsxiVmSelection) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EsxiVmSelectionModel

	// Read plan.
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Convert trafficmap_id (possibly typed) to raw UUID for FM
	tmVal := data.TrafficMapId.ValueString()
	rawTMID := tmVal
	if tmVal != "" {
		if id, err := commonutils.UUIDFromTypedID(tmVal); err == nil {
			rawTMID = id
		}
	}

	// Fetch the current map from FM.
	fmMapModel, err := commonresources.GetMSMapData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawTMID,
		"", // mapName not needed; we match by ID.
		"trafficMap",
		r.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.Diagnostics.AddError(
				"Traffic map not found",
				fmt.Sprintf("No trafficMap with id %q in monitoring session %q.",
					data.TrafficMapId.ValueString(), data.MonitoringSessionId.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Unable to get traffic map from FM",
			fmt.Sprintf("Error fetching traffic map: %v", err),
		)
		return
	}

	// Drive macFilterList from desired MAC addresses.
	fmMapModel.MacFilterList = commonresources.MacFilterListModel{
		Pass: make([]commonresources.MacFilterEntryModel, 0, len(data.MacAddresses)),
	}
	for _, mac := range data.MacAddresses {
		fmMapModel.MacFilterList.Pass = append(
			fmMapModel.MacFilterList.Pass,
			commonresources.MacFilterEntryModel{
				MacAddress: mac,
			},
		)
	}

	// Convert to Go map and send update to FM.
	goMap := commonresources.ModelMapToGoMap(ctx, fmMapModel)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "trafficMap",
				Operation:  "update",
				Map:        goMap,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update traffic map VM selection",
			fmt.Sprintf("macFilterList update failed: %v", err),
		)
		return
	}

	// Make TypeID from the raw traffic map UUID
	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleMap,
		commonutils.TypeEsxiVMWareSelection,
		rawTMID,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read refreshes mac_addresses from the underlying map's macFilterList.
func (r *EsxiVmSelection) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiVmSelectionModel

	// Read state.
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tmVal := data.TrafficMapId.ValueString()
	rawTMID := tmVal
	if tmVal != "" {
		if id, err := commonutils.UUIDFromTypedID(tmVal); err == nil {
			rawTMID = id
		}
	}

	// Fetch the current map from FM.
	fmMapModel, err := commonresources.GetMSMapData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawTMID,
		"", // mapName not needed; we match by ID.
		"trafficMap",
		r.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			// Map no longer exists; drop this resource from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Unable to get traffic map from FM",
			fmt.Sprintf("Error fetching traffic map: %v", err),
		)
		return
	}

	// Rebuild mac_addresses from the map's macFilterList.
	data.MacAddresses = make([]types.String, 0, len(fmMapModel.MacFilterList.Pass))
	for _, entry := range fmMapModel.MacFilterList.Pass {
		data.MacAddresses = append(data.MacAddresses, entry.MacAddress)
	}

	// Ensure ID is still set (in case of older state).
	if data.Id.IsUnknown() || data.Id.IsNull() {
		typedID, err := commonutils.MakeTypedID(
			commonutils.ModuleMap,
			commonutils.TypeEsxiVMWareSelection,
			rawTMID,
		)
		if err != nil {
			return
		}
		data.Id = types.StringValue(typedID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update replaces macFilterList with the new MAC list.
func (r *EsxiVmSelection) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EsxiVmSelectionModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tmVal := data.TrafficMapId.ValueString()
	rawTMID := tmVal
	if tmVal != "" {
		if id, err := commonutils.UUIDFromTypedID(tmVal); err == nil {
			rawTMID = id
		}
	}

	// Fetch the current map from FM.
	fmMapModel, err := commonresources.GetMSMapData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawTMID,
		"", // mapName not needed; we match by ID.
		"trafficMap",
		r.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.Diagnostics.AddError(
				"Traffic map not found",
				fmt.Sprintf("No trafficMap with id %q in monitoring session %q.",
					data.TrafficMapId.ValueString(), data.MonitoringSessionId.ValueString()),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Unable to get traffic map from FM",
			fmt.Sprintf("Error fetching traffic map: %v", err),
		)
		return
	}

	// Drive macFilterList from desired MAC addresses.
	fmMapModel.MacFilterList = commonresources.MacFilterListModel{
		Pass: make([]commonresources.MacFilterEntryModel, 0, len(data.MacAddresses)),
	}
	for _, mac := range data.MacAddresses {
		fmMapModel.MacFilterList.Pass = append(
			fmMapModel.MacFilterList.Pass,
			commonresources.MacFilterEntryModel{
				MacAddress: mac,
			},
		)
	}

	// Convert to Go map and send update to FM.
	goMap := commonresources.ModelMapToGoMap(ctx, fmMapModel)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "trafficMap",
				Operation:  "update",
				Map:        goMap,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update traffic map VM selection",
			fmt.Sprintf("macFilterList update failed: %v", err),
		)
		return
	}

	// ID remains the same; if missing (older state), set it.
	if data.Id.IsUnknown() || data.Id.IsNull() {
		typedID, err := commonutils.MakeTypedID(
			commonutils.ModuleMap,
			commonutils.TypeEsxiVMWareSelection,
			rawTMID,
		)
		if err != nil {
			return
		}
		data.Id = types.StringValue(typedID)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Delete clears macFilterList on the map and removes this resource from state.
func (r *EsxiVmSelection) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiVmSelectionModel

	// Read state.
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tmVal := data.TrafficMapId.ValueString()
	rawTMID := tmVal
	if tmVal != "" {
		if id, err := commonutils.UUIDFromTypedID(tmVal); err == nil {
			rawTMID = id
		}
	}

	// Fetch the current map from FM.
	fmMapModel, err := commonresources.GetMSMapData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawTMID,
		"", // mapName not needed; we match by ID.
		"trafficMap",
		r.fmClient,
	)
	if err != nil {
		// If map is gone, just remove this resource.
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Unable to get traffic map from FM",
			fmt.Sprintf("Error fetching traffic map: %v", err),
		)
		return
	}

	// Clear macFilterList.
	fmMapModel.MacFilterList = commonresources.MacFilterListModel{
		Pass: []commonresources.MacFilterEntryModel{},
	}

	// Convert to Go map and send update to FM.
	goMap := commonresources.ModelMapToGoMap(ctx, fmMapModel)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "trafficMap",
				Operation:  "update",
				Map:        goMap,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		r.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to clear VM selection from traffic map",
			fmt.Sprintf("macFilterList clear failed: %v", err),
		)
		return
	}

	// Remove from state.
	resp.State.RemoveResource(ctx)
}
