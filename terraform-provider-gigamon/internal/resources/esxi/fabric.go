// Copyright (c) Gigamon, Inc.

// Implements the Resrouces for the ESXI Fabric (VSeries) resources

package esxiresources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-log/tflog"

	"gigamon.com/terraform-provider-gigamon/internal/fmclient"

)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &EsxiFabric{}

// Esxi Fabric resoruce, which manages the images for ESXI platform
func NewEsxiFabric() resource.Resource {
	return &EsxiFabric{}
}

// EsxiFabric manages the Fabric for the ESXI platform
type EsxiFabric struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// EsxiIntfSpec describes the interface details of the Vseries spec
type EsxiIntfSpec struct {
	Name types.String `tfsdk:"interface_name"`
	MORef types.String `tfsdk:"interface_moref"`
	AddressMode types.String `tfsdk:"address_assignment_mode"`
}

// EsxiVmSpec describes the spec for a VM on ESXI 
type EsxiVmSpec struct {
	HostName types.String `tfsdk:"hostname"`
	HostMORef types.String `tfsdk:"host_moref"`
	MgmtIntf *EsxiIntfSpec `tfsdk:"management_interface_spec"`
	TunnelIntf *EsxiIntfSpec `tfsdk:"tunnel_interface_spec"`
	VmName types.String `tfsdk:"name"`
}

	
// EsxiFabricModel describes the fabric resource data model.
type EsxiFabricModel struct {
	Name types.String `tfsdk:"name"`
	ConnectionId types.String `tfsdk:"connection_id"`
	DatacenterMORef types.String `tfsdk:"datacenter_moref"`
	FormFactor types.String `tfsdk:"form_factor"`
	ImageId types.String `tfsdk:"image_id"`
	MgmtIntf *EsxiIntfSpec `tfsdk:"management_interface_spec"`
	TunnelIntf *EsxiIntfSpec `tfsdk:"tunnel_interface_spec"`
	HostSpec map[string]EsxiVmSpec `tfsdk:"host_vm_spec"`
	Id types.String `tfsdk:"id"`
}

func (f *EsxiFabric) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_esxi_fabric"
}

func (f *EsxiFabric) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Esxi Fabric",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the fabric",
				Required:            true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},

			"connection_id": schema.StringAttribute{
				MarkdownDescription: "Connection ID on which this fabric is to be deployed",
				Required: true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},

			"datacenter_moref": schema.StringAttribute{
				MarkdownDescription: "Vcenter MORef of the datacanter on which to deploy",
				Required: true,
				PlanModifiers: []planmodifier.String{
                    stringplanmodifier.RequiresReplace(),
                },
			},

			"form_factor": schema.StringAttribute{
				MarkdownDescription: "Form Factor for the Vseries Nodes",
				Required: true,
			},

			"image_id": schema.StringAttribute{
				MarkdownDescription: "Vseries Image to be loaded on the fabric nodes",
				Required: true,
			},

			"management_interface_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Management Interface spec common to all nodes in the fabric. Can be overrirden on a specific host, by providing the host specific Management Interface details",
				Optional: true,
				PlanModifiers: []planmodifier.Object{
                    objectplanmodifier.RequiresReplace(),
                },
				Attributes: map[string]schema.Attribute{
					"interface_name": schema.StringAttribute{
						MarkdownDescription:"Name of the management network",
						Optional: true,
					},
					"interface_moref": schema.StringAttribute{
						MarkdownDescription:"Vcenter MORefof the management network",
						Optional: true,
					},
					"address_assignment_mode": schema.StringAttribute{
						MarkdownDescription:"Scheme for IP address assignment DHCP/Static",
						Optional: true,
					},
                },
			},
			"tunnel_interface_spec": schema.SingleNestedAttribute{
				MarkdownDescription: "Tunnel Interface spec common to all nodes in the fabric. Can be overrirden on a specific host, by providing the host specific Management Interface details",
				Optional: true,
				PlanModifiers: []planmodifier.Object{
                    objectplanmodifier.RequiresReplace(),
                },
				Attributes: map[string]schema.Attribute{
					"interface_name": schema.StringAttribute{
						MarkdownDescription:"Name of the management network",
						Optional: true,
					},
					"interface_moref": schema.StringAttribute{
						MarkdownDescription:"Vcenter MORefof the management network",
						Optional: true,
					},
					"address_assignment_mode": schema.StringAttribute{
						MarkdownDescription:"Scheme for IP address assignment DHCP/Static",
						Optional: true,
					},
                },
			},
			"host_vm_spec": schema.MapNestedAttribute{
				MarkdownDescription: "Spec for the Vseries node on each host in this fabric",
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hostname": schema.StringAttribute{
							MarkdownDescription: "Host on which to start the Vseries node",
							Optional: true,
						},
						"host_moref": schema.StringAttribute{
							MarkdownDescription: "Vcenter MORef for this host",
							Required: true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the vseries node spun up on this host",
							Required: true,
						},
			            "management_interface_spec": schema.SingleNestedAttribute{
				            MarkdownDescription: "Management Interface spec specific to this host, overrides any varabile at the top level",
				            Optional: true,
				            PlanModifiers: []planmodifier.Object{
                                objectplanmodifier.RequiresReplace(),
                            },
				            Attributes: map[string]schema.Attribute{
					            "interface_name": schema.StringAttribute{
						            MarkdownDescription:"Name of the management network",
						            Optional: true,
					            },
					            "interface_moref": schema.StringAttribute{
						            MarkdownDescription:"Vcenter MORefof the management network",
						            Optional: true,
					            },
					            "address_assignment_mode": schema.StringAttribute{
						            MarkdownDescription:"Scheme for IP address assignment DHCP/Static",
						            Optional: true,
					            },
							},
						},
			            "tunnel_interface_spec": schema.SingleNestedAttribute{
				            MarkdownDescription: "Tunel Interface spec specific to this host, and overrides the default for all keys specified in this build",
				            Optional: true,
				            PlanModifiers: []planmodifier.Object{
                                objectplanmodifier.RequiresReplace(),
                            },
				            Attributes: map[string]schema.Attribute{
					            "interface_name": schema.StringAttribute{
						            MarkdownDescription:"Name of the management network",
						            Optional: true,
					            },
					            "interface_moref": schema.StringAttribute{
						            MarkdownDescription:"Vcenter MORefof the management network",
						            Optional: true,
					            },
					            "address_assignment_mode": schema.StringAttribute{
						            MarkdownDescription:"Scheme for IP address assignment DHCP/Static",
						            Optional: true,
					            },
							},
						},
                    },
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Fabric for later use",
				PlanModifiers: []planmodifier.String{
                   stringplanmodifier.UseStateForUnknown(),
               },
			},
		},
	}
}

func (f *EsxiFabric) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	f.fmClient = fmClient
}

func (f *EsxiFabric) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EsxiFabricModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Creating the fabric")

	// For now just simply set the ID field
	data.Id = types.StringValue("MyID")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (f *EsxiFabric) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EsxiFabricModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (f *EsxiFabric) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
    resp.Diagnostics.AddError(
         "Esxi Fabric does not support any modifications",
		 "ESXI Fabric can only be created/deleted. They cannot be modified",
	)
}

func (f *EsxiFabric) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EsxiFabricModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	return
}
