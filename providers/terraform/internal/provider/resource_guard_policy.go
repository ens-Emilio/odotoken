package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &GuardPolicyResource{}

type GuardPolicyResource struct {
	client *Client
}

type GuardPolicyResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	ClientIDs         types.List   `tfsdk:"client_ids"`
	BlockPatterns     types.List   `tfsdk:"block_patterns"`
	AllowPatterns     types.List   `tfsdk:"allow_patterns"`
	MaxTokens         types.Int64  `tfsdk:"max_tokens"`
	MaxRequestsPerMin types.Int64  `tfsdk:"max_requests_per_min"`
	Enabled           types.Bool   `tfsdk:"enabled"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

func NewGuardPolicyResource() resource.Resource {
	return &GuardPolicyResource{}
}

func (r *GuardPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_guard_policy"
}

func (r *GuardPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Gerencia uma política de segurança do OdoToken.",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true},
			"name":                schema.StringAttribute{Required: true},
			"client_ids":          schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"block_patterns":      schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"allow_patterns":      schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"max_tokens":          schema.Int64Attribute{Optional: true},
			"max_requests_per_min": schema.Int64Attribute{Optional: true},
			"enabled":             schema.BoolAttribute{Required: true},
			"created_at":          schema.StringAttribute{Computed: true},
			"updated_at":          schema.StringAttribute{Computed: true},
		},
	}
}

func (r *GuardPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GuardPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GuardPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy := &GuardPolicy{
		Name:    plan.Name.ValueString(),
		Enabled: plan.Enabled.ValueBool(),
	}
	if !plan.ClientIDs.IsNull() && !plan.ClientIDs.IsUnknown() {
		var clientIDs []string
		resp.Diagnostics.Append(plan.ClientIDs.ElementsAs(ctx, &clientIDs, false)...)
		policy.ClientIDs = clientIDs
	}
	if !plan.BlockPatterns.IsNull() && !plan.BlockPatterns.IsUnknown() {
		var patterns []string
		resp.Diagnostics.Append(plan.BlockPatterns.ElementsAs(ctx, &patterns, false)...)
		policy.BlockPatterns = patterns
	}
	if !plan.AllowPatterns.IsNull() && !plan.AllowPatterns.IsUnknown() {
		var patterns []string
		resp.Diagnostics.Append(plan.AllowPatterns.ElementsAs(ctx, &patterns, false)...)
		policy.AllowPatterns = patterns
	}
	if !plan.MaxTokens.IsNull() && !plan.MaxTokens.IsUnknown() {
		policy.MaxTokens = int(plan.MaxTokens.ValueInt64())
	}
	if !plan.MaxRequestsPerMin.IsNull() && !plan.MaxRequestsPerMin.IsUnknown() {
		policy.MaxRequestsPerMin = int(plan.MaxRequestsPerMin.ValueInt64())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.CreateGuardPolicy(ctx, policy)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao criar guard policy", err.Error())
		return
	}

	plan.ID = types.StringValue(result.ID)
	plan.CreatedAt = types.StringValue(result.CreatedAt)
	plan.UpdatedAt = types.StringValue(result.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GuardPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GuardPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.GetGuardPolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Erro ao ler guard policy", err.Error())
		return
	}

	state.Name = types.StringValue(result.Name)
	state.Enabled = types.BoolValue(result.Enabled)
	state.UpdatedAt = types.StringValue(result.UpdatedAt)
	if len(result.ClientIDs) > 0 {
		val, diags := types.ListValueFrom(ctx, types.StringType, result.ClientIDs)
		resp.Diagnostics.Append(diags...)
		state.ClientIDs = val
	} else if !state.ClientIDs.IsNull() {
		state.ClientIDs = types.ListNull(types.StringType)
	}
	if len(result.BlockPatterns) > 0 {
		val, diags := types.ListValueFrom(ctx, types.StringType, result.BlockPatterns)
		resp.Diagnostics.Append(diags...)
		state.BlockPatterns = val
	} else if !state.BlockPatterns.IsNull() {
		state.BlockPatterns = types.ListNull(types.StringType)
	}
	if len(result.AllowPatterns) > 0 {
		val, diags := types.ListValueFrom(ctx, types.StringType, result.AllowPatterns)
		resp.Diagnostics.Append(diags...)
		state.AllowPatterns = val
	} else if !state.AllowPatterns.IsNull() {
		state.AllowPatterns = types.ListNull(types.StringType)
	}
	if result.MaxTokens > 0 {
		state.MaxTokens = types.Int64Value(int64(result.MaxTokens))
	} else if !state.MaxTokens.IsNull() {
		state.MaxTokens = types.Int64Null()
	}
	if result.MaxRequestsPerMin > 0 {
		state.MaxRequestsPerMin = types.Int64Value(int64(result.MaxRequestsPerMin))
	} else if !state.MaxRequestsPerMin.IsNull() {
		state.MaxRequestsPerMin = types.Int64Null()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GuardPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GuardPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy := &GuardPolicy{
		Name:    plan.Name.ValueString(),
		Enabled: plan.Enabled.ValueBool(),
	}
	if !plan.ClientIDs.IsNull() && !plan.ClientIDs.IsUnknown() {
		var clientIDs []string
		resp.Diagnostics.Append(plan.ClientIDs.ElementsAs(ctx, &clientIDs, false)...)
		policy.ClientIDs = clientIDs
	}
	if !plan.BlockPatterns.IsNull() && !plan.BlockPatterns.IsUnknown() {
		var patterns []string
		resp.Diagnostics.Append(plan.BlockPatterns.ElementsAs(ctx, &patterns, false)...)
		policy.BlockPatterns = patterns
	}
	if !plan.AllowPatterns.IsNull() && !plan.AllowPatterns.IsUnknown() {
		var patterns []string
		resp.Diagnostics.Append(plan.AllowPatterns.ElementsAs(ctx, &patterns, false)...)
		policy.AllowPatterns = patterns
	}
	if !plan.MaxTokens.IsNull() && !plan.MaxTokens.IsUnknown() {
		policy.MaxTokens = int(plan.MaxTokens.ValueInt64())
	}
	if !plan.MaxRequestsPerMin.IsNull() && !plan.MaxRequestsPerMin.IsUnknown() {
		policy.MaxRequestsPerMin = int(plan.MaxRequestsPerMin.ValueInt64())
	}
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.UpdateGuardPolicy(ctx, plan.ID.ValueString(), policy)
	if err != nil {
		resp.Diagnostics.AddError("Erro ao atualizar guard policy", err.Error())
		return
	}

	plan.UpdatedAt = types.StringValue(result.UpdatedAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GuardPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GuardPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteGuardPolicy(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Erro ao deletar guard policy", err.Error())
	}
}