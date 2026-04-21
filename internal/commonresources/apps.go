// Copyright (c) Gigamon, Inc.

// Implements the APP Resrouces that are common across all environment

package commonresources

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-gigamon/internal/commonutils"
	"terraform-provider-gigamon/internal/fmclient"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &DedupConfig{}
var _ resource.Resource = &Slicing{}
var _ resource.Resource = &Dedup{}
var _ resource.Resource = &Masking{}
var _ resource.Resource = &HeaderStripping{}
var _ resource.Resource = &LoadBalancing{}
var _ resource.Resource = &Amx{}

// Dedup Config app resoruce, which manages the dedup configuration
//
// Dedup configuration is applied globally across all dedup instances in a MD.
func NewDedupConfig() resource.Resource {
	return &DedupConfig{}
}

// Slicing app resoruce, which manages the slicing applications
func NewSlicing() resource.Resource {
	return &Slicing{}
}

// Dedup app resource, which manages the dedup application instances
func NewDedup() resource.Resource {
	return &Dedup{}
}

// Masking app resource, which manages the masking application instances
func NewMasking() resource.Resource {
	return &Masking{}
}

// Header Stripping app resource, which manages header stripping instances
func NewHeaderStripping() resource.Resource {
	return &HeaderStripping{}
}

// Load Balancing app resource, which manages load balacing instances
func NewLoadBalancing() resource.Resource {
	return &LoadBalancing{}
}

// AMX app resource, which manages the AMX application instances
func NewAmx() resource.Resource {
	return &Amx{}
}

// Dedup config manages the dedup app config on a per MD basis
type DedupConfig struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Slicing manages the slicing app
type Slicing struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Dedup manages the dedup app instance on a monitoring session
type Dedup struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// Masking manages the masking app instance on a monitoring session
type Masking struct {
	fmClient *fmclient.FmClient // Instance to our FM http client instance
}

// HeaderStripping manages the header stripping app instance on a monitoring session
type HeaderStripping struct {
	fmClient *fmclient.FmClient
}

// LoadBalancing manages the load balancing app instance on a monitoring session
type LoadBalancing struct {
	fmClient *fmclient.FmClient
}

// Amx manages the AMX app instance on a monitoring session
type Amx struct {
	fmClient *fmclient.FmClient
}

// DedupConfig App model. The ID provided would be dedupconfig:<monitoring domain id>
type DedupConfigModel struct {
	MonitoringDomainId types.String `tfsdk:"monitoring_domain_id"`
	Action             types.String `tfsdk:"action"`
	Timer              types.Int32  `tfsdk:"timer"`
	IPTClass           types.String `tfsdk:"ipv6_traffic_class"`
	IPTos              types.String `tfsdk:"ipv4_tos_field"`
	TCPSeq             types.String `tfsdk:"tcp_sequence"`
	Vlan               types.String `tfsdk:"vlan"`
	Id                 types.String `tfsdk:"id"`
}

// Slicing App model
type SlicingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`
	Protocol            types.String `tfsdk:"protocol"`
	Offset              types.Int32  `tfsdk:"offset"`
}

// Masking App Model
type MaskingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`
	Protocol            types.String `tfsdk:"protocol"`
	Offset              types.Int32  `tfsdk:"offset"`
	Length              types.Int32  `tfsdk:"length"`
	Pattern             types.String `tfsdk:"pattern"`
	ContentType         types.String `tfsdk:"content_type"`
}

// Dedup App Model
type DedupModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`
	Description         types.String `tfsdk:"description"`
}

// Per‑protocol TF config for Header Stripping (nested blocks)
type HeaderStrippingVxlanConfig struct {
	VxlanId types.Int32 `tfsdk:"vxlan_id"`
}

type HeaderStrippingVlanConfig struct {
	VlanHeader types.String `tfsdk:"vlan_header"`
}

type HeaderStrippingFm6000TsConfig struct {
	TimestampFormat types.String `tfsdk:"timestamp_format"`
}

type HeaderStrippingErspanConfig struct {
	FlowId types.Int32 `tfsdk:"flow_id"`
}

type HeaderStrippingGenericConfig struct {
	Ah1              types.String `tfsdk:"ah1"`
	Offset           types.String `tfsdk:"offset"`
	OffsetRangeValue types.Int32  `tfsdk:"offset_range_value"`
	HeaderCount      types.Int32  `tfsdk:"header_count"`
	CustomLen        types.Int32  `tfsdk:"custom_len"`
	Ah2              types.String `tfsdk:"ah2"`
}

// Header Stripping App Model (top‑level TF resource model)
type HeaderStrippingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`

	// Computed from which block is set, like tunnel "type"
	Protocol types.String `tfsdk:"protocol"`

	// Blocks with parameters
	Vxlan    *HeaderStrippingVxlanConfig    `tfsdk:"vxlan"`
	Vlan     *HeaderStrippingVlanConfig     `tfsdk:"vlan"`
	Fm6000Ts *HeaderStrippingFm6000TsConfig `tfsdk:"fm6000_ts"`
	Erspan   *HeaderStrippingErspanConfig   `tfsdk:"erspan"`
	Generic  *HeaderStrippingGenericConfig  `tfsdk:"generic"`

	// Simple protocols (no per‑protocol attributes, just presence)
	Gtp          *HeaderStrippingEmptyConfig `tfsdk:"gtp"`
	Isl          *HeaderStrippingEmptyConfig `tfsdk:"isl"`
	Mpls         *HeaderStrippingEmptyConfig `tfsdk:"mpls"`
	MplsPlusVlan *HeaderStrippingEmptyConfig `tfsdk:"mpls_plus_vlan"`
	Vntag        *HeaderStrippingEmptyConfig `tfsdk:"vntag"`
	Geneve       *HeaderStrippingEmptyConfig `tfsdk:"geneve"`
}

type LoadBalancingStatelessConfig struct {
	HashFields    types.String `tfsdk:"hash_fields"`
	FieldLocation types.String `tfsdk:"field_location"`
}

// Enhanced LB config (ELB profile)
type LoadBalancingEnhancedConfig struct {
	Profile types.String `tfsdk:"profile"`
}

// Per-group config
type LoadBalancingGroupModel struct {
	AepId  types.Int32 `tfsdk:"aep_id"`
	Weight types.Int32 `tfsdk:"weight"`
}

// Top‑level TF model
type LoadBalancingModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Description         types.String `tfsdk:"description"`
	Id                  types.String `tfsdk:"id"`

	// Exactly one of these must be set
	Stateless *LoadBalancingStatelessConfig `tfsdk:"stateless"`
	Enhanced  *LoadBalancingEnhancedConfig  `tfsdk:"enhanced"`

	// Present in both Stateless and Enhanced (except greFlowid special‑case)
	Group []LoadBalancingGroupModel `tfsdk:"group"`
}

// Top-level TF model for AMX
type AmxModel struct {
	MonitoringSessionId types.String `tfsdk:"monitoring_session_id"`
	Alias               types.String `tfsdk:"alias"`
	Id                  types.String `tfsdk:"id"`

	Ingestor []AmxIngestorModel `tfsdk:"ingestor"`

	Exporter *AmxExporterModel `tfsdk:"exporter"`

	MobilityEnrichment []AmxMobilityEnrichmentModel `tfsdk:"mobility_enrichment"`
	WorkloadEnrichment []AmxWorkloadEnrichmentModel `tfsdk:"workload_enrichment"`
	OtherEnrichment    []AmxOtherEnrichmentModel    `tfsdk:"other_enrichment"`
}

// Ingestor
type AmxIngestorModel struct {
	Name types.String `tfsdk:"name"`
	Port types.Int32  `tfsdk:"port"`
	Type types.String `tfsdk:"type"` // ami, netflow, gtpc, gtpc_hier, aws, azure, vmware_esxi, vmware_nsxt, k8s
}

// Exporter (HTTP + Kafka)
type AmxExporterModel struct {
	Debug       types.Bool            `tfsdk:"debug"`
	HttpExport  []AmxHttpExportModel  `tfsdk:"http_export"`
	KafkaExport []AmxKafkaExportModel `tfsdk:"kafka_export"`
}

// HTTP export (cloud upload)
type AmxHttpExportModel struct {
	Name                  types.String `tfsdk:"name"`
	Enabled               types.Bool   `tfsdk:"enabled"`
	DataType              types.String `tfsdk:"data_type"` // ami, mobility, ami_enriched, netflow
	Endpoint              types.String `tfsdk:"endpoint"`
	Headers               types.List   `tfsdk:"headers"` // list(string)
	SecureKeys            types.List   `tfsdk:"secure_keys"`
	BindIPAddress         types.String `tfsdk:"bind_ip_address"`
	Format                types.String `tfsdk:"format"` // json
	Compress              types.Bool   `tfsdk:"compress"`
	FlushIntervalSeconds  types.Int32  `tfsdk:"flush_interval_seconds"`
	ParallelWorkers       types.Int32  `tfsdk:"parallel_workers"`
	MaxRetries            types.Int32  `tfsdk:"max_retries"`
	MaxRecordsPerBatch    types.Int32  `tfsdk:"max_records_per_batch"`
	SelfHealWindowSeconds types.Int32  `tfsdk:"self_heal_window_seconds"`
	UploadTimeoutSeconds  types.Int32  `tfsdk:"upload_timeout_seconds"`
	Labels                types.Map    `tfsdk:"labels"` // map(string)
}

// Kafka export
type AmxKafkaExportModel struct {
	Name                  types.String `tfsdk:"name"`
	Topic                 types.String `tfsdk:"topic"`
	Enabled               types.Bool   `tfsdk:"enabled"`
	Brokers               types.List   `tfsdk:"brokers"` // list(string)
	BindIPAddress         types.String `tfsdk:"bind_ip_address"`
	DataType              types.String `tfsdk:"data_type"` // ami, mobility, ami_enriched, netflow
	Format                types.String `tfsdk:"format"`    // json
	Compress              types.Bool   `tfsdk:"compress"`
	FlushIntervalSeconds  types.Int32  `tfsdk:"flush_interval_seconds"`
	ParallelWorkers       types.Int32  `tfsdk:"parallel_workers"`
	MaxRetries            types.Int32  `tfsdk:"max_retries"`
	MaxRecordsPerBatch    types.Int32  `tfsdk:"max_records_per_batch"`
	SelfHealWindowSeconds types.Int32  `tfsdk:"self_heal_window_seconds"`
	Labels                types.Map    `tfsdk:"labels"`           // map(string)
	ProducerConfigs       types.List   `tfsdk:"producer_configs"` // list(string)
}

// Common source information
type AmxSourceInfoModel struct {
	Name           types.String            `tfsdk:"name"`
	SourceSettings []AmxSourceSettingModel `tfsdk:"setting"`
}

type AmxSourceSettingModel struct {
	Secure types.Bool   `tfsdk:"secure"`
	File   types.String `tfsdk:"file"`  // optional: path to file (e.g. kubeconfig)
	Key    types.String `tfsdk:"key"`   // propertyKey
	Value  types.String `tfsdk:"value"` // propertyValue (plain)
}

// Mobility enrichment
type AmxMobilityEnrichmentModel struct {
	Name       types.String `tfsdk:"name"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Attributes types.List   `tfsdk:"attributes"` // list(string)
}

// Workload enrichment (per platform)
type AmxWorkloadEnrichmentModel struct {
	Aws           []AmxWorkloadPlatformModel `tfsdk:"aws"`
	Azure         []AmxWorkloadPlatformModel `tfsdk:"azure"`
	VmwareVcenter []AmxWorkloadPlatformModel `tfsdk:"vmware_vcenter"`
	Aks           []AmxWorkloadPlatformModel `tfsdk:"aks"`
}

type AmxWorkloadPlatformModel struct {
	Name       types.String         `tfsdk:"name"`
	Enabled    types.Bool           `tfsdk:"enabled"`
	Attributes types.List           `tfsdk:"attributes"` // list(string)
	Settings   types.Map            `tfsdk:"settings"`   // map(string)
	Sources    []AmxSourceInfoModel `tfsdk:"source"`
}

// Other enrichment
type AmxOtherEnrichmentModel struct {
	Name       types.String `tfsdk:"name"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	Attributes types.List   `tfsdk:"attributes"` // list(string)
	Settings   types.List   `tfsdk:"settings"`   // list(string)
}

// Per‑protocol sub‑structs for Header Stripping (FM payload)
type FMHeaderStrippingVxlan struct {
	VxlanId int32 `json:"vxlanId"` // 0 is valid, so no ,omitempty
}

type FMHeaderStrippingVlan struct {
	VlanHeader string `json:"vlanHeader"` // "outer"/"all"
}

type FMHeaderStrippingFm6000Ts struct {
	TimestampFormat string `json:"timestampFormat"`
}

