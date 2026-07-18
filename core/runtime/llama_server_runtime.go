package runtime

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	platformconfig "llamarig/config"
	"llamarig/platform/pidfile"
)

type LlamaServerConfig struct {
	Name, Executable                    string
	Argv                                []string
	Host                                string
	Port                                int
	Env                                 map[string]string
	Timeout                             time.Duration
	ReadinessPath                       string
	ReadinessTimeout, ReadinessInterval time.Duration
	PIDDir                              string
}

type LlamaServer struct {
	mu        sync.Mutex
	cfg       LlamaServerConfig
	cmd       *exec.Cmd
	done      chan error
	err       error
	ready     bool
	pid, pgid int
	now       func() time.Time
	client    *http.Client
}

func NewLlamaServer(cfg LlamaServerConfig) *LlamaServer {
	cfg.applyDefaults()
	return &LlamaServer{cfg: cfg, now: time.Now, client: http.DefaultClient}
}

func (r *LlamaServer) Status(context.Context) (Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statusLocked(), nil
}

func (r *LlamaServer) Start(ctx context.Context) (CommandResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.startLocked(ctx, "start")
}

func (r *LlamaServer) Stop(ctx context.Context) (CommandResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stopLocked(ctx, "stop")
}

func (r *LlamaServer) Recover(ctx context.Context) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	command, err := r.cfg.command()
	if err != nil {
		return false, err
	}
	file, err := r.pidFile()
	if err != nil {
		return false, err
	}
	pid, ok, err := file.Running()
	if err != nil || !ok {
		return false, err
	}
	if !pidfile.ExecutableMatches(pid, command.Executable) {
		_ = file.Remove(pid)
		return false, nil
	}
	r.adopt(pid)
	r.ready = r.probeReady(ctx) == nil
	return true, nil
}

func (r *LlamaServer) startLocked(ctx context.Context, action string) (CommandResult, error) {
	start := r.now()
	if r.isRunning() {
		err := NewError(ErrorRuntime, "llama-server is already running", nil)
		return r.commandResult(action, start, 1, "", err), err
	}
	command, err := r.cfg.command()
	if err != nil {
		return r.commandResult(action, start, 1, "", err), err
	}
	file, err := r.pidFile()
	if err != nil {
		return r.commandResult(action, start, 1, "", err), err
	}
	cmd := exec.Command(command.Executable, command.Argv...)
	cmd.Env = append(os.Environ(), envList(r.cfg.Env)...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		err = NewError(ErrorRuntime, "start llama-server failed", err)
		return r.commandResult(action, start, 1, "", err), err
	}
	r.started(cmd)
	recorded := make(chan struct{})
	go func(pid int) {
		err := cmd.Wait()
		<-recorded
		_ = file.Remove(pid)
		r.done <- err
	}(r.pid)
	pidErr := file.Write(r.pid)
	close(recorded)
	if pidErr != nil {
		err = NewError(ErrorRuntime, "write llama-server PID file failed", pidErr)
		return r.rollbackStart(ctx, action, start, err)
	}
	if err := r.waitReady(ctx); err != nil {
		r.err = err
		return r.rollbackStart(ctx, action, start, NewError(ErrorRuntime, err.Error(), nil))
	}
	return r.commandResult(action, start, 0, command.Display, nil), nil
}

func (r *LlamaServer) stopLocked(ctx context.Context, action string) (CommandResult, error) {
	start := r.now()
	if !r.isRunning() {
		return r.commandResult(action, start, 0, "", nil), nil
	}
	if err := r.stopProcess(ctx); err != nil {
		return r.commandResult(action, start, 1, "", err), err
	}
	return r.commandResult(action, start, 0, "", nil), nil
}

func (r *LlamaServer) rollbackStart(ctx context.Context, action string, start time.Time, err error) (CommandResult, error) {
	if cleanupErr := r.stopProcess(ctx); cleanupErr != nil {
		err = fmt.Errorf("%w; cleanup failed: %w", err, cleanupErr)
	}
	return r.commandResult(action, start, 1, "", err), err
}

func (r *LlamaServer) stopProcess(ctx context.Context) error {
	pid, pgid := r.pid, r.pgid
	if pgid <= 0 {
		pgid = pid
	}
	defer func() {
		if file, err := r.pidFile(); err == nil {
			_ = file.Remove(pid)
		}
		r.pid, r.pgid, r.ready = 0, 0, false
	}()
	if !r.isRunning() {
		return nil
	}
	if err := interruptProcessGroup(pid, pgid); err != nil {
		return fmt.Errorf("%s interrupt: %v", r.cfg.Name, err)
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, r.cfg.Timeout)
	defer cancel()
	if r.cmd != nil {
		return r.waitStartedProcess(timeoutCtx, pgid)
	}
	if err := waitPIDExit(timeoutCtx, pid); err != nil {
		killProcessGroup(pid, pgid)
		r.err = err
		return fmt.Errorf("%s stop timed out", r.cfg.Name)
	}
	return nil
}

