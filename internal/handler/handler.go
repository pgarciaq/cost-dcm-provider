// Package handler implements the DCM service provider contract for cost
// management: create, get, list, delete instances, and proxy usage/cost queries.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"

	oapigen "github.com/dcm-project/koku-cost-provider/internal/api/server"
	"github.com/dcm-project/koku-cost-provider/internal/health"
	"github.com/dcm-project/koku-cost-provider/internal/koku"
	"github.com/dcm-project/koku-cost-provider/internal/monitoring"
	"github.com/dcm-project/koku-cost-provider/internal/store"
	"github.com/dcm-project/koku-cost-provider/internal/util"
	"github.com/google/uuid"
)

var (
	_ oapigen.StrictServerInterface = (*Handler)(nil)

	safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
)

type Handler struct {
	store     InstanceStore
	koku      KokuClient
	publisher monitoring.StatusPublisher
	checker   *health.Checker
	logger    *slog.Logger
}

func New(s InstanceStore, k KokuClient, pub monitoring.StatusPublisher, checker *health.Checker, logger *slog.Logger) *Handler {
	return &Handler{
		store:     s,
		koku:      k,
		publisher: pub,
		checker:   checker,
		logger:    logger,
	}
}

// ---------- Health ----------

func (h *Handler) GetHealth(_ context.Context, _ oapigen.GetHealthRequestObject) (oapigen.GetHealthResponseObject, error) {
	result := h.checker.Check()
	return oapigen.GetHealth200JSONResponse(oapigen.Health{
		Type:    result.Type,
		Status:  result.Status,
		Path:    result.Path,
		Version: result.Version,
		Uptime:  result.Uptime,
	}), nil
}

// ---------- Create ----------

func (h *Handler) CreateInstance(ctx context.Context, req oapigen.CreateInstanceRequestObject) (oapigen.CreateInstanceResponseObject, error) {
	id := uuid.New().String()
	if req.Params.Id != nil && *req.Params.Id != "" {
		id = *req.Params.Id
	}

	if req.Body == nil {
		return oapigen.CreateInstance400ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INVALIDARGUMENT, 400, "Bad Request", "request body is required"),
		), nil
	}

	spec := req.Body.Spec

	if spec.Target.ResourceId == "" {
		return oapigen.CreateInstance400ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INVALIDARGUMENT, 400, "Bad Request", "target.resource_id is required"),
		), nil
	}

	if !safeIDPattern.MatchString(spec.Target.ResourceId) {
		return oapigen.CreateInstance400ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INVALIDARGUMENT, 400, "Bad Request", "target.resource_id contains invalid characters"),
		), nil
	}

	name := spec.Metadata.Name
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return oapigen.CreateInstance500ApplicationProblemPlusJSONResponse( //nolint:nilerr // error mapped to HTTP 500 response
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to serialize spec"),
		), nil
	}

	// For now use the target resource_id as cluster_id; in production this
	// would be resolved via ACM kubeconfig + ClusterVersion API.
	clusterID := spec.Target.ResourceId

	sourceName := name
	if sourceName == "" {
		sourceName = "dcm-" + id
	}

	// C1+C2 fix: Reserve the target in the store BEFORE calling Koku to
	// prevent orphaned Koku resources on duplicate target conflict.
	inst := &store.CostInstance{
		ID:               id,
		TargetResourceID: spec.Target.ResourceId,
		ClusterID:        clusterID,
		Name:             name,
		Status:           "PROVISIONING",
		StatusMessage:    "reserving target",
		SpecJSON:         string(specJSON),
	}

	if err := h.store.Create(inst); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return oapigen.CreateInstance409ApplicationProblemPlusJSONResponse(
				errResp(oapigen.ALREADYEXISTS, 409, "Conflict", "a cost instance already exists for this target cluster"),
			), nil
		}
		h.logger.Error("failed to reserve instance", "error", err)
		return oapigen.CreateInstance500ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to persist instance"),
		), nil
	}

	// Now create Koku resources (safe from orphans -- store row exists).
	src, err := h.koku.CreateSource(clusterID, sourceName)
	if err != nil {
		h.logger.Error("failed to create Koku source", "instance_id", id, "error", err)
		_ = h.store.UpdateStatus(id, "ERROR", "failed to create Koku source: "+err.Error())
		return oapigen.CreateInstance500ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to create Koku source"),
		), nil
	}
	inst.KokuSourceUUID = src.UUID
	inst.StatusMessage = "Koku source created, waiting for metering data"

	if spec.CostModel != nil && spec.CostModel.Rates != nil && len(*spec.CostModel.Rates) > 0 {
		rates := convertRates(*spec.CostModel.Rates)
		var markup *koku.CostModelMarkup
		if spec.CostModel.Markup != nil {
			markup = &koku.CostModelMarkup{
				Value: spec.CostModel.Markup.Value,
				Unit:  string(spec.CostModel.Markup.Unit),
			}
		}
		dist := "cpu"
		if spec.CostModel.Distribution != nil {
			dist = string(*spec.CostModel.Distribution)
		}
		cm, cmErr := h.koku.CreateCostModel(sourceName+"-cost-model", src.UUID, rates, markup, dist)
		if cmErr != nil {
			h.logger.Error("failed to create Koku cost model", "instance_id", id, "error", cmErr)
		} else {
			inst.KokuCostModelUUID = cm.UUID
		}
	}

	// Persist the Koku resource IDs back to the store row.
	if err := h.store.Update(inst); err != nil {
		h.logger.Error("failed to update instance with Koku IDs", "instance_id", id, "error", err)
	}

	if err := h.publisher.Publish(ctx, id, "PROVISIONING", "Koku source created, waiting for metering data"); err != nil {
		h.logger.Warn("failed to publish PROVISIONING event", "instance_id", id, "error", err)
	}

	h.logger.Info("instance created",
		"instance_id", id,
		"target", spec.Target.ResourceId,
		"cluster_id", clusterID,
		"koku_source", inst.KokuSourceUUID,
		"koku_cost_model", inst.KokuCostModelUUID,
	)

	return oapigen.CreateInstance201JSONResponse(toAPICostInstance(inst)), nil
}

