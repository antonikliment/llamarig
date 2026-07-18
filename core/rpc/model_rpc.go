package rpc

import (
	"cmp"
	"context"
	"llamarig/core/control"
	"llamarig/core/modelcatalog"
	"llamarig/core/modeldownload"
	"llamarig/core/modelpresets"
	controlv1 "llamarig/core/rpc/gen/v1"

	"sort"
	"time"

	"connectrpc.com/connect"
)

func (s *ControlService) ListLocalModels(ctx context.Context, _ *controlv1.ListLocalModelsRequest) (*controlv1.ListLocalModelsResponse, error) {
	if s.localModels == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "local model inventory is not configured"))
	}
	models, err := s.localModels.ListLocal(ctx)
	if err != nil {
		return nil, rpcError(mapModelError(err))
	}
	presets, err := s.manager.ListPresets(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	// Canonicalize preset paths once; CanonicalPath does disk I/O (symlink
	// resolution), so doing it per model would be O(models*presets) syscalls.
	canonical := modelpresets.CanonicalizeSections(presets)
	out := make([]*controlv1.LocalModel, 0, len(models))
	for _, model := range models {
		refs := modelpresets.FindReferencesCanonical(canonical, model.Path)
		usedBy := append(append([]string(nil), refs.ModelPaths...), refs.ModelDirs...)
		sort.Strings(usedBy)
		out = append(out, &controlv1.LocalModel{Path: model.Path, Filename: model.Filename, SizeBytes: model.SizeBytes, ModifiedAt: model.ModifiedAt.Format(time.RFC3339), UsedByPresets: usedBy, ModelPathPresets: refs.ModelPaths, ModelsDirPresets: refs.ModelDirs})
	}
	return &controlv1.ListLocalModelsResponse{Ok: true, Models: out}, nil
}

func (s *ControlService) DeleteLocalModel(ctx context.Context, req *controlv1.DeleteLocalModelRequest) (*controlv1.MutationResponse, error) {
	if req == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "request is required"))
	}
	if req.GetPath() == "" {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "local model path is required"))
	}
	result, err := s.manager.DeleteLocalModel(ctx, req.GetPath(), req.GetCascadePresets())
	if err != nil {
		return nil, rpcError(mapModelError(err))
	}
	return &controlv1.MutationResponse{Ok: true, Result: operationResultProto(result)}, nil
}

func (s *ControlService) ResolveModel(ctx context.Context, req *controlv1.ResolveModelRequest) (*controlv1.ResolveModelResponse, error) {
	if s.modelCatalog == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "model catalog is not configured"))
	}
	raw := cmp.Or(req.GetUrl(), req.GetModel())
	if raw == "" {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "model URL or name is required"))
	}
	resolution, err := s.modelCatalog.Resolve(ctx, raw)
	if err != nil {
		return nil, rpcError(mapModelError(err))
	}
	return &controlv1.ResolveModelResponse{Ok: true, Resolution: modelResolutionProto(resolution)}, nil
}

func (s *ControlService) ListModelCatalog(ctx context.Context, req *controlv1.ListModelCatalogRequest) (*controlv1.ListModelCatalogResponse, error) {
	if s.modelDiscoverer == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "model catalog discovery is not configured"))
	}
	listReq := modelcatalog.ListRequest{Limit: int(req.GetLimit()), Sort: req.GetSort(), Search: req.GetSearch(), MinFit: req.GetMinFit(), Task: req.GetTask(), LocalOnly: req.GetLocalOnly()}
	machine, err := s.machine.Machine(ctx)
	if err != nil {
		return nil, rpcError(control.CoreError(control.ErrorRuntime, "capture machine profile", err))
	}
	result, err := s.modelDiscoverer.List(ctx, listReq, machineProfile(machine))
	if err != nil {
		return nil, rpcError(mapModelError(err))
	}
	return &controlv1.ListModelCatalogResponse{
		Ok: true, Machine: &controlv1.MachineProfile{TotalRamBytes: result.Machine.TotalRAMBytes, AvailableRamBytes: result.Machine.AvailableRAMBytes, GpuName: result.Machine.GPUName, VramBytes: result.Machine.VRAMBytes, HasGpu: result.Machine.HasGPU}, Models: catalogModelProtos(result.Models),
		Cache: &controlv1.CacheState{Hit: result.Cache.Hit, Stale: result.Cache.Stale, Refreshing: result.Cache.Refreshing, UpdatedAt: result.Cache.UpdatedAt, TtlSeconds: result.Cache.TTLSeconds}, Errors: result.Errors,
	}, nil
}

