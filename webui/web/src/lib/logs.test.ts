import { describe, expect, it } from 'vitest';
import { parseLogText } from './logs';

describe('parseLogText', () => {
  it('classifies zap JSON lines as daemon entries by level', () => {
    const text = [
      '{"level":"info","ts":1782571471.83,"caller":"cmd/serve.go:48","msg":"starting control rpc","socket":"/run/control.sock"}',
      '{"level":"warn","ts":1782571472,"msg":"listen address is remote-capable","auth_token_env":"LLAMARIG_CONTROL_TOKEN"}',
      '{"level":"error","ts":1782571473,"msg":"stop llama runtime","error":"router stop timed out","stacktrace":"llamarig/cmd.shutdown\\n\\tmore"}'
    ].join('\n');

    const { daemon, llama } = parseLogText(text);
    expect(llama).toHaveLength(0);
    expect(daemon.map((e) => e.level)).toEqual(['info', 'warn', 'error']);
    expect(daemon[0].msg).toBe('starting control rpc');
    expect(daemon[0].fields).toEqual({ socket: '/run/control.sock' });
    expect(daemon[2].stacktrace).toContain('llamarig/cmd.shutdown');
  });

  it('classifies non-JSON lines as llama lines with I/W/E severity', () => {
    const text = [
      '[53069] 0.09.354 I srv  llama_server: model loaded',
      '[53069] 0.09.355 W srv  update_slots: slot context shift',
      '[53069] 0.09.360 E srv  init: failed to load model',
      '[53069] cmd_child_to_router:state:{"state":"ready"}'
    ].join('\n');

    const { daemon, llama } = parseLogText(text);
    expect(daemon).toHaveLength(0);
    expect(llama.map((l) => l.severity)).toEqual(['I', 'W', 'E', '']);
    expect(llama[3].text).toContain('cmd_child_to_router');
  });

  it('ignores blank lines and a malformed JSON line falls through to llama', () => {
    const { daemon, llama } = parseLogText('\n{"level":}\n\n');
    expect(daemon).toHaveLength(0);
    expect(llama).toHaveLength(1);
  });
});
