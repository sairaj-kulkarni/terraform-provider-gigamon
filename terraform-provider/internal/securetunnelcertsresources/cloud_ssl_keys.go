// Copyright (c) Gigamon, Inc.

// Implements the Resource for Uploading Cloud SSL Keys (Private Key + Certificate)

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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces
var _ resource.Resource = &CloudSSLKeys{}
var _ resource.ResourceWithImportState = &CloudSSLKeys{}

// Cloud SSL Keys resource
func NewCloudSSLKeys() resource.Resource {
	return &CloudSSLKeys{}
}

type CloudSSLKeys struct {
	fmClient *fmclient.FmClient
}

// CloudSSLKeysModel describes the resource data model
type CloudSSLKeysModel struct {
	Alias           types.String `tfsdk:"alias"`
	KeyStoreAlias   types.String `tfsdk:"key_store_alias"`
	PrivateKeyPath  types.String `tfsdk:"private_key_path"`
	CertificatePath types.String `tfsdk:"certificate_path"`

	Metadata types.Object `tfsdk:"metadata"`
}

var cloudSSLKeyMetadataType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"common_name":  types.StringType,
		"organization": types.StringType,
		"expiry":       types.StringType,
		"key_type":     types.StringType,
		"certificate":  types.BoolType,
		"private_key":  types.BoolType,
		"installed_on": types.StringType,
	},
}

// FM response from GET API
type fmCloudSSLKeyResponse struct {
	CloudSSLKey fmCloudSSLKey `json:"cloudSslKey"`
}

type fmCloudSSLKey struct {
	Alias         string `json:"alias"`
	KeyStoreAlias string `json:"keyStoreAlias"`
	Comment       string `json:"comment"`
	InstalledOn   string `json:"installedOn"`
	Certificate   bool   `json:"certificate"`
	PrivateKey    bool   `json:"privateKey"`
	CommonName    string `json:"cn"`
	Organization  string `json:"o"`
	Expiry        string `json:"expiry"`
	KeyType       string `json:"keyType"`
	Type          string `json:"type"`
}

func (r *CloudSSLKeys) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud_ssl_keys"
}

func (r *CloudSSLKeys) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon Cloud SSL Keys (private key + certificate).",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias for the Cloud SSL Keys entry in FM",
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

			"key_store_alias": schema.StringAttribute{
				MarkdownDescription: "Key Store Alias where SSL Keys are stored",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("DEFAULT_CLOUD_SSL_KS"),
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

			"private_key_path": schema.StringAttribute{
				MarkdownDescription: "Path to Private Key file (.key), which will be uploaded to FM",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"certificate_path": schema.StringAttribute{
				MarkdownDescription: "Path to Certificate file (.crt), which will be uploaded to FM",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},

			"metadata": schema.ObjectAttribute{
				MarkdownDescription: "Certificate metadata received from FM after successful upload",
				Computed:            true,
				AttributeTypes:      cloudSSLKeyMetadataType.AttrTypes,
			},
		},
	}
}

func (r *CloudSSLKeys) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*fmclient.FmClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", "Expected *FmClient")
		return
	}
	r.fmClient = client
}

// Upload Cloud SSL Keys files (Private Key and Certificate) one at a time from given path
func (r *CloudSSLKeys) uploadFile(ctx context.Context, alias, keyStoreAlias, keyType, path string) error {

	body, contentType, err := r.fmClient.PrepareFileUpload(
		ctx,
		path,
		"file",
		map[string]string{
			"alias":         alias,
			"keyStoreAlias": keyStoreAlias,
			"type":          keyType,
			"keyType":       "rsa",
		},
	)
	if err != nil {
		return err
	}

	// Query param alias
	params := map[string]string{
		"alias": alias,
	}

	tflog.Info(ctx, "Uploading Cloud SSL file via multipart", map[string]any{
		"alias": alias,
	})

	_, err = r.fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/ssl/keys/file",
		params,
		nil,
		body,
		contentType,
	)
	return err
}