type FMHeaderStrippingErspan struct {
	ErspanFlowId int32 `json:"erspanFlowId"` // required when erspan block exists
}

type FMHeaderStrippingGeneric struct {
	Ah1              string `json:"ah1"`                        // "none"/"eth"/...
	Offset           string `json:"offset"`                     // "start"/"end"/"offsetRange"
	OffsetRangeValue *int32 `json:"offsetRangeValue,omitempty"` // 0 is valid when offsetRange
	HeaderCount      int32  `json:"headerCount,omitempty"`
	CustomLen        int32  `json:"customLen,omitempty"`
	Ah2              string `json:"ah2"` // "none"/"eth"/...
}

// FM payload struct for Header Stripping (used in /monitoringSessions/{id}/update)
type FMHeaderStripping struct {
	Alias    string `json:"alias,omitempty"`
	Name     string `json:"name,omitempty"`
	Protocol string `json:"protocol,omitempty"`

	Vxlan    *FMHeaderStrippingVxlan    `json:"vxlan,omitempty"`
	Vlan     *FMHeaderStrippingVlan     `json:"vlan,omitempty"`
	Fm6000Ts *FMHeaderStrippingFm6000Ts `json:"fm6000Ts,omitempty"`
	Erspan   *FMHeaderStrippingErspan   `json:"erspan,omitempty"`
	Generic  *FMHeaderStrippingGeneric  `json:"generic,omitempty"`

	Id string `json:"id,omitempty"`
}

type HeaderStrippingEmptyConfig struct{}

type FMLoadBalancingStateless struct {
	HashFields    string `json:"hashFields,omitempty"`
	FieldLocation string `json:"fieldLocation,omitempty"`
}

type FMLoadBalancingEnhanced struct {
	Profile string `json:"profile,omitempty"`
}

type FMLoadBalancingGroup struct {
	AepId  int32 `json:"aepId,omitempty"`
	Weight int32 `json:"weight,omitempty"`
}

type FMLoadBalancing struct {
	Alias       string                    `json:"alias"`
	Name        string                    `json:"name"`
	Description string                    `json:"description,omitempty"`
	Id          string                    `json:"id,omitempty"`
	Stateless   *FMLoadBalancingStateless `json:"stateless,omitempty"`
	Enhanced    *FMLoadBalancingEnhanced  `json:"enhanced,omitempty"`
	Lbg         []FMLoadBalancingGroup    `json:"lbg,omitempty"`
}

// FM payload for AMX app
type FMAmx struct {
	Alias          string                `json:"alias,omitempty"`
	Name           string                `json:"name,omitempty"` // "ogw"
	Ingestor       []FMAmxIngestor       `json:"ingestor,omitempty"`
	Exporter       *FMAmxExporter        `json:"exporter,omitempty"`
	AttrEnrichment []FMAmxAttrEnrichment `json:"attrEnrichment,omitempty"`
	Id             string                `json:"id,omitempty"`
}

type FMAmxIngestor struct {
	Port int32  `json:"port,omitempty"`
	Type string `json:"type,omitempty"`
	Name string `json:"name,omitempty"`
}

type FMAmxExporter struct {
	CloudUpload []FMAmxCloudUpload `json:"cloudUpload"`
	Kafka       []FMAmxKafka       `json:"kafka,omitempty"`
	Debug       *bool              `json:"debug,omitempty"`
}

type FMAmxCloudUpload struct {
	Name                    string            `json:"name,omitempty"`
	Enable                  *bool             `json:"enable,omitempty"`
	Endpoint                string            `json:"endpoint,omitempty"`
	MaskEndpointApiKey      *bool             `json:"maskEndpointApiKey,omitempty"`
	SecureKeys              []string          `json:"secureKeys,omitempty"`
	Headers                 []string          `json:"headers"`
	IfaceIPAddress          string            `json:"ifaceIPAddress,omitempty"`
	Format                  string            `json:"format,omitempty"`
	Zip                     *bool             `json:"zip,omitempty"`
	Interval                *int32            `json:"interval,omitempty"`
	Writers                 *int32            `json:"writers,omitempty"`
	Retries                 *int32            `json:"retries,omitempty"`
	MaxEntries              *int32            `json:"maxEntries,omitempty"`
	SelfHealTimerWindow     *int32            `json:"selfHealTimerWindow,omitempty"`
	HttpClientUploadTimeout *int32            `json:"httpClientUploadTimeout,omitempty"`
	Labels                  map[string]string `json:"labels,omitempty"`
	Type                    string            `json:"type,omitempty"`
}

type FMAmxKafka struct {
	Name                string            `json:"name,omitempty"`
	Topic               string            `json:"topic,omitempty"`
	Enable              *bool             `json:"enable,omitempty"`
	Brokers             []string          `json:"brokers,omitempty"`
	IfaceIPAddress      string            `json:"ifaceIPAddress,omitempty"`
	Format              string            `json:"format,omitempty"`
	Zip                 *bool             `json:"zip,omitempty"`
	Interval            *int32            `json:"interval,omitempty"`
	Writers             *int32            `json:"writers,omitempty"`
	Retries             *int32            `json:"retries,omitempty"`
	MaxEntries          *int32            `json:"maxEntries,omitempty"`
	SelfHealTimerWindow *int32            `json:"selfHealTimerWindow,omitempty"`
	Labels              map[string]string `json:"labels,omitempty"`
	ProducerConfigs     []string          `json:"producerConfigs,omitempty"`
	Type                string            `json:"type,omitempty"`
}

type FMAmxAttrEnrichment struct {
	Name              string                   `json:"name,omitempty"`
	Type              string                   `json:"type,omitempty"`
	Attributes        []string                 `json:"attributes,omitempty"`
	Settings          []string                 `json:"settings,omitempty"`
	SourceInformation []FMAmxSourceInformation `json:"sourceInformation,omitempty"`
	Enable            *bool                    `json:"enable,omitempty"`
}

type FMAmxSourceInformation struct {
	Name           string               `json:"name,omitempty"`
	SourceSettings []FMAmxSourceSetting `json:"sourceSettings,omitempty"`
}

type FMAmxSourceSetting struct {
	SecureKey              bool   `json:"secureKey"`
	FileName               string `json:"fileName,omitempty"`
	PropertyKey            string `json:"propertyKey,omitempty"`
	PropertyValue          string `json:"propertyValue,omitempty"`
	PropertyValueEncrypted string `json:"propertyValueEncrypted,omitempty"`
}

// Dedup Config Application TF Hooks
func (decfg *DedupConfig) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dedup_md_config"
}

