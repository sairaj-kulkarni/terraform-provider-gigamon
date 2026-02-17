// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI cloud platform

package esxiresources

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/fmclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiImage{}
var _ resource.ResourceWithImportState = &EsxiImage{}

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
	FileName  types.String `tfsdk:"file_name"`
	ImageType types.String `tfsdk:"image_type"`
	Version   types.String `tfsdk:"version"`
	Vendor    types.String `tfsdk:"vendor"`
	Id        types.String `tfsdk:"id"`
	Timeout   types.Int32  `tfsdk:"timeout"`
}

// FM response for images
type FmImage struct {
	ImageName string `json:"imageName"`
	ImageType string `json:"imageType"`
	Version   string `json:"version"`
	Vendor    string `json:"vendor"`
}

// Structure representing the get images response from FM
type EsxiImageResp struct {
	Images []FmImage `json:"images"`
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"image_type": schema.StringAttribute{
				MarkdownDescription: "Type of the image that the file contains",
				Computed:            true,
			},
			"version": schema.StringAttribute{
				MarkdownDescription: "Version of the image that the file contains",
				Computed:            true,
			},
			"vendor": schema.StringAttribute{
				MarkdownDescription: "Vendor of the image that the file contains",
				Computed:            true,
			},
			"timeout": schema.Int32Attribute{
				MarkdownDescription: "Timeout(in seconds) for the image upload. Default 120 seconds",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(120),
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.RequiresReplace(),
				},
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

// Given the imageName, gets the image from FM and updates the TF state with the latest values
func (i *EsxiImage) readAndUpdate(ctx context.Context, data *EsxiImageModel, imageName string) error {

	fmImageData := EsxiImageResp{}

	imageResp, err := i.fmClient.DoRequest(
		ctx,
		"GET",
		"api/v1.3/cloud/vmware/images",
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		return fmt.Errorf("Get request of images failed: %w", err)
	}

	err = json.Unmarshal(imageResp, &fmImageData)
	if err != nil {
		return fmt.Errorf("Unable to convert resp to struct: %s error is: %w", string(imageResp), err)
	}

	// save into the Terraform state.
	for _, imageDetails := range fmImageData.Images {
		if imageDetails.ImageName == imageName {
			data.Id = types.StringValue(imageName)
			data.ImageType = types.StringValue(imageDetails.ImageType)
			data.Version = types.StringValue(imageDetails.Version)
			data.Vendor = types.StringValue(imageDetails.Vendor)
			return nil
		}
	}
	data.Id = types.StringValue("")
	return nil
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
			fmt.Sprintf("file: %s error is: %v", fileName, err),
		)
		return
	}

	timeout := data.Timeout.ValueInt32()
	myCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	_, err = i.fmClient.DoRequest(
		myCtx,
		"POST",
		"api/v1.3/cloud/vmware/images",
		nil,
		nil,
		body,
		contentType,
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to upload the file to FM",
			fmt.Sprintf("Unable to upload file: %s error is: %v", fileName, err),
		)
		return
	}

	imageName := filepath.Base(fileName)
	err = i.readAndUpdate(myCtx, &data, imageName)
	if err != nil || data.Id.ValueString() == "" {
		resp.Diagnostics.AddError(
			"Could not get the uploaded image from FM",
			fmt.Sprintf("%v", err),
		)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (i *EsxiImage) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiImageModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	imageName := data.Id.ValueString()
	err := i.readAndUpdate(ctx, &data, imageName)
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not get the uploaded image from FM",
			fmt.Sprintf("%v", err),
		)
	}

	if data.Id.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (i *EsxiImage) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Esxi Images does not support any modifications",
		"ESXI images can only be uploaded/deleted. They cannot be modified",
	)
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
		fmt.Sprintf("api/v1.3/cloud/vmware/images/%s", imageId),
		nil,
		nil,
		nil,
		"",
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Delete the image in FM",
			fmt.Sprintf("Unable to delete image: %s error is: %v", imageId, err),
		)
	}
	return
}

// Allows the user to import the existing configuration into their TF files. If the id is
// sufficient for the Read function to get hte current state, than just simply call the
// ImportStatePassThroughID. Otherwise set things up so that the read function can get the
// details, and populate that into the data in resp state
func (i *EsxiImage) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {

	// In the case of image upload, the file name is a local location where the ova is
	// present and is not stored in FM or the provider. Hence we need to pass this along
	// with id to the provider, so that it can fill it up in the returned data. Similar
	// for the timeout value

	var data EsxiImageModel

	// Read Terraform prior state data into the model

	idValue := strings.Split(req.ID, ":")
	if len(idValue) != 3 {
		resp.Diagnostics.AddError(
			"Import for image has wrong ID format",
			"Format should be <imageID>:<file_name>:<timeout value>",
		)
		return
	}
	timeout, err := strconv.ParseInt(idValue[2], 10, 32)
	if err != nil {
		resp.Diagnostics.AddError(
			"Import for image has wrong ID format",
			fmt.Sprintf("Format should be <imageID>:<file_name>:<timeout value>, timeout should be an int, not %s", idValue[2]),
		)
		return
	}

	data.Id = types.StringValue(idValue[0])
	data.FileName = types.StringValue(idValue[1])
	data.Timeout = types.Int32Value(int32(timeout))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
