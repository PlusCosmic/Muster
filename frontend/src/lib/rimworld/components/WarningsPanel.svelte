<script lang="ts">
  import { slide } from 'svelte/transition';
  import type { SortWarning } from '$lib/rimworld/types';
  import Icon, { type IconName } from '$lib/shell/components/Icon.svelte';

  interface Props {
    warnings: SortWarning[];
    onclose: () => void;
  }
  let { warnings, onclose }: Props = $props();

  interface KindStyle {
    label: string;
    icon: IconName;
    severity: 'error' | 'warn' | 'info';
  }

  const KINDS: Record<string, KindStyle> = {
    missingDependency: { label: 'Missing dependency', icon: 'alert', severity: 'error' },
    incompatible: { label: 'Incompatible', icon: 'x', severity: 'error' },
    cycle: { label: 'Load-order cycle', icon: 'refresh', severity: 'warn' },
    versionMismatch: { label: 'Version mismatch', icon: 'alert', severity: 'warn' },
    unknownMod: { label: 'Unknown mod', icon: 'info', severity: 'warn' },
    rulesDbUnavailable: { label: 'Rules DB unavailable', icon: 'info', severity: 'info' }
  };

  function styleFor(kind: string): KindStyle {
    return KINDS[kind] ?? { label: kind, icon: 'info', severity: 'info' };
  }

  let collapsed = $state(false);

  const groups = $derived.by(() => {
    const map = new Map<string, SortWarning[]>();
    for (const w of warnings) {
      const list = map.get(w.kind);
      if (list) list.push(w);
      else map.set(w.kind, [w]);
    }
    const order = Object.keys(KINDS);
    return [...map.entries()].sort(
      ([a], [b]) =>
        (order.indexOf(a) === -1 ? 99 : order.indexOf(a)) -
        (order.indexOf(b) === -1 ? 99 : order.indexOf(b))
    );
  });

  const worst = $derived(
    warnings.some((w) => styleFor(w.kind).severity === 'error')
      ? 'error'
      : warnings.some((w) => styleFor(w.kind).severity === 'warn')
        ? 'warn'
        : 'info'
  );
</script>

<section class="panel {worst}" transition:slide={{ duration: 160 }}>
  <header>
    <button class="toggle" onclick={() => (collapsed = !collapsed)} aria-expanded={!collapsed}>
      <Icon name={collapsed ? 'chevronRight' : 'chevronDown'} size={14} />
      <Icon name="alert" size={14} />
      <span class="title">Sort results</span>
      <span class="total">{warnings.length}</span>
    </button>
    <div class="legend">
      {#each groups as [kind, items] (kind)}
        <span class="tag {styleFor(kind).severity}">{styleFor(kind).label} · {items.length}</span>
      {/each}
    </div>
    <button class="btn btn-ghost btn-icon" onclick={onclose} aria-label="Dismiss warnings">
      <Icon name="x" size={14} />
    </button>
  </header>

  {#if !collapsed}
    <div class="body">
      {#each groups as [kind, items] (kind)}
        {@const s = styleFor(kind)}
        <div class="group {s.severity}">
          <div class="group-head">
            <Icon name={s.icon} size={13} />
            <span>{s.label}</span>
            <span class="kind mono">{kind}</span>
          </div>
          <ul>
            {#each items as w, i (kind + i)}
              <li>
                {#if w.packageId}<code class="pid">{w.packageId}</code>{/if}
                <span data-selectable>{w.message}</span>
              </li>
            {/each}
          </ul>
        </div>
      {/each}
    </div>
  {/if}
</section>

<style>
  .panel {
    flex: none;
    max-height: 34vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-panel);
    border: 1px solid var(--border);
    border-left-width: 3px;
    border-radius: var(--r-lg);
    overflow: hidden;
  }
  .panel.error {
    border-left-color: var(--danger);
  }
  .panel.warn {
    border-left-color: var(--warn);
  }
  .panel.info {
    border-left-color: var(--info);
  }

  header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 8px 6px 10px;
    flex: none;
  }

  .toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 3px 6px;
    font: inherit;
    font-size: 12px;
    font-weight: 600;
    color: var(--text);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    cursor: pointer;
  }
  .toggle:hover {
    background: var(--bg-hover);
  }
  .panel.error .toggle {
    color: #f2b3b1;
  }
  .panel.warn .toggle {
    color: #f0cd8e;
  }

  .title {
    letter-spacing: 0.01em;
  }

  .total {
    font-family: var(--font-mono);
    font-size: 10.5px;
    padding: 0 5px;
    color: var(--text-muted);
    background: var(--bg-sunken);
    border-radius: 99px;
  }

  .legend {
    flex: 1;
    display: flex;
    flex-wrap: wrap;
    gap: 5px;
    min-width: 0;
  }

  .tag {
    padding: 1px 7px;
    font-size: 10.5px;
    font-weight: 600;
    border-radius: 99px;
    border: 1px solid transparent;
  }
  .tag.error {
    color: var(--danger);
    background: var(--danger-soft);
    border-color: rgba(224, 82, 79, 0.3);
  }
  .tag.warn {
    color: var(--warn);
    background: var(--warn-soft);
    border-color: rgba(224, 163, 60, 0.3);
  }
  .tag.info {
    color: var(--info);
    background: var(--info-soft);
    border-color: rgba(91, 157, 217, 0.3);
  }

  .body {
    overflow-y: auto;
    padding: 0 10px 10px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .group {
    border-radius: var(--r-md);
    border: 1px solid var(--border-subtle);
    background: var(--bg-sunken);
    padding: 7px 9px;
  }
  .group.error {
    border-color: rgba(224, 82, 79, 0.28);
    background: var(--danger-soft);
  }
  .group.warn {
    border-color: rgba(224, 163, 60, 0.25);
    background: var(--warn-soft);
  }
  .group.info {
    border-color: rgba(91, 157, 217, 0.25);
    background: var(--info-soft);
  }

  .group-head {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11.5px;
    font-weight: 700;
    letter-spacing: 0.02em;
    margin-bottom: 5px;
  }
  .group.error .group-head {
    color: var(--danger);
  }
  .group.warn .group-head {
    color: var(--warn);
  }
  .group.info .group-head {
    color: var(--info);
  }

  .kind {
    margin-left: auto;
    font-weight: 400;
    opacity: 0.65;
  }

  ul {
    margin: 0;
    padding: 0;
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: 3px;
  }

  li {
    display: flex;
    align-items: baseline;
    gap: 7px;
    font-size: 12px;
    color: var(--text);
    line-height: 1.45;
  }

  .pid {
    flex: none;
    font-family: var(--font-mono);
    font-size: 10.5px;
    padding: 0 5px;
    color: var(--text-muted);
    background: rgba(0, 0, 0, 0.25);
    border-radius: 3px;
  }
</style>
