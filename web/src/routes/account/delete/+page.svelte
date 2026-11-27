<script lang="ts">
  import { goto } from '$app/navigation';
  import { tick } from 'svelte';
  import { apiJSON } from '#lib/api';

  let error = '';
  let recoverUntil = '';
  let deleting = false;
  let dialog: HTMLDialogElement;
  let cancelButton: HTMLButtonElement;

  async function openConfirm() {
    if (!dialog.open) dialog.showModal();
    await tick();
    cancelButton?.focus();
  }

  function closeConfirm() {
    dialog.close();
  }

  async function remove() {
    error = '';
    deleting = true;
    try {
      const result = await apiJSON<{ recover_until: string }>('/api/v1/account', {
        method: 'DELETE'
      });
      dialog.close();
      recoverUntil = result.recover_until;
      setTimeout(() => goto('/'), 2500);
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not delete that account.';
    } finally {
      deleting = false;
    }
  }
</script>

<svelte:head>
  <title>Delete account · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if recoverUntil}
    <header class="page-head">
      <div>
        <p class="kicker">Scheduled</p>
        <h1>Your account is hidden</h1>
      </div>
    </header>
    <div class="sheet-card stack">
      <p>
        Nothing is visible to other players. Signing in again before
        {new Date(recoverUntil).toLocaleDateString('en-GB')} restores the account. After that it is
        deleted.
      </p>
      <a class="btn btn-primary align-self-start" href="/">Return to the start</a>
    </div>
  {:else}
    <header class="page-head">
      <div>
        <p class="kicker">Careful</p>
        <h1>Delete account</h1>
      </div>
    </header>
    <div class="sheet-card stack">
      <p>
        Your profile, photos, matches and likes are hidden immediately. You have 30 days to recover by
        signing in again. After 30 days the account is deleted.
      </p>
      <p class="muted mb-0">
        Download your data first if you want a copy. Reports you made are used only for the safety review.
      </p>
      {#if error}
        <p class="alert-box mb-0" role="alert">{error}</p>
      {/if}
      <div class="row">
        <button class="btn btn-danger" disabled={deleting} onclick={openConfirm}>
          {deleting ? 'Deleting…' : 'Delete my account'}
        </button>
        <a class="btn btn-outline-secondary" href="/settings">Keep my account</a>
      </div>
    </div>
  {/if}
</section>

<dialog bind:this={dialog} class="delete-dialog" aria-labelledby="delete-title" aria-describedby="delete-description">
  <div class="dialog-head">
    <div>
      <p class="kicker">Confirm</p>
      <h2 id="delete-title" class="mb-0">Delete your account?</h2>
    </div>
  </div>
  <p id="delete-description">
    Your profile is hidden now and deleted after 30 days. Signing in before then restores it.
  </p>
  {#if error}<p class="alert-box" role="alert">{error}</p>{/if}
  <div class="dialog-actions">
    <button class="btn btn-danger" type="button" onclick={remove} disabled={deleting}>
      {deleting ? 'Deleting…' : 'Delete my account'}
    </button>
    <button class="btn btn-outline-secondary" type="button" bind:this={cancelButton} onclick={closeConfirm}>
      Cancel
    </button>
  </div>
</dialog>

<style>
  .delete-dialog {
    width: min(100% - 2rem, 30rem);
    margin: auto;
    padding: 1.5rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-top), var(--surface) 72%);
    color: var(--ink);
    box-shadow: var(--card-highlight), var(--tc-shadow-3);
  }

  .delete-dialog::backdrop {
    background: var(--backdrop);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
  }

  .dialog-head {
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
</style>
