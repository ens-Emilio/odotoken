package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &HealthDataSource{}

type HealthDataSource struct {
	client *Client
}

type HealthDataSourceModel struct {
	Status   types.String `tfsdk:"status"`
	Version  types.String `tfsdk:"version"`
	Uptime   types.String `tfsdk:"uptime"`
	DBStatus types.String `tfsdk:"db_status"`
}

func NewHealthDataSource() datasource.DataSource {
	return &HealthDataSource{}
}

func (d *HealthDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_health"
}

func (d *HealthDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Retorna o status de saúde do OdoToken.",
		Attributes: map[string]schema.Attribute{
			"status":    schema.StringAttribute{Computed: true},
			"version":   schema.StringAttribute{Computed: true},
			"uptime":    schema.StringAttribute{Computed: true},
			"db_status": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *HealthDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Erro de configuração", "Tipo de cliente inesperado")
		return
	}
	d.client = client
}

func (d *HealthDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	health, err := d.client.GetHealth(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao obter health", err.Error())
		return
	}

	state := HealthDataSourceModel{
		Status:   types.StringValue(health.Status),
		Version:  types.StringValue(health.Version),
		Uptime:   types.StringValue(health.Uptime),
		DBStatus: types.StringValue(health.DBStatus),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}