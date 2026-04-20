// Copyright (c) Gigamon, Inc.

// Implements the Resources for Secure Tunnel Certs Apply (Pushes the certificates to Monitoring Domains)

package securetunnelcertsresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SecureTunnelCertsApply{}
var _ resource.ResourceWithConfigure = &SecureTunnelCertsApply{}

// Resource for Secure Tunnel Certs Apply
func NewSecureTunnelCertsApply() resource.Resource {
	return &SecureTunnelCertsApply{}
}

type SecureTunnelCertsApply struct {
	fmClient *fmclient.FmClient
}

// SecureTunnelCertsApplyModel describes the resource data model
type SecureTunnelCertsApplyModel struct {
	MonitoringDomainIds types.Set    `tfsdk:"monitoring_domain_ids"`
	UctvCACertAlias     types.String `tfsdk:"uctv_ca_cert_alias"`
	VsnSSLKeyAlias      types.String `tfsdk:"vsn_ssl_key_alias"`
	KeyStoreAlias       types.String `tfsdk:"key_store_alias"`
}

func (s *SecureTunnelCertsApply) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secure_tunnel_certs_apply"
}

func (s *SecureTunnelCertsApply) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Applies the Secure Tunnel Certificates for a set of Monitoring Domains. " +
			"Create / Update applies the Secure Tunnel Certificates via POST/PUT APIs. Read/Delete have no effect.",

		Attributes: map[string]schema.Attribute{
			"monitoring_domain_ids": schema.SetAttribute{
				MarkdownDescription: "Set of Monitoring Domain IDs (TypedID) to apply the Secure Tunnel Certificates",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
			"uctv_ca_cert_alias": schema.StringAttribute{
				MarkdownDescription: "UCTV Agent CA certificate alias (agentCaCertAlias). Required only for UCTV → VSN tunnels.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"vsn_ssl_key_alias": schema.StringAttribute{
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

func (s *SecureTunnelCertsApply) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	s.fmClient = fmClient
}

// Check if any of the certificates are changed from prior state, which needs Re-Apply
// Attribute uctv_ca_cert_alias is optional. Removal (value -> null) is intentionally ignored because no unset API exists
func (s *SecureTunnelCertsApply) certsReApplyRequired(plan, state SecureTunnelCertsApplyModel) bool {
	// Required VSN certs
	if plan.VsnSSLKeyAlias.IsUnknown() || plan.KeyStoreAlias.IsUnknown() {
		return true
	}
	if state.VsnSSLKeyAlias.IsUnknown() || state.KeyStoreAlias.IsUnknown() {
		return true
	}

	if plan.VsnSSLKeyAlias.IsNull() || plan.KeyStoreAlias.IsNull() {
		return true
	}
	if state.VsnSSLKeyAlias.IsNull() || state.KeyStoreAlias.IsNull() {
		return true
	}

	// Optional UCTV CA cert handling
	if state.UctvCACertAlias.IsNull() && !plan.UctvCACertAlias.IsNull() {
		return true
	}

	if !state.UctvCACertAlias.IsNull() && !plan.UctvCACertAlias.IsNull() {
		if plan.UctvCACertAlias.ValueString() != state.UctvCACertAlias.ValueString() {
			return true
		}
	}

	return plan.VsnSSLKeyAlias.ValueString() != state.VsnSSLKeyAlias.ValueString() ||
		plan.KeyStoreAlias.ValueString() != state.KeyStoreAlias.ValueString()
}

// Extract the Raw UUIDs from TypedIDs
func (s *SecureTunnelCertsApply) extractMDUUIDs(ctx context.Context, mdIDs types.Set) ([]string, error) {
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

// Secure Tunnel Certificates Apply logic pertaining to one MD
func (s *SecureTunnelCertsApply) applyForOneMD(ctx context.Context, mdUUID string, plan SecureTunnelCertsApplyModel) error {
	// CA cert is optional; needed only for UCTV → VSN secure tunnels
	if !plan.UctvCACertAlias.IsNull() {
		_, err := s.fmClient.DoRequest(
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
	}

	_, err := s.fmClient.DoRequest(
		ctx,
		"PUT",
		"api/v1.3/cloud/vseries/vsnSslCert/update",
		map[string]string{
			"monitoringDomainId": mdUUID,
			"sslKey":             plan.VsnSSLKeyAlias.ValueString(),
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

// Iterate all MDs and apply the Secure Tunnel Certificates
func (s *SecureTunnelCertsApply) applyToUUIDs(ctx context.Context, mdUUIDs []string, plan SecureTunnelCertsApplyModel) error {
	for _, mdUUID := range mdUUIDs {
		if err := s.applyForOneMD(ctx, mdUUID, plan); err != nil {
			return err
		}
	}
	return nil
}

func (s *SecureTunnelCertsApply) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SecureTunnelCertsApplyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	targetUUIDs, err := s.extractMDUUIDs(ctx, plan.MonitoringDomainIds)
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_ids", err.Error())
		return
	}

	tflog.Info(ctx, "Applying Secure Tunnel Certificates to Monitoring Domains (Create)", map[string]any{
		"md_count": len(targetUUIDs),
	})

	if err := s.applyToUUIDs(ctx, targetUUIDs, plan); err != nil {
		resp.Diagnostics.AddError("Unable to apply Secure Tunnel Certificates", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (s *SecureTunnelCertsApply) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Apply-only: no remote read and no state changes required.
}

func (s *SecureTunnelCertsApply) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state SecureTunnelCertsApplyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planUUIDs, err := s.extractMDUUIDs(ctx, plan.MonitoringDomainIds)
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_ids in plan", err.Error())
		return
	}
	stateUUIDs, err := s.extractMDUUIDs(ctx, state.MonitoringDomainIds)
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_ids in state", err.Error())
		return
	}

	reApplyRequired := s.certsReApplyRequired(plan, state)

	var targets []string
	if reApplyRequired {
		targets = planUUIDs
		tflog.Info(ctx, "Secure Tunnel Certificates changed; applying to all Monitoring Domains", map[string]any{
			"md_count": len(targets),
		})
	} else {
		targets = diffAddedUUIDs(planUUIDs, stateUUIDs)
		tflog.Info(ctx, "Monitoring Domain IDs changed without Secure Tunnel Certificates; applying only to newly added Monitoring Domains", map[string]any{
			"added_count": len(targets),
		})
	}

	if len(targets) > 0 {
		if err := s.applyToUUIDs(ctx, targets, plan); err != nil {
			resp.Diagnostics.AddError("Unable to apply Secure Tunnel Certificates", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (s *SecureTunnelCertsApply) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Apply-only: no remote delete/unset (no API).
}