func (s *ControlService) WatchModelCatalog(ctx context.Context, _ *controlv1.WatchModelCatalogRequest, stream *connect.ServerStream[controlv1.ModelCatalogEvent]) error {
	if s.refreshNotifier == nil {
		return rpcError(control.Errorf(control.ErrorInvalidInput, "model catalog refresh events are not configured"))
	}
	events, unsubscribe := s.refreshNotifier.Subscribe()
	defer unsubscribe()
	for {
		select {
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := stream.Send(&controlv1.ModelCatalogEvent{Type: event.Type, Ok: event.Error == "", Error: event.Error}); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *ControlService) StartModelDownload(ctx context.Context, req *controlv1.StartModelDownloadRequest) (*controlv1.ModelDownloadResponse, error) {
	if s.modelDownloader == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "model downloader is not configured"))
	}
	if req.GetUrl() == "" {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "model URL is required"))
	}
	job, err := s.modelDownloader.Start(ctx, modeldownload.Request{URL: req.GetUrl(), Filename: req.GetFilename(), Force: req.GetForce()})
	if err != nil {
		return nil, rpcError(mapModelError(err))
	}
	return &controlv1.ModelDownloadResponse{Ok: true, Download: modelDownloadProto(job)}, nil
}

func (s *ControlService) GetModelDownload(ctx context.Context, req *controlv1.GetModelDownloadRequest) (*controlv1.ModelDownloadResponse, error) {
	return downloaderAction(ctx, s.modelDownloader, req.GetId(), modeldownload.Downloader.Get)
}

func (s *ControlService) CancelModelDownload(ctx context.Context, req *controlv1.CancelModelDownloadRequest) (*controlv1.ModelDownloadResponse, error) {
	return downloaderAction(ctx, s.modelDownloader, req.GetId(), modeldownload.Downloader.Cancel)
}

func downloaderAction(ctx context.Context, dl modeldownload.Downloader, id string, op func(modeldownload.Downloader, context.Context, string) (modeldownload.Job, error)) (*controlv1.ModelDownloadResponse, error) {
	if dl == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "model downloader is not configured"))
	}
	if id == "" {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "download ID is required"))
	}
	job, err := op(dl, ctx, id)
	if err != nil {
		return nil, rpcError(mapModelError(err))
	}
	return &controlv1.ModelDownloadResponse{Ok: true, Download: modelDownloadProto(job)}, nil
}

func (s *ControlService) ApplyModelDownloadToPreset(ctx context.Context, req *controlv1.ApplyModelDownloadToPresetRequest) (*controlv1.MutationResponse, error) {
	if s.modelDownloader == nil {
		return nil, rpcError(control.Errorf(control.ErrorInvalidInput, "model downloader is not configured"))
	}
	if err := validateApplyModelDownloadRequest(req); err != nil {
		return nil, rpcError(err)
	}
	job, err := s.modelDownloader.Get(ctx, req.GetId())
	if err != nil {
		return nil, rpcError(mapModelError(err))
	}
	if err := requireCompletedModelDownload(job); err != nil {
		return nil, rpcError(err)
	}
	section, err := s.manager.GetPreset(ctx, req.GetPreset())
	if err != nil {
		return nil, rpcError(err)
	}
	original := section.Values["model"]
	if req.GetPreview() {
		return &controlv1.MutationResponse{Ok: true, PreviewDiff: &controlv1.TextDiff{Original: original, Updated: job.TargetPath}}, nil
	}
	section.Values["model"] = job.TargetPath
	delete(section.Values, "models-dir")
	if _, err := s.manager.PutPreset(ctx, section, false); err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.MutationResponse{Ok: true}, nil
}

func requireCompletedModelDownload(job modeldownload.Job) error {
	if job.State == modeldownload.StateCompleted || job.State == modeldownload.StateAlreadyDownloaded {
		return nil
	}
	return control.Errorf(control.ErrorConflict, "download %s is %s, not completed", job.ID, job.State)
}

func validateApplyModelDownloadRequest(req *controlv1.ApplyModelDownloadToPresetRequest) error {
	if req.GetPreset() == "" {
		return control.Errorf(control.ErrorInvalidInput, "preset is required")
	}
	if req.GetId() == "" {
		return control.Errorf(control.ErrorInvalidInput, "download ID is required")
	}
	return nil
}
