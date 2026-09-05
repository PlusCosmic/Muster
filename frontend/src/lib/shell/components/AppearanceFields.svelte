<script lang="ts">
  // The theme and title-bar rows every game's settings modal shows. The
  // modal owns the drafts (so Cancel can revert previews) and passes them in.
  import { THEMES, theme } from '$lib/shell/stores/theme.svelte';
  import { titlebar } from '$lib/shell/stores/titlebar.svelte';

  interface Props {
    draftTheme: string;
    draftTitlebar: boolean;
    ontheme: (id: string) => void;
    ontitlebar: (shown: boolean) => void;
  }
  let { draftTheme, draftTitlebar, ontheme, ontitlebar }: Props = $props();
</script>

<div class="field theme-row">
  <div>
    <label class="label" for="app-theme">Theme</label>
    <p class="hint">
      {THEMES.find((t) => t.id === draftTheme)?.description}
      {#if draftTheme !== theme.current}— previewing; Save to keep it{/if}
    </p>
  </div>
  <select id="app-theme" class="input select" value={draftTheme} onchange={(e) => ontheme(e.currentTarget.value)}>
    {#each THEMES as t (t.id)}
      <option value={t.id}>{t.name}</option>
    {/each}
  </select>
</div>
<div class="field theme-row">
  <div>
    <label class="label" for="app-titlebar">Window title bar</label>
    <p class="hint">
      Turn off if your window manager draws its own decorations — or none at all.
      {#if draftTitlebar !== titlebar.current}— previewing; Save to keep it{/if}
    </p>
  </div>
  <input id="app-titlebar" type="checkbox" class="check" checked={draftTitlebar} onchange={(e) => ontitlebar(e.currentTarget.checked)} />
</div>

<style>
  .theme-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-top: 10px;
  }
  .theme-row .hint {
    margin-top: 2px;
  }
  .select {
    width: 180px;
    flex: none;
    cursor: pointer;
    -webkit-appearance: none;
    appearance: none;
    padding-right: 28px;
    background-image: linear-gradient(45deg, transparent 50%, var(--text-muted) 50%),
      linear-gradient(135deg, var(--text-muted) 50%, transparent 50%);
    background-position: calc(100% - 16px) 55%, calc(100% - 11px) 55%;
    background-size: 5px 5px;
    background-repeat: no-repeat;
  }
  .check {
    width: 18px;
    height: 18px;
    flex: none;
    accent-color: var(--accent);
    cursor: pointer;
  }
</style>
