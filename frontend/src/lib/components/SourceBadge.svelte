<script lang="ts">
  import type { ModSource } from '$lib/types';

  interface Props {
    source: ModSource;
    /** Rendered for mods that are in the active list but not installed. */
    unknown?: boolean;
  }
  let { source, unknown = false }: Props = $props();

  const LABEL: Record<ModSource, string> = {
    official: 'DLC',
    local: 'LOCAL',
    workshop: 'STEAM'
  };
  const TITLE: Record<ModSource, string> = {
    official: 'Official Ludeon content (Core or expansion)',
    local: 'Local mod in the game’s Mods/ folder',
    workshop: 'Steam Workshop subscription'
  };
</script>

{#if unknown}
  <span class="badge missing" title="Not found among installed mods">?</span>
{:else}
  <span class="badge {source}" title={TITLE[source]}>{LABEL[source]}</span>
{/if}

<style>
  .badge {
    flex: none;
    padding: 1px 5px;
    font-size: 9.5px;
    font-weight: 700;
    letter-spacing: 0.06em;
    line-height: 1.5;
    border-radius: 3px;
    border: 1px solid currentColor;
    opacity: 0.85;
  }
  .official {
    color: var(--src-official);
  }
  .local {
    color: var(--src-local);
  }
  .workshop {
    color: var(--src-workshop);
  }
  .missing {
    color: var(--danger);
    padding: 1px 6px;
  }
</style>
