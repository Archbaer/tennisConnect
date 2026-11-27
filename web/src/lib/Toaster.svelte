<script lang="ts">
  import ToastItem from '#lib/ToastItem.svelte';
  import { toasts } from '#lib/toasts';
</script>

<div class="toaster" role="status" aria-live="polite">
  {#each $toasts as toast (toast.id)}
    <ToastItem {toast} />
  {/each}
</div>

<style>
  .toaster {
    position: fixed;
    left: 50%;
    bottom: calc(1rem + env(safe-area-inset-bottom));
    z-index: 40;
    display: grid;
    gap: 0.5rem;
    width: min(24rem, calc(100% - 1.5rem));
    transform: translateX(-50%);
    pointer-events: none;
  }

  @media (min-width: 768px) {
    .toaster {
      left: auto;
      right: 1rem;
      bottom: 1rem;
      width: min(24rem, calc(100% - 2rem));
      transform: none;
    }
  }

  @media (max-width: 767px) {
    :global(.with-tabs) .toaster {
      bottom: calc(4.4rem + env(safe-area-inset-bottom) + 0.75rem);
    }
  }
</style>
