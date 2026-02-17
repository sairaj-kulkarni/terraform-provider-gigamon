// Copyright (c) Gigamon, Inc.

// Implements the Resources for (Third Party Orchestration) AnyCloud Connection

package anycloudresources

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AnyCloudConnection{}
var _ resource.ResourceWithImportState = &AnyCloudConnection{}

// AnyCloud Connection resource, which manages the connection for the Third Party Orchestration platform
func NewAnyCloudConnection() resource.Resource {
	return &AnyCloudConnection{}
}

// AnyCloud Connection manages the connection for the Third Party Orchestration platform
type AnyCloudConnection struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// AnyCloudConnectionModel describes the resource data model.
type AnyCloudConnectionModel struct {
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	TappingMethod      types.String `tfsdk:"tapping_method"`
	Alias              types.String `tfsdk:"alias"`
	Id                 types.String `tfsdk:"id"`
	Status             types.String `tfsdk:"status"`
}

// FM response for Connection API
type AnyCloudFmConnection struct {
	MonitoringDomainId string `json:"monitoringDomainId"`
	TappingMethod      string `json:"tappingMethod"`
	Alias              string `json:"alias"`
	Id                 string `json:"id,omitempty"`
	Status             string `json:"status,omitempty"`
}

func (c *AnyCloudConnection) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_anycloud_connection"
}

func (c *AnyCloudConnection) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Third Party Orchestration (AnyCloud) Connection",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name of the Connection",
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
			"monitoring_domain_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring Domain ID to attach this connection to",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tapping_method": schema.StringAttribute{
				MarkdownDescription: "Type of tapping method to use",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("uctv"),
				Validators: []validator.String{
					stringvalidator.OneOf("uctv", "none"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Connectivity status of this connection",
				Computed:            true,
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

func (c *AnyCloudConnection) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (c *AnyCloudConnection) getConnectionByAlias(ctx context.Context, data *AnyCloudConnectionModel, alias string) error {

	fmConnectionData := struct {
		AnyCloudFmConnections []AnyCloudFmConnection `json:"anyCloudConnections"`
	}{}

	connResp, err := c.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/anyCloud/connections",
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("Get request of AnyCloud Connection: %s, failed with error %w", alias, err)
	}

	err = json.Unmarshal(connResp, &fmConnectionData)
	if err != nil {
		return fmt.Errorf("Unable to convert connResp to struct: %s error is: %w", string(connResp), err)
	}

	// Save into the Terraform state
	for _, connDetails := range fmConnectionData.AnyCloudFmConnections {
		if connDetails.Alias == alias {
			// Make TypedID from raw UUID received from FM
			typedID, err := commonutils.MakeTypedID(commonutils.ModuleMonitoringDomain, commonutils.TypeAnyCloud, connDetails.MonitoringDomainId)
			if err != nil {
				return err
			}
			data.MonitoringDomainId = types.StringValue(typedID)
			data.TappingMethod = types.StringValue(connDetails.TappingMethod)
			data.Alias = types.StringValue(connDetails.Alias)

			// Make TypedID from raw UUID received from FM
			typedID, err = commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeAnyCloud, connDetails.Id)
			if err != nil {
				return err
			}
			data.Id = types.StringValue(typedID)
			data.Status = types.StringValue(connDetails.Status)
			return nil
		}
	}

	return fmt.Errorf("Unable to find %s in FM Response %s and JSON Struct %v for AnyCloud Connection", alias, string(connResp), fmConnectionData)
}

