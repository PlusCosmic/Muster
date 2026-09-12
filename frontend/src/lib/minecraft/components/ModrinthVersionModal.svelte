<script lang="ts">
  // Pick which version of a Modrinth modpack to hold: used both when a pack
  // is first added (from a LookupModrinth result) and later from its card.
  import Icon from '$lib/shell/components/Icon.svelte';
  import Modal from '$lib/shell/components/Modal.svelte';
  import { openExternal } from '$lib/shell/api';
  import { absoluteTime } from '$lib/shell/format';
  import type { ModrinthVersion } from '$lib/minecraft/types';

  interface Props {
    title: string;
    name: string;
    description?: string;
    pageUrl?: string;
    versions: ModrinthVersion[];
    /** Preselected version number. */
    suggested: string;
    /** The version the pack is held at now, when it is already added. */
    held?: string | null;
    /** The version installed on disk, when any. */
    installed?: string | null;
    confirmLabel: string;
    busy?: boolean;
    onconfirm: (version: string) => void;
    onclose: () => void;
  }

  let { title, name, description, pageUrl, versions, suggested, held = null, installed = null, confirmLabel, busy = false, onconfirm, onclose }: Props =
    $props();

  // svelte-ignore state_referenced_locally
  let chosen = $state(suggested);
  // Betas and alphas are hidden until asked for, unless the pack has no
  // releases at all or the preselected one is a pre-release.
  // svelte-ignore state_referenced_locally
  let showPrerelease = $state(
    !versions.some((v) => v.type === 'release') || versions.find((v) => v.number === suggested)?.type !== 'release'
  );

  const shown = $derived(versions.filter((v) => showPrerelease || v.type === 'release' || v.number === held || v.number === installed));
  const prereleaseCount = $derived(versions.filter((v) => v.type !== 'release').length);
  const newest = $derived(versions.find((v) => v.type === 'release')?.number ?? versions[0]?.number ?? null);
  const unchanged = $derived(held !== null && chosen === held);

  const shortDate = (ms: number) => (ms ? absoluteTime(ms).split(',')[0] : '');
</script>

<Modal {title} subtitle={name} width={560} {onclose} dismissable={!busy}>
  {#if description}
    <p class="desc">{description}</p>
  {/if}
  <div class="toolbar">
    <span class="count">{shown.length} of {versions.length} version{versions.length === 1 ? '' : 's'}</span>
    {#if prereleaseCount > 0}
      <label class="toggle">
        <input type="checkbox" bind:checked={showPrerelease} disabled={busy} />
        Show betas and alphas ({prereleaseCount})
      </label>
    {/if}
    {#if pageUrl}
      <button class="linkish" onclick={() => openExternal(pageUrl)}>Modrinth <Icon name="externalLink" size={11} /></button>
    {/if}
  </div>

  {#if versions.length === 0}
    <p class="empty">This pack has no versions with a downloadable file yet.</p>
  {:else}
    <ul class="versions" role="radiogroup" aria-label="Version">
      {#each shown as v (v.id)}
        <li>
          <label class="row" class:chosen={chosen === v.number}>
            <input type="radio" name="modrinth-version" value={v.number} bind:group={chosen} disabled={busy} />
            <span class="number mono">{v.number}</span>
            <span class="tags">
              {#if v.type !== 'release'}<span class="tag pre">{v.type}</span>{/if}
              {#if v.number === newest}<span class="tag">newest release</span>{/if}
              {#if v.number === held}<span class="tag held">held</span>{/if}
              {#if v.number === installed && installed !== held}<span class="tag">installed</span>{/if}
            </span>
            <span class="meta">
              {#if v.gameVersions.length}Minecraft {v.gameVersions.join(', ')}{/if}
              {#if v.loaders.length} · {v.loaders.join(', ')}{/if}
            </span>
            <span class="date">{shortDate(v.publishedAtMs)}</span>
          </label>
        </li>
      {/each}
    </ul>
    <p class="hint">
      Muster installs exactly the version you choose and stays on it. Pick an older one to match a server; come
      back here to move.
    </p>
  {/if}

  {#snippet footer()}
    <button class="btn" onclick={onclose} disabled={busy}>Cancel</button>
    <button class="btn btn-primary" disabled={busy || !chosen || unchanged} onclick={() => onconfirm(chosen)}>
      {busy ? 'Working…' : unchanged ? `Held at v${chosen}` : `${confirmLabel} v${chosen}`}
    </button>
  {/snippet}
</Modal>

<style>
  .desc {
    margin: 0 0 12px;
    font-size: 13px;
    line-height: 1.5;
    color: var(--text-muted);
  }
  .toolbar {
    display: flex;
    align-items: center;
    gap: 14px;
    margin-bottom: 8px;
    font-size: 12px;
    color: var(--text-faint);
  }
  .toolbar .count {
    flex: 1;
  }
  .toggle {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    color: var(--text-muted);
  }
  .linkish {
    display: inline-flex;
    align-items: center;
    gap: 3px;
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
  .versions {
    max-height: 46vh;
    margin: 0;
    padding: 0;
    overflow-y: auto;
    list-style: none;
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    background: var(--bg-app);
  }
  .row {
    display: grid;
    grid-template-columns: auto minmax(90px, auto) 1fr auto;
    grid-template-areas:
      'radio number tags date'
      'radio meta meta date';
    column-gap: 10px;
    row-gap: 2px;
    align-items: center;
    padding: 8px 12px;
    cursor: pointer;
    border-bottom: 1px solid var(--border-subtle);
  }
  li:last-child .row {
    border-bottom: none;
  }
  .row:hover {
    background: var(--bg-hover);
  }
  .row.chosen {
    background: var(--bg-raised);
  }
  .row input {
    grid-area: radio;
    margin: 0;
  }
  .number {
    grid-area: number;
    font-size: 13px;
    font-weight: 600;
    color: var(--text);
  }
  .tags {
    grid-area: tags;
    display: flex;
    gap: 6px;
  }
  .tag {
    padding: 1px 7px;
    font-size: 10px;
    color: var(--text-muted);
    background: var(--bg-raised);
    border-radius: 999px;
  }
  .tag.pre {
    color: var(--warn);
  }
  .tag.held {
    color: var(--accent);
  }
  .meta {
    grid-area: meta;
    font-size: 11px;
    color: var(--text-faint);
  }
  .date {
    grid-area: date;
    font-size: 11px;
    color: var(--text-faint);
    white-space: nowrap;
  }
  .empty {
    margin: 0;
    padding: 20px;
    text-align: center;
    font-size: 13px;
    color: var(--text-faint);
  }
  .hint {
    margin: 10px 0 0;
    font-size: 12px;
    color: var(--text-faint);
  }
</style>