func (decfg *DedupConfig) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon Dedup Config Schema",

		Attributes: map[string]schema.Attribute{
			"monitoring_domain_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring domain ID for this dedup config",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "Action to take on the duplicate packets",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("drop"),
				Validators: []validator.String{
					stringvalidator.OneOf("drop", "count"),
				},
			},
			"timer": schema.Int32Attribute{
				MarkdownDescription: "Time to wait for duplicates in micro seconds",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(50000),
				Validators: []validator.Int32{
					int32validator.AtLeast(10),
					int32validator.AtMost(500000),
				},
			},
			"ipv6_traffic_class": schema.StringAttribute{
				MarkdownDescription: "include or ignore the IPv6 Traffic Class filed",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"ipv4_tos_field": schema.StringAttribute{
				MarkdownDescription: "include or ignore the IPv4 TOS/DSCP field",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"tcp_sequence": schema.StringAttribute{
				MarkdownDescription: "include or ignore the TCP Sequence Number field",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("include"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"vlan": schema.StringAttribute{
				MarkdownDescription: "include or ignore the VLAN ID field in l2 header",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("ignore"),
				Validators: []validator.String{
					stringvalidator.OneOf("include", "ignore"),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Session for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (decfg *DedupConfig) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	decfg.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (decfg *DedupConfig) createFMStruct(data *DedupConfigModel) *FMDedupConfig {
	return &FMDedupConfig{
		Action:   data.Action.ValueString(),
		Timer:    data.Timer.ValueInt32(),
		IPTClass: data.IPTClass.ValueString(),
		IPTos:    data.IPTos.ValueString(),
		Vlan:     data.Vlan.ValueString(),
		TCPSeq:   data.TCPSeq.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (decfg *DedupConfig) updateTFStruct(data *DedupConfigModel, fmData *FMDedupConfig) {
	data.Action = types.StringValue(fmData.Action)
	data.IPTClass = types.StringValue(fmData.IPTClass)
	data.IPTos = types.StringValue(fmData.IPTos)
	data.Vlan = types.StringValue(fmData.Vlan)
	data.TCPSeq = types.StringValue(fmData.TCPSeq)
	data.Timer = types.Int32Value(fmData.Timer)
}

// Create call for new Dedup Config Object
// This is a MD single instance in FM, and there is no need to create as it is already present
// just do a PUT to update the values as desired by the user, and return our ID for this
func (decfg *DedupConfig) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DedupConfigModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types
	fmData := decfg.createFMStruct(&data)
	gsData := GsParams{
		GsParamsName: "gsParams",
		Dedup:        *fmData,
	}

	err := SetGsParams(
		ctx,
		data.MonitoringDomainId.ValueString(),
		&gsData,
		decfg.fmClient,
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Set the Dedup parameters",
			fmt.Sprintf("error while setting the dedup parameters: %v", err),
		)
		return
	}

	// Build typed ID for this dedup config: app::dedup::<mdUUID>
	mdUUID, err := commonutils.UUIDFromTypedID(data.MonitoringDomainId.ValueString())
	if err != nil {
		return
	}

	typedID, err := commonutils.MakeTypedID(commonutils.ModuleApp, commonutils.TypeDedup, mdUUID)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (decfg *DedupConfig) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DedupConfigModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	gsParams, err := GetGsParams(
		ctx,
		data.MonitoringDomainId.ValueString(),
		decfg.fmClient,
	)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Get dedup parameters",
			fmt.Sprintf("dedup configuration get failed: %v", err),
		)
		return
	}

	// Save updated data into Terraform state
	decfg.updateTFStruct(&data, &gsParams.Dedup)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (decfg *DedupConfig) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData DedupConfigModel

	// Read desired config from the plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build FM payload from the plan
	fmData := decfg.createFMStruct(&planData)

	gsData := GsParams{
		GsParamsName: "gsParams",
		Dedup:        *fmData,
	}

	// Send plan to FM
	err := SetGsParams(
		ctx,
		planData.MonitoringDomainId.ValueString(),
		&gsData,
		decfg.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Update the Dedup parameters",
			fmt.Sprintf("error while updating the dedup parameters: %v", err),
		)
		return
	}

	decfg.updateTFStruct(&planData, fmData)

	// Final state = plan (normalized via updateTFStruct)
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (decfg *DedupConfig) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Nothing to do in delete, as this is a permanent singleton object in FM
}

// Slicing Application TF Hooks
func (s *Slicing) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_slicing"
}

func (s *Slicing) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Gigamon APP Slicing Schema",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this slicing application",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session ID on which to deploy this APP",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Protocol to check and skip before applying the offset",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				Validators: []validator.String{
					stringvalidator.OneOf("none", "ipv4", "ipv6", "udp", "tcp", "ftp-data", "https", "ssh", "gtp", "gtp-ipv4", "gtp-udp", "gtp-tcp"),
				},
			},
			"offset": schema.Int32Attribute{
				MarkdownDescription: "Offset at which to slice.",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(64),
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this Monitoring Session for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (s *Slicing) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	s.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (s *Slicing) createFMStruct(data *SlicingModel) *FMSlicing {
	return &FMSlicing{
		Alias:    data.Alias.ValueString(),
		Name:     "slicing",
		Protocol: data.Protocol.ValueString(),
		Offset:   data.Offset.ValueInt32(),
		Id:       data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (s *Slicing) updateTFStruct(data *SlicingModel, fmData *FMSlicing) {
	data.Protocol = types.StringValue(fmData.Protocol)
	data.Offset = types.Int32Value(fmData.Offset)
}

// Create call for new Slicing App Instance
func (s *Slicing) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SlicingModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmData := s.createFMStruct(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "create",
				Application: fmData,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		s.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create slicing app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleApp,
		commonutils.TypeSlicing,
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (s *Slicing) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SlicingModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	slicingData := FMSlicing{}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	err = GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawID,
		"slicing",
		"",
		&slicingData,
		s.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		tflog.Info(ctx, "Slicing app data read failed ******", nil)
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		tflog.Info(ctx, "Not a not found error Slicing app data read failed ******", nil)
		resp.Diagnostics.AddError(
			"Unable to Get Slicing App details",
			fmt.Sprintf("unable to get Slicing App details. error is %v", err),
		)
		return
	}

	// Save updated data into Terraform state
	s.updateTFStruct(&data, &slicingData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (s *Slicing) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData SlicingModel

	// Read desired values from the plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Copy the TF Types over to regular GO types and get the content body
	fmData := s.createFMStruct(&planData)

	rawID, err := commonutils.UUIDFromTypedID(planData.Id.ValueString())
	if err != nil {
		return
	}
	fmData.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		s.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update slicing app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	// Let FM override computed/FM-owned fields (protocol, offset, id)
	s.updateTFStruct(&planData, fmData)

	// Final state = plan + FM-owned overrides
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (s *Slicing) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SlicingModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMSlicing{
					Id:   rawID,
					Name: "Application",
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		s.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete slicing app",
			fmt.Sprintf("app deeltion failed: %s", err),
		)
	}
}

// Dedup Application TF Hooks
func (d *Dedup) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_dedup"
}

func (d *Dedup) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon APP Dedup Schema",
		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this dedup application",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session ID on which to deploy this APP",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description for this dedup app",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this App instance for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (d *Dedup) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	d.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (d *Dedup) createFMStruct(data *DedupModel) *FMDedup {
	return &FMDedup{
		Alias:       data.Alias.ValueString(),
		Name:        "dedup",
		Description: data.Description.ValueString(),
		Id:          data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (d *Dedup) updateTFStruct(data *DedupModel, fmData *FMDedup) {

	if fmData.Description != "" {
		data.Description = types.StringValue(fmData.Description)
	}
}

// Create call for new Dedup App Instance
func (d *Dedup) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DedupModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmData := d.createFMStruct(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "create",
				Application: fmData,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		d.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create dedup app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleApp,
		commonutils.TypeDedup,
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *Dedup) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DedupModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	dedupData := FMDedup{}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	err = GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawID,
		"dedup",
		"",
		&dedupData,
		d.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		tflog.Info(ctx, "**** Dedup app data read failed ******", nil)
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		tflog.Info(ctx, "*** Not a not found error dedup app data read failed ******", nil)
		resp.Diagnostics.AddError(
			"Unable to Get Dedup App details",
			fmt.Sprintf("unable to get Dedup App details. error is %v", err),
		)
		return
	}
	tflog.Info(ctx, "**** Dedup app data read SUCCESS ******", nil)

	d.updateTFStruct(&data, &dedupData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *Dedup) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData DedupModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fmData := d.createFMStruct(&planData)

	rawID, err := commonutils.UUIDFromTypedID(planData.Id.ValueString())
	if err != nil {
		return
	}
	fmData.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		d.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update dedup app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	// Overlay FM-owned fields (id, description normalization)
	d.updateTFStruct(&planData, fmData)

	// Final state = plan + FM overrides
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (d *Dedup) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DedupModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMDedup{
					Id:   rawID,
					Name: "Application",
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		d.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete dedup app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}

// Masking Application TF Hooks
func (m *Masking) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_masking"
}

func (m *Masking) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon APP Masking Schema",
		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this masking application",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session ID on which to deploy this APP",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "If specified the offset if calcualted from the end of this protocol header, if none, the offset starts from the first byte f the packet",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				Validators: []validator.String{
					stringvalidator.OneOf("none", "ipv4", "ipv6", "udp", "tcp", "ftp-data", "https", "ssh", "gtp", "gtp-ipv4", "gtp-udp", "gtp-tcp", "sip"),
				},
			},
			"offset": schema.Int32Attribute{
				MarkdownDescription: "Offset at which to start masking, relative to the protocol field value",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(64),
			},
			"length": schema.Int32Attribute{
				MarkdownDescription: "Number of bytes to mask from offset. Not valid for protocol sip, but required otherwise",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
				},
			},
			"pattern": schema.StringAttribute{
				MarkdownDescription: "one byte hex value, which is the pattern to be written. Not valid for sip protocol but required otherwise",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^0[xX][0-9a-fA-F]{2}$`),
						"mut be a hexadecimal 1 byte value e.g. 0x08 or 0xFF",
					),
				},
			},
			"content_type": schema.StringAttribute{
				MarkdownDescription: "For SIP, indicates which packets to mask. Must if protocol is sip",
				Optional:            true,
			},

			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this App instance for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Initial Configure call, to initialize the Provider
func (m *Masking) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	m.fmClient = fmClient
}

// Create a FM DS from the TF DS and return the same
func (m *Masking) createFMStruct(data *MaskingModel) *FMMasking {
	return &FMMasking{
		Alias:       data.Alias.ValueString(),
		Name:        "masking",
		Protocol:    data.Protocol.ValueString(),
		Offset:      data.Offset.ValueInt32(),
		Length:      data.Length.ValueInt32(),
		Pattern:     data.Pattern.ValueString(),
		ContentType: data.ContentType.ValueString(),
		Id:          data.Id.ValueString(),
	}
}

// Update the TF Data from the FM struct
func (m *Masking) updateTFStruct(data *MaskingModel, fmData *FMMasking) {
	data.Protocol = types.StringValue(fmData.Protocol)
	data.Offset = types.Int32Value(fmData.Offset)
	if fmData.Protocol == "sip" {
		data.ContentType = types.StringValue(fmData.ContentType)
	} else {
		data.Length = types.Int32Value(fmData.Length)
		data.Pattern = types.StringValue(fmData.Pattern)
	}
}

// Validates the input parameters
func (m *Masking) validateParams(data *MaskingModel) error {
	if data.Protocol.ValueString() == "sip" {
		if data.ContentType.IsNull() || data.ContentType.IsUnknown() {
			return fmt.Errorf("for sip protocol, the content_type parameter must be specified")
		}
		if !data.Length.IsNull() || !data.Pattern.IsNull() {
			return fmt.Errorf(
				"For sip protocol, the fields length and pattern are not allowed",
			)
		}
	} else {
		if data.Length.IsNull() || data.Pattern.IsNull() {
			return fmt.Errorf(
				"for all non sip protocols, the length and pattern field is mandatory",
			)
		}
		if !data.ContentType.IsNull() {
			return fmt.Errorf("for non sip protocols, the content_type field is not valid")
		}
	}
	return nil
}

// Create call for new Masking App Instance
func (m *Masking) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data MaskingModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := m.validateParams(&data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid parameters specified",
			fmt.Sprintf("Invalid parameters for masking app: %s", err),
		)
		return
	}

	fmData := m.createFMStruct(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "create",
				Application: fmData,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		m.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create masking app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleApp,
		commonutils.TypeMasking,
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (m *Masking) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data MaskingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	maskingData := FMMasking{}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	err = GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawID,
		"masking",
		"",
		&maskingData,
		m.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to Get Masking App details",
			fmt.Sprintf("unable to get Masking App details. error is %v", err),
		)
		return
	}

	m.updateTFStruct(&data, &maskingData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (m *Masking) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData MaskingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := m.validateParams(&planData)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update masking app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	fmData := m.createFMStruct(&planData)

	rawID, err := commonutils.UUIDFromTypedID(planData.Id.ValueString())
	if err != nil {
		return
	}
	fmData.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		m.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update masking app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	// Let FM override computed/FM-owned fields
	m.updateTFStruct(&planData, fmData)

	// Final state = plan + FM-owned overrides
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (m *Masking) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data MaskingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMMasking{
					Id:   rawID,
					Name: "Application",
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		m.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete masking app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}

func (h *HeaderStripping) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_header_stripping"
}

func simpleHSBlock(desc string) schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: desc,
		Attributes:          map[string]schema.Attribute{},
	}
}

func vxlanHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "VXLAN header stripping config (protocol = vxlan)",

		Attributes: map[string]schema.Attribute{
			"vxlan_id": schema.Int32Attribute{
				MarkdownDescription: "24‑bit VXLAN ID to strip; 0 strips all VXLAN IDs",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(16777215),
				},
			},
		},
	}
}

func vlanHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "VLAN header stripping config (protocol = vlan)",
		Attributes: map[string]schema.Attribute{
			"vlan_header": schema.StringAttribute{
				MarkdownDescription: "Target VLAN header(s) to remove",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("all"),
				Validators: []validator.String{
					stringvalidator.OneOf("outer", "all"),
				},
			},
		},
	}
}

func fm6000TsHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "FM6000Ts header stripping config (protocol = fm6000Ts)",
		Attributes: map[string]schema.Attribute{
			"timestamp_format": schema.StringAttribute{
				MarkdownDescription: "Timestamp format",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("none"),
				Validators: []validator.String{
					stringvalidator.OneOf("none"),
				},
			},
		},
	}
}

func erspanHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "ERSPAN header stripping config (protocol = erspan)",
		Attributes: map[string]schema.Attribute{
			"flow_id": schema.Int32Attribute{
				MarkdownDescription: "ERSPAN flow ID; 0 matches all flows",
				Optional:            true,
				Computed:            true,
				Default:             int32default.StaticInt32(0),
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(1023),
				},
			},
		},
	}
}

func genericHSBlock() schema.SingleNestedBlock {
	return schema.SingleNestedBlock{
		MarkdownDescription: "Generic header stripping config (protocol = generic)",
		Attributes: map[string]schema.Attribute{
			"ah1": schema.StringAttribute{
				MarkdownDescription: "First anchor header",
				Optional:            true, // was Required: true
				Validators: []validator.String{
					stringvalidator.OneOf("none", "eth", "vlan", "mpls", "ipv4", "ipv6"),
				},
			},
			"offset": schema.StringAttribute{
				MarkdownDescription: "Offset mode: start, end, offsetRange",
				Optional:            true, // was Required: true
				Validators: []validator.String{
					stringvalidator.OneOf("start", "end", "offsetRange"),
				},
			},
			"offset_range_value": schema.Int32Attribute{
				MarkdownDescription: "Offset from AH1 when offset = offsetRange (0–1500); 0 is valid",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
					int32validator.AtMost(1500),
				},
			},
			"header_count": schema.Int32Attribute{
				MarkdownDescription: "Number of headers to remove",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(32),
				},
			},
			"custom_len": schema.Int32Attribute{
				MarkdownDescription: "Length (bytes) of header to strip",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(1),
					int32validator.AtMost(1500),
				},
			},
			"ah2": schema.StringAttribute{
				MarkdownDescription: "Second anchor header",
				Optional:            true, // was Required: true
				Validators: []validator.String{
					stringvalidator.OneOf("none", "eth", "vlan", "mpls"),
				},
			},
		},
	}
}

func (h *HeaderStripping) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("vxlan"),
			path.MatchRoot("vlan"),
			path.MatchRoot("fm6000_ts"),
			path.MatchRoot("erspan"),
			path.MatchRoot("generic"),
			path.MatchRoot("gtp"),
			path.MatchRoot("isl"),
			path.MatchRoot("mpls"),
			path.MatchRoot("mpls_plus_vlan"),
			path.MatchRoot("vntag"),
			path.MatchRoot("geneve"),
		),
	}
}

func (h *HeaderStripping) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon APP Header Stripping Schema",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this header stripping application",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("headerStrip"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session ID on which to deploy this APP",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"protocol": schema.StringAttribute{
				MarkdownDescription: "Header Stripping protocol (computed from the selected block).",
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						"vxlan",
						"vlan",
						"fm6000Ts",
						"erspan",
						"generic",
						"gtp",
						"isl",
						"mpls",
						"mplsPlusVlan",
						"vntag",
						"geneve",
					),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this App instance for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			// parameterized protocols
			"vxlan":     vxlanHSBlock(),
			"vlan":      vlanHSBlock(),
			"fm6000_ts": fm6000TsHSBlock(),
			"erspan":    erspanHSBlock(),
			"generic":   genericHSBlock(),

			// simple protocols (presence = select protocol)
			"gtp":            simpleHSBlock("GTP header stripping (protocol = gtp)"),
			"isl":            simpleHSBlock("ISL header stripping (protocol = isl)"),
			"mpls":           simpleHSBlock("MPLS header stripping (protocol = mpls)"),
			"mpls_plus_vlan": simpleHSBlock("MPLS+VLAN header stripping (protocol = mplsPlusVlan)"),
			"vntag":          simpleHSBlock("VN‑tag header stripping (protocol = vntag)"),
			"geneve":         simpleHSBlock("Geneve header stripping (protocol = geneve)"),
		},
	}
}

func inferHeaderStripProtocol(
	vx *HeaderStrippingVxlanConfig,
	vl *HeaderStrippingVlanConfig,
	ts *HeaderStrippingFm6000TsConfig,
	er *HeaderStrippingErspanConfig,
	ge *HeaderStrippingGenericConfig,
	gtp *HeaderStrippingEmptyConfig,
	isl *HeaderStrippingEmptyConfig,
	mpls *HeaderStrippingEmptyConfig,
	mplsPlusVlan *HeaderStrippingEmptyConfig,
	vntag *HeaderStrippingEmptyConfig,
	geneve *HeaderStrippingEmptyConfig,
) string {
	switch {
	case vx != nil:
		return "vxlan"
	case vl != nil:
		return "vlan"
	case ts != nil:
		return "fm6000Ts"
	case er != nil:
		return "erspan"
	case ge != nil:
		return "generic"
	case gtp != nil:
		return "gtp"
	case isl != nil:
		return "isl"
	case mpls != nil:
		return "mpls"
	case mplsPlusVlan != nil:
		return "mplsPlusVlan"
	case vntag != nil:
		return "vntag"
	case geneve != nil:
		return "geneve"
	default:
		return ""
	}
}

func (h *HeaderStripping) createFMStruct(data *HeaderStrippingModel) *FMHeaderStripping {
	p := inferHeaderStripProtocol(
		data.Vxlan,
		data.Vlan,
		data.Fm6000Ts,
		data.Erspan,
		data.Generic,
		data.Gtp,
		data.Isl,
		data.Mpls,
		data.MplsPlusVlan,
		data.Vntag,
		data.Geneve,
	)

	if p != "" {
		data.Protocol = types.StringValue(p)
	}

	fm := &FMHeaderStripping{
		Alias:    data.Alias.ValueString(),
		Name:     "headerStrip",
		Protocol: p,
		Id:       data.Id.ValueString(),
	}

	switch p {
	case "vxlan":
		var vxid int32
		if data.Vxlan != nil && !data.Vxlan.VxlanId.IsNull() && !data.Vxlan.VxlanId.IsUnknown() {
			vxid = data.Vxlan.VxlanId.ValueInt32()
		}
		fm.Vxlan = &FMHeaderStrippingVxlan{
			VxlanId: vxid,
		}

	case "vlan":
		var vh string
		if data.Vlan != nil && !data.Vlan.VlanHeader.IsNull() && !data.Vlan.VlanHeader.IsUnknown() {
			vh = data.Vlan.VlanHeader.ValueString()
		}
		fm.Vlan = &FMHeaderStrippingVlan{
			VlanHeader: vh, // will be "all" if user omitted it
		}

	case "fm6000Ts":
		ts := "none"
		if data.Fm6000Ts != nil && !data.Fm6000Ts.TimestampFormat.IsNull() && !data.Fm6000Ts.TimestampFormat.IsUnknown() && data.Fm6000Ts.TimestampFormat.ValueString() != "" {
			ts = data.Fm6000Ts.TimestampFormat.ValueString()
		}
		fm.Fm6000Ts = &FMHeaderStrippingFm6000Ts{
			TimestampFormat: ts,
		}

	case "erspan":
		var flowId int32
		if data.Erspan != nil && !data.Erspan.FlowId.IsNull() && !data.Erspan.FlowId.IsUnknown() {
			flowId = data.Erspan.FlowId.ValueInt32()
		}
		fm.Erspan = &FMHeaderStrippingErspan{
			ErspanFlowId: flowId,
		}

	case "generic":
		if data.Generic != nil {
			// OffsetRangeValue: pointer so we can omit the field entirely if user didn't set it.
			var offRangePtr *int32
			if !data.Generic.OffsetRangeValue.IsNull() && !data.Generic.OffsetRangeValue.IsUnknown() {
				v := data.Generic.OffsetRangeValue.ValueInt32()
				offRangePtr = &v
			}

			// HeaderCount, CustomLen: plain int32; 0 means "unset" and is omitted by ,omitempty.
			var hdrCount int32
			if !data.Generic.HeaderCount.IsNull() && !data.Generic.HeaderCount.IsUnknown() {
				hdrCount = data.Generic.HeaderCount.ValueInt32()
			}

			var cLen int32
			if !data.Generic.CustomLen.IsNull() && !data.Generic.CustomLen.IsUnknown() {
				cLen = data.Generic.CustomLen.ValueInt32()
			}

			fm.Generic = &FMHeaderStrippingGeneric{
				Ah1:              data.Generic.Ah1.ValueString(),
				Offset:           data.Generic.Offset.ValueString(),
				OffsetRangeValue: offRangePtr,
				HeaderCount:      hdrCount,
				CustomLen:        cLen,
				Ah2:              data.Generic.Ah2.ValueString(),
			}
		}

	case "gtp", "isl", "mpls", "mplsPlusVlan", "vntag", "geneve":
		// No extra FM blocks; protocol alone is enough.
	}

	return fm
}

// Overlay FM-owned fields into TF state
func (h *HeaderStripping) updateTFStruct(data *HeaderStrippingModel, fmData *FMHeaderStripping) {
	if fmData.Protocol != "" {
		data.Protocol = types.StringValue(fmData.Protocol)
	}

	// Clear all blocks first
	data.Vxlan = nil
	data.Vlan = nil
	data.Fm6000Ts = nil
	data.Erspan = nil
	data.Generic = nil
	data.Gtp = nil
	data.Isl = nil
	data.Mpls = nil
	data.MplsPlusVlan = nil
	data.Vntag = nil
	data.Geneve = nil

	if fmData.Vxlan != nil {
		data.Vxlan = &HeaderStrippingVxlanConfig{
			VxlanId: types.Int32Value(fmData.Vxlan.VxlanId),
		}
	}

	if fmData.Vlan != nil {
		data.Vlan = &HeaderStrippingVlanConfig{
			VlanHeader: types.StringValue(fmData.Vlan.VlanHeader),
		}
	}

	if fmData.Fm6000Ts != nil {
		data.Fm6000Ts = &HeaderStrippingFm6000TsConfig{
			TimestampFormat: types.StringValue(fmData.Fm6000Ts.TimestampFormat),
		}
	}

	if fmData.Erspan != nil {
		data.Erspan = &HeaderStrippingErspanConfig{
			FlowId: types.Int32Value(fmData.Erspan.ErspanFlowId),
		}
	}

	if fmData.Generic != nil {
		// offsetRangeValue: pointer in FM, null when absent
		var offRange types.Int32
		if fmData.Generic.OffsetRangeValue != nil {
			offRange = types.Int32Value(*fmData.Generic.OffsetRangeValue)
		} else {
			offRange = types.Int32Null()
		}

		// headerCount/customLen: 0 from FM means "unset" for us,
		// because TF validators require >= 1, so we map 0 -> null
		var hdrCount types.Int32
		if fmData.Generic.HeaderCount != 0 {
			hdrCount = types.Int32Value(fmData.Generic.HeaderCount)
		} else {
			hdrCount = types.Int32Null()
		}

		var cLen types.Int32
		if fmData.Generic.CustomLen != 0 {
			cLen = types.Int32Value(fmData.Generic.CustomLen)
		} else {
			cLen = types.Int32Null()
		}

		data.Generic = &HeaderStrippingGenericConfig{
			Ah1:              types.StringValue(fmData.Generic.Ah1),
			Offset:           types.StringValue(fmData.Generic.Offset),
			OffsetRangeValue: offRange,
			HeaderCount:      hdrCount,
			CustomLen:        cLen,
			Ah2:              types.StringValue(fmData.Generic.Ah2),
		}
	}

	// Simple protocols: re‑set empty blocks
	switch fmData.Protocol {
	case "gtp":
		data.Gtp = &HeaderStrippingEmptyConfig{}
	case "isl":
		data.Isl = &HeaderStrippingEmptyConfig{}
	case "mpls":
		data.Mpls = &HeaderStrippingEmptyConfig{}
	case "mplsPlusVlan":
		data.MplsPlusVlan = &HeaderStrippingEmptyConfig{}
	case "vntag":
		data.Vntag = &HeaderStrippingEmptyConfig{}
	case "geneve":
		data.Geneve = &HeaderStrippingEmptyConfig{}
	}
}

// validateParams enforces cross-field semantics for the generic block.
func (h *HeaderStripping) validateParams(data *HeaderStrippingModel) error {
	if data.Generic != nil {
		// ah1 required when generic is used
		if data.Generic.Ah1.IsNull() || data.Generic.Ah1.IsUnknown() || data.Generic.Ah1.ValueString() == "" {
			return fmt.Errorf("in generic block, ah1 must be specified")
		}

		// ah2 required when generic is used
		if data.Generic.Ah2.IsNull() || data.Generic.Ah2.IsUnknown() || data.Generic.Ah2.ValueString() == "" {
			return fmt.Errorf("in generic block, ah2 must be specified")
		}

		// offset required when generic is used
		if data.Generic.Offset.IsNull() || data.Generic.Offset.IsUnknown() {
			return fmt.Errorf("in generic block, offset must be specified")
		}
		off := data.Generic.Offset.ValueString()

		if off == "offsetRange" {
			if data.Generic.OffsetRangeValue.IsNull() || data.Generic.OffsetRangeValue.IsUnknown() {
				return fmt.Errorf("in generic block with offset='offsetRange', offset_range_value is required")
			}
		} else {
			if !data.Generic.OffsetRangeValue.IsNull() && !data.Generic.OffsetRangeValue.IsUnknown() {
				return fmt.Errorf("offset_range_value is only valid when offset='offsetRange'")
			}
		}
	}

	return nil
}

func (h *HeaderStripping) Configure(
	ctx context.Context,
	req resource.ConfigureRequest,
	resp *resource.ConfigureResponse,
) {
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

	h.fmClient = fmClient
}

func (h *HeaderStripping) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data HeaderStrippingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := h.validateParams(&data); err != nil {
		resp.Diagnostics.AddError(
			"Invalid parameters specified",
			fmt.Sprintf("Invalid parameters for header stripping app: %s", err),
		)
		return
	}

	fmData := h.createFMStruct(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "create",
				Application: fmData,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		h.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create header stripping app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleApp,
		commonutils.TypeHeaderStripping,
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (h *HeaderStripping) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data HeaderStrippingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hs := FMHeaderStripping{}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	err = GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawID,
		"headerStrip", // app type
		"",
		&hs,
		h.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to Get Header Stripping App details",
			fmt.Sprintf("unable to get Header Stripping App details: %v", err),
		)
		return
	}

	h.updateTFStruct(&data, &hs)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (h *HeaderStripping) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData HeaderStrippingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := h.validateParams(&planData); err != nil {
		resp.Diagnostics.AddError(
			"Unable to update header stripping app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	fmData := h.createFMStruct(&planData)

	rawID, err := commonutils.UUIDFromTypedID(planData.Id.ValueString())
	if err != nil {
		return
	}
	fmData.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		h.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update header stripping app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	h.updateTFStruct(&planData, fmData)

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (h *HeaderStripping) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data HeaderStrippingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMHeaderStripping{
					Id:   rawID,
					Name: "Application", // matches Slicing/Dedup delete convention
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		h.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete header stripping app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}

func (lb *LoadBalancing) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_load_balancing"
}

func (lb *LoadBalancing) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		// Stateless xor Enhanced
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("stateless"),
			path.MatchRoot("enhanced"),
		),
	}
}

func (lb *LoadBalancing) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon APP Load Balancing Schema",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Name for this load balancing application",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Monitoring session ID on which to deploy this APP",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Optional description for this load balancing app",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of this App instance for later use",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			// Stateless LB
			"stateless": schema.SingleNestedBlock{
				MarkdownDescription: "Stateless load balancing configuration",
				Attributes: map[string]schema.Attribute{
					"hash_fields": schema.StringAttribute{
						MarkdownDescription: "Hash field selection",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf(
								"ipOnly",
								"ipAndPort",
								"fiveTuple",
								"gtpuTeid",
								"greFlowid",
							),
						},
					},
					"field_location": schema.StringAttribute{
						MarkdownDescription: "Field location (inner/outer) for applicable hash fields",
						Optional:            true,
						Computed:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("inner", "outer"),
						},
					},
				},
			},

			// Enhanced LB
			"enhanced": schema.SingleNestedBlock{
				MarkdownDescription: "Enhanced load balancing configuration (ELB profile)",
				Attributes: map[string]schema.Attribute{
					"profile": schema.StringAttribute{
						MarkdownDescription: "Enhanced LB profile to use",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf(
								"FmAuto-StatefulApplication-profile",
								"FmAuto-EgressScale-profile",
							),
						},
					},
				},
			},

			// Load balancing groups
			"group": schema.ListNestedBlock{
				MarkdownDescription: "List of load balancing groups (application endpoints and weights)",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"aep_id": schema.Int32Attribute{
							MarkdownDescription: "AEP Id of endpoint (2–64)",
							Required:            true,
							Validators: []validator.Int32{
								int32validator.AtLeast(2),
								int32validator.AtMost(64),
							},
						},
						"weight": schema.Int32Attribute{
							MarkdownDescription: "Weight for this endpoint (1–100); all weights must sum to 100",
							Required:            true,
							Validators: []validator.Int32{
								int32validator.AtLeast(1),
								int32validator.AtMost(100),
							},
						},
					},
				},
			},
		},
	}
}

func (lb *LoadBalancing) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	lb.fmClient = fmClient
}

func (lb *LoadBalancing) validateParams(data *LoadBalancingModel) error {
	var hash string

	// Stateless‑specific rules
	if data.Stateless != nil {
		if data.Stateless.HashFields.IsNull() || data.Stateless.HashFields.IsUnknown() {
			return fmt.Errorf("in stateless block, hash_fields must be specified")
		}
		hash = data.Stateless.HashFields.ValueString()

		switch hash {
		case "greFlowid":
			// For greFlowid, field_location and groups are managed by the provider.
			if !data.Stateless.FieldLocation.IsNull() && !data.Stateless.FieldLocation.IsUnknown() {
				return fmt.Errorf("for hash_fields=greFlowid, field_location is managed by the provider and must not be specified")
			}
			if len(data.Group) > 0 {
				return fmt.Errorf("for hash_fields=greFlowid, group blocks are managed by the provider and must not be specified")
			}

		case "gtpuTeid":
			// Disallow field_location for gtpuTeid as well
			if !data.Stateless.FieldLocation.IsNull() && !data.Stateless.FieldLocation.IsUnknown() {
				return fmt.Errorf("for hash_fields=gtpuTeid, field_location is not used and must not be specified")
			}

		default:
			// All other hashes: field_location required (inner/outer)
			if data.Stateless.FieldLocation.IsNull() || data.Stateless.FieldLocation.IsUnknown() {
				return fmt.Errorf("field_location must be specified for hash_fields=%s", hash)
			}
		}
	}

	// Enhanced: profile must be present and non‑empty (schema already enforces enum)
	if data.Enhanced != nil {
		if data.Enhanced.Profile.IsNull() || data.Enhanced.Profile.IsUnknown() || data.Enhanced.Profile.ValueString() == "" {
			return fmt.Errorf("in enhanced block, profile must be specified")
		}
	}

	// Group constraints apply only when hash != greFlowid
	if hash != "greFlowid" && len(data.Group) > 0 {
		if len(data.Group) < 2 {
			return fmt.Errorf("at least two load balancing groups are required when group is specified")
		}
		if len(data.Group) > 63 {
			return fmt.Errorf("maximum 63 load balancing groups are allowed")
		}

		var total int32
		seenAeps := make(map[int32]struct{})

		for i, g := range data.Group {
			// weight must be set
			if g.Weight.IsNull() || g.Weight.IsUnknown() {
				return fmt.Errorf("group[%d].weight must be specified", i)
			}
			total += g.Weight.ValueInt32()

			// aep_id must be set
			if g.AepId.IsNull() || g.AepId.IsUnknown() {
				return fmt.Errorf("group[%d].aep_id must be specified", i)
			}
			aep := g.AepId.ValueInt32()
			if aep < 2 || aep > 64 {
				return fmt.Errorf("group[%d].aep_id must be between 2 and 64; got %d", i, aep)
			}
			if _, exists := seenAeps[aep]; exists {
				return fmt.Errorf("duplicate aep_id %d across group entries is not allowed", aep)
			}
			seenAeps[aep] = struct{}{}
		}

		// Mirror FM / UI behavior: allow <= 100, reject > 100
		if total > 100 {
			return fmt.Errorf("sum of all group weights must be <= 100; got %d", total)
		}
	}

	return nil
}

// validateHashTransition ensures we do not cross the greFlowid boundary
// (anything → greFlowid, or greFlowid → anything, including Enhanced)
// while this LB app is still used as source in any links. Terraform cannot
// safely auto‑delete gigamon_link resources, so we force the user to
// remove/update them first.
func (lb *LoadBalancing) validateHashTransition(
	ctx context.Context,
	planData *LoadBalancingModel,
	stateData *LoadBalancingModel,
) error {
	// oldGre: state is stateless with hash_fields == greFlowid
	oldGre := false
	if stateData != nil && stateData.Stateless != nil &&
		!stateData.Stateless.HashFields.IsNull() &&
		!stateData.Stateless.HashFields.IsUnknown() &&
		stateData.Stateless.HashFields.ValueString() == "greFlowid" {
		oldGre = true
	}

	// newGre: plan is stateless with hash_fields == greFlowid
	newGre := false
	if planData != nil && planData.Stateless != nil &&
		!planData.Stateless.HashFields.IsNull() &&
		!planData.Stateless.HashFields.IsUnknown() &&
		planData.Stateless.HashFields.ValueString() == "greFlowid" {
		newGre = true
	}

	// No greFlowid on either side → nothing special.
	if !oldGre && !newGre {
		return nil
	}
	// Both sides greFlowid → not crossing boundary.
	if oldGre && newGre {
		return nil
	}

	// We are crossing the greFlowid boundary in some direction:
	//   - enhanced / other stateless -> greFlowid stateless
	//   - greFlowid stateless -> enhanced / other stateless
	msID := planData.MonitoringSessionId.ValueString()
	links, err := GetMSLinks(ctx, msID, lb.fmClient)
	if err != nil {
		return fmt.Errorf("failed to read monitoring session links: %w", err)
	}

	// convert typed TF ID to raw FM ID for comparison
	rawLbID, err := commonutils.UUIDFromTypedID(stateData.Id.ValueString())
	if err != nil {
		return err
	}

	// Derive oldHash/newHash strings for a helpful error.
	oldHash := "enhanced"
	if stateData != nil && stateData.Stateless != nil &&
		!stateData.Stateless.HashFields.IsNull() &&
		!stateData.Stateless.HashFields.IsUnknown() {
		oldHash = stateData.Stateless.HashFields.ValueString()
	}

	newHash := "enhanced"
	if planData != nil && planData.Stateless != nil &&
		!planData.Stateless.HashFields.IsNull() &&
		!planData.Stateless.HashFields.IsUnknown() {
		newHash = planData.Stateless.HashFields.ValueString()
	}

	for _, lnk := range links {
		// IMPORTANT CHANGE: compare against rawLbID instead of typed ID
		if lnk.Source.Id == rawLbID {
			return fmt.Errorf(
				"load balancing app %q is still used as source by link (id=%q, source_aep_id=%d). "+
					"Update or delete the corresponding gigamon_link resource "+
					"in your Terraform configuration before changing hash_fields from %q to %q.",
				stateData.Id.ValueString(), lnk.Id, lnk.Source.AepId, oldHash, newHash,
			)
		}
	}

	return nil
}

// validateGroupRemovals ensures we are not removing any LB groups (AEPs)
// that are still referenced by links in this Monitoring Session.
func (lb *LoadBalancing) validateGroupRemovals(
	ctx context.Context,
	planData *LoadBalancingModel,
	stateData *LoadBalancingModel,
) error {
	// Only relevant for stateless LB (groups are ignored for greFlowid)
	if planData.Stateless == nil {
		return nil
	}

	hash := planData.Stateless.HashFields.ValueString()
	if hash == "greFlowid" {
		return nil
	}

	// Build sets of AEP IDs "before" (state) and "after" (plan)
	before := make(map[int32]struct{})
	for _, g := range stateData.Group {
		if !g.AepId.IsNull() && !g.AepId.IsUnknown() {
			before[g.AepId.ValueInt32()] = struct{}{}
		}
	}

	after := make(map[int32]struct{})
	for _, g := range planData.Group {
		if !g.AepId.IsNull() && !g.AepId.IsUnknown() {
			after[g.AepId.ValueInt32()] = struct{}{}
		}
	}

	// AEPs being removed = present before, absent after
	removed := make(map[int32]struct{})
	for aep := range before {
		if _, still := after[aep]; !still {
			removed[aep] = struct{}{}
		}
	}

	if len(removed) == 0 {
		return nil
	}

	// Look at all links in this Monitoring Session
	msID := planData.MonitoringSessionId.ValueString()
	links, err := GetMSLinks(ctx, msID, lb.fmClient)
	if err != nil {
		return fmt.Errorf("failed to read monitoring session links: %w", err)
	}

	// Convert typed TF ID to raw FM ID for comparison
	rawLbID, err := commonutils.UUIDFromTypedID(stateData.Id.ValueString())
	if err != nil {
		return err
	}
	for _, lnk := range links {
		// IMPORTANT CHANGE: compare against rawLbID instead of typed ID
		if lnk.Source.Id != rawLbID {
			continue
		}
		aep := lnk.Source.AepId
		if _, isRemoved := removed[aep]; isRemoved {
			return fmt.Errorf(
				"Group with aep_id=%d is still used as source_aep_id by link %q. "+
					"Delete or update that link before removing this group.",
				aep, lnk.Id,
			)
		}
	}

	return nil
}

func (lb *LoadBalancing) createFMStruct(data *LoadBalancingModel) *FMLoadBalancing {
	fm := &FMLoadBalancing{
		Alias:       data.Alias.ValueString(),
		Name:        "lb", // Application name is fixed
		Description: data.Description.ValueString(),
		Id:          data.Id.ValueString(),
	}

	var hash string

	// Stateless
	if data.Stateless != nil {
		hash = data.Stateless.HashFields.ValueString()

		st := &FMLoadBalancingStateless{
			HashFields: hash,
		}

		switch hash {
		case "greFlowid":
			// UI behaviour: greFlowid always uses "outer"
			st.FieldLocation = "outer"

		case "gtpuTeid":
			// no field_location in payload for gtpuTeid

		default:
			// other hashes: use user‑specified field_location or default "outer"
			if !data.Stateless.FieldLocation.IsNull() && !data.Stateless.FieldLocation.IsUnknown() {
				st.FieldLocation = data.Stateless.FieldLocation.ValueString()
			} else {
				st.FieldLocation = "outer"
			}
		}

		fm.Stateless = st
	}

	// Enhanced
	if data.Enhanced != nil {
		fm.Enhanced = &FMLoadBalancingEnhanced{
			Profile: data.Enhanced.Profile.ValueString(),
		}
	}

	// Groups
	if hash == "greFlowid" {
		// UI always sends at least one LBG, even though the user never sees it.
		fm.Lbg = []FMLoadBalancingGroup{
			{
				AepId:  2,
				Weight: 1,
			},
		}
	} else if len(data.Group) > 0 {
		fm.Lbg = make([]FMLoadBalancingGroup, len(data.Group))
		for i, g := range data.Group {
			// validateParams has already ensured AepId and Weight are set and valid
			aepId := g.AepId.ValueInt32()
			weight := g.Weight.ValueInt32()

			fm.Lbg[i] = FMLoadBalancingGroup{
				AepId:  aepId,
				Weight: weight,
			}
		}
	}

	return fm
}

func (lb *LoadBalancing) updateTFStruct(data *LoadBalancingModel, fmData *FMLoadBalancing) {
	// Top‑level
	if fmData.Description != "" {
		data.Description = types.StringValue(fmData.Description)
	}

	// Clear nested state
	data.Stateless = nil
	data.Enhanced = nil
	data.Group = nil

	var hash string

	// Stateless
	if fmData.Stateless != nil {
		hash = fmData.Stateless.HashFields

		st := &LoadBalancingStatelessConfig{
			HashFields: types.StringValue(hash),
		}

		// For greFlowid we *never* expose field_location to Terraform.
		if hash == "greFlowid" {
			st.FieldLocation = types.StringNull()
		} else {
			if fmData.Stateless.FieldLocation != "" {
				st.FieldLocation = types.StringValue(fmData.Stateless.FieldLocation)
			} else {
				st.FieldLocation = types.StringNull()
			}
		}

		data.Stateless = st
	}

	// Enhanced
	if fmData.Enhanced != nil {
		data.Enhanced = &LoadBalancingEnhancedConfig{
			Profile: types.StringValue(fmData.Enhanced.Profile),
		}
	}

	// Groups: only expose to Terraform when hash != greFlowid
	if hash != "greFlowid" && len(fmData.Lbg) > 0 {
		data.Group = make([]LoadBalancingGroupModel, len(fmData.Lbg))
		for i, g := range fmData.Lbg {
			data.Group[i] = LoadBalancingGroupModel{
				AepId:  types.Int32Value(g.AepId),
				Weight: types.Int32Value(g.Weight),
			}
		}
	}
}

// Create call for new Load Balancing App Instance
func (lb *LoadBalancing) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data LoadBalancingModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := lb.validateParams(&data); err != nil {
		resp.Diagnostics.AddError(
			"Invalid parameters specified",
			fmt.Sprintf("Invalid parameters for load balancing app: %s", err),
		)
		return
	}

	fmData := lb.createFMStruct(&data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "create",
				Application: fmData,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		lb.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create load balancing app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	// Populate computed fields (including group[*].aep_id) from FM payload
	lb.updateTFStruct(&data, fmData)

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleApp,
		commonutils.TypeLoadBalancing,
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (lb *LoadBalancing) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data LoadBalancingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lbData := FMLoadBalancing{}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}
	err = GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawID,
		"lb",
		"",
		&lbData,
		lb.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to Get Load Balancing App details",
			fmt.Sprintf("unable to get Load Balancing App details. error is %v", err),
		)
		return
	}

	lb.updateTFStruct(&data, &lbData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (lb *LoadBalancing) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData LoadBalancingModel
	var stateData LoadBalancingModel

	// Read desired config and prior state
	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &stateData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Normal validation
	if err := lb.validateParams(&planData); err != nil {
		resp.Diagnostics.AddError(
			"Unable to update load balancing app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	// block changing anything <-> greFlowid while links still exist
	if err := lb.validateHashTransition(ctx, &planData, &stateData); err != nil {
		resp.Diagnostics.AddError(
			"Cannot change hash_fields to/from greFlowid while load balancing app still has links",
			err.Error(),
		)
		return
	}

	// Extra semantic check: don't remove groups that are still linked
	if err := lb.validateGroupRemovals(ctx, &planData, &stateData); err != nil {
		resp.Diagnostics.AddError(
			"Cannot remove load balancing group that is still linked",
			err.Error(),
		)
		return
	}

	// FM update
	fmData := lb.createFMStruct(&planData)

	rawID, err := commonutils.UUIDFromTypedID(planData.Id.ValueString())
	if err != nil {
		return
	}
	fmData.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		lb.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update load balancing app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	lb.updateTFStruct(&planData, fmData)
	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (lb *LoadBalancing) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data LoadBalancingModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMLoadBalancing{
					Id:   rawID,
					Name: "Application",
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		lb.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete load balancing app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}

// AMX Application TF Hooks

func (a *Amx) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_amx"
}

func (a *Amx) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	a.fmClient = fmClient
}

func (a *Amx) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gigamon AMX (Application Metadata Exporter) application schema.",

		Attributes: map[string]schema.Attribute{
			"alias": schema.StringAttribute{
				MarkdownDescription: "Alias for this AMX application.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"monitoring_session_id": schema.StringAttribute{
				MarkdownDescription: "Cloud monitoring session ID on which to deploy this AMX app.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Typed ID of this AMX application instance.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},

		Blocks: map[string]schema.Block{
			"ingestor": schema.ListNestedBlock{
				MarkdownDescription: "AMX ingestors (where AMX receives metadata). Supported fields are name, port, and type. At least one is recommended.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Optional name for this ingestor.",
							Optional:            true,
						},
						"port": schema.Int32Attribute{
							MarkdownDescription: "Listening port (1–65535).",
							Required:            true,
							Validators: []validator.Int32{
								int32validator.AtLeast(1),
								int32validator.AtMost(65535),
							},
						},
						"type": schema.StringAttribute{
							MarkdownDescription: "Ingestor type.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.OneOf(
									"ami",
									"mobility",
									"netflow",
								),
							},
						},
					},
				},
			},

			"exporter": schema.SingleNestedBlock{
				MarkdownDescription: "Exporters sending AMX JSON records to tools (HTTP and Kafka). At least one exporter is required.",
				Attributes: map[string]schema.Attribute{
					"debug": schema.BoolAttribute{
						MarkdownDescription: "Enable AMX exporter debug logging (advanced).",
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
					},
				},
				Blocks: map[string]schema.Block{
					"http_export": schema.ListNestedBlock{
						MarkdownDescription: "HTTP/HTTPS exports (cloud tool exports).",
						NestedObject: schema.NestedBlockObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									MarkdownDescription: "Unique alias for this HTTP export.",
									Required:            true,
								},
								"enabled": schema.BoolAttribute{
									MarkdownDescription: "Whether this HTTP export is enabled.",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(true),
								},
								"data_type": schema.StringAttribute{
									MarkdownDescription: "Type of data exported: AMI, Mobility Control, AMI Enriched, or NetFlow/IPFIX.",
									Optional:            true,
									Computed:            true,
									Default:             stringdefault.StaticString("ami"),
									Validators: []validator.String{
										stringvalidator.OneOf(
											"ami",
											"mobility",
											"ami_enriched",
											"netflow",
										),
									},
								},
								"endpoint": schema.StringAttribute{
									MarkdownDescription: "Target HTTP/HTTPS endpoint URL.",
									Required:            true,
								},
								"secure_keys": schema.ListAttribute{
									ElementType:         types.StringType,
									MarkdownDescription: "Names of headers/fields that should be treated as secure keys.",
									Optional:            true,
								},
								"headers": schema.ListAttribute{
									ElementType:         types.StringType,
									MarkdownDescription: "HTTP headers to send (e.g. Authorization: Bearer ...).",
									Optional:            true,
								},
								"bind_ip_address": schema.StringAttribute{
									MarkdownDescription: "Local source IP address to bind for outgoing connections (advanced).",
									Optional:            true,
								},
								"format": schema.StringAttribute{
									MarkdownDescription: "Payload format.",
									Computed:            true,
									Default:             stringdefault.StaticString("json"),
									Validators: []validator.String{
										stringvalidator.OneOf("json"),
									},
								},
								"compress": schema.BoolAttribute{
									MarkdownDescription: "Compress uploads with gzip.",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(true),
								},
								"flush_interval_seconds": schema.Int32Attribute{
									MarkdownDescription: "Upload interval in seconds.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(30),
									Validators: []validator.Int32{
										int32validator.AtLeast(10),
										int32validator.AtMost(1800),
									},
								},
								"parallel_workers": schema.Int32Attribute{
									MarkdownDescription: "Number of parallel upload workers.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(4),
								},
								"max_retries": schema.Int32Attribute{
									MarkdownDescription: "Number of retries before giving up.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(4),
									Validators: []validator.Int32{
										int32validator.AtLeast(4),
									},
								},
								"max_records_per_batch": schema.Int32Attribute{
									MarkdownDescription: "Maximum records per HTTP batch.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(1000),
								},
								"self_heal_window_seconds": schema.Int32Attribute{
									MarkdownDescription: "Self-heal timer window in seconds.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(0),
								},
								"upload_timeout_seconds": schema.Int32Attribute{
									MarkdownDescription: "HTTP client upload timeout in seconds.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(10),
								},
								"labels": schema.MapAttribute{
									ElementType:         types.StringType,
									MarkdownDescription: "Static labels attached to all records from this exporter.",
									Optional:            true,
								},
							},
						},
					},

					"kafka_export": schema.ListNestedBlock{
						MarkdownDescription: "Kafka exports streaming AMX JSON to Kafka topics.",
						NestedObject: schema.NestedBlockObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									MarkdownDescription: "Unique alias for this Kafka export.",
									Required:            true,
								},
								"topic": schema.StringAttribute{
									MarkdownDescription: "Kafka topic name.",
									Required:            true,
								},
								"enabled": schema.BoolAttribute{
									MarkdownDescription: "Whether this Kafka export is enabled.",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(true),
								},
								"brokers": schema.ListAttribute{
									ElementType:         types.StringType,
									MarkdownDescription: "List of Kafka brokers (host:port or IP:port). At least one required.",
									Required:            true,
								},
								"bind_ip_address": schema.StringAttribute{
									MarkdownDescription: "Local source IP address to bind for outgoing connections (advanced).",
									Optional:            true,
								},
								"data_type": schema.StringAttribute{
									MarkdownDescription: "Type of data exported: AMI, Mobility Control, AMI Enriched, or NetFlow/IPFIX.",
									Optional:            true,
									Computed:            true,
									Default:             stringdefault.StaticString("ami"),
									Validators: []validator.String{
										stringvalidator.OneOf(
											"ami",
											"mobility",
											"ami_enriched",
											"netflow",
										),
									},
								},
								"format": schema.StringAttribute{
									MarkdownDescription: "Payload format.",
									Computed:            true,
									Default:             stringdefault.StaticString("json"),
									Validators: []validator.String{
										stringvalidator.OneOf("json"),
									},
								},
								"compress": schema.BoolAttribute{
									MarkdownDescription: "Compress records before sending to Kafka.",
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(false),
								},
								"flush_interval_seconds": schema.Int32Attribute{
									MarkdownDescription: "Flush interval in seconds.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(30),
								},
								"parallel_workers": schema.Int32Attribute{
									MarkdownDescription: "Number of producer workers.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(4),
								},
								"max_retries": schema.Int32Attribute{
									MarkdownDescription: "Number of retries per batch.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(4),
									Validators: []validator.Int32{
										int32validator.AtLeast(4),
									},
								},
								"max_records_per_batch": schema.Int32Attribute{
									MarkdownDescription: "Maximum records per Kafka batch.",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(1000),
								},
								"self_heal_window_seconds": schema.Int32Attribute{
									MarkdownDescription: "Self-heal timer window (seconds).",
									Optional:            true,
									Computed:            true,
									Default:             int32default.StaticInt32(0),
								},
								"labels": schema.MapAttribute{
									ElementType:         types.StringType,
									MarkdownDescription: "Static labels added to all records from this exporter.",
									Optional:            true,
								},
								"producer_configs": schema.ListAttribute{
									ElementType:         types.StringType,
									MarkdownDescription: "Additional Kafka producer configurations (key=value strings).",
									Optional:            true,
								},
							},
						},
					},
				},
			},

			// Mobility enrichment
			"mobility_enrichment": schema.ListNestedBlock{
				MarkdownDescription: "Optional mobility enrichment (at most one).",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of this mobility enrichment.",
							Required:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether this mobility enrichment is enabled.",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(true),
						},
						"attributes": schema.ListAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Mobility attribute names to export.",
							Optional:            true,
						},
					},
				},
			},

			// Workload enrichment
			"workload_enrichment": schema.ListNestedBlock{
				MarkdownDescription: "Optional workload enrichment for AWS, Azure, VMware vCenter, AKS.",
				NestedObject: schema.NestedBlockObject{
					Blocks: map[string]schema.Block{
						"aws":            workloadPlatformBlock("AWS workload enrichment"),
						"azure":          workloadPlatformBlock("Azure workload enrichment"),
						"vmware_vcenter": workloadPlatformBlock("VMware vCenter workload enrichment"),
						"aks":            workloadPlatformBlock("Azure Kubernetes Service workload enrichment"),
					},
				},
			},

			// Other enrichment (generic)
			"other_enrichment": schema.ListNestedBlock{
				MarkdownDescription: "Additional generic enrichments of type 'other'.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of this generic enrichment.",
							Required:            true,
						},
						"enabled": schema.BoolAttribute{
							MarkdownDescription: "Whether this enrichment is enabled.",
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(true),
						},
						"attributes": schema.ListAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Attribute names to export.",
							Optional:            true,
						},
						"settings": schema.ListAttribute{
							ElementType:         types.StringType,
							MarkdownDescription: "Advanced settings for this 'other' enrichment. Each string is sent as-is to AMX (matches FM UI Settings list). Use only under Gigamon guidance.",
							Optional:            true,
						},
					},
				},
			},
		},
	}
}

// Common block builder for workload platforms
func workloadPlatformBlock(desc string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		MarkdownDescription: desc,
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					MarkdownDescription: "Name of this workload enrichment profile.",
					Required:            true,
				},
				"enabled": schema.BoolAttribute{
					MarkdownDescription: "Whether this workload enrichment is enabled.",
					Optional:            true,
					Computed:            true,
					Default:             booldefault.StaticBool(true),
				},
				"attributes": schema.ListAttribute{
					ElementType:         types.StringType,
					MarkdownDescription: "Workload attribute names to export.",
					Optional:            true,
				},
				"settings": schema.MapAttribute{
					ElementType:         types.StringType,
					MarkdownDescription: "Additional workload settings as key/value pairs. Keys and values are passed through to AMX as \"key=value\" strings; use only under Gigamon guidance.",
					Optional:            true,
				},
			},
			Blocks: map[string]schema.Block{
				"source": schema.ListNestedBlock{
					MarkdownDescription: "One or more workload sources (e.g., accounts, clusters).",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"name": schema.StringAttribute{
								MarkdownDescription: "Name/label of this source (e.g., account or cluster name).",
								Required:            true,
							},
						},
						Blocks: map[string]schema.Block{
							"setting": schema.ListNestedBlock{
								MarkdownDescription: "Key/value settings for this source (e.g., credentials, kubeconfig).",
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										"secure": schema.BoolAttribute{
											MarkdownDescription: "Whether this value is a secret (AMX will encrypt it).",
											Optional:            true,
											Computed:            true,
											Default:             booldefault.StaticBool(true),
										},
										"file": schema.StringAttribute{
											MarkdownDescription: "Optional path to a file whose contents will be used as the value.",
											Optional:            true,
										},
										"key": schema.StringAttribute{
											MarkdownDescription: "Property key (e.g. k8s_kubeconfig, azure_client_id, aws_access_key_id). For AKS workload, a setting with key = \"k8s_kubeconfig\" is required and must carry the kubeconfig file.",
											Required:            true,
										},
										"value": schema.StringAttribute{
											MarkdownDescription: "Property value (plain text; used when file is not specified). For AKS workload, provide kubeconfig content in file and/or value for key = \"k8s_kubeconfig\".",
											Optional:            true,
											Sensitive:           true,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (a *Amx) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		// At least one exporter (http or kafka)
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("exporter").AtName("http_export"),
			path.MatchRoot("exporter").AtName("kafka_export"),
		),
	}
}

// Build FM payload from TF model
func (a *Amx) createFMStruct(ctx context.Context, data *AmxModel) *FMAmx {
	fm := &FMAmx{
		Alias: data.Alias.ValueString(),
		Name:  "ogw",
		Id:    data.Id.ValueString(),
	}

	// Ingestor
	if len(data.Ingestor) > 0 {
		fm.Ingestor = make([]FMAmxIngestor, 0, len(data.Ingestor))
		for _, in := range data.Ingestor {
			// Map Terraform-friendly type to FM type
			t := in.Type.ValueString()
			fmType := t
			if t == "mobility" {
				fmType = "gtpc"
			}

			fm.Ingestor = append(fm.Ingestor, FMAmxIngestor{
				Name: in.Name.ValueString(),
				Port: in.Port.ValueInt32(),
				Type: fmType,
			})
		}
	}

	// Exporter
	if data.Exporter != nil {
		exp := &FMAmxExporter{
			CloudUpload: []FMAmxCloudUpload{},
			Kafka:       []FMAmxKafka{},
		}

		if !data.Exporter.Debug.IsNull() && !data.Exporter.Debug.IsUnknown() {
			v := data.Exporter.Debug.ValueBool()
			exp.Debug = &v
		}

		// HTTP exports
		if len(data.Exporter.HttpExport) > 0 {
			exp.CloudUpload = make([]FMAmxCloudUpload, 0, len(data.Exporter.HttpExport))
			for _, he := range data.Exporter.HttpExport {
				var (
					enPtr, zipPtr                                *bool
					intervalPtr, workersPtr, retriesPtr          *int32
					maxEntriesPtr, selfHealPtr, uploadTimeoutPtr *int32
				)

				if !he.Enabled.IsNull() && !he.Enabled.IsUnknown() {
					v := he.Enabled.ValueBool()
					enPtr = &v
				}
				if !he.Compress.IsNull() && !he.Compress.IsUnknown() {
					v := he.Compress.ValueBool()
					zipPtr = &v
				}
				if !he.FlushIntervalSeconds.IsNull() && !he.FlushIntervalSeconds.IsUnknown() {
					v := he.FlushIntervalSeconds.ValueInt32()
					intervalPtr = &v
				}
				if !he.ParallelWorkers.IsNull() && !he.ParallelWorkers.IsUnknown() {
					v := he.ParallelWorkers.ValueInt32()
					workersPtr = &v
				}
				if !he.MaxRetries.IsNull() && !he.MaxRetries.IsUnknown() {
					v := he.MaxRetries.ValueInt32()
					retriesPtr = &v
				}
				if !he.MaxRecordsPerBatch.IsNull() && !he.MaxRecordsPerBatch.IsUnknown() {
					v := he.MaxRecordsPerBatch.ValueInt32()
					maxEntriesPtr = &v
				}
				if !he.SelfHealWindowSeconds.IsNull() && !he.SelfHealWindowSeconds.IsUnknown() {
					v := he.SelfHealWindowSeconds.ValueInt32()
					selfHealPtr = &v
				}
				if !he.UploadTimeoutSeconds.IsNull() && !he.UploadTimeoutSeconds.IsUnknown() {
					v := he.UploadTimeoutSeconds.ValueInt32()
					uploadTimeoutPtr = &v
				}

				headers := []string{}
				if !he.Headers.IsNull() && !he.Headers.IsUnknown() {
					var hs []types.String
					_ = he.Headers.ElementsAs(ctx, &hs, false)
					for _, h := range hs {
						headers = append(headers, h.ValueString())
					}
				}

				var secure []string
				if !he.SecureKeys.IsNull() && !he.SecureKeys.IsUnknown() {
					var sk []types.String
					_ = he.SecureKeys.ElementsAs(ctx, &sk, false)
					for _, s := range sk {
						secure = append(secure, s.ValueString())
					}
				}

				var labels map[string]string
				if !he.Labels.IsNull() && !he.Labels.IsUnknown() {
					labels = map[string]string{}
					var lm map[string]types.String
					_ = he.Labels.ElementsAs(ctx, &lm, false)
					for k, v := range lm {
						labels[k] = v.ValueString()
					}
				}

				tfType := he.DataType.ValueString()
				if tfType == "" {
					tfType = "ami"
				}
				fmType := mapAmxDataTypeTFToFM(tfType)

				exp.CloudUpload = append(exp.CloudUpload, FMAmxCloudUpload{
					Name:                    he.Name.ValueString(),
					Enable:                  enPtr,
					Endpoint:                he.Endpoint.ValueString(),
					SecureKeys:              secure,
					Headers:                 headers,
					IfaceIPAddress:          he.BindIPAddress.ValueString(),
					Format:                  "json",
					Zip:                     zipPtr,
					Interval:                intervalPtr,
					Writers:                 workersPtr,
					Retries:                 retriesPtr,
					MaxEntries:              maxEntriesPtr,
					SelfHealTimerWindow:     selfHealPtr,
					HttpClientUploadTimeout: uploadTimeoutPtr,
					Labels:                  labels,
					Type:                    fmType,
				})
			}
		}

		// Kafka exports
		if len(data.Exporter.KafkaExport) > 0 {
			exp.Kafka = make([]FMAmxKafka, 0, len(data.Exporter.KafkaExport))
			for _, ke := range data.Exporter.KafkaExport {
				var (
					enPtr, zipPtr                       *bool
					intervalPtr, workersPtr, retriesPtr *int32
					maxEntriesPtr, selfHealPtr          *int32
				)

				if !ke.Enabled.IsNull() && !ke.Enabled.IsUnknown() {
					v := ke.Enabled.ValueBool()
					enPtr = &v
				}
				if !ke.Compress.IsNull() && !ke.Compress.IsUnknown() {
					v := ke.Compress.ValueBool()
					zipPtr = &v
				}
				if !ke.FlushIntervalSeconds.IsNull() && !ke.FlushIntervalSeconds.IsUnknown() {
					v := ke.FlushIntervalSeconds.ValueInt32()
					intervalPtr = &v
				}
				if !ke.ParallelWorkers.IsNull() && !ke.ParallelWorkers.IsUnknown() {
					v := ke.ParallelWorkers.ValueInt32()
					workersPtr = &v
				}
				if !ke.MaxRetries.IsNull() && !ke.MaxRetries.IsUnknown() {
					v := ke.MaxRetries.ValueInt32()
					retriesPtr = &v
				}
				if !ke.MaxRecordsPerBatch.IsNull() && !ke.MaxRecordsPerBatch.IsUnknown() {
					v := ke.MaxRecordsPerBatch.ValueInt32()
					maxEntriesPtr = &v
				}
				if !ke.SelfHealWindowSeconds.IsNull() && !ke.SelfHealWindowSeconds.IsUnknown() {
					v := ke.SelfHealWindowSeconds.ValueInt32()
					selfHealPtr = &v
				}

				var brokers []string
				if !ke.Brokers.IsNull() && !ke.Brokers.IsUnknown() {
					var bs []types.String
					_ = ke.Brokers.ElementsAs(ctx, &bs, false)
					for _, b := range bs {
						brokers = append(brokers, b.ValueString())
					}
				}

				var labels map[string]string
				if !ke.Labels.IsNull() && !ke.Labels.IsUnknown() {
					labels = map[string]string{}
					var lm map[string]types.String
					_ = ke.Labels.ElementsAs(ctx, &lm, false)
					for k, v := range lm {
						labels[k] = v.ValueString()
					}
				}

				var prodCfg []string
				if !ke.ProducerConfigs.IsNull() && !ke.ProducerConfigs.IsUnknown() {
					var pc []types.String
					_ = ke.ProducerConfigs.ElementsAs(ctx, &pc, false)
					for _, p := range pc {
						prodCfg = append(prodCfg, p.ValueString())
					}
				}

				tfType := ke.DataType.ValueString()
				if tfType == "" {
					tfType = "ami"
				}
				fmType := mapAmxDataTypeTFToFM(tfType)

				exp.Kafka = append(exp.Kafka, FMAmxKafka{
					Name:                ke.Name.ValueString(),
					Topic:               ke.Topic.ValueString(),
					Enable:              enPtr,
					Brokers:             brokers,
					IfaceIPAddress:      ke.BindIPAddress.ValueString(),
					Format:              "json",
					Zip:                 zipPtr,
					Interval:            intervalPtr,
					Writers:             workersPtr,
					Retries:             retriesPtr,
					MaxEntries:          maxEntriesPtr,
					SelfHealTimerWindow: selfHealPtr,
					Labels:              labels,
					ProducerConfigs:     prodCfg,
					Type:                fmType,
				})
			}
		}

		fm.Exporter = exp
	}

	// Enrichment: Mobility
	for _, me := range data.MobilityEnrichment {
		attr := listStringOrEmpty(ctx, me.Attributes)
		var enPtr *bool
		if !me.Enabled.IsNull() && !me.Enabled.IsUnknown() {
			v := me.Enabled.ValueBool()
			enPtr = &v
		}
		fm.AttrEnrichment = append(fm.AttrEnrichment, FMAmxAttrEnrichment{
			Name:       me.Name.ValueString(),
			Type:       "mobility",
			Attributes: attr,
			Enable:     enPtr,
		})
	}

	// Enrichment: Workload platforms
	for _, we := range data.WorkloadEnrichment {
		// AWS
		for i := range we.Aws {
			fm.AttrEnrichment = append(
				fm.AttrEnrichment,
				buildFMWorkload("workload_aws", &we.Aws[i], ctx),
			)
		}
		// Azure
		for i := range we.Azure {
			fm.AttrEnrichment = append(
				fm.AttrEnrichment,
				buildFMWorkload("workload_azure", &we.Azure[i], ctx),
			)
		}
		// VMware vCenter
		for i := range we.VmwareVcenter {
			fm.AttrEnrichment = append(
				fm.AttrEnrichment,
				buildFMWorkload("workload_vmware_esxi", &we.VmwareVcenter[i], ctx),
			)
		}
		// AKS
		for i := range we.Aks {
			fm.AttrEnrichment = append(
				fm.AttrEnrichment,
				buildFMWorkload("workload_k8s", &we.Aks[i], ctx),
			)
		}
	}

	// Enrichment: Other
	for _, oe := range data.OtherEnrichment {
		attr := listStringOrEmpty(ctx, oe.Attributes)
		sett := listStringOrEmpty(ctx, oe.Settings)
		var enPtr *bool
		if !oe.Enabled.IsNull() && !oe.Enabled.IsUnknown() {
			v := oe.Enabled.ValueBool()
			enPtr = &v
		}
		fm.AttrEnrichment = append(fm.AttrEnrichment, FMAmxAttrEnrichment{
			Name:       oe.Name.ValueString(),
			Type:       "other",
			Attributes: attr,
			Settings:   sett,
			Enable:     enPtr,
		})
	}

	return fm
}

func listStringOrEmpty(ctx context.Context, l types.List) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var ss []types.String
	_ = l.ElementsAs(ctx, &ss, false)
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.ValueString())
	}
	return out
}

func mapStringToKVList(ctx context.Context, m types.Map) []string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}

	var mm map[string]types.String
	_ = m.ElementsAs(ctx, &mm, false)

	out := make([]string, 0, len(mm))
	for k, v := range mm {
		out = append(out, fmt.Sprintf("%s=%s", k, v.ValueString()))
	}
	return out
}

func buildFMWorkload(fmType string, p *AmxWorkloadPlatformModel, ctx context.Context) FMAmxAttrEnrichment {
	attr := listStringOrEmpty(ctx, p.Attributes)
	sett := mapStringToKVList(ctx, p.Settings)
	var enPtr *bool
	if !p.Enabled.IsNull() && !p.Enabled.IsUnknown() {
		v := p.Enabled.ValueBool()
		enPtr = &v
	}

	var srcInfos []FMAmxSourceInformation
	for _, src := range p.Sources {
		var settings []FMAmxSourceSetting
		for _, s := range src.SourceSettings {
			key := s.Key.ValueString()
			var val string

			if !s.File.IsNull() && !s.File.IsUnknown() && s.File.ValueString() != "" {
				// For now, assume user passes the contents directly in value or file path is not read;
				// provider can be extended later to read file contents.
				val = s.Value.ValueString()
			} else {
				val = s.Value.ValueString()
			}

			secure := true
			if !s.Secure.IsNull() && !s.Secure.IsUnknown() {
				secure = s.Secure.ValueBool()
			}

			settings = append(settings, FMAmxSourceSetting{
				SecureKey:     secure,
				FileName:      s.File.ValueString(),
				PropertyKey:   key,
				PropertyValue: val,
			})
		}
		srcInfos = append(srcInfos, FMAmxSourceInformation{
			Name:           src.Name.ValueString(),
			SourceSettings: settings,
		})
	}

	return FMAmxAttrEnrichment{
		Name:              p.Name.ValueString(),
		Type:              fmType,
		Attributes:        attr,
		Settings:          sett,
		SourceInformation: srcInfos,
		Enable:            enPtr,
	}
}

// Overlay FM-owned fields into TF state (basic: alias/ingestor/exporter)
// NOTE: enrichment mapping back is optional and can be added later.
func (a *Amx) updateTFStruct(ctx context.Context, data *AmxModel, fmData *FMAmx) {
	data.Alias = types.StringValue(fmData.Alias)

	// Ingestor
	data.Ingestor = nil
	if len(fmData.Ingestor) > 0 {
		data.Ingestor = make([]AmxIngestorModel, len(fmData.Ingestor))
		for i, in := range fmData.Ingestor {
			// Map FM type back to Terraform-friendly value
			tfType := in.Type
			if in.Type == "gtpc" || in.Type == "gtpc_hier" {
				tfType = "mobility"
			}

			data.Ingestor[i] = AmxIngestorModel{
				Name: types.StringValue(in.Name),
				Port: types.Int32Value(in.Port),
				Type: types.StringValue(tfType),
			}
		}
	}

	// Exporter
	data.Exporter = nil
	if fmData.Exporter != nil {
		exp := &AmxExporterModel{}

		if fmData.Exporter.Debug != nil {
			exp.Debug = types.BoolValue(*fmData.Exporter.Debug)
		} else {
			exp.Debug = types.BoolNull()
		}

		// HTTP
		if len(fmData.Exporter.CloudUpload) > 0 {
			exp.HttpExport = make([]AmxHttpExportModel, len(fmData.Exporter.CloudUpload))
			for i, he := range fmData.Exporter.CloudUpload {
				headers, _ := types.ListValueFrom(ctx, types.StringType, he.Headers)
				secure, _ := types.ListValueFrom(ctx, types.StringType, he.SecureKeys)
				labels, _ := types.MapValueFrom(ctx, types.StringType, he.Labels)

				exp.HttpExport[i] = AmxHttpExportModel{
					Name:                  types.StringValue(he.Name),
					Enabled:               boolPtrToTF(he.Enable),
					DataType:              types.StringValue(mapAmxDataTypeFMToTF(he.Type)),
					Endpoint:              types.StringValue(he.Endpoint),
					Headers:               headers,
					SecureKeys:            secure,
					BindIPAddress:         stringOrNull(he.IfaceIPAddress),
					Format:                types.StringValue("json"),
					Compress:              boolPtrToTF(he.Zip),
					FlushIntervalSeconds:  int32PtrToTF(he.Interval),
					ParallelWorkers:       int32PtrToTF(he.Writers),
					MaxRetries:            int32PtrToTF(he.Retries),
					MaxRecordsPerBatch:    int32PtrToTF(he.MaxEntries),
					SelfHealWindowSeconds: int32PtrToTF(he.SelfHealTimerWindow),
					UploadTimeoutSeconds:  int32PtrToTF(he.HttpClientUploadTimeout),
					Labels:                labels,
				}
			}
		}

		// Kafka
		if len(fmData.Exporter.Kafka) > 0 {
			exp.KafkaExport = make([]AmxKafkaExportModel, len(fmData.Exporter.Kafka))
			for i, ke := range fmData.Exporter.Kafka {
				brokers, _ := types.ListValueFrom(ctx, types.StringType, ke.Brokers)
				labels, _ := types.MapValueFrom(ctx, types.StringType, ke.Labels)
				prodCfg, _ := types.ListValueFrom(ctx, types.StringType, ke.ProducerConfigs)

				exp.KafkaExport[i] = AmxKafkaExportModel{
					Name:                  types.StringValue(ke.Name),
					Topic:                 types.StringValue(ke.Topic),
					Enabled:               boolPtrToTF(ke.Enable),
					Brokers:               brokers,
					BindIPAddress:         stringOrNull(ke.IfaceIPAddress),
					DataType:              types.StringValue(mapAmxDataTypeFMToTF(ke.Type)),
					Format:                types.StringValue("json"),
					Compress:              boolPtrToTF(ke.Zip),
					FlushIntervalSeconds:  int32PtrToTF(ke.Interval),
					ParallelWorkers:       int32PtrToTF(ke.Writers),
					MaxRetries:            int32PtrToTF(ke.Retries),
					MaxRecordsPerBatch:    int32PtrToTF(ke.MaxEntries),
					SelfHealWindowSeconds: int32PtrToTF(ke.SelfHealTimerWindow),
					Labels:                labels,
					ProducerConfigs:       prodCfg,
				}
			}
		}

		data.Exporter = exp
	}

	// For now we leave enrichment blocks as-is (state-driven), because FM
	// may not echo all fields. This avoids churn/drift until we need full
	// round-trip mapping.
}

func mapAmxDataTypeTFToFM(tfType string) string {
	switch tfType {
	case "ami":
		return "ami"
	case "mobility":
		return "gtpc"
	case "ami_enriched":
		return "ami_enriched"
	case "netflow":
		return "netflow"
	default:
		// Preserve unknown values if present.
		return tfType
	}
}

func mapAmxDataTypeFMToTF(fmType string) string {
	switch fmType {
	case "ami":
		return "ami"
	case "ami_enriched":
		return "ami_enriched"
	case "gtpc", "gtpc_hier":
		return "mobility"
	case "netflow":
		return "netflow"
	default:
		return fmType
	}
}

func boolPtrToTF(p *bool) types.Bool {
	if p == nil {
		return types.BoolNull()
	}
	return types.BoolValue(*p)
}

func int32PtrToTF(p *int32) types.Int32 {
	if p == nil {
		return types.Int32Null()
	}
	return types.Int32Value(*p)
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// Semantic validation of AMX plan
func (a *Amx) validateAmxPlan(ctx context.Context, data *AmxModel) error {
	// 1) netflow-only ingestor cannot have enrichment
	hasIngestor := len(data.Ingestor) > 0
	onlyNetflow := true
	for _, in := range data.Ingestor {
		t := in.Type.ValueString()
		if t != "netflow" && t != "" {
			onlyNetflow = false
			break
		}
	}
	hasEnrichment := len(data.MobilityEnrichment) > 0 || len(data.WorkloadEnrichment) > 0 || len(data.OtherEnrichment) > 0
	if hasIngestor && onlyNetflow && hasEnrichment {
		return fmt.Errorf("metadata enrichment is not supported when all ingestors are Netflow/IPFIX; add a non-netflow ingestor or remove enrichment blocks")
	}

	// 2) at most one mobility enrichment
	if len(data.MobilityEnrichment) > 1 {
		return fmt.Errorf("only one mobility_enrichment block is allowed")
	}

	// 3) at most one workload_enrichment
	if len(data.WorkloadEnrichment) > 1 {
		return fmt.Errorf("only one workload_enrichment block is allowed")
	}

	// 4) if workload_enrichment is present, ensure any platform block has at least one source
	if len(data.WorkloadEnrichment) == 1 {
		we := data.WorkloadEnrichment[0]

		checkSrcList := func(name string, list []AmxWorkloadPlatformModel) error {
			for _, p := range list {
				if len(p.Sources) == 0 {
					return fmt.Errorf("workload_enrichment.%s must have at least one source block", name)
				}
			}
			return nil
		}

		if err := checkSrcList("aws", we.Aws); err != nil {
			return err
		}
		if err := checkSrcList("azure", we.Azure); err != nil {
			return err
		}
		if err := checkSrcList("vmware_vcenter", we.VmwareVcenter); err != nil {
			return err
		}
		if err := checkSrcList("aks", we.Aks); err != nil {
			return err
		}
		if err := validateAksSources(ctx, &we); err != nil {
			return err
		}
	}

	return nil
}

func validateAksSources(ctx context.Context, we *AmxWorkloadEnrichmentModel) error {
	_ = ctx

	for _, platform := range we.Aks {
		if len(platform.Sources) == 0 {
			return fmt.Errorf("workload_enrichment.aks must have at least one source block")
		}

		for _, src := range platform.Sources {
			hasKubeconfig := false
			for _, s := range src.SourceSettings {
				if s.Key.ValueString() == "k8s_kubeconfig" {
					fileNonEmpty := !s.File.IsNull() && !s.File.IsUnknown() && s.File.ValueString() != ""
					valueNonEmpty := !s.Value.IsNull() && !s.Value.IsUnknown() && s.Value.ValueString() != ""
					if fileNonEmpty || valueNonEmpty {
						hasKubeconfig = true
						break
					}
				}
			}

			if !hasKubeconfig {
				return fmt.Errorf("workload_enrichment.aks source %q must have a setting with key \"k8s_kubeconfig\" and non-empty file or value (kubeconfig is required for AKS workload enrichment)", src.Name.ValueString())
			}
		}
	}

	return nil
}

// Create call for new AMX App Instance
func (a *Amx) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AmxModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := a.validateAmxPlan(ctx, &data); err != nil {
		resp.Diagnostics.AddError(
			"Invalid AMX configuration",
			err.Error(),
		)
		return
	}

	fmData := a.createFMStruct(ctx, &data)

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "create",
				Application: fmData,
			},
		},
	}

	id, err := commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		a.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create AMX app",
			fmt.Sprintf("app creation failed: %s", err),
		)
		return
	}

	typedID, err := commonutils.MakeTypedID(
		commonutils.ModuleApp,
		commonutils.TypeAmx,
		id,
	)
	if err != nil {
		return
	}
	data.Id = types.StringValue(typedID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (a *Amx) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AmxModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	oldState := data

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	fmData := FMAmx{}
	err = GetMSAppData(
		ctx,
		data.MonitoringSessionId.ValueString(),
		rawID,
		"ogw",
		"",
		&fmData,
		a.fmClient,
	)
	if err != nil {
		var fmErr *fmclient.FMErrors
		if errors.As(err, &fmErr) {
			if fmErr.ErrorCode() == fmclient.ObjectNotFound {
				// App was deleted out-of-band
				resp.State.RemoveResource(ctx)
				return
			}
		}
		resp.Diagnostics.AddError(
			"Unable to Get AMX App details",
			fmt.Sprintf("unable to get AMX App details. error is %v", err),
		)
		return
	}

	// Update from FM
	a.updateTFStruct(ctx, &data, &fmData)
	// --- Preserve write-only/sensitive fields (headers & secure_keys) from old state ---
	if oldState.Exporter != nil && data.Exporter != nil {
		// HTTP exports: match by name
		for i, newHE := range data.Exporter.HttpExport {
			for _, oldHE := range oldState.Exporter.HttpExport {
				if newHE.Name.ValueString() == oldHE.Name.ValueString() {
					data.Exporter.HttpExport[i].Headers = oldHE.Headers
					data.Exporter.HttpExport[i].SecureKeys = oldHE.SecureKeys
					break
				}
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (a *Amx) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planData AmxModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &planData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := a.validateAmxPlan(ctx, &planData); err != nil {
		resp.Diagnostics.AddError(
			"Unable to update AMX app",
			err.Error(),
		)
		return
	}

	fmData := a.createFMStruct(ctx, &planData)

	rawID, err := commonutils.UUIDFromTypedID(planData.Id.ValueString())
	if err != nil {
		return
	}
	fmData.Id = rawID

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType:  "application",
				Operation:   "update",
				Application: fmData,
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		planData.MonitoringSessionId.ValueString(),
		a.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to update AMX app",
			fmt.Sprintf("app update failed: %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &planData)...)
}

func (a *Amx) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AmxModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawID, err := commonutils.UUIDFromTypedID(data.Id.ValueString())
	if err != nil {
		return
	}

	updateReq := commonutils.UpdateReq{
		Requests: []commonutils.UpdateObject{
			{
				EntityType: "application",
				Operation:  "delete",
				Application: FMAmx{
					Id:   rawID,
					Name: "Application",
				},
			},
		},
	}

	_, err = commonutils.UpdateMonSess(
		ctx,
		&updateReq,
		data.MonitoringSessionId.ValueString(),
		a.fmClient,
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to delete AMX app",
			fmt.Sprintf("app deletion failed: %s", err),
		)
	}
}