// ---------- Get ----------

func (h *Handler) GetInstance(_ context.Context, req oapigen.GetInstanceRequestObject) (oapigen.GetInstanceResponseObject, error) {
	inst, err := h.store.Get(req.InstanceId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return oapigen.GetInstance404ApplicationProblemPlusJSONResponse(
				errResp(oapigen.NOTFOUND, 404, "Not Found", "cost instance not found"),
			), nil
		}
		return oapigen.GetInstance500ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to retrieve instance"),
		), nil
	}
	return oapigen.GetInstance200JSONResponse(toAPICostInstance(inst)), nil
}

// ---------- List ----------

func (h *Handler) ListInstances(_ context.Context, req oapigen.ListInstancesRequestObject) (oapigen.ListInstancesResponseObject, error) {
	pageSize := 50
	if req.Params.MaxPageSize != nil {
		pageSize = int(*req.Params.MaxPageSize)
	}

	offset := 0
	if req.Params.PageToken != nil && *req.Params.PageToken != "" {
		parsed, err := strconv.Atoi(*req.Params.PageToken)
		if err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	instances, total, err := h.store.List(pageSize, offset)
	if err != nil {
		return oapigen.ListInstances500ApplicationProblemPlusJSONResponse( //nolint:nilerr // error mapped to HTTP 500 response
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to list instances"),
		), nil
	}

	apiInstances := make([]oapigen.CostInstance, 0, len(instances))
	for i := range instances {
		apiInstances = append(apiInstances, toAPICostInstance(&instances[i]))
	}

	var nextToken *string
	if int64(offset+pageSize) < total {
		t := fmt.Sprintf("%d", offset+pageSize)
		nextToken = &t
	}

	return oapigen.ListInstances200JSONResponse(oapigen.CostInstanceList{
		Instances:     &apiInstances,
		NextPageToken: nextToken,
	}), nil
}

// ---------- Delete ----------

func (h *Handler) DeleteInstance(ctx context.Context, req oapigen.DeleteInstanceRequestObject) (oapigen.DeleteInstanceResponseObject, error) {
	inst, err := h.store.Get(req.InstanceId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return oapigen.DeleteInstance404ApplicationProblemPlusJSONResponse(
				errResp(oapigen.NOTFOUND, 404, "Not Found", "cost instance not found"),
			), nil
		}
		return oapigen.DeleteInstance500ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to retrieve instance"),
		), nil
	}

	// C4 fix: prevent double-delete
	if inst.Status == "DELETED" {
		return oapigen.DeleteInstance204Response{}, nil
	}

	if inst.KokuCostModelUUID != "" {
		if err := h.koku.DeleteCostModel(inst.KokuCostModelUUID); err != nil {
			h.logger.Error("failed to delete Koku cost model", "instance_id", inst.ID, "error", err)
		}
	}

	if inst.KokuSourceUUID != "" {
		if err := h.koku.PauseSource(inst.KokuSourceUUID); err != nil {
			h.logger.Error("failed to pause Koku source", "instance_id", inst.ID, "error", err)
		}
	}

	if err := h.store.UpdateStatus(inst.ID, "DELETED", "instance deleted"); err != nil {
		h.logger.Error("failed to update status", "instance_id", inst.ID, "error", err)
	}

	if err := h.publisher.Publish(ctx, inst.ID, "DELETED", "instance deleted"); err != nil {
		h.logger.Warn("failed to publish DELETED event", "instance_id", inst.ID, "error", err)
	}

	h.logger.Info("instance deleted",
		"instance_id", inst.ID,
		"target", inst.TargetResourceID,
		"koku_source", inst.KokuSourceUUID,
		"koku_cost_model", inst.KokuCostModelUUID,
	)

	return oapigen.DeleteInstance204Response{}, nil
}

// ---------- Usage proxy ----------

