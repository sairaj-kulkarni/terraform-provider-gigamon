// Copyright (c) Gigamon, Inc.

// Implements the Resources for Monitoring Domain SSL Config (Apply-Only)

package commonresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MonitoringDomainSSLConfig{}
var _ resource.ResourceWithConfigure = &MonitoringDomainSSLConfig{}

// Resource for Monitoring Domain SSL Config Push
func NewMonitoringDomainSSLConfig() resource.Resource {
	return &MonitoringDomainSSLConfig{}
}

type MonitoringDomainSSLConfig struct {
	fmClient *fmclient.FmClient
}

// MonitoringDomainSSLConfigModel describes the resource data model
type MonitoringDomainSSLConfigModel struct {
	MonitoringDomainIds types.Set    `tfsdk:"monitoring_domain_ids"`
	UctvCACertAlias     types.String `tfsdk:"uctv_ca_cert_alias"`
	VsnSSLKey           types.String `tfsdk:"vsn_ssl_key"`
	KeyStoreAlias       types.String `tfsdk:"key_store_alias"`
}

func (r *MonitoringDomainSSLConfig) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitoring_domain_ssl_config"
}

func (r *MonitoringDomainSSLConfig) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Apply-only SSL configuration for a set of Monitoring Domains. " +
			"Create/Update pushes SSL settings via POST/PUT APIs. Read/Delete have no effect.",

		Attributes: map[string]schema.Attribute{
			"monitoring_domain_ids": schema.SetAttribute{
				MarkdownDescription: "Set of Monitoring Domain IDs (TypedID) to apply this SSL configuration to.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"uctv_ca_cert_alias": schema.StringAttribute{
				MarkdownDescription: "UCTV Agent CA certificate alias (agentCaCertAlias).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"vsn_ssl_key": schema.StringAttribute{
				MarkdownDescription: "VSeries SSL key alias (sslKey).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"key_store_alias": schema.StringAttribute{
				MarkdownDescription: "Keystore alias (keyStoreAlias).",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

func (r *MonitoringDomainSSLConfig) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// Check if any of the certificates are changed from prior state
func (r *MonitoringDomainSSLConfig) certsChanged(plan, state MonitoringDomainSSLConfigModel) bool {
	if plan.UctvCACertAlias.IsUnknown() || plan.VsnSSLKey.IsUnknown() || plan.KeyStoreAlias.IsUnknown() {
		return true
	}
	if state.UctvCACertAlias.IsUnknown() || state.VsnSSLKey.IsUnknown() || state.KeyStoreAlias.IsUnknown() {
		return true
	}

	if plan.UctvCACertAlias.IsNull() || plan.VsnSSLKey.IsNull() || plan.KeyStoreAlias.IsNull() {
		return true
	}
	if state.UctvCACertAlias.IsNull() || state.VsnSSLKey.IsNull() || state.KeyStoreAlias.IsNull() {
		return true
	}

	return plan.UctvCACertAlias.ValueString() != state.UctvCACertAlias.ValueString() ||
		plan.VsnSSLKey.ValueString() != state.VsnSSLKey.ValueString() ||
		plan.KeyStoreAlias.ValueString() != state.KeyStoreAlias.ValueString()
}

// Extract the Raw UUIDs from TypedIDs
func (r *MonitoringDomainSSLConfig) extractMDUUIDs(ctx context.Context, mdIDs types.Set) ([]string, error) {
	if mdIDs.IsNull() || mdIDs.IsUnknown() {
		return nil, fmt.Errorf("monitoring_domain_ids is null or unknown")
	}

	var idStrs []string
	diags := mdIDs.ElementsAs(ctx, &idStrs, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to decode monitoring_domain_ids")
	}

	uuids := make([]string, 0, len(idStrs))
	for _, typedID := range idStrs {
		raw, err := commonutils.UUIDFromTypedID(typedID)
		if err != nil {
			return nil, fmt.Errorf("invalid monitoring_domain_id %q: %w", typedID, err)
		}
		uuids = append(uuids, raw)
	}
	return uuids, nil
}

// Get newly added MD Ids list using state and plan diff
func diffAddedUUIDs(planUUIDs, stateUUIDs []string) []string {
	stateSet := make(map[string]struct{}, len(stateUUIDs))
	for _, s := range stateUUIDs {
		stateSet[s] = struct{}{}
	}

	newlyAdded := make([]string, 0)
	for _, p := range planUUIDs {
		if _, ok := stateSet[p]; !ok {
			newlyAdded = append(newlyAdded, p)
		}
	}
	return newlyAdded
}

// Push the Certs logic pertaining to one MD
func (r *MonitoringDomainSSLConfig) applyForOneMD(ctx context.Context, mdUUID string, plan MonitoringDomainSSLConfigModel) error {
	_, err := r.fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/uctvs/caCert",
		map[string]string{
			"monitoringDomainId": mdUUID,
			"agentCaCertAlias":   plan.UctvCACertAlias.ValueString(),
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("POST caCert failed for monitoringDomainId=%s: %w", mdUUID, err)
	}

	_, err = r.fmClient.DoRequest(
		ctx,
		"PUT",
		"api/v1.3/cloud/vseries/vsnSslCert/update",
		map[string]string{
			"monitoringDomainId": mdUUID,
			"sslKey":             plan.VsnSSLKey.ValueString(),
			"keyStoreAlias":      plan.KeyStoreAlias.ValueString(),
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("PUT vsnSslCert failed for monitoringDomainId=%s: %w", mdUUID, err)
	}

	return nil
}

// Iterate all MDs and push the Config
func (r *MonitoringDomainSSLConfig) applyToUUIDs(ctx context.Context, mdUUIDs []string, plan MonitoringDomainSSLConfigModel) error {
	for _, mdUUID := range mdUUIDs {
		if err := r.applyForOneMD(ctx, mdUUID, plan); err != nil {
			return err
		}
	}
	return nil
}

func (r *MonitoringDomainSSLConfig) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MonitoringDomainSSLConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetUUIDs, err := r.extractMDUUIDs(ctx, plan.MonitoringDomainIds)
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_ids", err.Error())
		return
	}

	tflog.Info(ctx, "Applying SSL config to Monitoring Domains (Create)", map[string]any{
		"md_count": len(targetUUIDs),
	})

	if err := r.applyToUUIDs(ctx, targetUUIDs, plan); err != nil {
		resp.Diagnostics.AddError("Unable to apply SSL config", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MonitoringDomainSSLConfig) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Apply-only: no remote read and no state changes required.
}

func (r *MonitoringDomainSSLConfig) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state MonitoringDomainSSLConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planUUIDs, err := r.extractMDUUIDs(ctx, plan.MonitoringDomainIds)
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_ids in plan", err.Error())
		return
	}
	stateUUIDs, err := r.extractMDUUIDs(ctx, state.MonitoringDomainIds)
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_ids in state", err.Error())
		return
	}

	certsChanged := r.certsChanged(plan, state)

	var targets []string
	if certsChanged {
		targets = planUUIDs
		tflog.Info(ctx, "SSL config changed; applying to all Monitoring Domains", map[string]any{
			"md_count": len(targets),
		})
	} else {
		targets = diffAddedUUIDs(planUUIDs, stateUUIDs)
		tflog.Info(ctx, "Monitoring Domain set changed without SSL changes; applying only to newly added Monitoring Domains", map[string]any{
			"added_count": len(targets),
		})
	}

	if len(targets) > 0 {
		if err := r.applyToUUIDs(ctx, targets, plan); err != nil {
			resp.Diagnostics.AddError("Unable to apply SSL config", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MonitoringDomainSSLConfig) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Apply-only: no remote delete/unset (no API).
}