func (r *LlamaServer) waitReady(ctx context.Context) error {
	deadline := r.now().Add(r.cfg.ReadinessTimeout)
	for {
		if !r.isRunning() {
			return fmt.Errorf("%s exited before readiness: %v", r.cfg.Name, r.err)
		}
		if err := r.probeReady(ctx); err == nil {
			r.ready = true
			return nil
		}
		if !r.now().Before(deadline) {
			return fmt.Errorf("%s readiness timed out", r.cfg.Name)
		}
		timer := time.NewTimer(r.cfg.ReadinessInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *LlamaServer) probeReady(ctx context.Context) error {
	addr := net.JoinHostPort(readinessHost(r.cfg.Host), strconv.Itoa(r.cfg.Port))
	if r.cfg.ReadinessPath == "" {
		conn, err := net.DialTimeout("tcp", addr, r.cfg.ReadinessInterval)
		if err != nil {
			return err
		}
		return conn.Close()
	}
	reqCtx, cancel := context.WithTimeout(ctx, r.cfg.ReadinessInterval)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://"+addr+r.cfg.ReadinessPath, nil)
	if err != nil {
		return err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 500 {
		return nil
	}
	return fmt.Errorf("readiness status %d", resp.StatusCode)
}

func (r *LlamaServer) statusLocked() Status {
	state, pid := Stopped, 0
	if r.isRunning() {
		state, pid = Starting, r.pid
		if r.ready {
			state = Running
		}
	} else if r.pid > 0 {
		if file, err := r.pidFile(); err == nil {
			_ = file.Remove(r.pid)
		}
	}
	detail := fmt.Sprintf("%s stopped", r.cfg.Name)
	if pid > 0 {
		detail = fmt.Sprintf("%s pid=%d", r.cfg.Name, pid)
	} else if r.err != nil {
		detail = fmt.Sprintf("%s stopped: %v", r.cfg.Name, r.err)
	}
	lastErr := ""
	if r.err != nil {
		lastErr = r.err.Error()
	}
	process := ProcessStatus{Name: r.cfg.Name, State: state, PID: pid, Host: r.cfg.Host, Port: r.cfg.Port, Ready: r.ready, LastError: lastErr}
	return Status{State: state, Detail: detail, CheckedAt: r.now().UTC(), Processes: []ProcessStatus{process}}
}

func (r *LlamaServer) commandResult(action string, start time.Time, exitCode int, stdout string, err error) CommandResult {
	result := CommandResult{Action: action, ExitCode: exitCode, Stdout: stdout, DurationMS: r.now().Sub(start).Milliseconds()}
	if err != nil {
		result.Stderr = err.Error()
	}
	return result
}

func (r *LlamaServer) started(cmd *exec.Cmd) {
	r.cmd, r.done, r.err, r.ready = cmd, make(chan error, 1), nil, false
	r.pid = cmd.Process.Pid
	r.pgid, _ = syscall.Getpgid(r.pid)
}

func (r *LlamaServer) adopt(pid int) {
	r.cmd, r.done, r.err, r.ready = nil, nil, nil, false
	r.pid = pid
	r.pgid, _ = syscall.Getpgid(pid)
}

func (r *LlamaServer) isRunning() bool {
	if r.pid <= 0 {
		return false
	}
	if r.cmd != nil {
		select {
		case err := <-r.done:
			r.err, r.ready = err, false
			return false
		default:
		}
	}
	return pidfile.Alive(r.pid)
}

func (r *LlamaServer) pidFile() (pidfile.File, error) {
	name := r.cfg.Name
	if r.cfg.PIDDir == "" {
		return pidfile.File{}, fmt.Errorf("llama PID directory is not configured")
	}
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return pidfile.File{}, fmt.Errorf("invalid llama process name %q", name)
	}
	return pidfile.New(filepath.Join(r.cfg.PIDDir, name+".pid")), nil
}

func (r *LlamaServer) waitStartedProcess(ctx context.Context, pgid int) error {
	select {
	case err := <-r.done:
		r.err = err
		return nil
	case <-ctx.Done():
		killProcessGroup(r.pid, pgid)
		select {
		case err := <-r.done:
			r.err = err
		case <-time.After(time.Second):
		}
		return fmt.Errorf("%s stop timed out", r.cfg.Name)
	}
}

func (c *LlamaServerConfig) applyDefaults() {
	c.Name = cmp.Or(c.Name, platformconfig.DefaultLlamaExecutable)
	c.Executable = cmp.Or(c.Executable, platformconfig.DefaultLlamaExecutable)
	c.Host = cmp.Or(c.Host, platformconfig.DefaultLlamaHost)
	c.Port = cmp.Or(c.Port, platformconfig.DefaultLlamaPort)
	c.Timeout = cmp.Or(c.Timeout, 10*time.Second)
	c.ReadinessTimeout = cmp.Or(c.ReadinessTimeout, platformconfig.DefaultReadinessTimeout)
	c.ReadinessInterval = cmp.Or(c.ReadinessInterval, platformconfig.DefaultReadinessInterval)
	if c.PIDDir == "" {
		home, err := platformconfig.LlamaRigHome()
		if err == nil {
			c.PIDDir = filepath.Join(home, "run", "llama")
		}
	}
}

type command struct {
	Executable string
	Argv       []string
	Display    string
}

func (c *LlamaServerConfig) command() (command, error) {
	if c.Executable == "" {
		return command{}, Errorf(ErrorInvalidInput, "%s executable is required", c.Name)
	}
	return command{Executable: c.Executable, Argv: append([]string(nil), c.Argv...), Display: strings.Join(append([]string{c.Executable}, c.Argv...), " ")}, nil
}

func interruptProcessGroup(pid, pgid int) error {
	if err := syscall.Kill(-pgid, syscall.SIGINT); err != nil {
		return syscall.Kill(pid, syscall.SIGINT)
	}
	return nil
}

func killProcessGroup(pid, pgid int) {
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func waitPIDExit(ctx context.Context, pid int) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if !pidfile.Alive(pid) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}

func readinessHost(host string) string {
	if host == "" || host == "0.0.0.0" || host == "::" {
		return platformconfig.DefaultLlamaHost
	}
	return host
}
