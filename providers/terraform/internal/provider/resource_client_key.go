package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ClientKeyResource{}

type ClientKeyResource struct {
	client *Client
}

type ClientKeyResourceModel struct {
	ID          types.String  `tfsdk:"id"`
	Name        types.String  `tfsdk:"name"`
	Key         types.String  `tfsdk:"key"`
	Scopes      types.List    `tfsdk:"scopes"`
	RateLimit   types.Int64   `tfsdk:"rate_limit"`
	BudgetLimit types.Float64 `tfsdk:"budget_limit"`
	Tags        types.Map     `tfsdk:"tags"`
	CreatedAt   types.String  `tfsdk:"created_at"`
	UpdatedAt   types.String  `tfsdk:"updated_at"`
}

func NewClientKeyResource() resource.Resource {
	return &ClientKeyResource{}
}

func (r *ClientKeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_client_key"
}

func (r *ClientKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Gerencia uma chave de API do OdoToken.",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true},
			"name":         schema.StringAttribute{Required: true},
			"key":          schema.StringAttribute{Computed: true, Sensitive: true},
			"scopes":       schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"rate_limit":   schema.Int64Attribute{Optional: true},
			"budget_limit": schema.Float64Attribute{Optional: true},
			"tags":         schema.MapAttribute{Optional: true, ElementType: types.StringType},
			"created_at":   schema.StringAttribute{Computed: true},
			"updated_at":   schema.StringAttribute{Computed: true},
		},
	}
}

func (r *ClientKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ClientKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ClientKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key := &ClientKey{Name: plan.Name.ValueString()}
	if !plan.Scopes.IsNull() && !plan.Scopes.IsUnknown() {
		var scopes []string
		resp.Diagnostics.Append(plan.Scopes.ElementsAs(ctx, &scopes, false)...)
		key.Scopes = scopes
	}
	if !plan.RateLimit.IsNull() && !plan.RateLimit.IsUnknown() {
		key.RateLimit = int(plan.RateLimit.ValueInt64())
	}
	if !plan.BudgetLimit.IsNull() && !plan.BudgetLimit.IsUnknown() {
		key.BudgetLimit = plan.BudgetLimit.ValueFloat64()
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		tags := make(map[string]string)
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		key.Tags = tags
	}
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.CreateClientKey(ctx, key)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao criar client key", err.Error())
		return
	}

	plan.ID = types.StringValue(result.ID)
	plan.Key = types.StringValue(result.Key)
	plan.CreatedAt = types.StringValue(result.CreatedAt)
	plan.UpdatedAt = types.StringValue(result.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ClientKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ClientKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetClientKey(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Erro ao ler client key", err.Error())
		return
	}

	state.Name = types.StringValue(result.Name)
	state.UpdatedAt = types.StringValue(result.UpdatedAt)
	if len(result.Scopes) > 0 {
		scopesVal, diags := types.ListValueFrom(ctx, types.StringType, result.Scopes)
		resp.Diagnostics.Append(diags...)
		state.Scopes = scopesVal
	} else if !state.Scopes.IsNull() {
		state.Scopes = types.ListNull(types.StringType)
	}
	if result.RateLimit > 0 {
		state.RateLimit = types.Int64Value(int64(result.RateLimit))
	} else if !state.RateLimit.IsNull() {
		state.RateLimit = types.Int64Null()
	}
	if result.BudgetLimit > 0 {
		state.BudgetLimit = types.Float64Value(result.BudgetLimit)
	} else if !state.BudgetLimit.IsNull() {
		state.BudgetLimit = types.Float64Null()
	}
	if len(result.Tags) > 0 {
		tagsVal, diags := types.MapValueFrom(ctx, types.StringType, result.Tags)
		resp.Diagnostics.Append(diags...)
		state.Tags = tagsVal
	} else if !state.Tags.IsNull() {
		state.Tags = types.MapNull(types.StringType)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ClientKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ClientKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	key := &ClientKey{Name: plan.Name.ValueString()}
	if !plan.Scopes.IsNull() && !plan.Scopes.IsUnknown() {
		var scopes []string
		resp.Diagnostics.Append(plan.Scopes.ElementsAs(ctx, &scopes, false)...)
		key.Scopes = scopes
	}
	if !plan.RateLimit.IsNull() && !plan.RateLimit.IsUnknown() {
		key.RateLimit = int(plan.RateLimit.ValueInt64())
	}
	if !plan.BudgetLimit.IsNull() && !plan.BudgetLimit.IsUnknown() {
		key.BudgetLimit = plan.BudgetLimit.ValueFloat64()
	}
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		tags := make(map[string]string)
		resp.Diagnostics.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
		key.Tags = tags
	}
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.UpdateClientKey(ctx, plan.ID.ValueString(), key)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao atualizar client key", err.Error())
		return
	}

	plan.UpdatedAt = types.StringValue(result.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ClientKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ClientKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteClientKey(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Erro ao deletar client key", err.Error())
	}
}