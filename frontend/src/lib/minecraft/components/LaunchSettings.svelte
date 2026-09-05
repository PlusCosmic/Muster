<script lang="ts">
  // Per-machine launch settings for one pack. The pack recommends; the user
  // decides, within what this machine can give.
  import Icon from '$lib/shell/components/Icon.svelte';
  import { packs } from '$lib/minecraft/stores/packs.svelte';
  import type { LaunchSettings, Pack } from '$lib/minecraft/types';

  let { pack, onclose }: { pack: Pack; onclose?: () => void } = $props();

  const STEP = 512;
  const MIN = 1024;
  const maxHeap = $derived(packs.detected?.maxHeapMb || 0);
  const totalMb = $derived(packs.detected?.totalMemoryMb || 0);
  const sliderMax = $derived(maxHeap > 0 ? maxHeap : Math.max(pack.launch.maxMemoryMb, 16384));

  // The draft starts from the pack's current settings and then belongs to
  // the user; a later change to `pack` must not clobber what they typed.
  // svelte-ignore state_referenced_locally
  let maxMemoryMb = $state(pack.launch.maxMemoryMb);
  // svelte-ignore state_referenced_locally
  let preallocate = $state(pack.launch.minMemoryMb !== null);
  // svelte-ignore state_referenced_locally
  let argsText = $state(pack.launch.args.join(' '));
  // svelte-ignore state_referenced_locally
  let follow = $state(pack.launch.followRecommendedArgs);
  let saving = $state(false);

  const gb = (mb: number) => (mb / 1024).toFixed(1).replace(/\.0$/, '');
  const recommendedMax = $derived(pack.recommendedMaxMemoryMb);
  const recommendedArgs = $derived(pack.recommendedArgs.join(' '));
  const machineShort = $derived(recommendedMax > 0 && maxHeap > 0 && recommendedMax > maxHeap);

  const draft = $derived<LaunchSettings>({
    maxMemoryMb,
    minMemoryMb: preallocate ? maxMemoryMb : null,
    args: follow ? pack.recommendedArgs : argsText.trim() === '' ? [] : argsText.trim().split(/\s+/),
    followRecommendedArgs: follow
  });
  const dirty = $derived(JSON.stringify(draft) !== JSON.stringify(pack.launch));

  function editArgs(v: string) {
    argsText = v;
    follow = false;
  }
  function useRecommendedArgs() {
    argsText = recommendedArgs;
    follow = true;
  }
  async function save() {
    saving = true;
    const ok = await packs.setLaunch(pack.id, draft);
    saving = false;
    if (ok) onclose?.();
  }
  async function reset() {
    saving = true;
    const ok = await packs.resetLaunch(pack.id);
    saving = false;
    if (ok) {
      const l = packs.packs.find((p) => p.id === pack.id)?.launch;
      if (l) {
        maxMemoryMb = l.maxMemoryMb;
        preallocate = l.minMemoryMb !== null;
        argsText = l.args.join(' ');
        follow = l.followRecommendedArgs;
      }
    }
  }
</script>

<section class="launch">
  <header>
    <strong>Launch settings</strong>
    <span class="who">for this computer</span>
    {#if onclose}
      <span class="spacer"></span>
      <button class="btn btn-ghost btn-icon" title="Close" onclick={onclose}><Icon name="x" size={14} /></button>
    {/if}
  </header>

  <p class="rec">
    {#if recommendedMax > 0}
      This pack recommends <strong>{gb(recommendedMax)} GB</strong> of memory.
    {:else}
      This pack makes no memory recommendation.
    {/if}
    {#if totalMb > 0}
      This computer has {gb(totalMb)} GB in total, so up to {gb(maxHeap)} GB can go to the game.
    {/if}
    {#if machineShort}
      <span class="warn"><Icon name="alert" size={11} /> That is less than recommended; expect longer load times or stutter.</span>
    {/if}
  </p>

  <div class="field">
    <div class="row">
      <label class="label" for="mem-{pack.id}">Memory</label>
      <span class="value mono">{gb(maxMemoryMb)} GB</span>
    </div>
    <input
      id="mem-{pack.id}"
      type="range"
      min={MIN}
      max={sliderMax}
      step={STEP}
      bind:value={maxMemoryMb}
      aria-valuetext="{gb(maxMemoryMb)} GB"
    />
    <div class="ticks"><span>{gb(MIN)} GB</span>{#if recommendedMax >= MIN && recommendedMax <= sliderMax}<span class="tick-rec" style:left="{((recommendedMax - MIN) / (sliderMax - MIN)) * 100}%">recommended</span>{/if}<span>{gb(sliderMax)} GB</span></div>
    <label class="check-row">
      <input type="checkbox" bind:checked={preallocate} />
      <span>Reserve the full amount at start (<span class="mono">-Xms</span>). Smoother once loaded; slower to start.</span>
    </label>
  </div>

  <div class="field">
    <div class="row">
      <label class="label" for="args-{pack.id}">JVM arguments</label>
      {#if follow}
        <span class="following"><span class="dot"></span> following the pack</span>
      {:else}
        <button class="linkish" onclick={useRecommendedArgs}>Use recommended</button>
      {/if}
    </div>
    <input
      id="args-{pack.id}"
      class="input mono"
      spellcheck="false"
      autocomplete="off"
      placeholder="none"
      value={follow ? recommendedArgs : argsText}
      oninput={(e) => editArgs(e.currentTarget.value)}
    />
    <p class="hint">Space-separated. The Minecraft launcher cannot pass an argument that itself contains a space.</p>
  </div>

  <footer>
    <button class="btn btn-primary" disabled={!dirty || saving} onclick={save}>
      {saving ? 'Saving…' : 'Save'}
    </button>
    {#if pack.launchCustomised}
      <button class="btn" disabled={saving} onclick={reset} title="Forget these settings and go back to the recommendation for this computer">
        <Icon name="undo" size={14} /> Reset to recommended
      </button>
    {/if}
  </footer>
</section>

<style>
  .launch {
    display: grid;
    gap: 12px;
    padding: 12px 14px;
    background: var(--bg-sunken);
    border: 1px solid var(--border-subtle);
    border-radius: var(--r-md);
    font-size: 12px;
  }
  header {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  header strong {
    font-size: 12px;
  }
  .who {
    color: var(--text-faint);
  }
  .spacer {
    flex: 1;
  }
  .rec {
    margin: 0;
    color: var(--text-muted);
    line-height: 1.5;
  }
  .rec strong {
    color: var(--text);
  }
  .warn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--warn);
  }
  .field {
    display: grid;
    gap: 6px;
  }
  .row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .value {
    font-size: 13px;
    font-weight: 700;
  }
  input[type='range'] {
    width: 100%;
    accent-color: var(--accent);
  }
  .ticks {
    position: relative;
    display: flex;
    justify-content: space-between;
    height: 14px;
    font-size: 10px;
    color: var(--text-faint);
  }
  .tick-rec {
    position: absolute;
    transform: translateX(-50%);
    color: var(--accent);
  }
  .check-row {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    margin-top: 4px;
    color: var(--text-muted);
    cursor: pointer;
  }
  .check-row input {
    margin-top: 2px;
    accent-color: var(--accent);
  }
  .following {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    color: var(--text-faint);
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--ok, #6fbf73);
  }
  .linkish {
    padding: 0;
    font: inherit;
    color: var(--accent);
    background: none;
    border: none;
    cursor: pointer;
  }
  .linkish:hover {
    text-decoration: underline;
  }
  .hint {
    margin: 0;
  }
  footer {
    display: flex;
    gap: 8px;
  }
</style>
