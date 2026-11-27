<script lang="ts">
  import { tick } from 'svelte';
  import { apiJSON, reportCategories } from '#lib/api';

  export let targetId = '';
  export let targetName = '';
  export let onBlocked: (() => void) | undefined = undefined;
  export let onReported: (() => void) | undefined = undefined;

  let dialog: HTMLDialogElement;
  let cancelButton: HTMLButtonElement;
  let mode: 'report' | 'block' = 'report';
  let category = reportCategories[0][0];
  let details = '';
  let busy = false;
  let error = '';
  let reported = false;

  export async function openReport() {
    mode = 'report';
    reset();
    await show();
  }

  export async function openBlock() {
    mode = 'block';
    reset();
    await show();
  }

  async function show() {
    if (!dialog.open) dialog.showModal();
    await tick();
    if (mode === 'block') {
      cancelButton?.focus();
    } else {
      dialog.querySelector<HTMLInputElement>('input[type="radio"]')?.focus();
    }
  }

  function close() {
    dialog.close();
  }

  function reset() {
    category = reportCategories[0][0];
    details = '';
    busy = false;
    error = '';
    reported = false;
  }

  function blockInstead() {
    mode = 'block';
    error = '';
    tick().then(() => cancelButton?.focus());
  }

  async function submitReport(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    busy = true;
    try {
      await apiJSON('/api/v1/reports', {
        method: 'POST',
        body: JSON.stringify({ target_id: targetId, category, details })
      });
      reported = true;
      onReported?.();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not send that report.';
    } finally {
      busy = false;
    }
  }

  async function block() {
    error = '';
    busy = true;
    try {
      await apiJSON('/api/v1/blocks', {
        method: 'POST',
        body: JSON.stringify({ target_id: targetId })
      });
      dialog.close();
      onBlocked?.();
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not block that player.';
    } finally {
      busy = false;
    }
  }
</script>

<dialog bind:this={dialog} class="safety-dialog" aria-labelledby="safety-title" onclose={reset}>
  <div class="dialog-head">
    <div>
      <p class="score mb-1">{mode === 'report' ? 'Safety' : 'Block'}</p>
      <h2 id="safety-title" class="mb-0">
        {mode === 'report' ? `Report ${targetName}` : `Block ${targetName}?`}
      </h2>
    </div>
    <button class="icon-button dark" type="button" onclick={close} aria-label="Close">
      <i class="bi bi-x-lg" aria-hidden="true"></i>
    </button>
  </div>

  {#if reported}
    <div class="stack">
      <p class="notice mb-0">Thanks — we've got it. We'll review this within 72 hours. We won't tell {targetName}.</p>
      <button class="btn btn-primary align-self-start" type="button" onclick={close}>Done</button>
    </div>
  {:else if mode === 'report'}
    <p class="muted">
      Tell us what happened with {targetName}. We'll review it within 72 hours.
    </p>
    <form class="stack" onsubmit={submitReport}>
      <fieldset>
        <legend class="score mb-2">Why are you reporting {targetName}?</legend>
        <div class="radio-list">
          {#each reportCategories as [value, label]}
            <label class="radio-row">
              <input type="radio" name="category" value={value} bind:group={category} />
              <span>{label}</span>
            </label>
          {/each}
        </div>
      </fieldset>
      <label>
        <span>Details <span class="muted">(optional)</span></span>
        <textarea maxlength="500" bind:value={details} placeholder="What should we know?"></textarea>
      </label>
      {#if error}<p class="alert-box mb-0" role="alert">{error}</p>{/if}
      <div class="dialog-actions">
        <button class="btn btn-primary" type="submit" disabled={busy}>{busy ? 'Sending…' : 'Submit report'}</button>
        <button class="btn btn-outline-danger" type="button" onclick={blockInstead}>Block instead</button>
        <button class="btn btn-outline-secondary" type="button" onclick={close}>Cancel</button>
      </div>
    </form>
  {:else}
    <div class="stack">
      <p class="mb-0">
        You won't see each other in Discover. Any match between you closes and Instagram access is removed.
        {targetName} won't be told.
      </p>
      {#if error}<p class="alert-box mb-0" role="alert">{error}</p>{/if}
      <div class="dialog-actions">
        <button class="btn btn-danger" type="button" onclick={block} disabled={busy}>
          {busy ? 'Blocking…' : 'Block'}
        </button>
        <button class="btn btn-outline-secondary" type="button" bind:this={cancelButton} onclick={close}>
          Cancel
        </button>
      </div>
    </div>
  {/if}
</dialog>

<style>
  .safety-dialog {
    width: min(100% - 2rem, 32rem);
    max-height: min(90dvh, 46rem);
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

  .safety-dialog::backdrop {
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

  .dialog-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .dialog-actions .btn {
    flex: 1 1 10rem;
  }

  .radio-list {
    display: grid;
    gap: 0.25rem;
  }

  .radio-row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    min-height: 2.75rem;
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    font-weight: 500;
    cursor: pointer;
  }

  .radio-row:has(input:checked) {
    border-color: var(--court);
    background: var(--surface-hover);
  }

  .radio-row input {
    width: 1.15rem;
    height: 1.15rem;
    min-height: 0;
    margin: 0;
    accent-color: var(--court);
  }

  fieldset {
    margin: 0;
    padding: 0;
    border: 0;
  }

  @media (max-width: 520px) {
    .safety-dialog {
      width: 100%;
      max-height: 92dvh;
      margin: auto 0 0;
      border-radius: var(--radius-control) var(--radius-control) 0 0;
      padding: 1.25rem 1.25rem calc(1.25rem + env(safe-area-inset-bottom));
    }
  }
</style>
