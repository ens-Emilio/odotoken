package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &PriceRuleResource{}

type PriceRuleResource struct {
	client *Client
}

type PriceRuleResourceModel struct {
	ID          types.String  `tfsdk:"id"`
	ModelPattern types.String `tfsdk:"model_pattern"`
	ClientID    types.String  `tfsdk:"client_id"`
	InputPrice  types.Float64 `tfsdk:"input_price"`
	OutputPrice types.Float64 `tfsdk:"output_price"`
	Currency    types.String  `tfsdk:"currency"`
	Priority    types.Int64   `tfsdk:"priority"`
	CreatedAt   types.String  `tfsdk:"created_at"`
	UpdatedAt   types.String  `tfsdk:"updated_at"`
}

func NewPriceRuleResource() resource.Resource {
	return &PriceRuleResource{}
}

func (r *PriceRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_price_rule"
}

func (r *PriceRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Gerencia uma regra de preço do OdoToken.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true},
			"model_pattern": schema.StringAttribute{Required: true},
			"client_id":     schema.StringAttribute{Optional: true},
			"input_price":   schema.Float64Attribute{Required: true},
			"output_price":  schema.Float64Attribute{Required: true},
			"currency":      schema.StringAttribute{Optional: true, Computed: true},
			"priority":      schema.Int64Attribute{Optional: true},
			"created_at":    schema.StringAttribute{Computed: true},
			"updated_at":    schema.StringAttribute{Computed: true},
		},
	}
}

func (r *PriceRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*Client)
	if !ok {
		resp.Diagnostics.AddError("Erro de configuração", "Tipo de cliente inesperado")
		return
	}
	r.client = client
}

func (r *PriceRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PriceRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule := &PriceRule{
		ModelPattern: plan.ModelPattern.ValueString(),
		InputPrice:   plan.InputPrice.ValueFloat64(),
		OutputPrice:  plan.OutputPrice.ValueFloat64(),
	}
	if !plan.ClientID.IsNull() {
		rule.ClientID = plan.ClientID.ValueString()
	}
	if !plan.Currency.IsNull() {
		rule.Currency = plan.Currency.ValueString()
	}
	if !plan.Priority.IsNull() {
		rule.Priority = int(plan.Priority.ValueInt64())
	}

	result, err := r.client.CreatePriceRule(ctx, rule)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao criar price rule", err.Error())
		return
	}

	plan.ID = types.StringValue(result.ID)
	plan.CreatedAt = types.StringValue(result.CreatedAt)
	plan.UpdatedAt = types.StringValue(result.UpdatedAt)
	if result.Currency != "" {
		plan.Currency = types.StringValue(result.Currency)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PriceRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PriceRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetPriceRule(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Erro ao ler price rule", err.Error())
		return
	}

	state.ModelPattern = types.StringValue(result.ModelPattern)
	state.InputPrice = types.Float64Value(result.InputPrice)
	state.OutputPrice = types.Float64Value(result.OutputPrice)
	state.UpdatedAt = types.StringValue(result.UpdatedAt)
	if result.ClientID != "" {
		state.ClientID = types.StringValue(result.ClientID)
	} else if !state.ClientID.IsNull() {
		state.ClientID = types.StringNull()
	}
	if result.Currency != "" {
		state.Currency = types.StringValue(result.Currency)
	} else if !state.Currency.IsNull() {
		state.Currency = types.StringNull()
	}
	if result.Priority > 0 {
		state.Priority = types.Int64Value(int64(result.Priority))
	} else if !state.Priority.IsNull() {
		state.Priority = types.Int64Null()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PriceRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PriceRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rule := &PriceRule{
		ModelPattern: plan.ModelPattern.ValueString(),
		InputPrice:   plan.InputPrice.ValueFloat64(),
		OutputPrice:  plan.OutputPrice.ValueFloat64(),
	}
	if !plan.ClientID.IsNull() && !plan.ClientID.IsUnknown() {
		rule.ClientID = plan.ClientID.ValueString()
	}
	if !plan.Currency.IsNull() && !plan.Currency.IsUnknown() {
		rule.Currency = plan.Currency.ValueString()
	}
	if !plan.Priority.IsNull() && !plan.Priority.IsUnknown() {
		rule.Priority = int(plan.Priority.ValueInt64())
	}

	result, err := r.client.UpdatePriceRule(ctx, plan.ID.ValueString(), rule)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao atualizar price rule", err.Error())
		return
	}

	plan.UpdatedAt = types.StringValue(result.UpdatedAt)
	if result.Currency != "" {
		plan.Currency = types.StringValue(result.Currency)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PriceRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PriceRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeletePriceRule(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Erro ao deletar price rule", err.Error())
	}
}