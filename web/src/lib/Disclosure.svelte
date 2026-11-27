<script lang="ts">
  import { onMount, type Snippet } from 'svelte';

  let {
    label,
    level = 3,
    icon = '',
    class: className = '',
    children
  }: {
    label: string;
    level?: 2 | 3;
    icon?: string;
    class?: string;
    children: import('svelte').Snippet;
  } = $props();

  let open = $state(false);

  onMount(() => {
    const wide = window.matchMedia('(min-width: 768px)');
    const sync = () => (open = wide.matches);
    sync();
    wide.addEventListener('change', sync);
    return () => wide.removeEventListener('change', sync);
  });
</script>

<details class="disclosure {className}" bind:open>
  <summary>
    <svelte:element this={`h${level}`} class="disclosure-title">
      {#if icon}<i class="bi {icon}" aria-hidden="true"></i>{/if}
      {label}
    </svelte:element>
    <i class="bi bi-chevron-down disclosure-chevron" aria-hidden="true"></i>
  </summary>
  <div class="disclosure-body">
    {@render children()}
  </div>
</details>
