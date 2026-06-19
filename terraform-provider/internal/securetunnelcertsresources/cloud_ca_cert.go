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

// Implements the Resource for Uploading Cloud CA Certificate

package securetunnelcertsresources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &CloudCaCert{}
var _ resource.ResourceWithImportState = &CloudCaCert{}

// Cloud CA Certificate resource
func NewCloudCaCert() resource.Resource {
	return &CloudCaCert{}
}

type CloudCaCert struct {
	fmClient *fmclient.FmClient
}

// CloudCaCertModel describes the resource data model
type CloudCaCertModel struct {
	Alias           types.String `tfsdk:"alias"`
	CertificatePath types.String `tfsdk:"certificate_path"`
	Certificates    types.List   `tfsdk:"certificates"`
}

// FM response from GET API
type fmCloudCaCertResponse struct {
	Alias        string                `json:"alias"`
	Certificates []fmCloudCertMetadata `json:"certificates"`
}

type fmCloudCertMetadata struct {
	DateNotAfter  string `json:"dateNotAfter"`
	DateNotBefore string `json:"dateNotBefore"`
	Algorithm     string `json:"algo"`
	Version       int64  `json:"version"`
	Issuer        string `json:"issuer"`
	Name          string `json:"name"`
}

var cloudCaCertMetadataType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"date_not_after":  types.StringType,
		"date_not_before": types.StringType,
		"algorithm":       types.StringType,
		"version":         types.Int64Type,
		"issuer":          types.StringType,
		"name":            types.StringType,
	},
}

func (ca *CloudCaCert) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_ca_cert"
}