func (r *CloudSSLKeys) readSSLKeys(ctx context.Context, data *CloudSSLKeysModel, alias string) error {
	var fmResp fmCloudSSLKeyResponse

	respBytes, err := r.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/ssl/keys/%s", alias),
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
		return fmt.Errorf(
			"Get request for Cloud SSL Keys (Private Key + Certificate) alias %q failed: %w",
			alias, err,
		)
	}

	if err := json.Unmarshal(respBytes, &fmResp); err != nil {
		return fmt.Errorf("Unable to unmarshal Cloud SSL Keys response: %s error: %w", string(respBytes), err)
	}

	// Populate data
	meta := fmResp.CloudSSLKey

	// Identity attributes
	data.Alias = types.StringValue(meta.Alias)
	data.KeyStoreAlias = types.StringValue(meta.KeyStoreAlias)

	// Metadata
	metadata, diags := types.ObjectValue(
		cloudSSLKeyMetadataType.AttrTypes,
		map[string]attr.Value{
			"common_name":  types.StringValue(meta.CommonName),
			"organization": types.StringValue(meta.Organization),
			"expiry":       types.StringValue(meta.Expiry),
			"key_type":     types.StringValue(meta.KeyType),
			"certificate":  types.BoolValue(meta.Certificate),
			"private_key":  types.BoolValue(meta.PrivateKey),
			"installed_on": types.StringValue(meta.InstalledOn),
		},
	)
	if diags.HasError() {
		return fmt.Errorf("failed to build Cloud SSL keys metadata object")
	}

	data.Metadata = metadata

	return nil
}

func (r *CloudSSLKeys) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CloudSSLKeysModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Alias.IsNull() || data.Alias.IsUnknown() || strings.TrimSpace(data.Alias.ValueString()) == "" {
		resp.Diagnostics.AddError("Missing alias", "Cannot create Cloud SSL Keys, because 'alias' is null/unknown/empty.")
		return
	}
	alias := data.Alias.ValueString()

	if data.KeyStoreAlias.IsNull() || data.KeyStoreAlias.IsUnknown() || strings.TrimSpace(data.KeyStoreAlias.ValueString()) == "" {
		resp.Diagnostics.AddError("Missing alias", "Cannot create Cloud SSL Keys, because 'keyStoreAlias' is null/unknown/empty.")
		return
	}
	keyStoreAlias := data.KeyStoreAlias.ValueString()

	if data.PrivateKeyPath.IsNull() || data.PrivateKeyPath.IsUnknown() {
		resp.Diagnostics.AddError("Missing private_key_path", "private_key_path must be provided on create")
		return
	}

	if data.CertificatePath.IsNull() || data.CertificatePath.IsUnknown() {
		resp.Diagnostics.AddError("Missing certificate_path", "certificate_path must be provided on create")
		return
	}

	// 1. Upload private key from file
	if err := r.uploadFile(ctx, alias, keyStoreAlias, "private", data.PrivateKeyPath.ValueString()); err != nil {
		resp.Diagnostics.AddError("Private key file upload failed", err.Error())
		return
	}

	// 2. Upload certificate from file
	if err := r.uploadFile(ctx, alias, keyStoreAlias, "certificate", data.CertificatePath.ValueString()); err != nil {

		// Delete partial uploaded resource
		_, _ = r.fmClient.DoRequest(
			ctx,
			"DELETE",
			fmt.Sprintf("api/v1.3/cloud/ssl/keys/%s", alias),
			nil,
			nil,
			nil,
			"",
		)

		resp.Diagnostics.AddError("Certificate file upload failed", err.Error())
		return
	}

	// Read back metadata into state
	err := r.readSSLKeys(ctx, &data, alias)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read Cloud SSL keys after create",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudSSLKeys) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CloudSSLKeysModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.Alias.IsNull() || data.Alias.IsUnknown() || strings.TrimSpace(data.Alias.ValueString()) == "" {
		// During import, alias is not yet populated. Skip Read()
		return
	}

	if err := r.readSSLKeys(ctx, &data, data.Alias.ValueString()); err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read SSL keys from FM", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CloudSSLKeys) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Files path change trigger replace, Update has nothing to do
}

func (r *CloudSSLKeys) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CloudSSLKeysModel

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

	tflog.Info(ctx, "Deleting Cloud SSL Keys", map[string]any{"alias": alias})

	_, err := r.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/ssl/keys/%s", alias),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete Cloud SSL Keys",
			fmt.Sprintf("Alias %q error: %v", alias, err),
		)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *CloudSSLKeys) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var data CloudSSLKeysModel

	alias := strings.TrimSpace(req.ID)
	if alias == "" {
		resp.Diagnostics.AddError("Invalid import id", "Import id cannot be empty. Use the Cloud SSL Keys alias")
		return
	}

	err := r.readSSLKeys(ctx, &data, alias)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to import Cloud SSL Keys from FM",
			fmt.Sprintf("Failed to import Cloud SSL Keys with alias=%q: %v", alias, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
