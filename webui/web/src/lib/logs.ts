// Parses the raw daemon log tail (~/.llamarig/run/llamarig.log) into the two views
// the TUI shows: structured zap entries and raw llama-server lines. Mirrors the
// classification in adapters/tui/tabs/logs.go (parseZapLine / splitLlamaLine).

export type ZapEntry = {
  level: string;
  ts: number;
  msg: string;
  caller: string;
  stacktrace: string;
  fields: Record<string, unknown>;
};

export type LlamaLine = {
  severity: 'I' | 'W' | 'E' | '';
  text: string;
};

const reserved = new Set(['level', 'ts', 'msg', 'caller', 'stacktrace']);
const llamaSeverity = /\s([IWE])\s/;

export function parseLogText(text: string): { daemon: ZapEntry[]; llama: LlamaLine[] } {
  const daemon: ZapEntry[] = [];
  const llama: LlamaLine[] = [];
  for (const line of text.split('\n')) {
    if (!line.trim()) continue;
    const entry = parseZapLine(line);
    if (entry) daemon.push(entry);
    else llama.push(parseLlamaLine(line));
  }
  return { daemon, llama };
}

function parseZapLine(line: string): ZapEntry | null {
  let raw: Record<string, unknown>;
  try {
    raw = JSON.parse(line);
  } catch {
    return null;
  }
  if (!raw || typeof raw !== 'object' || typeof raw.level !== 'string' || !raw.level) return null;
  const fields: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(raw)) {
    if (!reserved.has(key)) fields[key] = value;
  }
  return {
    level: raw.level,
    ts: typeof raw.ts === 'number' ? raw.ts : 0,
    msg: typeof raw.msg === 'string' ? raw.msg : '',
    caller: typeof raw.caller === 'string' ? raw.caller : '',
    stacktrace: typeof raw.stacktrace === 'string' ? raw.stacktrace : '',
    fields
  };
}

function parseLlamaLine(line: string): LlamaLine {
  const match = llamaSeverity.exec(line);
  return { severity: (match?.[1] as LlamaLine['severity']) ?? '', text: line };
}
