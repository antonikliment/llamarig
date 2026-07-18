package rpc

import (
	"cmp"
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	platformconfig "llamarig/config"
	"llamarig/core/control"
	"llamarig/core/modelcatalog"
	"llamarig/core/modeldownload"
	controlv1 "llamarig/core/rpc/gen/v1"
	"llamarig/core/rpc/gen/v1/controlv1connect"
	"llamarig/core/signals"
	"llamarig/internal/buildinfo"
)

const ServiceName = "llamarig-server"
const errorKindHeader = platformconfig.ProjectDisplayName + "-Error-Kind"

type RPCDependencies struct {
	Manager         *control.Manager
	ModelCatalog    modelcatalog.Catalog
	ModelDiscoverer modelcatalog.Discoverer
	LocalModels     modelcatalog.LocalLister
	RefreshNotifier modelcatalog.RefreshNotifier
	Machine         signals.MachineCollector
	Signals         signals.Collector
	ModelDownloader modeldownload.Downloader
	Events          *control.EventStore
	ServiceName     string
}

type ControlService struct {
	controlv1connect.UnimplementedControlServiceHandler

	manager         *control.Manager
	modelCatalog    modelcatalog.Catalog
	modelDiscoverer modelcatalog.Discoverer
	localModels     modelcatalog.LocalLister
	refreshNotifier modelcatalog.RefreshNotifier
	machine         signals.MachineCollector
	signals         signals.Collector
	modelDownloader modeldownload.Downloader
	events          *control.EventStore
	serviceName     string
	presetSources   *presetSourceCache
}

func NewControlService(deps RPCDependencies) *ControlService {
	if deps.Manager == nil {
		panic("rpc: Manager dependency is required")
	}
	machine := deps.Machine
	if machine == nil {
		machine = &signals.GopsutilCollector{}
	}
	serviceName := cmp.Or(deps.ServiceName, ServiceName)
	return &ControlService{
		manager:         deps.Manager,
		modelCatalog:    deps.ModelCatalog,
		modelDiscoverer: deps.ModelDiscoverer,
		localModels:     deps.LocalModels,
		refreshNotifier: deps.RefreshNotifier,
		machine:         machine,
		signals:         deps.Signals,
		modelDownloader: deps.ModelDownloader,
		events:          deps.Events,
		serviceName:     serviceName,
		presetSources:   newPresetSourceCache(),
	}
}

func (s *ControlService) Health(_ context.Context, _ *controlv1.HealthRequest) (*controlv1.HealthResponse, error) {
	return &controlv1.HealthResponse{Ok: true, Service: s.serviceName}, nil
}