// Given the Connection ID, gets the details from FM and updates the TF state with the latest values
func (c *AnyCloudConnection) getConnectionById(ctx context.Context, data *AnyCloudConnectionModel, id string) error {

	fmConnectionData := struct {
		AnyCloudConnection AnyCloudFmConnection `json:"anyCloudConnection"`
	}{}

	// Extract raw UUID from TypedID
	rawID, err := commonutils.UUIDFromTypedID(id)
	if err != nil {
		return fmt.Errorf("Invalid connection id %q: %w", id, err)
	}

	connResp, err := c.fmClient.DoRequest(
		ctx,
		"GET",
		fmt.Sprintf("api/v1.3/cloud/anyCloud/connections/%s", rawID),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("Get request of AnyCloud Connection ID: %s, failed with error %w", id, err)
	}

	err = json.Unmarshal(connResp, &fmConnectionData)
	if err != nil {
		return fmt.Errorf("Unable to convert connResp to struct: %s error is: %w", string(connResp), err)
	}

	// Populate the data
	typedID, err := commonutils.MakeTypedID(commonutils.ModuleMonitoringDomain, commonutils.TypeAnyCloud, fmConnectionData.AnyCloudConnection.MonitoringDomainId)
	if err != nil {
		return err
	}
	data.MonitoringDomainId = types.StringValue(typedID)

	data.TappingMethod = types.StringValue(fmConnectionData.AnyCloudConnection.TappingMethod)
	data.Alias = types.StringValue(fmConnectionData.AnyCloudConnection.Alias)

	typedID, err = commonutils.MakeTypedID(commonutils.ModuleConnection, commonutils.TypeAnyCloud, fmConnectionData.AnyCloudConnection.Id)
	if err != nil {
		return err
	}
	data.Id = types.StringValue(typedID)

	data.Status = types.StringValue(fmConnectionData.AnyCloudConnection.Status)

	return nil
}

func (c *AnyCloudConnection) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AnyCloudConnectionModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Extract raw UUID from TypedID
	rawID, err := commonutils.UUIDFromTypedID(data.MonitoringDomainId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_id", err.Error())
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmConnection := AnyCloudFmConnection{
		MonitoringDomainId: rawID,
		TappingMethod:      data.TappingMethod.ValueString(),
		Alias:              data.Alias.ValueString(),
	}

	jsonData, err := json.Marshal(fmConnection)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct AnyCloudFmConnection to JSON",
			fmt.Sprintf("converting: %v error is: %v", fmConnection, err),
		)
		return
	}

	tflog.Info(ctx, "Creating Connection", map[string]any{
		"struct":   fmConnection,
		"jsonBody": string(jsonData),
	})

	_, err = c.fmClient.DoRequest(
		ctx,
		"POST",
		"api/v1.3/cloud/anyCloud/connections",
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create connection for anyCloud",
			fmt.Sprintf("Connection Create: %v error is: %v", fmConnection, err),
		)
		return
	}

	// Poll until status becomes "connected" or timeout expires
	const pollInterval = 10 * time.Second
	const waitTimeout = 2 * time.Minute

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		err = c.getConnectionByAlias(waitCtx, &data, fmConnection.Alias)
		if err != nil {
			lastErr = err
			tflog.Warn(ctx, "GET call to AnyCloud connection status failed", map[string]any{
				"alias": fmConnection.Alias,
				"error": err.Error(),
			})
		} else if strings.EqualFold(data.Status.ValueString(), "connected") {
			// Connected -> write final state and return
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		} else {
			tflog.Info(ctx, "Connection status not moved to connected yet", map[string]any{
				"alias":  fmConnection.Alias,
				"status": data.Status.ValueString(),
			})
		}

		select {
		case <-ticker.C:
			// keep polling
		case <-waitCtx.Done():
			msg := "Timeout waiting for AnyCloud connection to become connected."
			if lastErr != nil {
				msg = fmt.Sprintf("%s Last error: %v", msg, lastErr)
			}
			resp.Diagnostics.AddError("Connection status did not move to connected in time", msg)
			return
		}
	}
}

