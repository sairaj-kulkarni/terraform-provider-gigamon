// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI cloud platform

package resources

import (
	"context"
	"encoding/json"
	"path/filepath"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

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
	FileName types.String `tfsdk:"file_name",json:"imageName"`
	ImageType types.String `tfsdk:"image_type",json:"imageType"`
	Version types.String `tfsdk:"version",json:"version"`
	Vendor types.String `tfsdk:"vendor",json:"vendor"`
	Id types.String `tfsdk:"id",json:"-"`
}

// Structure representing the get images response from FM
type EsxiImageResp struct {
	Images []EsxiImageModel `json:"images"`
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

	// File to upload to FM
	fileName := data.FileName.ValueString()

	// Prepare the content body and content-header type
	body, contentType, err := i.fmClient.PrepareFileUpload(ctx, fileName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to prepare file for upload",
			fmt.Sprintf("file: %s error is: %s", fileName, err),
		)
		return
	}

	_, err = i.fmClient.DoRequest(
		ctx,
		"POST",
		0,
		"api/v1.3/cloud/vmware/images",
		nil,
		nil,
		body,
		contentType,
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to upload the file to FM",
			fmt.Sprintf("Unable to upload file: %s error is: %s", fileName, err),
		)
		return
	}

	// Get the details of the file that we just uploaded
	fmImageData := EsxiImageResp{}

	imageResp, err := i.fmClient.DoRequest(
		ctx,
		"GET",
		0,
		"api/v1.3/cloud/vmware/images",
		nil,
		nil,
		nil,
		"",
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to get the status of all images in the system",
			fmt.Sprintf("Unable to get status of images: %s error is: %s", fileName, err),
		)
		return
	}

	err = json.Unmarshal(imageResp, &fmImageData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to convert the images response to JSON",
			fmt.Sprintf("Unable to convert resp to struct: %s error is: %s", string(imageResp), err),
		)
		return
	}

	// save into the Terraform state.
	imageName := filepath.Base(fileName)
	for _, imageDetails := range fmImageData.Images {
		if imageDetails.FileName.ValueString() == imageName {
	        data.Id = types.StringValue(imageName)
	        data.ImageType = imageDetails.ImageType
	        data.Version = imageDetails.Version
			data.Vendor = imageDetails.Vendor
	        // Save data into Terraform state
	        resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
    resp.Diagnostics.AddError(
        "Json unmarshal did not populate the correct fields",
		fmt.Sprintf("Error in Json Conversion: %s: %v : %d", string(imageResp), fmImageData, len(fmImageData.Images)),
	)
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

	imageId := data.Id.ValueString()
	_, err := i.fmClient.DoRequest(
		ctx,
		"DELETE",
		0,
		fmt.Sprintf("api/v1.3/cloud/vmware/images/%s", imageId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to upload the file to FM",
			fmt.Sprintf("Unable to delete image: %s error is: %s", imageId, err),
		)
	}
	return
}
