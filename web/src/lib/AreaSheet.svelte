<script lang="ts">
  import { tick } from 'svelte';
  import { apiJSON } from '#lib/api';
  import AreaEditor from '#lib/AreaEditor.svelte';
  import { loadRadius, saveRadius } from '#lib/preferences';
  import { showToast } from '#lib/toasts';

  export let onSaved: ((label: string) => void) | undefined = undefined;
  export let announceToast = true;

  let dialog: HTMLDialogElement;
  let heading: HTMLHeadingElement;
  let areaLabel = '';
  let radius = 10;
  let loading = false;

  export async function open() {
    loading = true;
    radius = loadRadius();
    if (!dialog.open) dialog.showModal();
    await tick();
    heading?.focus();
    try {
      areaLabel = (await apiJSON<{ area_label: string }>('/api/v1/location')).area_label;
    } catch {
      areaLabel = '';
    } finally {
      loading = false;
    }
  }

  function close() {
    dialog.close();
  }

  function changeRadius() {
    saveRadius(radius);
  }

  function saved(label: string) {
    areaLabel = label;
    dialog.close();
    showToast(`Area saved: ${label}.`, { announce: announceToast });
    onSaved?.(label);
  }
</script>

<dialog bind:this={dialog} class="area-dialog" aria-labelledby="area-title">
  <div class="dialog-head">
    <div>
      <p class="score mb-1">Where you play</p>
      <h2 id="area-title" class="mb-0" tabindex="-1" bind:this={heading}>Area and distance</h2>
    </div>
    <button class="icon-button dark" type="button" onclick={close} aria-label="Close">
      <i class="bi bi-x-lg" aria-hidden="true"></i>
    </button>
  </div>

  {#if loading}
    <p role="status">Loading your area…</p>
  {:else}
    <div class="stack">
      <label>
        Discovery radius
        <select class="form-select" bind:value={radius} onchange={changeRadius}>
          <option value={5}>5 miles</option>
          <option value={10}>10 miles</option>
          <option value={20}>20 miles</option>
          <option value={30}>30 miles</option>
        </select>
      </label>
      <AreaEditor initialArea={areaLabel} onSaved={saved} />
    </div>
  {/if}
</dialog>

<style>
  .area-dialog {
    width: min(100% - 2rem, 32rem);
    max-height: min(90dvh, 48rem);
    margin: auto;
    padding: 1.5rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-top), var(--surface) 72%);
    color: var(--ink);
    box-shadow: var(--card-highlight), var(--tc-shadow-3);
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .area-dialog::backdrop {
    background: var(--backdrop);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
  }

  .dialog-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 1rem;
    margin-bottom: 1rem;
  }

  .dialog-head h2:focus {
    outline: none;
  }

  @media (max-width: 520px) {
    .area-dialog {
      width: 100%;
      max-height: 92dvh;
      margin: auto 0 0;
      border-radius: var(--radius-control) var(--radius-control) 0 0;
      padding: 1.25rem 1.25rem calc(1.25rem + env(safe-area-inset-bottom));
    }
  }
</style>
