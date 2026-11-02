package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &OdoTokenProvider{}

type OdoTokenProvider struct {
	version string
}

type OdoTokenProviderModel struct {
	Endpoint types.String `tfsdk:"endpoint"`
	APIKey   types.String `tfsdk:"api_key"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OdoTokenProvider{
			version: version,
		}
	}
}

func (p *OdoTokenProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "odotoken"
	resp.Version = p.version
}

func (p *OdoTokenProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provider para gerenciar recursos do OdoToken via Terraform.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Description: "URL base da API do OdoToken. Pode ser configurada via variável de ambiente ODOTOKEN_ENDPOINT.",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "Chave de API para autenticação. Pode ser configurada via variável de ambiente ODOTOKEN_API_KEY.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *OdoTokenProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config OdoTokenProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := os.Getenv("ODOTOKEN_ENDPOINT")
	if !config.Endpoint.IsNull() {
		endpoint = config.Endpoint.ValueString()
	}
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}

	apiKey := os.Getenv("ODOTOKEN_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"API Key não configurada",
			"A API Key deve ser fornecida via atributo 'api_key' ou variável de ambiente ODOTOKEN_API_KEY.",
		)
		return
	}

	client := NewClient(endpoint, apiKey)
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *OdoTokenProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewClientKeyResource,
		NewPriceRuleResource,
		NewGuardPolicyResource,
	}
}

func (p *OdoTokenProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewClientKeysDataSource,
		NewModelsDataSource,
		NewHealthDataSource,
	}
}