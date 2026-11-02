package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ModelsDataSource{}

type ModelsDataSource struct {
	client *Client
}

type ModelsDataSourceModel struct {
	Models []types.String `tfsdk:"models"`
}

func NewModelsDataSource() datasource.DataSource {
	return &ModelsDataSource{}
}

func (d *ModelsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_models"
}

func (d *ModelsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lista os modelos disponíveis no OdoToken.",
		Attributes: map[string]schema.Attribute{
			"models": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *ModelsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ModelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	models, err := d.client.ListModels(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao listar modelos", err.Error())
		return
	}

	var state ModelsDataSourceModel
	for _, m := range models {
		state.Models = append(state.Models, types.StringValue(m))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}