func (c *AnyCloudConnection) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AnyCloudConnectionModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		resp.Diagnostics.AddError("Missing AnyCloud Connection ID", "Cannot read because 'id' is null/unknown/empty.")
		return
	}

	err := c.getConnectionById(ctx, &data, data.Id.ValueString())
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) && fmErr.ErrorCode() == fmclient.ObjectNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Could not read AnyCloud Connection Details from FM",
			fmt.Sprintf("ID: %s error: %v", data.Id.ValueString(), err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (c *AnyCloudConnection) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData, stateData AnyCloudConnectionModel

	// Read Terraform plan and prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate ID
	if stateData.Id.IsNull() || stateData.Id.IsUnknown() || stateData.Id.ValueString() == "" {
		resp.Diagnostics.AddError("Missing AnyCloud Connection ID", "Cannot update because 'id' is missing in state.")
		return
	}

	// Extract raw UUID from TypedID
	mdId, err := commonutils.UUIDFromTypedID(stateData.MonitoringDomainId.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid monitoring_domain_id in state", err.Error())
		return
	}
	typedID := stateData.Id.ValueString()
	connId, err := commonutils.UUIDFromTypedID(typedID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid connection id in state", err.Error())
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmConnection := AnyCloudFmConnection{
		MonitoringDomainId: mdId,
		Alias:              stateData.Alias.ValueString(),
		TappingMethod:      planData.TappingMethod.ValueString(),
	}

	jsonData, err := json.Marshal(fmConnection)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert struct to JSON",
			fmt.Sprintf("converting: %v error is: %v", fmConnection, err),
		)
		return
	}

	tflog.Info(ctx, "Updating AnyCloud Connection", map[string]any{
		"struct":   fmConnection,
		"jsonBody": string(jsonData),
	})

	_, err = c.fmClient.DoRequest(
		ctx,
		"PATCH",
		fmt.Sprintf("api/v1.3/cloud/anyCloud/connections/%s", connId),
		nil,
		nil,
		bytes.NewBuffer(jsonData),
		"application/json",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update the AnyCloud Connection",
			fmt.Sprintf("AnyCloud Connection: %v error is: %v", fmConnection, err),
		)
		return
	}

	// Poll until status becomes "connected" or timeout expires
	const pollInterval = 10 * time.Second
	const waitTimeout = 2 * time.Minute

	waitCtx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastErr error
	for {
		err = c.getConnectionById(waitCtx, &stateData, typedID)
		if err != nil {
			lastErr = err
			tflog.Warn(ctx, "GET call to AnyCloud connection status failed", map[string]any{
				"ID":    connId,
				"error": err.Error(),
			})
		} else if strings.EqualFold(stateData.Status.ValueString(), "connected") {
			// Connected -> write final state and return
			resp.Diagnostics.Append(resp.State.Set(ctx, &stateData)...)
			return
		} else {
			tflog.Info(ctx, "Connection status not moved to connected yet", map[string]any{
				"ID":     connId,
				"status": stateData.Status.ValueString(),
			})
		}

		select {
		case <-ticker.C:
			// keep polling
		case <-waitCtx.Done():
			msg := "Timeout waiting for AnyCloud connection to become connected."
			if lastErr != nil {
				msg = fmt.Sprintf("%s Last error: %v", msg, lastErr)
			}
			resp.Diagnostics.AddError("Connection status did not move to connected in time", msg)
			return
		}
	}
}

func (c *AnyCloudConnection) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AnyCloudConnectionModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Id.IsNull() || data.Id.IsUnknown() || data.Id.ValueString() == "" {
		return
	}

	// Extract raw UUID from TypedID
	connId, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid connection id in state", err.Error())
		return
	}

	_, err = c.fmClient.DoRequest(
		ctx,
		"DELETE",
		fmt.Sprintf("api/v1.3/cloud/anyCloud/connections/%s", connId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete the Connection from FM",
			fmt.Sprintf("Unable to delete Connection: %s (%s) error is: %v", data.Alias.ValueString(), data.Id.ValueString(), err),
		)
	}
}

func (c *AnyCloudConnection) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var data AnyCloudConnectionModel

	alias := strings.TrimSpace(req.ID)
	if alias == "" {
		resp.Diagnostics.AddError("Invalid import id", "Import id cannot be empty. Use the Connection alias")
		return
	}

	err := c.getConnectionByAlias(ctx, &data, alias)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to import AnyCloud Connection",
			fmt.Sprintf("Failed to import connection with alias=%q: %v", alias, err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
