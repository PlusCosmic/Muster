<script lang="ts">
  import type { Snippet } from 'svelte';
  import '../app.css';
  import DialogHost from '$lib/components/DialogHost.svelte';
  import ToastHost from '$lib/components/ToastHost.svelte';
  import { theme } from '$lib/stores/theme.svelte';
  import { titlebar } from '$lib/stores/titlebar.svelte';

  let { children }: { children: Snippet } = $props();

  theme.apply();
  titlebar.apply();
</script>

{@render children()}

<DialogHost />
<ToastHost />