func (ca *CloudCaCert) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon Cloud CA Certificate Upload.",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias for the Cloud CA certificate entry in FM",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[A-Za-z0-9_-]+$`),
						`Invalid characters (Only alphanumeric, "-" and "_" are allowed)`,
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"certificate_path": schema.StringAttribute{
				MarkdownDescription: "Path to CA certificate file (.crt), which will be uploaded to FM.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"certificates": schema.ListNestedAttribute{
				MarkdownDescription: "Certificate metadata received from FM after successful upload",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"date_not_after": schema.StringAttribute{
							Computed: true,
						},
						"date_not_before": schema.StringAttribute{
							Computed: true,
						},
						"algorithm": schema.StringAttribute{
							Computed: true,
						},
						"version": schema.Int64Attribute{
							Computed: true,
						},
						"issuer": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func (ca *CloudCaCert) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	ca.fmClient = fmClient
}

// Given the Alias, gets the details from FM and updates the TF state with the latest values
func (ca *CloudCaCert) getCaCertByAlias(ctx context.Context, data *CloudCaCertModel, alias string) error {
	var fmData fmCloudCaCertResponse

	respBytes, err := ca.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/nodes/caCert/%s", alias),
		nil,
		nil,
		nil,
		"",
	)

	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			return fmErr
		}
		return fmt.Errorf("Get request for Cloud CA Cert alias %q failed: %w", alias, err)
	}

	if err := json.Unmarshal(respBytes, &fmData); err != nil {
		return fmt.Errorf("Unable to unmarshal CA cert response: %s error: %w", string(respBytes), err)
	}

	//Populate data
	data.Alias = types.StringValue(fmData.Alias)

	elems := make([]attr.Value, 0, len(fmData.Certificates))

	for _, cert := range fmData.Certificates {
		obj, diags := types.ObjectValue(
			cloudCaCertMetadataType.AttrTypes,
			map[string]attr.Value{
				"date_not_after":  types.StringValue(cert.DateNotAfter),
				"date_not_before": types.StringValue(cert.DateNotBefore),
				"algorithm":       types.StringValue(cert.Algorithm),
				"version":         types.Int64Value(cert.Version),
				"issuer":          types.StringValue(cert.Issuer),
				"name":            types.StringValue(cert.Name),
			},
		)
		if diags.HasError() {
			return fmt.Errorf("failed to build certificate metadata object")
		}
		elems = append(elems, obj)
	}

	listVal, diags := types.ListValue(cloudCaCertMetadataType, elems)
	if diags.HasError() {
		return fmt.Errorf("failed to build certificates list")
	}
	data.Certificates = listVal

	return nil
}

// Cloud CA Certificate upload using Certificate path
func (ca *CloudCaCert) uploadFile(ctx context.Context, method, alias, certPath string) error {

	body, contentType, err := ca.fmClient.PrepareFileUpload(
		ctx,
		certPath,
		"certificate",
		nil,
	)
	if err != nil {
		return err
	}

	// Query param alias
	qp := map[string]string{
		"alias": alias,
	}

	tflog.Info(ctx, "Uploading Cloud CA Cert via multipart", map[string]any{
		"alias": alias,
	})

	_, err = ca.fmClient.DoRequest(
		ctx,
		method,
		"api/v1.3/cloud/nodes/caCert/file",
		qp,
		nil,
		body,
		contentType,
	)
	if err != nil {
		return fmt.Errorf("CA cert multipart upload failed for alias %q: %w", alias, err)
	}

	return nil
}

func (ca *CloudCaCert) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudCaCertModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Alias.IsNull() || data.Alias.IsUnknown() || strings.TrimSpace(data.Alias.ValueString()) == "" {
		resp.Diagnostics.AddError("Missing alias", "Cannot create Cloud CA Certificate, because 'alias' is null/unknown/empty.")
		return
	}
	alias := data.Alias.ValueString()

	// Validate CA Certificate Path
	certPath := ""
	if !data.CertificatePath.IsNull() && !data.CertificatePath.IsUnknown() {
		certPath = strings.TrimSpace(data.CertificatePath.ValueString())
	}

	if certPath == "" {
		resp.Diagnostics.AddError(
			"Missing certificate_path",
			"'certificate_path' must be provided when creating/replacing the CA cert resource.",
		)
		return
	}

	// Upload the certificate
	if err := ca.uploadFile(ctx, "POST", alias, certPath); err != nil {
		resp.Diagnostics.AddError("Unable to upload CA certificate file", err.Error())
		return
	}

	// Read back metadata into state
	err := ca.getCaCertByAlias(ctx, &data, alias)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read CA certificate after create",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (ca *CloudCaCert) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudCaCertModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Alias.IsNull() || data.Alias.IsUnknown() || strings.TrimSpace(data.Alias.ValueString()) == "" {
		resp.Diagnostics.AddError("Missing alias", "Cannot read CA certificate because 'alias' is null/unknown/empty.")
		return
	}
	alias := data.Alias.ValueString()

	err := ca.getCaCertByAlias(ctx, &data, alias)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Could not read Cloud CA Certificate from FM",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (ca *CloudCaCert) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// File path changes trigger replace, Update has nothing to do
}

func (ca *CloudCaCert) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudCaCertModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Alias.IsNull() || data.Alias.IsUnknown() || strings.TrimSpace(data.Alias.ValueString()) == "" {
		// nothing to delete
		resp.State.RemoveResource(ctx)
		return
	}
	alias := data.Alias.ValueString()

	tflog.Info(ctx, "Deleting Cloud CA Cert", map[string]any{"alias": alias})

	_, err := ca.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/nodes/caCert/%s", alias),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete Cloud CA Certificate",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (ca *CloudCaCert) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var data CloudCaCertModel

	alias := strings.TrimSpace(req.ID)
	if alias == "" {
		resp.Diagnostics.AddError("Invalid import id", "Import id cannot be empty. Use the CA certificate alias")
		return
	}

	err := ca.getCaCertByAlias(ctx, &data, alias)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to import Cloud CA Certificate from FM",
			fmt.Sprintf("Failed to import Cloud CA Certificate with alias=%q: %v", alias, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