func (h *Handler) GetUsage(_ context.Context, req oapigen.GetUsageRequestObject) (oapigen.GetUsageResponseObject, error) {
	inst, err := h.store.Get(req.InstanceId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return oapigen.GetUsage404ApplicationProblemPlusJSONResponse(
				errResp(oapigen.NOTFOUND, 404, "Not Found", "cost instance not found"),
			), nil
		}
		return oapigen.GetUsage500ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to retrieve instance"),
		), nil
	}

	params := url.Values{}
	if req.Params.StartDate != nil {
		params.Set("start_date", req.Params.StartDate.Format("2006-01-02"))
	}
	if req.Params.EndDate != nil {
		params.Set("end_date", req.Params.EndDate.Format("2006-01-02"))
	}

	data, err := h.koku.GetReports(inst.ClusterID, string(req.Metric), params)
	if err != nil {
		h.logger.Error("failed to get usage from Koku", "error", err)
		return oapigen.GetUsage502ApplicationProblemPlusJSONResponse(
			errResp(oapigen.BADGATEWAY, 502, "Bad Gateway", "failed to retrieve usage data from Koku"),
		), nil
	}

	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return oapigen.GetUsage200JSONResponse(result), nil
}

// ---------- Cost report proxy ----------

func (h *Handler) GetCostReport(_ context.Context, req oapigen.GetCostReportRequestObject) (oapigen.GetCostReportResponseObject, error) {
	inst, err := h.store.Get(req.InstanceId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return oapigen.GetCostReport404ApplicationProblemPlusJSONResponse(
				errResp(oapigen.NOTFOUND, 404, "Not Found", "cost instance not found"),
			), nil
		}
		return oapigen.GetCostReport500ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to retrieve instance"),
		), nil
	}

	params := url.Values{}
	if req.Params.StartDate != nil {
		params.Set("start_date", req.Params.StartDate.Format("2006-01-02"))
	}
	if req.Params.EndDate != nil {
		params.Set("end_date", req.Params.EndDate.Format("2006-01-02"))
	}

	data, err := h.koku.GetReports(inst.ClusterID, "costs", params)
	if err != nil {
		h.logger.Error("failed to get cost report from Koku", "error", err)
		return oapigen.GetCostReport502ApplicationProblemPlusJSONResponse(
			errResp(oapigen.BADGATEWAY, 502, "Bad Gateway", "failed to retrieve cost report from Koku"),
		), nil
	}

	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return oapigen.GetCostReport200JSONResponse(result), nil
}

// ---------- Forecast proxy ----------

func (h *Handler) GetCostForecast(_ context.Context, req oapigen.GetCostForecastRequestObject) (oapigen.GetCostForecastResponseObject, error) {
	inst, err := h.store.Get(req.InstanceId)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return oapigen.GetCostForecast404ApplicationProblemPlusJSONResponse(
				errResp(oapigen.NOTFOUND, 404, "Not Found", "cost instance not found"),
			), nil
		}
		return oapigen.GetCostForecast500ApplicationProblemPlusJSONResponse(
			errResp(oapigen.INTERNAL, 500, "Internal Server Error", "failed to retrieve instance"),
		), nil
	}

	data, err := h.koku.GetForecasts(inst.ClusterID, nil)
	if err != nil {
		h.logger.Error("failed to get forecast from Koku", "error", err)
		return oapigen.GetCostForecast502ApplicationProblemPlusJSONResponse(
			errResp(oapigen.BADGATEWAY, 502, "Bad Gateway", "failed to retrieve forecast from Koku"),
		), nil
	}

	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return oapigen.GetCostForecast200JSONResponse(result), nil
}

// ---------- Helpers ----------

func toAPICostInstance(inst *store.CostInstance) oapigen.CostInstance {
	path := "instances/" + inst.ID
	status := oapigen.CostInstanceStatus(inst.Status)
	createTime := inst.CreatedAt
	updateTime := inst.UpdatedAt

	ci := oapigen.CostInstance{
		Id:            util.Ptr(inst.ID),
		Path:          &path,
		Status:        &status,
		StatusMessage: util.Ptr(inst.StatusMessage),
		ClusterId:     util.Ptr(inst.ClusterID),
		CreateTime:    &createTime,
		UpdateTime:    &updateTime,
	}

	if inst.SpecJSON != "" {
		var spec oapigen.CostSpec
		if err := json.Unmarshal([]byte(inst.SpecJSON), &spec); err == nil {
			ci.Spec = spec
		}
	}

	return ci
}

func convertRates(rates []oapigen.Rate) []koku.CostModelRate {
	result := make([]koku.CostModelRate, 0, len(rates))
	for _, r := range rates {
		costType := "Infrastructure"
		if r.CostType != nil {
			costType = string(*r.CostType)
		}
		result = append(result, koku.CostModelRate{
			Metric:   koku.CostModelMetric{Name: string(r.Metric)},
			CostType: costType,
			TieredRates: []koku.CostModelTieredRate{
				{Value: r.Value, Unit: "USD"},
			},
		})
	}
	return result
}

func errResp(errType oapigen.ErrorType, status int32, title, detail string) oapigen.Error {
	return oapigen.Error{
		Type:   errType,
		Title:  title,
		Status: util.Ptr(status),
		Detail: util.Ptr(detail),
	}
}
