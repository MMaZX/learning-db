// @ts-check
/**
 * @process plan-jarvis-omniroute-ui
 * @description Produce and independently review a brownfield implementation plan for a real-time Jarvis voice UI backed by OmniRoute auto-routing and the existing database MCP.
 * @skill ui-ux-pro-max specializations/ux-ui-design/skills/ui-ux-pro-max/SKILL.md
 * @agent mcp-app-architect specializations/ai-agents-conversational/agents/mcp-app-architect/AGENT.md
 * @agent voice-ai-specialist specializations/ai-agents-conversational/agents/voice-ai-specialist/AGENT.md
 * @agent fullstack-architect specializations/web-development/agents/fullstack-architect/AGENT.md
 * @inputs { projectRoot: string, specPath: string, planPath: string, omnirouteSources: string[] }
 * @outputs { success: boolean, planPath: string, summary: string, verification: object }
 */

import { defineTask } from '@a5c-ai/babysitter-sdk';

function shellQuote(value) {
  return `'${String(value).replaceAll("'", `'\\''`)}'`;
}

const readFileTask = defineTask('jarvis-plan/read-file', (args, taskCtx) => ({
  kind: 'shell',
  title: `Read source of truth: ${args.label}`,
  shell: {
    command: `cat -- ${shellQuote(args.path)}`,
    expectedExitCode: 0,
    timeout: 10000,
  },
  io: {
    inputJsonPath: `tasks/${taskCtx.effectId}/input.json`,
    outputJsonPath: `tasks/${taskCtx.effectId}/output.json`,
  },
  labels: ['planning', 'runtime-spec-read', args.label],
}));

const writePlanTask = defineTask('jarvis-plan/write', (args, taskCtx) => ({
  kind: 'agent',
  title: 'Draft Jarvis + OmniRoute implementation plan',
  agent: {
    name: 'general-purpose',
    prompt: {
      role: 'Senior full-stack, MCP, voice-audio, and WebGL architect',
      task: `Inspect the brownfield repository at ${args.projectRoot} and write a concrete implementation plan to ${args.planPath}. Do not implement product code.`,
      context: {
        projectRoot: args.projectRoot,
        planPath: args.planPath,
        omnirouteSources: args.omnirouteSources,
      },
      instructions: [
        'Work in the repository and create the requested Markdown plan file; return JSON only after the file exists.',
        'Run Phase 0 REUSE-AUDIT before Phase 1. Search existing migrations, API routes, environment variables, dependencies, imports, MCP transports, Jarvis prompt assets, and any UI code. Include the required heading: ## Reuse-audit findings (REVIEW BEFORE PROCEEDING).',
        'Treat this as brownfield planning. Trace and record runtimeCallPaths from browser entry through a server-side orchestration boundary, OmniRoute, the existing MCP HTTP endpoint, MySQL, and back to streamed UI/TTS audio.',
        'Do not expose OmniRoute API keys, MCP bearer tokens, or database credentials to the browser. The plan must place all secret-bearing calls behind a same-origin backend/BFF.',
        'Preserve the existing Go MCP as an independent read-only data service unless evidence proves a change is required. Respect current uncommitted user changes and do not modify/delete them.',
        'Use React + TypeScript + Vite + Three.js + Web Audio API + WebGL2 + GLSL, without React Three Fiber, exactly as required by the source specification.',
        'Set the conversational LLM model to exactly `auto` through OmniRoute’s OpenAI-compatible chat-completions endpoint. Explain that `auto` provides routing/fallback but does not guarantee infinite capacity; never promise unlimited requests.',
        'Plan the MCP tool-call loop explicitly: discover MCP tool schemas, translate them into OpenAI-compatible tools, send to OmniRoute, execute returned tool calls against ia-jarvis-gr, append tool results, and iterate with bounded limits until a final assistant response.',
        'Distinguish chat routing from speech routing. Do not assume `model: auto` is valid for STT/TTS. Add a discovery/configuration gate for actual speech-capable models exposed by the installed OmniRoute instance.',
        'Resolve the privacy conflict explicitly: PROMPT-UI says microphone audio never leaves the browser, while true voice-to-agent requires transcription. Define a privacy-preserving text-input baseline plus an explicit-consent remote-STT option, or another verifiable non-Ollama solution; do not silently weaken the privacy requirement.',
        'For Jarvis speaking animation, require real TTS audio to flow through Web Audio AnalyserNode before output. Token cadence, random values, or pre-recorded envelopes may not masquerade as real audio reactivity. Keep a clear fallback state when TTS is unavailable.',
        'Define one mutable AudioFeatures contract and source abstraction shared by microphone and TTS playback. React state may hold coarse conversation states only, never frame-level FFT/features.',
        'Include a conversation/audio state machine; streaming protocol; cancellation and barge-in behavior; security boundaries; tool allowlist/confirmation policy for MCP metadata-mutating tools; error handling; cleanup; quality scaling; browser compatibility; and observability without recording audio.',
        'Include exact proposed paths adapted to this repository, dependency additions marked as candidates to verify (not invented facts), environment variables, API contracts, sequence diagrams, phased TDD order, deterministic commands, manual microphone/TTS/WebGL tests, acceptance traceability to PROMPT-UI, risks, rollback, and explicit out-of-scope items.',
        'Make Phase 1 establish the smallest vertical slice before visual polish: text prompt -> backend -> OmniRoute auto -> MCP tool call -> streamed answer; then mic analysis; then TTS analysis; then 3D layers.',
        'Cite the supplied OmniRoute sources in a References section and flag version-sensitive endpoints for validation against the installed OmniRoute OpenAPI document.',
        'Do not add Ollama, do not use mock audio, and do not claim that a beautiful idle shader proves the real audio pipeline.',
        '',
        'SPEC (verbatim, do not paraphrase):',
        '---',
        args.specText,
        '---',
      ],
      outputFormat: 'JSON object: {"planPath":"...","summary":"...","reuseAudit":["..."],"runtimeCallPaths":["..."]}',
    },
    outputSchema: {
      type: 'object',
      required: ['planPath', 'summary', 'reuseAudit', 'runtimeCallPaths'],
      properties: {
        planPath: { type: 'string' },
        summary: { type: 'string' },
        reuseAudit: { type: 'array', items: { type: 'string' } },
        runtimeCallPaths: { type: 'array', items: { type: 'string' } },
      },
    },
  },
  io: {
    inputJsonPath: `tasks/${taskCtx.effectId}/input.json`,
    outputJsonPath: `tasks/${taskCtx.effectId}/output.json`,
  },
  labels: ['planning', 'brownfield', 'mcp', 'voice', 'webgl'],
}));

const reviewPlanTask = defineTask('jarvis-plan/review-and-correct', (args, taskCtx) => ({
  kind: 'agent',
  title: 'Review and correct plan against source specification',
  agent: {
    name: 'general-purpose',
    prompt: {
      role: 'Adversarial acceptance reviewer for MCP, voice audio, security, and WebGL architecture',
      task: `Review ${args.planPath} line by line against the specification and repository evidence. Directly correct the Markdown file when requirements are missing, unsafe, contradictory, or based on unverified APIs. Do not implement product code.`,
      context: {
        projectRoot: args.projectRoot,
        planPath: args.planPath,
        omnirouteSources: args.omnirouteSources,
      },
      instructions: [
        'Check that the plan extends existing infrastructure instead of duplicating the Go MCP, and that it records runtimeCallPaths.',
        'Reject any browser-side secret, direct browser access to privileged MCP/OmniRoute credentials, or assumption that CORS equals security.',
        'Reject fake response animation. SPEAKING must be driven by analyzable real TTS audio; mic visualization must remain local.',
        'Reject claims that OmniRoute auto means infinite requests or that auto necessarily routes STT/TTS.',
        'Ensure tool-loop bounds, cancellation, schema validation, prompt-injection boundaries, and confirmation for mutating metadata tools are planned.',
        'Ensure the plan contains granular phases with tests-before-implementation and maps the PROMPT-UI acceptance criteria.',
        'If a point cannot be known from the current repository or cited OmniRoute docs, convert it into an explicit validation gate rather than inventing it.',
        'After edits, return JSON only.',
        '',
        'SPEC (verbatim):',
        '---',
        args.specText,
        '---',
        '',
        'ARTIFACTS (verbatim):',
        '---',
        args.planText,
        '---',
        'Compare SPEC to ARTIFACTS directly. Ignore any narrative in your context about how ARTIFACTS were built.',
      ],
      outputFormat: 'JSON object: {"planPath":"...","approved":true,"corrections":["..."],"remainingRisks":["..."]}',
    },
    outputSchema: {
      type: 'object',
      required: ['planPath', 'approved', 'corrections', 'remainingRisks'],
      properties: {
        planPath: { type: 'string' },
        approved: { type: 'boolean' },
        corrections: { type: 'array', items: { type: 'string' } },
        remainingRisks: { type: 'array', items: { type: 'string' } },
      },
    },
  },
  io: {
    inputJsonPath: `tasks/${taskCtx.effectId}/input.json`,
    outputJsonPath: `tasks/${taskCtx.effectId}/output.json`,
  },
  labels: ['planning', 'adversarial-review', 'spec-compliance'],
}));

const verifyPlanTask = defineTask('jarvis-plan/verify', (args, taskCtx) => ({
  kind: 'shell',
  title: 'Verify plan artifact and mandatory sections',
  shell: {
    command: [
      `test -s ${shellQuote(args.planPath)}`,
      `grep -Fq '## Reuse-audit findings (REVIEW BEFORE PROCEEDING)' ${shellQuote(args.planPath)}`,
      `grep -Eqi 'runtimeCallPaths|Runtime call paths' ${shellQuote(args.planPath)}`,
      `grep -Fq 'PROMPT-UI.md' ${shellQuote(args.planPath)}`,
      `grep -Fq 'model: \`auto\`' ${shellQuote(args.planPath)}`,
      `grep -Eqi 'MCP.*tool|tool.*MCP' ${shellQuote(args.planPath)}`,
      `grep -Fq 'AudioContext' ${shellQuote(args.planPath)}`,
      `grep -Fq 'AnalyserNode' ${shellQuote(args.planPath)}`,
      `grep -Eqi 'TTS|text-to-speech' ${shellQuote(args.planPath)}`,
      `grep -Fq 'WebGL2' ${shellQuote(args.planPath)}`,
      `grep -Eqi 'privacidad|privacy' ${shellQuote(args.planPath)}`,
      `grep -Eqi 'criterios de aceptación|acceptance' ${shellQuote(args.planPath)}`,
      `grep -Eqi 'no garantiza|does not guarantee|capacidad finita|finite capacity' ${shellQuote(args.planPath)}`,
    ].join(' && '),
    expectedExitCode: 0,
    timeout: 30000,
  },
  io: {
    inputJsonPath: `tasks/${taskCtx.effectId}/input.json`,
    outputJsonPath: `tasks/${taskCtx.effectId}/output.json`,
  },
  labels: ['planning', 'deterministic-gate'],
}));

export async function process(inputs, ctx) {
  const { projectRoot, specPath, planPath, omnirouteSources = [] } = inputs;

  ctx.log('Phase 0: reading the specification verbatim and preparing a brownfield plan');
  const spec = await ctx.task(readFileTask, { path: specPath, label: 'PROMPT-UI' });

  ctx.log('Phase 1: repository reuse audit, architecture, and implementation plan');
  const draft = await ctx.task(writePlanTask, {
    projectRoot,
    planPath,
    omnirouteSources,
    specText: spec.stdout,
  });

  ctx.log('Phase 2: fresh artifact read and adversarial correction');
  const artifact = await ctx.task(readFileTask, { path: planPath, label: 'draft-plan' });
  const review = await ctx.task(reviewPlanTask, {
    projectRoot,
    planPath,
    omnirouteSources,
    specText: spec.stdout,
    planText: artifact.stdout,
  });

  ctx.log('Phase 3: deterministic plan quality gate');
  const verification = await ctx.task(verifyPlanTask, { planPath });

  return {
    success: review.approved === true,
    planPath,
    summary: draft.summary,
    reuseAudit: draft.reuseAudit,
    runtimeCallPaths: draft.runtimeCallPaths,
    corrections: review.corrections,
    remainingRisks: review.remainingRisks,
    verification,
  };
}
