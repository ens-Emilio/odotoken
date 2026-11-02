package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ClientKeysDataSource{}

type ClientKeysDataSource struct {
	client *Client
}

type ClientKeysDataSourceModel struct {
	Keys []ClientKeyItem `tfsdk:"keys"`
}

type ClientKeyItem struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.String `tfsdk:"created_at"`
}

func NewClientKeysDataSource() datasource.DataSource {
	return &ClientKeysDataSource{}
}

func (d *ClientKeysDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_client_keys"
}

func (d *ClientKeysDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lista todas as chaves de API do OdoToken.",
		Attributes: map[string]schema.Attribute{
			"keys": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true},
						"name":       schema.StringAttribute{Computed: true},
						"created_at": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ClientKeysDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ClientKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	keys, err := d.client.ListClientKeys(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao listar client keys", err.Error())
		return
	}

	var state ClientKeysDataSourceModel
	for _, k := range keys {
		state.Keys = append(state.Keys, ClientKeyItem{
			ID:        types.StringValue(k.ID),
			Name:      types.StringValue(k.Name),
			CreatedAt: types.StringValue(k.CreatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}