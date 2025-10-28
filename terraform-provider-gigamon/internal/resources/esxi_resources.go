// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI cloud platform

package resources

import (
	"context"
	"path/filepath"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"

)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiImage{}

// Esxi Image resoruce, which manages the images for ESXI platform
func NewEsxiImage() resource.Resource {
	return &EsxiImage{}
}

// EsxiImages manages the images for the ESXI platform
type EsxiImage struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// GigamonResourceModel describes the resource data model.
type EsxiImageModel struct {
	FileName types.String `tfsdk:"file_name"`
	ImageType types.String `tfsdk:"image_type"`
	Version types.String `tfsdk:"version"`
	Vendor types.String `tfsdk:"vendor"`
	Id types.String `tfsdk:"id"`
}

func (i *EsxiImage) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_image"
}

func (i *EsxiImage) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Esxi Image",

		Attributes: map[string]schema.Attribute{
			"file_name": schema.StringAttribute{
				MarkdownDescription: "Name of the file that contains the image",
				Required:            true,
			},
			"image_type": schema.StringAttribute{
				MarkdownDescription: "Type of the image that the file contains",
				Computed: true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Version of the image that the file contains",
				Computed: true,
			},
			"vendor": schema.StringAttribute{
				MarkdownDescription: "Vendor of the image that the file contains",
				Computed: true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this image for later use",
				PlanModifiers: []planmodifier.String{
                   stringplanmodifier.UseStateForUnknown(),
               },
			},
		},
	}
}

func (i *EsxiImage) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	i.fmClient = fmClient
}

func (i *EsxiImage) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EsxiImageModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Upload this image
	fileName := data.FileName.ValueString()

	// Let us make sure that the file exists and is readable
	_, err := os.Stat(fileName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to access the file",
			fmt.Sprintf("Unable to access file: %s error is: %s", fileName, err),
		)
		return
	}

	err = i.fmClient.UploadImage(ctx, 0, fileName, "api/v1.3/cloud/vmware/images")

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to upload the file to FM",
			fmt.Sprintf("Unable to upload file: %s error is: %s", fileName, err),
		)
		return
	}

	// Get the details of the file that we just uploaded
	imageName := filepath.Base(fileName)

	// save into the Terraform state.
	data.Id = types.StringValue(imageName)
	data.ImageType = types.StringValue("vseries")
	data.Version = types.StringValue("6.12.00")
	data.Vendor = types.StringValue("Gigamon")

	// Write logs using the tflog package
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (i *EsxiImage) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiImageModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (i *EsxiImage) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data EsxiImageModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (i *EsxiImage) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiImageModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
}