func (s *ControlService) GetInfo(ctx context.Context, _ *controlv1.GetInfoRequest) (*controlv1.GetInfoResponse, error) {
	info, err := s.manager.GetInfo(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	return &controlv1.GetInfoResponse{Ok: true, Service: s.serviceName, Info: &controlv1.RuntimeInfo{Router: &controlv1.RouterInfo{Status: info.Router.Status, Detail: info.Router.Detail, CheckedAt: info.Router.CheckedAt}, PresetsCount: int32(info.PresetsCount), DefaultPreset: info.DefaultPreset, AutostartPresets: info.AutostartPresets}, Build: &controlv1.BuildInfo{Version: buildinfo.Version, Commit: buildinfo.Commit, CommitTime: buildinfo.CommitTime}}, nil
}

func (s *ControlService) GetSignals(ctx context.Context, _ *controlv1.GetSignalsRequest) (*controlv1.GetSignalsResponse, error) {
	snapshot, err := s.snapshot(ctx)
	if err != nil {
		return nil, err
	}
	return &controlv1.GetSignalsResponse{Ok: true, Signals: signalsSnapshotProto(snapshot)}, nil
}

func runtimeStatusProto(status control.RuntimeStatus) *controlv1.RuntimeStatus {
	presets := mapProto(status.Presets, func(preset control.RuntimePreset) *controlv1.RuntimePreset {
		return &controlv1.RuntimePreset{Name: preset.Name, State: preset.State, Ready: preset.Ready}
	})
	return &controlv1.RuntimeStatus{State: status.State, Detail: status.Detail, CheckedAt: status.CheckedAt.Format(time.RFC3339), Presets: presets}
}

func operationResultProto(result control.OperationResult) *controlv1.CommandResult {
	return &controlv1.CommandResult{Target: result.Target, Action: result.Action, Status: result.Status, Message: result.Message, DurationMs: result.DurationMS}
}

func signalsSnapshotProto(snapshot signals.Snapshot) *controlv1.SignalsSnapshot {
	return &controlv1.SignalsSnapshot{
		CapturedAt: snapshot.CapturedAt, Host: &controlv1.HostSnapshot{Hostname: snapshot.Host.Hostname, Os: snapshot.Host.OS, Platform: snapshot.Host.Platform},
		Memory: &controlv1.MemorySnapshot{AvailableBytes: snapshot.Memory.AvailableBytes, TotalBytes: snapshot.Memory.TotalBytes, UsedBytes: snapshot.Memory.UsedBytes, UsedPercent: snapshot.Memory.UsedPercent},
		Disks:  diskSnapshotProtos(snapshot.Disks), Cpu: &controlv1.CPUSnapshot{LogicalCores: int32(snapshot.CPU.LogicalCores), UsedPercent: snapshot.CPU.UsedPercent}, Gpu: gpuSnapshotProtos(snapshot.GPU), Runtime: runtimeProcessSnapshotProtos(snapshot.Runtime), Warnings: snapshot.Warnings,
	}
}

func runtimeResourcesProto(snapshot signals.Snapshot) *controlv1.RuntimeResources {
	return &controlv1.RuntimeResources{AvailableRamBytes: snapshot.Memory.AvailableBytes, TotalRamBytes: snapshot.Memory.TotalBytes, UsedRamBytes: snapshot.Memory.UsedBytes, MemoryUsedPercent: snapshot.Memory.UsedPercent, CpuLogicalCores: int32(snapshot.CPU.LogicalCores), CpuUsedPercent: snapshot.CPU.UsedPercent, Disks: diskSnapshotProtos(snapshot.Disks), Gpu: gpuSnapshotProtos(snapshot.GPU), Runtime: runtimeProcessSnapshotProtos(snapshot.Runtime), Warnings: snapshot.Warnings}
}

func gpuSnapshotProtos(gpus []signals.GPUStats) []*controlv1.GPUSnapshot {
	return mapProto(gpus, func(gpu signals.GPUStats) *controlv1.GPUSnapshot {
		return &controlv1.GPUSnapshot{Name: gpu.Name, Backend: gpu.Backend, TotalVramBytes: gpu.TotalVRAMBytes, UsedVramBytes: gpu.UsedVRAMBytes, UtilizationPercent: gpu.UtilizationPercent, Source: gpu.Source, TemperatureCelsius: gpu.TemperatureCelsius}
	})
}

func runtimeProcessSnapshotProtos(processes []signals.RuntimeProcessStats) []*controlv1.RuntimeProcessSnapshot {
	return mapProto(processes, func(process signals.RuntimeProcessStats) *controlv1.RuntimeProcessSnapshot {
		return &controlv1.RuntimeProcessSnapshot{Name: process.Name, Pid: int64(process.PID), RssBytes: process.RSSBytes, CpuPercent: process.CPUPercent, Command: process.Command}
	})
}

func diskSnapshotProtos(disks []signals.DiskStats) []*controlv1.DiskSnapshot {
	return mapProto(disks, func(disk signals.DiskStats) *controlv1.DiskSnapshot {
		return &controlv1.DiskSnapshot{Label: disk.Label, Path: disk.Path, TotalBytes: disk.TotalBytes, FreeBytes: disk.FreeBytes, UsedBytes: disk.UsedBytes, UsedPercent: disk.UsedPercent}
	})
}

func eventsAfter(events []control.Event, afterID string) []control.Event {
	if afterID == "" {
		return events
	}
	for i, event := range events {
		if event.ID == afterID {
			return events[i+1:]
		}
	}
	return events
}

func eventProto(id string, event control.Event) *controlv1.Event {
	id = cmp.Or(id, event.ID)
	return &controlv1.Event{Id: id, Action: event.Action, Time: event.Time, Success: event.Success, ErrorKind: string(event.ErrorKind), Duration: event.Duration}
}

func machineProfile(snapshot signals.MachineSnapshot) modelcatalog.MachineProfile {
	profile := modelcatalog.MachineProfile{TotalRAMBytes: int64(snapshot.Memory.TotalBytes), AvailableRAMBytes: int64(snapshot.Memory.AvailableBytes)}
	for _, gpu := range snapshot.GPU {
		if !profile.HasGPU || gpu.TotalVRAMBytes > uint64(profile.VRAMBytes) {
			profile.GPUName, profile.VRAMBytes, profile.HasGPU = gpu.Name, int64(gpu.TotalVRAMBytes), true
		}
	}
	return profile
}

func modelResolutionProto(resolution modelcatalog.Resolution) *controlv1.ModelResolution {
	return &controlv1.ModelResolution{Ok: resolution.OK, Source: &controlv1.ModelSource{Kind: resolution.Source.Kind, Owner: resolution.Source.Owner, Repo: resolution.Source.Repo, Url: resolution.Source.URL}, LlamaCpp: &controlv1.LlamaCPPCompatibility{Compatible: resolution.LlamaCPP.Compatible, HfRef: resolution.LlamaCPP.HFRef, Reason: resolution.LlamaCPP.Reason}, Files: mapProto(resolution.Files, modelFileProto), Description: resolution.Description, Params: resolution.Params, Architecture: resolution.Architecture, ContextLength: resolution.ContextLength, IsMoe: resolution.IsMoE}
}

func modelFileProto(file modelcatalog.File) *controlv1.ModelFile {
	return &controlv1.ModelFile{Filename: file.Filename, Quant: file.Quant, SizeBytes: file.SizeBytes, Downloadable: file.Downloadable, LocalPath: file.LocalPath, Exists: file.Exists, EstimatedRamBytes: file.EstimatedRAMBytes, EstimatedVramBytes: file.EstimatedVRAMBytes, FitLevel: file.FitLevel, FitReason: file.FitReason}
}

func catalogModelProtos(models []modelcatalog.CatalogModel) []*controlv1.CatalogModel {
	return mapProto(models, func(model modelcatalog.CatalogModel) *controlv1.CatalogModel {
		item := &controlv1.CatalogModel{Id: model.ID, Owner: model.Owner, Repo: model.Repo, Url: model.URL, Downloads: model.Downloads, Likes: model.Likes, LastModified: model.LastModified, Tags: model.Tags, License: model.License, Files: mapProto(model.Files, modelFileProto), Fit: &controlv1.MachineFit{Level: model.Fit.Level, Reason: model.Fit.Reason, RequiredRamBytes: model.Fit.RequiredRAMBytes, AvailableRamBytes: model.Fit.AvailableRAMBytes, RequiredVramBytes: model.Fit.RequiredVRAMBytes, AvailableVramBytes: model.Fit.AvailableVRAMBytes}, Score: model.Score, Params: model.Params, Architecture: model.Architecture, ContextLength: model.ContextLength, IsMoe: model.IsMoE}
		if model.BestFile != nil {
			item.BestFile = modelFileProto(*model.BestFile)
		}
		return item
	})
}

func modelDownloadProto(job modeldownload.Job) *controlv1.ModelDownload {
	return &controlv1.ModelDownload{
		Id: job.ID, State: job.State, ReceivedBytes: uint64(job.ReceivedBytes), TotalBytes: uint64(job.TotalBytes), Error: job.Error,
		Url: job.URL, Filename: job.Filename, TargetPath: job.TargetPath, Percent: job.Percent, StartedAt: job.StartedAt, CompletedAt: job.CompletedAt,
	}
}

func rpcError(err error) error {
	if err == nil {
		return nil
	}
	kind := control.Kind(err)
	code := connect.CodeUnknown
	switch kind {
	case control.ErrorInvalidInput:
		code = connect.CodeInvalidArgument
	case control.ErrorPermission:
		code = connect.CodePermissionDenied
	case control.ErrorNotFound:
		code = connect.CodeNotFound
	case control.ErrorConflict:
		code = connect.CodeFailedPrecondition
	case control.ErrorTimeout:
		code = connect.CodeDeadlineExceeded
	case control.ErrorRuntime:
		code = connect.CodeInternal
	}
	connectErr := connect.NewError(code, err)
	if kind != "" {
		connectErr.Meta().Set(errorKindHeader, string(kind))
	}
	return connectErr
}

func ErrorKindFromRPC(err error) control.ErrorKind {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return control.Kind(err)
	}
	if kind := connectErr.Meta().Get(errorKindHeader); kind != "" {
		return control.ErrorKind(kind)
	}
	switch connectErr.Code() {
	case connect.CodeInvalidArgument:
		return control.ErrorInvalidInput
	case connect.CodePermissionDenied:
		return control.ErrorPermission
	case connect.CodeNotFound:
		return control.ErrorNotFound
	case connect.CodeFailedPrecondition:
		return control.ErrorConflict
	case connect.CodeDeadlineExceeded:
		return control.ErrorTimeout
	default:
		return control.ErrorRuntime
	}
}

func mapProto[I any, O any](items []I, fn func(I) *O) []*O {
	out := make([]*O, 0, len(items))
	for _, item := range items {
		out = append(out, fn(item))
	}
	return out
}

func mapModelError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, modelcatalog.ErrInvalidInput) || errors.Is(err, modeldownload.ErrInvalidInput) {
		return control.CoreError(control.ErrorInvalidInput, err.Error(), err)
	}
	if errors.Is(err, modelcatalog.ErrNotFound) || errors.Is(err, modeldownload.ErrNotFound) {
		return control.CoreError(control.ErrorNotFound, err.Error(), err)
	}
	if errors.Is(err, modeldownload.ErrConflict) {
		return control.CoreError(control.ErrorConflict, err.Error(), err)
	}
	return err
}
